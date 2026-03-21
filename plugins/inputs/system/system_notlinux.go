//go:build !linux

package system

import "github.com/influxdata/telegraf"

func (s *System) Init() error {
	return s.initCommon(crossPlatformCollectors)
}

func (s *System) gatherPlatformInfo(_ telegraf.Accumulator) {}
