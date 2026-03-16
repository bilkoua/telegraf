//go:build linux

package node_info

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/metric"
	"github.com/influxdata/telegraf/testutil"
)

const sampleOSReleaseDebian = `# os-release(5) file for Debian
PRETTY_NAME="Debian GNU/Linux 13 (trixie)"
NAME="Debian GNU/Linux"
VERSION_ID="13"
VERSION="13 (trixie)"
VERSION_CODENAME=trixie
ID=debian
HOME_URL="https://www.debian.org/"
SUPPORT_URL="https://www.debian.org/support"
BUG_REPORT_URL="https://bugs.debian.org/"
`

const sampleOSReleaseUbuntu = `NAME="Ubuntu"
VERSION="22.04.3 LTS (Jammy Jellyfish)"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 22.04.3 LTS"
VERSION_ID="22.04"
VERSION_CODENAME=jammy
`

const sampleOSReleaseRocky = `NAME="Rocky Linux"
VERSION="9.3 (Blue Onyx)"
ID="rocky"
ID_LIKE="rhel centos fedora"
VERSION_ID="9.3"
PLATFORM_ID="platform:el9"
PRETTY_NAME="Rocky Linux 9.3 (Blue Onyx)"
`

const sampleOSReleaseArch = `NAME="Arch Linux"
PRETTY_NAME="Arch Linux"
ID=arch
BUILD_ID=rolling
ANSI_COLOR="38;2;23;147;209"
HOME_URL="https://archlinux.org/"
`

const sampleOSReleaseAlpine = `NAME="Alpine Linux"
ID=alpine
VERSION_ID=3.19.0
PRETTY_NAME="Alpine Linux v3.19"
HOME_URL="https://alpinelinux.org/"
BUG_REPORT_URL="https://gitlab.alpinelinux.org/alpine/aports/-/issues"
`

const sampleOSReleaseFedoraServer = `NAME="Fedora Linux"
VERSION="39 (Server Edition)"
ID=fedora
VERSION_ID=39
VERSION_CODENAME=""
PRETTY_NAME="Fedora Linux 39 (Server Edition)"
VARIANT="Server Edition"
VARIANT_ID=server
`

func TestParseOSRelease(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:  "debian",
			input: sampleOSReleaseDebian,
			expected: map[string]string{
				"PRETTY_NAME":      "Debian GNU/Linux 13 (trixie)",
				"NAME":             "Debian GNU/Linux",
				"VERSION_ID":       "13",
				"VERSION":          "13 (trixie)",
				"VERSION_CODENAME": "trixie",
				"ID":               "debian",
				"HOME_URL":         "https://www.debian.org/",
				"SUPPORT_URL":      "https://www.debian.org/support",
				"BUG_REPORT_URL":   "https://bugs.debian.org/",
			},
		},
		{
			name:  "ubuntu with ID_LIKE",
			input: sampleOSReleaseUbuntu,
			expected: map[string]string{
				"NAME":             "Ubuntu",
				"VERSION":          "22.04.3 LTS (Jammy Jellyfish)",
				"ID":               "ubuntu",
				"ID_LIKE":          "debian",
				"PRETTY_NAME":      "Ubuntu 22.04.3 LTS",
				"VERSION_ID":       "22.04",
				"VERSION_CODENAME": "jammy",
			},
		},
		{
			name:  "rocky linux / RHEL-like with double-quoted ID",
			input: sampleOSReleaseRocky,
			expected: map[string]string{
				"NAME":        "Rocky Linux",
				"VERSION":     "9.3 (Blue Onyx)",
				"ID":          "rocky",
				"ID_LIKE":     "rhel centos fedora",
				"VERSION_ID":  "9.3",
				"PLATFORM_ID": "platform:el9",
				"PRETTY_NAME": "Rocky Linux 9.3 (Blue Onyx)",
			},
		},
		{
			name:  "arch linux rolling (no VERSION keys)",
			input: sampleOSReleaseArch,
			expected: map[string]string{
				"NAME":        "Arch Linux",
				"PRETTY_NAME": "Arch Linux",
				"ID":          "arch",
				"BUILD_ID":    "rolling",
				"ANSI_COLOR":  "38;2;23;147;209",
				"HOME_URL":    "https://archlinux.org/",
			},
		},
		{
			name:  "alpine linux minimal",
			input: sampleOSReleaseAlpine,
			expected: map[string]string{
				"NAME":           "Alpine Linux",
				"ID":             "alpine",
				"VERSION_ID":     "3.19.0",
				"PRETTY_NAME":    "Alpine Linux v3.19",
				"HOME_URL":       "https://alpinelinux.org/",
				"BUG_REPORT_URL": "https://gitlab.alpinelinux.org/alpine/aports/-/issues",
			},
		},
		{
			name:  "fedora server with VARIANT",
			input: sampleOSReleaseFedoraServer,
			expected: map[string]string{
				"NAME":             "Fedora Linux",
				"VERSION":          "39 (Server Edition)",
				"ID":               "fedora",
				"VERSION_ID":       "39",
				"VERSION_CODENAME": "",
				"PRETTY_NAME":      "Fedora Linux 39 (Server Edition)",
				"VARIANT":          "Server Edition",
				"VARIANT_ID":       "server",
			},
		},
		{
			name: "unquoted values",
			input: `ID=ubuntu
VERSION_ID=22.04
`,
			expected: map[string]string{
				"ID":         "ubuntu",
				"VERSION_ID": "22.04",
			},
		},
		{
			name: "single-quoted values",
			input: `NAME='My Linux'
ID=mylinux
`,
			expected: map[string]string{
				"NAME": "My Linux",
				"ID":   "mylinux",
			},
		},
		{
			name:     "empty input",
			input:    "",
			expected: map[string]string{},
		},
		{
			name: "comments and blank lines are skipped",
			input: `# comment

ID=test
`,
			expected: map[string]string{
				"ID": "test",
			},
		},
		{
			name: "value containing equals sign",
			input: `SOME_KEY=val=ue=extra
`,
			expected: map[string]string{
				"SOME_KEY": "val=ue=extra",
			},
		},
		{
			name: "value with empty double quotes",
			input: `VERSION_CODENAME=""
`,
			expected: map[string]string{
				"VERSION_CODENAME": "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseOSRelease(strings.NewReader(tt.input))
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestUnquoteOSReleaseValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"hello world"`, "hello world"},
		{`'hello world'`, "hello world"},
		{`noquotes`, "noquotes"},
		{`""`, ""},
		{`''`, ""},
		{`"`, `"`},
		{`'`, `'`},
		{`"mismatched'`, `"mismatched'`},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			require.Equal(t, tt.expected, unquoteOSReleaseValue(tt.input))
		})
	}
}

func TestUtsFieldToString(t *testing.T) {
	t.Run("int8 array", func(t *testing.T) {
		field := [8]int8{'L', 'i', 'n', 'u', 'x', 0, 0, 0}
		require.Equal(t, "Linux", utsFieldToString(field[:]))
	})
	t.Run("byte array", func(t *testing.T) {
		field := [8]byte{'L', 'i', 'n', 'u', 'x', 0, 0, 0}
		require.Equal(t, "Linux", utsFieldToString(field[:]))
	})
	t.Run("nul at start produces empty string", func(t *testing.T) {
		field := [4]int8{0, 'a', 'b', 'c'}
		require.Empty(t, utsFieldToString(field[:]))
	})
	t.Run("no nul terminator fills whole array", func(t *testing.T) {
		field := [4]int8{'a', 'b', 'c', 'd'}
		require.Equal(t, "abcd", utsFieldToString(field[:]))
	})
	t.Run("empty array", func(t *testing.T) {
		var field [0]int8
		require.Empty(t, utsFieldToString(field[:]))
	})
}

func TestInitDefaults(t *testing.T) {
	plugin := &NodeInfo{
		Log: testutil.Logger{},
	}
	require.NoError(t, plugin.Init())
	require.Equal(t, defaultHostEtc, plugin.PathEtc)
	require.Equal(t, defaultHostSys, plugin.PathSys)
	require.Equal(t, []string{"os", "dmi", "uname"}, plugin.Collect)
}

func TestInitCustomPaths(t *testing.T) {
	plugin := &NodeInfo{
		PathEtc: "/custom/etc",
		PathSys: "/custom/sys",
		Log:     testutil.Logger{},
	}
	require.NoError(t, plugin.Init())
	require.Equal(t, "/custom/etc", plugin.PathEtc)
	require.Equal(t, "/custom/sys", plugin.PathSys)
}

func TestInitInvalidCollectOption(t *testing.T) {
	plugin := &NodeInfo{
		Collect: []string{"os", "bogus"},
		Log:     testutil.Logger{},
	}
	err := plugin.Init()
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid collect option")
}

func TestInitValidCollectSubset(t *testing.T) {
	plugin := &NodeInfo{
		Collect: []string{"uname"},
		Log:     testutil.Logger{},
	}
	require.NoError(t, plugin.Init())
	require.Equal(t, []string{"uname"}, plugin.Collect)
}

// setupEtcDir creates a temporary etc directory with the given os-release
// content.  Returns the path to the tmp root that serves as host_etc.
func setupEtcDir(t *testing.T, content string) string {
	t.Helper()
	td := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(td, "os-release"), []byte(content), 0o600))
	return td
}

// setupDMIDir creates a fake /sys/class/dmi/id/ tree and returns the "sys"
// root path.
func setupDMIDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dmiDir := filepath.Join(t.TempDir(), "class", "dmi", "id")
	require.NoError(t, os.MkdirAll(dmiDir, 0o750))
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dmiDir, name), []byte(content+"\n"), 0o600))
	}
	// Return three levels up: id → dmi → class → <root>
	return filepath.Dir(filepath.Dir(filepath.Dir(dmiDir)))
}

func TestGatherOSInfoDebian(t *testing.T) {
	etcDir := setupEtcDir(t, sampleOSReleaseDebian)

	plugin := &NodeInfo{
		PathEtc: etcDir,
		PathSys: etcDir, // unused for this test but must be set
		Collect: []string{"os"},
		Log:     testutil.Logger{},
	}
	require.NoError(t, plugin.Init())

	var acc testutil.Accumulator
	require.NoError(t, plugin.Gather(&acc))
	require.Empty(t, acc.Errors)

	expected := []telegraf.Metric{
		metric.New(
			"node_os",
			map[string]string{
				"id":               "debian",
				"id_like":          "",
				"name":             "Debian GNU/Linux",
				"pretty_name":      "Debian GNU/Linux 13 (trixie)",
				"variant":          "",
				"variant_id":       "",
				"version":          "13 (trixie)",
				"version_codename": "trixie",
				"version_id":       "13",
			},
			map[string]interface{}{"info": int64(1)},
			time.Unix(0, 0),
			telegraf.Gauge,
		),
	}

	testutil.RequireMetricsEqual(t, expected, acc.GetTelegrafMetrics(), testutil.IgnoreTime())
}

func TestGatherOSInfoArch(t *testing.T) {
	// Arch Linux has no VERSION, VERSION_ID, VERSION_CODENAME, VARIANT keys.
	etcDir := setupEtcDir(t, sampleOSReleaseArch)

	plugin := &NodeInfo{
		PathEtc: etcDir,
		PathSys: etcDir,
		Collect: []string{"os"},
		Log:     testutil.Logger{},
	}
	require.NoError(t, plugin.Init())

	var acc testutil.Accumulator
	require.NoError(t, plugin.Gather(&acc))
	require.Empty(t, acc.Errors)

	metrics := acc.GetTelegrafMetrics()
	require.Len(t, metrics, 1)

	tags := metrics[0].Tags()
	require.Equal(t, "arch", tags["id"])
	require.Equal(t, "Arch Linux", tags["name"])
	// Missing keys must appear as empty-string tags.
	require.Empty(t, tags["version"])
	require.Empty(t, tags["version_id"])
	require.Empty(t, tags["version_codename"])
	require.Empty(t, tags["variant"])
	require.Empty(t, tags["variant_id"])
	require.Empty(t, tags["id_like"])

	_, ok := metrics[0].GetField("info")
	require.True(t, ok)
}

func TestGatherOSInfoAlpine(t *testing.T) {
	etcDir := setupEtcDir(t, sampleOSReleaseAlpine)

	plugin := &NodeInfo{
		PathEtc: etcDir,
		PathSys: etcDir,
		Collect: []string{"os"},
		Log:     testutil.Logger{},
	}
	require.NoError(t, plugin.Init())

	var acc testutil.Accumulator
	require.NoError(t, plugin.Gather(&acc))
	require.Empty(t, acc.Errors)

	metrics := acc.GetTelegrafMetrics()
	require.Len(t, metrics, 1)

	tags := metrics[0].Tags()
	require.Equal(t, "alpine", tags["id"])
	require.Equal(t, "3.19.0", tags["version_id"])
	require.Empty(t, tags["version"])
	require.Empty(t, tags["version_codename"])

	_, ok := metrics[0].GetField("info")
	require.True(t, ok)
}

func TestGatherOSInfoFedoraVariant(t *testing.T) {
	etcDir := setupEtcDir(t, sampleOSReleaseFedoraServer)

	plugin := &NodeInfo{
		PathEtc: etcDir,
		PathSys: etcDir,
		Collect: []string{"os"},
		Log:     testutil.Logger{},
	}
	require.NoError(t, plugin.Init())

	var acc testutil.Accumulator
	require.NoError(t, plugin.Gather(&acc))
	require.Empty(t, acc.Errors)

	metrics := acc.GetTelegrafMetrics()
	require.Len(t, metrics, 1)

	tags := metrics[0].Tags()
	require.Equal(t, "fedora", tags["id"])
	require.Equal(t, "Server Edition", tags["variant"])
	require.Equal(t, "server", tags["variant_id"])
	require.Empty(t, tags["version_codename"])

	_, ok := metrics[0].GetField("info")
	require.True(t, ok)
}

func TestGatherOSInfoFallbackToUsrLib(t *testing.T) {
	// Simulate a system where /etc/os-release is absent but
	// /usr/lib/os-release exists (common in some containers).
	td := t.TempDir()
	// Do NOT create td/os-release.
	usrLib := filepath.Join(td, "..", "usr", "lib")
	require.NoError(t, os.MkdirAll(usrLib, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(usrLib, "os-release"), []byte(sampleOSReleaseAlpine), 0o600))

	plugin := &NodeInfo{
		PathEtc: td,
		PathSys: td,
		Collect: []string{"os"},
		Log:     testutil.Logger{},
	}
	require.NoError(t, plugin.Init())

	var acc testutil.Accumulator
	require.NoError(t, plugin.Gather(&acc))
	require.Empty(t, acc.Errors)

	metrics := acc.GetTelegrafMetrics()
	require.Len(t, metrics, 1)
	require.Equal(t, "alpine", metrics[0].Tags()["id"])
}

func TestGatherOSInfoMissingBothFiles(t *testing.T) {
	td := t.TempDir()

	plugin := &NodeInfo{
		PathEtc: td,
		PathSys: td,
		Collect: []string{"os"},
		Log:     testutil.Logger{},
	}
	// Init succeeds (warns but does not fail) even when os-release is missing.
	require.NoError(t, plugin.Init())
	// The osTags cache must be nil so that Gather skips the metric.
	require.Nil(t, plugin.osTags)

	var acc testutil.Accumulator
	require.NoError(t, plugin.Gather(&acc))

	// No metrics and no errors — the warning was already logged during Init.
	require.Empty(t, acc.GetTelegrafMetrics())
	require.Empty(t, acc.Errors)
}

func TestGatherDMIInfo(t *testing.T) {
	sysRoot := setupDMIDir(t, map[string]string{
		"bios_date":         "04/01/2014",
		"bios_release":      "0.0",
		"bios_vendor":       "SeaBIOS",
		"bios_version":      "1.16.3-debian-1.16.3-2",
		"board_asset_tag":   "",
		"board_name":        "Standard PC",
		"board_serial":      "board-serial-001",
		"board_vendor":      "QEMU",
		"board_version":     "1.0",
		"chassis_asset_tag": "",
		"chassis_serial":    "",
		"chassis_vendor":    "QEMU",
		"chassis_version":   "pc-q35-10.0",
		"product_family":    "",
		"product_name":      "Standard PC (Q35 + ICH9, 2009)",
		"product_serial":    "",
		"product_sku":       "",
		"product_uuid":      "11111111-2222-3333-4444-555555555555",
		"product_version":   "pc-q35-10.0",
		"sys_vendor":        "QEMU",
	})

	etcDir := setupEtcDir(t, sampleOSReleaseDebian)

	plugin := &NodeInfo{
		PathEtc: etcDir,
		PathSys: sysRoot,
		Collect: []string{"dmi"},
		Log:     testutil.Logger{},
	}
	require.NoError(t, plugin.Init())

	var acc testutil.Accumulator
	require.NoError(t, plugin.Gather(&acc))
	require.Empty(t, acc.Errors)

	metrics := acc.GetTelegrafMetrics()
	require.Len(t, metrics, 1)
	require.Equal(t, "node_dmi", metrics[0].Name())

	tags := metrics[0].Tags()
	require.Equal(t, "04/01/2014", tags["bios_date"])
	require.Equal(t, "0.0", tags["bios_release"])
	require.Equal(t, "SeaBIOS", tags["bios_vendor"])
	require.Equal(t, "1.16.3-debian-1.16.3-2", tags["bios_version"])
	require.Equal(t, "Standard PC", tags["board_name"])
	require.Equal(t, "QEMU", tags["system_vendor"])
	require.Equal(t, "Standard PC (Q35 + ICH9, 2009)", tags["product_name"])
	require.Equal(t, "11111111-2222-3333-4444-555555555555", tags["product_uuid"])

	value, ok := metrics[0].GetField("info")
	require.True(t, ok)
	require.Equal(t, int64(1), value)
}

func TestGatherDMIInfoMissingFiles(t *testing.T) {
	// Only provide a subset of DMI files; the rest should default to empty strings.
	sysRoot := setupDMIDir(t, map[string]string{
		"bios_vendor":  "TestVendor",
		"product_name": "TestProduct",
		"sys_vendor":   "TestSystemVendor",
	})

	plugin := &NodeInfo{
		PathSys: sysRoot,
		PathEtc: sysRoot,
		Collect: []string{"dmi"},
		Log:     testutil.Logger{},
	}
	require.NoError(t, plugin.Init())

	var acc testutil.Accumulator
	require.NoError(t, plugin.Gather(&acc))
	require.Empty(t, acc.Errors)

	metrics := acc.GetTelegrafMetrics()
	require.Len(t, metrics, 1)

	tags := metrics[0].Tags()
	require.Equal(t, "TestVendor", tags["bios_vendor"])
	require.Equal(t, "TestProduct", tags["product_name"])
	require.Equal(t, "TestSystemVendor", tags["system_vendor"])
	// Missing files must produce empty tag values, not absent tags.
	require.Empty(t, tags["bios_date"])
	require.Empty(t, tags["chassis_vendor"])
	require.Empty(t, tags["product_uuid"])

	_, ok := metrics[0].GetField("info")
	require.True(t, ok)
}

func TestGatherDMIInfoDirectoryMissing(t *testing.T) {
	// Simulate ARM board or container where /sys/class/dmi/id/ is absent.
	td := t.TempDir()

	plugin := &NodeInfo{
		PathSys: td,
		PathEtc: td,
		Collect: []string{"dmi"},
		Log:     testutil.Logger{},
	}
	require.NoError(t, plugin.Init())

	var acc testutil.Accumulator
	require.NoError(t, plugin.Gather(&acc))

	// No metric should be emitted and no error should be accumulated.
	require.Empty(t, acc.Errors)
	require.Empty(t, acc.GetTelegrafMetrics())
}

func TestGatherUnameInfo(t *testing.T) {
	plugin := &NodeInfo{
		PathEtc: t.TempDir(),
		PathSys: t.TempDir(),
		Collect: []string{"uname"},
		Log:     testutil.Logger{},
	}
	require.NoError(t, plugin.Init())

	var acc testutil.Accumulator
	require.NoError(t, plugin.Gather(&acc))
	require.Empty(t, acc.Errors)

	metrics := acc.GetTelegrafMetrics()
	require.Len(t, metrics, 1)
	require.Equal(t, "node_uname", metrics[0].Name())

	tags := metrics[0].Tags()

	// We cannot predict exact kernel values on the CI machine, but we can
	// assert that the mandatory tags are present and non-empty.
	for _, key := range []string{"sysname", "release", "machine", "nodename", "version"} {
		v, ok := tags[key]
		require.Truef(t, ok, "tag %q is missing", key)
		require.NotEmptyf(t, v, "tag %q should not be empty", key)
	}
	// domainname may legitimately be "(none)" but the tag must exist.
	_, ok := tags["domainname"]
	require.True(t, ok, "tag \"domainname\" is missing")

	value, ok := metrics[0].GetField("info")
	require.True(t, ok)
	require.Equal(t, int64(1), value)
}

func TestCollectOnlyOS(t *testing.T) {
	etcDir := setupEtcDir(t, sampleOSReleaseDebian)

	plugin := &NodeInfo{
		PathEtc: etcDir,
		PathSys: t.TempDir(),
		Collect: []string{"os"},
		Log:     testutil.Logger{},
	}
	require.NoError(t, plugin.Init())

	var acc testutil.Accumulator
	require.NoError(t, plugin.Gather(&acc))
	require.Empty(t, acc.Errors)

	names := metricNames(acc.GetTelegrafMetrics())
	require.Contains(t, names, "node_os")
	require.NotContains(t, names, "node_dmi")
	require.NotContains(t, names, "node_uname")
}

func TestCollectOnlyUname(t *testing.T) {
	plugin := &NodeInfo{
		PathEtc: t.TempDir(),
		PathSys: t.TempDir(),
		Collect: []string{"uname"},
		Log:     testutil.Logger{},
	}
	require.NoError(t, plugin.Init())

	var acc testutil.Accumulator
	require.NoError(t, plugin.Gather(&acc))
	require.Empty(t, acc.Errors)

	names := metricNames(acc.GetTelegrafMetrics())
	require.Contains(t, names, "node_uname")
	require.NotContains(t, names, "node_os")
	require.NotContains(t, names, "node_dmi")
}

func TestCollectOnlyDMI(t *testing.T) {
	sysRoot := setupDMIDir(t, map[string]string{
		"sys_vendor": "TestVendor",
	})

	plugin := &NodeInfo{
		PathEtc: t.TempDir(),
		PathSys: sysRoot,
		Collect: []string{"dmi"},
		Log:     testutil.Logger{},
	}
	require.NoError(t, plugin.Init())

	var acc testutil.Accumulator
	require.NoError(t, plugin.Gather(&acc))
	require.Empty(t, acc.Errors)

	names := metricNames(acc.GetTelegrafMetrics())
	require.Contains(t, names, "node_dmi")
	require.NotContains(t, names, "node_os")
	require.NotContains(t, names, "node_uname")
}

func TestGatherAllMetrics(t *testing.T) {
	etcDir := setupEtcDir(t, sampleOSReleaseDebian)

	sysRoot := setupDMIDir(t, map[string]string{
		"sys_vendor":   "QEMU",
		"product_name": "Standard PC",
	})

	plugin := &NodeInfo{
		PathEtc: etcDir,
		PathSys: sysRoot,
		Log:     testutil.Logger{},
	}
	require.NoError(t, plugin.Init())

	var acc testutil.Accumulator
	require.NoError(t, plugin.Gather(&acc))
	require.Empty(t, acc.Errors)

	names := metricNames(acc.GetTelegrafMetrics())
	require.Contains(t, names, "node_os")
	require.Contains(t, names, "node_dmi")
	require.Contains(t, names, "node_uname")
}

func TestGatherMultipleCollectRuns(t *testing.T) {
	// Verify that repeated Gather() calls produce consistent results.
	etcDir := setupEtcDir(t, sampleOSReleaseDebian)

	plugin := &NodeInfo{
		PathEtc: etcDir,
		PathSys: t.TempDir(),
		Collect: []string{"os", "uname"},
		Log:     testutil.Logger{},
	}
	require.NoError(t, plugin.Init())

	for i := 0; i < 3; i++ {
		var acc testutil.Accumulator
		require.NoError(t, plugin.Gather(&acc))
		require.Empty(t, acc.Errors)

		names := metricNames(acc.GetTelegrafMetrics())
		require.Contains(t, names, "node_os")
		require.Contains(t, names, "node_uname")
		require.NotContains(t, names, "node_dmi")
	}
}

func TestReadFileTrimmed(t *testing.T) {
	td := t.TempDir()

	t.Run("normal value with newline", func(t *testing.T) {
		p := filepath.Join(td, "normal")
		require.NoError(t, os.WriteFile(p, []byte("SeaBIOS\n"), 0o600))
		v, err := readFileTrimmed(p)
		require.NoError(t, err)
		require.Equal(t, "SeaBIOS", v)
	})

	t.Run("value with extra whitespace", func(t *testing.T) {
		p := filepath.Join(td, "whitespace")
		require.NoError(t, os.WriteFile(p, []byte("  QEMU  \n"), 0o600))
		v, err := readFileTrimmed(p)
		require.NoError(t, err)
		require.Equal(t, "QEMU", v)
	})

	t.Run("empty file", func(t *testing.T) {
		p := filepath.Join(td, "empty")
		require.NoError(t, os.WriteFile(p, []byte(""), 0o600))
		v, err := readFileTrimmed(p)
		require.NoError(t, err)
		require.Empty(t, v)
	})

	t.Run("missing file returns error", func(t *testing.T) {
		_, err := readFileTrimmed(filepath.Join(td, "nonexistent"))
		require.Error(t, err)
	})
}

func metricNames(metrics []telegraf.Metric) []string {
	names := make([]string, 0, len(metrics))
	for _, m := range metrics {
		names = append(names, m.Name())
	}
	return names
}
