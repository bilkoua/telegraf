# System Input Plugin

This plugin gathers general system statistics like system load, uptime or the
number of users logged in. It is similar to the unix `uptime` command.

On Linux it also collects static node-identity metrics (OS release, DMI/SMBIOS,
and uname), similar to [prometheus-node-exporter][node-exporter].

⭐ Telegraf v0.1.6
🏷️ system
💻 all

[node-exporter]: https://github.com/prometheus/node_exporter

## Global configuration options <!-- @/docs/includes/plugin_config.md -->

Plugins support additional global and plugin configuration settings for tasks
such as modifying metrics, tags, and fields, creating aliases, and configuring
plugin ordering. See [CONFIGURATION.md][CONFIGURATION.md] for more details.

[CONFIGURATION.md]: ../../../docs/CONFIGURATION.md#plugins

## Configuration

```toml @sample.conf
# Read metrics about system load, uptime, users, and node identity
[[inputs.system]]
  ## Metric groups to collect.
  ## Available options:
  ##   load   - load averages (load1, load5, load15)
  ##   users  - logged-in user counts (n_users, n_unique_users)
  ##   n_cpus - CPU counts (n_cpus, n_physical_cpus)
  ##   uptime - system uptime (uptime, uptime_format)
  ##   os     - OS release info from /etc/os-release         (Linux only)
  ##   dmi    - DMI/SMBIOS hardware info from /sys/class/dmi  (Linux only)
  ##   uname  - kernel info from uname(2)                     (Linux only)
  ## By default all groups available on the current platform are collected.
  # collect = ["load", "users", "n_cpus", "uptime", "os", "dmi", "uname"]

  ## Path to the host /etc directory (used by the "os" collector).
  ## Useful when running inside a container with the host filesystem mounted.
  # host_etc = "/etc"

  ## Path to the host /sys directory (used by the "dmi" collector).
  ## Useful when running inside a container with the host filesystem mounted.
  # host_sys = "/sys"
```

### Permissions

The `n_users` field requires read access to `/var/run/utmp`, and may require the
`telegraf` user to be added to the `utmp` group on some systems. If this file
does not exist `n_users` will be skipped.

The `n_unique_users` shows the count of unique usernames logged in. This way if
a user has multiple sessions open/started they would only get counted once. The
same requirements for `n_users` apply.

### Container / privileged usage (Linux)

When Telegraf runs inside a container but needs to inspect the **host**
filesystem, mount the host paths and point the plugin at them:

```toml
[[inputs.system]]
  host_etc = "/host/etc"
  host_sys = "/host/sys"
```

Some DMI files under `/sys/class/dmi/id/` (e.g. `product_serial`,
`board_serial`, `product_uuid`) are readable only by `root`. When running as
an unprivileged user those fields will appear as empty strings in the metric
tags; no error is returned.

On platforms where `/sys/class/dmi/id/` does not exist (ARM SBCs,
unprivileged containers, etc.) the `system_dmi` metric is silently skipped.

> [!TIP]
> Any collector group can be disabled by removing it from the `collect` list.
> For example, to collect only load averages and uptime:
>
> ```toml
> collect = ["load", "uptime"]
> ```

## Metrics

### `system`

All fields are emitted in the `system` measurement. Each field is only present
when its collector group is enabled in `collect`.

| Field             | Group    | Type    | Description                                    |
|-------------------|----------|---------|------------------------------------------------|
| `load1`           | `load`   | float   | 1-minute load average                          |
| `load5`           | `load`   | float   | 5-minute load average                          |
| `load15`          | `load`   | float   | 15-minute load average                         |
| `n_users`         | `users`  | integer | Number of logged-in user sessions              |
| `n_unique_users`  | `users`  | integer | Number of unique logged-in usernames           |
| `n_cpus`          | `n_cpus` | integer | Number of logical CPUs                         |
| `n_physical_cpus` | `n_cpus` | integer | Number of physical CPUs                        |
| `uptime`          | `uptime` | integer | System uptime in seconds                       |
| `uptime_format`   | `uptime` | string  | Human-readable uptime (deprecated, use uptime) |

### `system_os` (Linux only)

Sourced from `/etc/os-release` ([os-release(5)][os-release]). Not all
distributions provide every key; missing keys are set to empty strings
internally (visible in Prometheus output, omitted in InfluxDB line protocol).
Each measurement has a single field `info` (integer gauge, always `1`).

| Tag                | Description                                      | Example                          |
|--------------------|--------------------------------------------------|----------------------------------|
| `id`               | Distribution identifier                          | `debian`                         |
| `id_like`          | Space-separated list of related distribution IDs | `rhel centos fedora`             |
| `name`             | Human-readable distribution name                 | `Debian GNU/Linux`               |
| `pretty_name`      | Human-readable name including version            | `Debian GNU/Linux 13 (trixie)`   |
| `variant`          | Variant of the distribution (if any)             | `Server Edition`                 |
| `variant_id`       | Machine-readable variant identifier              | `server`                         |
| `version`          | Version string                                   | `13 (trixie)`                    |
| `version_codename` | Release codename                                 | `trixie`                         |
| `version_id`       | Machine-readable version identifier              | `13`                             |

[os-release]: https://www.freedesktop.org/software/systemd/man/os-release.html

### `system_dmi` (Linux only)

Sourced from individual files under `/sys/class/dmi/id/`
([DMI/SMBIOS][smbios]). Tag names match the source file names, except
`system_vendor` which reads from `sys_vendor`. Absent or unreadable fields
are set to empty strings internally (visible in Prometheus output, omitted in
InfluxDB line protocol).
Each measurement has a single field `info` (integer gauge, always `1`).

| Tag                 | Description                           | Example                                  |
|---------------------|---------------------------------------|------------------------------------------|
| `bios_date`         | BIOS release date                     | `04/01/2014`                             |
| `bios_release`      | BIOS major.minor release number       | `0.0`                                    |
| `bios_vendor`       | BIOS vendor name                      | `SeaBIOS`                                |
| `bios_version`      | BIOS version string                   | `1.16.3-debian-1.16.3-2`                 |
| `board_asset_tag`   | Baseboard asset tag                   | `board-asset-tag`                        |
| `board_name`        | Baseboard product name                | `Standard PC (Q35 + ICH9, 2009)`         |
| `board_serial`      | Baseboard serial number *(root only)* | `board-serial-001`                       |
| `board_vendor`      | Baseboard manufacturer                | `QEMU`                                   |
| `board_version`     | Baseboard version                     | `pc-q35-10.0`                            |
| `chassis_asset_tag` | Chassis asset tag                     | `chassis-asset-tag`                      |
| `chassis_serial`    | Chassis serial number *(root only)*   | `chassis-serial-001`                     |
| `chassis_vendor`    | Chassis manufacturer                  | `QEMU`                                   |
| `chassis_version`   | Chassis version                       | `pc-q35-10.0`                            |
| `product_family`    | Product family                        | `QEMU Virtual Machine`                   |
| `product_name`      | Product name                          | `Standard PC (Q35 + ICH9, 2009)`         |
| `product_serial`    | Product serial number *(root only)*   | `product-serial-001`                     |
| `product_sku`       | Product SKU number                    | `pc-q35-10.0`                            |
| `product_uuid`      | Product UUID *(root only)*            | `11111111-2222-3333-4444-555555555555`   |
| `product_version`   | Product version                       | `pc-q35-10.0`                            |
| `system_vendor`     | System manufacturer                   | `QEMU`                                   |

[smbios]: https://www.dmtf.org/standards/smbios

### `system_uname` (Linux only)

Sourced from the `uname(2)` system call.
Each measurement has a single field `info` (integer gauge, always `1`).

| Tag          | Description                                    | Example                                      |
|--------------|------------------------------------------------|----------------------------------------------|
| `sysname`    | Operating system name                          | `Linux`                                      |
| `nodename`   | Node hostname                                  | `worker-01.example.com`                      |
| `release`    | Kernel release string                          | `6.12.57+deb13-amd64`                        |
| `version`    | Kernel version / build info                    | `#1 SMP PREEMPT_DYNAMIC Debian 6.12.57-1`    |
| `machine`    | Hardware architecture                          | `x86_64`                                     |
| `domainname` | NIS domain name (`(none)` when not configured) | `(none)`                                     |

## Example Output

```text
system,host=worker-01 load1=3.72,load5=2.4,load15=2.1,n_users=3i,n_unique_users=2i,n_cpus=4i,n_physical_cpus=2i 1748000000000000000
system,host=worker-01 uptime=1249632i 1748000000000000000
system,host=worker-01 uptime_format="14 days, 11:07" 1748000000000000000
system_os,host=worker-01,id=debian,name=Debian\ GNU/Linux,pretty_name=Debian\ GNU/Linux\ 13\ (trixie),version=13\ (trixie),version_codename=trixie,version_id=13 info=1i 1748000000000000000
system_dmi,bios_date=04/01/2014,bios_release=0.0,bios_vendor=SeaBIOS,bios_version=1.16.3-debian-1.16.3-2,chassis_vendor=QEMU,chassis_version=pc-q35-10.0,host=worker-01,product_name=Standard\ PC\ (Q35\ +\ ICH9\,\ 2009),product_version=pc-q35-10.0,system_vendor=QEMU info=1i 1748000000000000000
system_uname,domainname=(none),host=worker-01,machine=x86_64,nodename=worker-01.example.com,release=6.12.57+deb13-amd64,sysname=Linux,version=#1\ SMP\ PREEMPT_DYNAMIC\ Debian\ 6.12.57-1\ (2025-11-05) info=1i 1748000000000000000
```

## Example Output (Prometheus)

When using the [Prometheus output plugin][prom-output] or
[Prometheus client plugin][prom-client], Telegraf converts each field into
its own Prometheus metric by appending the field name to the measurement name.

[prom-output]: ../../../plugins/outputs/prometheus_client/README.md
[prom-client]: ../../../plugins/outputs/prometheus_client/README.md

```text
# HELP system_load15 Telegraf collected metric
# TYPE system_load15 gauge
system_load15{host="worker-01"} 2.1

# HELP system_load1 Telegraf collected metric
# TYPE system_load1 gauge
system_load1{host="worker-01"} 3.72

# HELP system_load5 Telegraf collected metric
# TYPE system_load5 gauge
system_load5{host="worker-01"} 2.4

# HELP system_n_cpus Telegraf collected metric
# TYPE system_n_cpus gauge
system_n_cpus{host="worker-01"} 4

# HELP system_n_physical_cpus Telegraf collected metric
# TYPE system_n_physical_cpus gauge
system_n_physical_cpus{host="worker-01"} 2

# HELP system_n_unique_users Telegraf collected metric
# TYPE system_n_unique_users gauge
system_n_unique_users{host="worker-01"} 2

# HELP system_n_users Telegraf collected metric
# TYPE system_n_users gauge
system_n_users{host="worker-01"} 3

# HELP system_uptime Telegraf collected metric
# TYPE system_uptime counter
system_uptime{host="worker-01"} 1249632

# HELP system_os_info Telegraf collected metric
# TYPE system_os_info gauge
system_os_info{host="worker-01",id="debian",id_like="",name="Debian GNU/Linux",pretty_name="Debian GNU/Linux 13 (trixie)",variant="",variant_id="",version="13 (trixie)",version_codename="trixie",version_id="13"} 1

# HELP system_dmi_info Telegraf collected metric
# TYPE system_dmi_info gauge
system_dmi_info{bios_date="04/01/2014",bios_release="0.0",bios_vendor="SeaBIOS",bios_version="1.16.3-debian-1.16.3-2",board_asset_tag="",board_name="",board_serial="",board_vendor="",board_version="",chassis_asset_tag="",chassis_serial="",chassis_vendor="QEMU",chassis_version="pc-q35-10.0",host="worker-01",product_family="",product_name="Standard PC (Q35 + ICH9, 2009)",product_serial="",product_sku="",product_uuid="",product_version="pc-q35-10.0",system_vendor="QEMU"} 1

# HELP system_uname_info Telegraf collected metric
# TYPE system_uname_info gauge
system_uname_info{domainname="(none)",host="worker-01",machine="x86_64",nodename="worker-01.example.com",release="6.12.57+deb13-amd64",sysname="Linux",version="#1 SMP PREEMPT_DYNAMIC Debian 6.12.57-1 (2025-11-05)"} 1
```
