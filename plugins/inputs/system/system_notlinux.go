//go:build !linux

package system

import "github.com/influxdata/telegraf"

type platformData struct{} //nolint:unused // not used on non-Linux, needed for System struct

func (s *System) Init() error {
	return s.initCommon(crossPlatformCollectors)
}

func (*System) gatherPlatformInfo(_ telegraf.Accumulator) {}
