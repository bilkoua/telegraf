//go:generate ../../../tools/readme_config_includer/generator
//go:build !linux

package node_info

import (
	_ "embed"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
)

//go:embed sample.conf
var sampleConfig string

// NodeInfo is a stub for non-Linux platforms.
type NodeInfo struct {
	Log telegraf.Logger `toml:"-"`
}

func (*NodeInfo) SampleConfig() string { return sampleConfig }

func (n *NodeInfo) Init() error {
	n.Log.Warn("Current platform is not supported")
	return nil
}

func (*NodeInfo) Gather(_ telegraf.Accumulator) error { return nil }

func init() {
	inputs.Add("node_info", func() telegraf.Input {
		return &NodeInfo{}
	})
}
