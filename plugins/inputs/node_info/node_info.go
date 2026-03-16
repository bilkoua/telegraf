//go:generate ../../../tools/readme_config_includer/generator
//go:build linux

package node_info

import (
	"bufio"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/internal/choice"
	"github.com/influxdata/telegraf/plugins/inputs"
)

//go:embed sample.conf
var sampleConfig string

const (
	defaultHostEtc = "/etc"
	defaultHostSys = "/sys"
)

// Available metric groups that can be selected via the "collect" option.
var availableCollectors = []string{"os", "dmi", "uname"}

// dmiFiles maps the Telegraf tag name to the corresponding file name under
// /sys/class/dmi/id/.  The order is deterministic so that log messages are
// reproducible across runs.
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

// osReleaseKeys are the keys extracted from os-release(5) in the order they
// appear as Telegraf tags.
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

// NodeInfo collects static node information: OS release, DMI/SMBIOS, and
// uname data.  Each metric is a gauge with a constant value of 1 and all
// informational fields encoded as tags, mirroring the prometheus-node-exporter
// node_os_info / node_dmi_info / node_uname_info metrics.
//
// Because the underlying data is static (it only changes on OS upgrade,
// hardware swap, or reboot), all information is read once during Init() and
// cached.  Gather() emits the pre-built tags with zero I/O and minimal
// allocations.
type NodeInfo struct {
	PathEtc string   `toml:"host_etc"`
	PathSys string   `toml:"host_sys"`
	Collect []string `toml:"collect"`

	Log telegraf.Logger `toml:"-"`

	// Pre-resolved collect flags — O(1) lookup in Gather().
	collectOS    bool
	collectDMI   bool
	collectUname bool

	// Cached tag maps, populated once during Init().  A nil map means the
	// data source was unavailable and the corresponding metric should be
	// skipped.
	osTags    map[string]string
	dmiTags   map[string]string
	unameTags map[string]string
}

func (*NodeInfo) SampleConfig() string { return sampleConfig }

// Init validates configuration, resolves defaults, reads all static data
// sources, and caches the results for the lifetime of the process.
func (n *NodeInfo) Init() error {
	if n.PathEtc == "" {
		n.PathEtc = defaultHostEtc
	}
	if n.PathSys == "" {
		n.PathSys = defaultHostSys
	}

	// Default: collect everything.
	if len(n.Collect) == 0 {
		n.Collect = availableCollectors
	}
	if err := choice.CheckSlice(n.Collect, availableCollectors); err != nil {
		return fmt.Errorf("invalid collect option: %w", err)
	}

	n.collectOS = choice.Contains("os", n.Collect)
	n.collectDMI = choice.Contains("dmi", n.Collect)
	n.collectUname = choice.Contains("uname", n.Collect)

	// --- cache os-release ------------------------------------------------
	if n.collectOS {
		tags, err := n.initOSTags()
		if err != nil {
			n.Log.Warnf("Could not read os-release: %v; node_os_info will not be emitted", err)
		} else {
			n.osTags = tags
		}
	}

	// --- cache DMI -------------------------------------------------------
	if n.collectDMI {
		n.dmiTags = n.initDMITags()
	}

	// --- cache uname -----------------------------------------------------
	if n.collectUname {
		tags, err := initUnameTags()
		if err != nil {
			n.Log.Warnf("Could not read uname info: %v; node_uname_info will not be emitted", err)
		} else {
			n.unameTags = tags
		}
	}

	return nil
}

// Gather emits the cached info metrics.  No file I/O or syscalls are
// performed; the only work is copying the pre-built tag maps into the
// accumulator.
func (n *NodeInfo) Gather(acc telegraf.Accumulator) error {
	if n.osTags != nil {
		acc.AddGauge("node_os", map[string]interface{}{"info": int64(1)}, n.osTags)
	}
	if n.dmiTags != nil {
		acc.AddGauge("node_dmi", map[string]interface{}{"info": int64(1)}, n.dmiTags)
	}
	if n.unameTags != nil {
		acc.AddGauge("node_uname", map[string]interface{}{"info": int64(1)}, n.unameTags)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Init-time data readers
// ---------------------------------------------------------------------------

// initOSTags reads the os-release file (with fallback) and returns the
// tag map for node_os_info.
func (n *NodeInfo) initOSTags() (map[string]string, error) {
	info, err := n.readOSRelease()
	if err != nil {
		return nil, err
	}

	tags := make(map[string]string, len(osReleaseKeys))
	for _, key := range osReleaseKeys {
		tags[strings.ToLower(key)] = info[key] // missing key → ""
	}
	return tags, nil
}

// readOSRelease tries the primary and fallback locations for the os-release
// file, as specified by os-release(5).
func (n *NodeInfo) readOSRelease() (map[string]string, error) {
	primary := filepath.Join(n.PathEtc, "os-release")
	fallback := filepath.Join(n.PathEtc, "..", "usr", "lib", "os-release")

	f, err := os.Open(primary)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("opening %q: %w", primary, err)
		}
		n.Log.Debugf("Primary os-release not found at %q, trying fallback", primary)
		f, err = os.Open(fallback)
		if err != nil {
			return nil, fmt.Errorf("opening os-release (tried %q and %q): %w", primary, fallback, err)
		}
	}
	defer f.Close()

	return parseOSRelease(f)
}

// parseOSRelease parses a KEY="value" formatted file (as defined by the
// os-release(5) man page) and returns a map of the key/value pairs.
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

// unquoteOSReleaseValue strips optional surrounding double- or single-quotes
// from a value as produced by os-release(5).
func unquoteOSReleaseValue(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// initDMITags reads every DMI file under /sys/class/dmi/id/ and returns
// the tag map for node_dmi_info.  If the DMI directory does not exist at
// all (ARM boards, containers) it returns nil.
func (n *NodeInfo) initDMITags() map[string]string {
	dmiDir := filepath.Join(n.PathSys, "class", "dmi", "id")

	if _, err := os.Stat(dmiDir); os.IsNotExist(err) {
		n.Log.Debugf("DMI directory %q does not exist, skipping node_dmi_info", dmiDir)
		return nil
	}

	tags := make(map[string]string, len(dmiFiles))
	for _, entry := range dmiFiles {
		value, err := readFileTrimmed(filepath.Join(dmiDir, entry.filename))
		if err != nil {
			n.Log.Debugf("Reading DMI file %q: %v", entry.filename, err)
			value = ""
		}
		tags[entry.tag] = value
	}
	return tags
}

// initUnameTags calls the uname(2) syscall and returns the tag map for
// node_uname_info.
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

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// utsFieldToString converts a NUL-terminated character array from a
// unix.Utsname field to a Go string.  The type parameter T covers both
// int8 (e.g. Linux/amd64) and byte/uint8 (e.g. Linux/arm64) variants of
// the utsname struct.
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

// readFileTrimmed reads the entire content of path and returns it with
// surrounding whitespace (including the trailing newline) stripped.
func readFileTrimmed(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func init() {
	inputs.Add("node_info", func() telegraf.Input {
		return &NodeInfo{}
	})
}
