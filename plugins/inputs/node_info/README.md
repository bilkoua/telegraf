# Node Info Input Plugin

Collects static node information and exposes it as labeled gauge metrics with a
constant value of `1`, mirroring the behavior of
[prometheus-node-exporter][node-exporter] for the `node_os_info`,
`node_dmi_info`, and `node_uname_info` metrics.

⭐ Telegraf v1.34.0
🏷️ system
💻 linux

[node-exporter]: https://github.com/prometheus/node_exporter

## Global configuration options <!-- @/docs/includes/plugin_config.md -->

Plugins support additional global and plugin configuration settings for tasks
such as modifying metrics, tags, and fields, creating aliases, and configuring
plugin ordering. See [CONFIGURATION.md][CONFIGURATION.md] for more details.

[CONFIGURATION.md]: ../../../docs/CONFIGURATION.md#plugins

## Configuration

```toml @sample.conf
# Collect node OS, DMI, and uname information (analogous to prometheus-node-exporter)
# This plugin ONLY supports Linux
[[inputs.node_info]]
  ## Path to the host /etc directory.
  ## Useful when running inside a container with the host filesystem mounted.
  ## Defaults:
  # host_etc = "/etc"

  ## Path to the host /sys directory.
  ## Useful when running inside a container with the host filesystem mounted.
  ## Defaults:
  # host_sys = "/sys"

  ## Metric groups to collect.
  ## Available options: "os", "dmi", "uname"
  ## Defaults:
  # collect = ["os", "dmi", "uname"]
```

### Container / privileged usage

When Telegraf runs inside a container but needs to inspect the **host**
filesystem, mount the host paths and point the plugin at them:

```toml
[[inputs.node_info]]
  host_etc = "/host/etc"
  host_sys = "/host/sys"
```

Some DMI files under `/sys/class/dmi/id/` (e.g. `product_serial`,
`board_serial`, `product_uuid`) are readable only by `root`.  When running as
an unprivileged user those fields will appear as empty strings in the metric
tags; no error is returned.

> [!NOTE]
> On platforms where `/sys/class/dmi/id/` does not exist (ARM SBCs,
> unprivileged containers, etc.) the `node_dmi` metric is silently skipped.
> To avoid the directory lookup entirely, set `collect = ["os", "uname"]`.

## Metrics

Each measurement has a single field `info` (integer, gauge, always `1`)
with labels encoded as tags.

### `node_os_info`

Sourced from `/etc/os-release` ([os-release(5)][os-release]).  Not all
distributions provide every key; missing keys appear as empty-string tags.

| Tag                | Description                                      |
|--------------------|--------------------------------------------------|
| `id`               | Distribution identifier                          |
| `id_like`          | Space-separated list of related distribution IDs |
| `name`             | Human-readable distribution name                 |
| `pretty_name`      | Human-readable name including version            |
| `variant`          | Variant of the distribution (if any)             |
| `variant_id`       | Machine-readable variant identifier              |
| `version`          | Version string                                   |
| `version_codename` | Release codename                                 |
| `version_id`       | Machine-readable version identifier              |

[os-release]: https://www.freedesktop.org/software/systemd/man/os-release.html

### `node_dmi_info`

Sourced from individual files under `/sys/class/dmi/id/`
([DMI/SMBIOS][smbios]).  Tag names match the source file names, except
`system_vendor` which reads from `sys_vendor`.  Absent or unreadable fields
are reported as empty strings.

| Tag                | Description                           |
|--------------------|---------------------------------------|
| `bios_date`        | BIOS release date                     |
| `bios_release`     | BIOS major.minor release number       |
| `bios_vendor`      | BIOS vendor name                      |
| `bios_version`     | BIOS version string                   |
| `board_asset_tag`  | Baseboard asset tag                   |
| `board_name`       | Baseboard product name                |
| `board_serial`     | Baseboard serial number *(root only)* |
| `board_vendor`     | Baseboard manufacturer                |
| `board_version`    | Baseboard version                     |
| `chassis_asset_tag`| Chassis asset tag                     |
| `chassis_serial`   | Chassis serial number *(root only)*   |
| `chassis_vendor`   | Chassis manufacturer                  |
| `chassis_version`  | Chassis version                       |
| `product_family`   | Product family                        |
| `product_name`     | Product name                          |
| `product_serial`   | Product serial number *(root only)*   |
| `product_sku`      | Product SKU number                    |
| `product_uuid`     | Product UUID *(root only)*            |
| `product_version`  | Product version                       |
| `system_vendor`    | System manufacturer                   |

[smbios]: https://www.dmtf.org/standards/smbios

### `node_uname_info`

Sourced from the `uname(2)` system call.

| Tag          | Description                                    | Example                                    |
|--------------|------------------------------------------------|--------------------------------------------|
| `sysname`    | Operating system name                          | `Linux`                                    |
| `nodename`   | Node hostname                                  | `worker-01.example.com`                    |
| `release`    | Kernel release string                          | `6.12.57+deb13-amd64`                      |
| `version`    | Kernel version / build info                    | `#1 SMP PREEMPT_DYNAMIC Debian 6.12.57-1`  |
| `machine`    | Hardware architecture                          | `x86_64`                                   |
| `domainname` | NIS domain name (`(none)` when not configured) | `(none)`                                   |

## Example Output

```text
node_os,host=worker-01,id=debian,id_like=,name=Debian\ GNU/Linux,pretty_name=Debian\ GNU/Linux\ 13\ (trixie),variant=,variant_id=,version=13\ (trixie),version_codename=trixie,version_id=13 info=1i 1748000000000000000
node_dmi,bios_date=04/01/2014,bios_release=0.0,bios_vendor=SeaBIOS,bios_version=1.16.3-debian-1.16.3-2,board_asset_tag=,board_name=,board_serial=,board_vendor=,board_version=,chassis_asset_tag=,chassis_serial=,chassis_vendor=QEMU,chassis_version=pc-q35-10.0,host=worker-01,product_family=,product_name=Standard\ PC\ (Q35\ +\ ICH9\,\ 2009),product_serial=,product_sku=,product_uuid=,product_version=pc-q35-10.0,system_vendor=QEMU info=1i 1748000000000000000
node_uname,domainname=(none),host=worker-01,machine=x86_64,nodename=worker-01.example.com,release=6.12.57+deb13-amd64,sysname=Linux,version=#1\ SMP\ PREEMPT_DYNAMIC\ Debian\ 6.12.57-1\ (2025-11-05) info=1i 1748000000000000000
```
