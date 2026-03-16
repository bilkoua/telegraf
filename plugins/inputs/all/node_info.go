//go:build !custom || inputs || inputs.node_info

package all

import _ "github.com/influxdata/telegraf/plugins/inputs/node_info" // register plugin
