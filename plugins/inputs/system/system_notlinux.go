//go:build !linux

package system

import "github.com/influxdata/telegraf"

type platformData struct{}

func (s *System) Init() error {
	return s.initCommon(crossPlatformCollectors)
}

func (*System) gatherPlatformInfo(_ telegraf.Accumulator) {}
