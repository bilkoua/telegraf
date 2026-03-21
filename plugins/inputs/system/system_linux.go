//go:build linux

package system

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/internal/choice"
)

const (
	defaultHostEtc = "/etc"
	defaultHostSys = "/sys"
)

type platformData struct { //nolint:unused // used on Linux, needed for System struct
	collectOS    bool
	collectDMI   bool
	collectUname bool

	osTags    map[string]string
	dmiTags   map[string]string
	unameTags map[string]string
}

var linuxCollectors = []string{"os", "dmi", "uname"}

var dmiFiles = []struct {
	tag      string
	filename string
}{
	{"bios_date", "bios_date"},
	{"bios_release", "bios_release"},
	{"bios_vendor", "bios_vendor"},
	{"bios_version", "bios_version"},
	{"board_asset_tag", "board_asset_tag"},
	{"board_name", "board_name"},
	{"board_serial", "board_serial"},
	{"board_vendor", "board_vendor"},
	{"board_version", "board_version"},
	{"chassis_asset_tag", "chassis_asset_tag"},
	{"chassis_serial", "chassis_serial"},
	{"chassis_vendor", "chassis_vendor"},
	{"chassis_version", "chassis_version"},
	{"product_family", "product_family"},
	{"product_name", "product_name"},
	{"product_serial", "product_serial"},
	{"product_sku", "product_sku"},
	{"product_uuid", "product_uuid"},
	{"product_version", "product_version"},
	// The kernel exposes this as "sys_vendor"; we surface it as
	// "system_vendor" to match the prometheus-node-exporter label name.
	{"system_vendor", "sys_vendor"},
}

var osReleaseKeys = []string{
	"ID",
	"ID_LIKE",
	"NAME",
	"PRETTY_NAME",
	"VARIANT",
	"VARIANT_ID",
	"VERSION",
	"VERSION_CODENAME",
	"VERSION_ID",
}

func (s *System) Init() error {
	if s.PathEtc == "" {
		s.PathEtc = defaultHostEtc
	}
	if s.PathSys == "" {
		s.PathSys = defaultHostSys
	}

	available := make([]string, 0, len(crossPlatformCollectors)+len(linuxCollectors))
	available = append(available, crossPlatformCollectors...)
	available = append(available, linuxCollectors...)
	if err := s.initCommon(available); err != nil {
		return err
	}

	s.collectOS = choice.Contains("os", s.Collect)
	s.collectDMI = choice.Contains("dmi", s.Collect)
	s.collectUname = choice.Contains("uname", s.Collect)

	// --- cache os-release ------------------------------------------------
	if s.collectOS {
		tags, err := s.initOSTags()
		if err != nil {
			s.Log.Warnf("Could not read os-release: %v; system_os will not be emitted", err)
		} else {
			s.osTags = tags
		}
	}

	// --- cache DMI -------------------------------------------------------
	if s.collectDMI {
		s.dmiTags = s.initDMITags()
	}

	// --- cache uname -----------------------------------------------------
	if s.collectUname {
		tags, err := initUnameTags()
		if err != nil {
			s.Log.Warnf("Could not read uname info: %v; system_uname will not be emitted", err)
		} else {
			s.unameTags = tags
		}
	}

	return nil
}

func (s *System) gatherPlatformInfo(acc telegraf.Accumulator) {
	if s.osTags != nil {
		acc.AddGauge("system_os", map[string]interface{}{"info": int64(1)}, s.osTags)
	}
	if s.dmiTags != nil {
		acc.AddGauge("system_dmi", map[string]interface{}{"info": int64(1)}, s.dmiTags)
	}
	if s.unameTags != nil {
		acc.AddGauge("system_uname", map[string]interface{}{"info": int64(1)}, s.unameTags)
	}
}

func (s *System) initOSTags() (map[string]string, error) {
	info, err := s.readOSRelease()
	if err != nil {
		return nil, err
	}

	tags := make(map[string]string, len(osReleaseKeys))
	for _, key := range osReleaseKeys {
		tags[strings.ToLower(key)] = info[key] // missing key → ""
	}
	return tags, nil
}

func (s *System) readOSRelease() (map[string]string, error) {
	primary := filepath.Join(s.PathEtc, "os-release")
	fallback := filepath.Join(s.PathEtc, "..", "usr", "lib", "os-release")

	f, err := os.Open(primary)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("opening %q: %w", primary, err)
		}
		s.Log.Debugf("Primary os-release not found at %q, trying fallback", primary)
		f, err = os.Open(fallback)
		if err != nil {
			return nil, fmt.Errorf("opening os-release (tried %q and %q): %w", primary, fallback, err)
		}
	}
	defer f.Close()

	return parseOSRelease(f)
}

func parseOSRelease(r io.Reader) (map[string]string, error) {
	result := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		result[strings.TrimSpace(key)] = unquoteOSReleaseValue(strings.TrimSpace(value))
	}
	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("reading os-release: %w", err)
	}
	return result, nil
}

func unquoteOSReleaseValue(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func (s *System) initDMITags() map[string]string {
	dmiDir := filepath.Join(s.PathSys, "class", "dmi", "id")

	if _, err := os.Stat(dmiDir); os.IsNotExist(err) {
		s.Log.Debugf("DMI directory %q does not exist, skipping system_dmi", dmiDir)
		return nil
	}

	tags := make(map[string]string, len(dmiFiles))
	for _, entry := range dmiFiles {
		value, err := readFileTrimmed(filepath.Join(dmiDir, entry.filename))
		if err != nil {
			s.Log.Debugf("Reading DMI file %q: %v", entry.filename, err)
			value = ""
		}
		tags[entry.tag] = value
	}
	return tags
}

func initUnameTags() (map[string]string, error) {
	var utsname unix.Utsname
	if err := unix.Uname(&utsname); err != nil {
		return nil, fmt.Errorf("calling uname: %w", err)
	}

	tags := map[string]string{
		"domainname": utsFieldToString(utsname.Domainname[:]),
		"machine":    utsFieldToString(utsname.Machine[:]),
		"nodename":   utsFieldToString(utsname.Nodename[:]),
		"release":    utsFieldToString(utsname.Release[:]),
		"sysname":    utsFieldToString(utsname.Sysname[:]),
		"version":    utsFieldToString(utsname.Version[:]),
	}
	return tags, nil
}

func utsFieldToString[T byte | int8](field []T) string {
	b := make([]byte, 0, len(field))
	for _, c := range field {
		if c == 0 {
			break
		}
		b = append(b, byte(c))
	}
	return string(b)
}

func readFileTrimmed(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
