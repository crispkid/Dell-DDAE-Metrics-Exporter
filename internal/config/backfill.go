package config

import (
	"fmt"
	"time"
)

type BackfillConfig struct {
	Enabled                                                   bool
	Lookback, Overlap, Interval, CycleTimeout, RescanInterval time.Duration
	MaxPages, DetailMax, Concurrency, MaxPending              int
}

func loadBackfill(lookup lookupFunc, prefix string, parent bool, request, retention time.Duration) (BackfillConfig, error) {
	var c BackfillConfig
	var err error
	c.Enabled, err = boolean(lookup, prefix+"ENABLED", false)
	if err != nil || !c.Enabled {
		return c, err
	}
	if !parent {
		return c, fmt.Errorf("%sENABLED requires its monitoring pipeline", prefix)
	}
	for _, v := range []struct {
		name          string
		dst           *time.Duration
		def, min, max time.Duration
	}{
		{"LOOKBACK", &c.Lookback, 24 * time.Hour, time.Hour, 720 * time.Hour}, {"OVERLAP", &c.Overlap, 2 * time.Minute, time.Second, time.Hour}, {"INTERVAL", &c.Interval, 30 * time.Second, 5 * time.Second, time.Hour}, {"CYCLE_TIMEOUT", &c.CycleTimeout, 20 * time.Second, time.Second, time.Hour}, {"RESCAN_INTERVAL", &c.RescanInterval, time.Hour, 5 * time.Second, 720 * time.Hour}} {
		*v.dst, err = duration(lookup, prefix+v.name, v.def)
		if err != nil {
			return c, err
		}
		if *v.dst < v.min || *v.dst > v.max {
			return c, fmt.Errorf("%s%s outside bounds", prefix, v.name)
		}
	}
	if c.Lookback > retention || c.Overlap > c.Lookback || c.CycleTimeout <= request || c.CycleTimeout >= c.Interval || c.RescanInterval < c.Interval || c.RescanInterval > c.Lookback {
		return c, fmt.Errorf("%s timing or retention constraints invalid", prefix)
	}
	for _, v := range []struct {
		name          string
		dst           *int
		def, min, max int
	}{{"MAX_PAGES_PER_CYCLE", &c.MaxPages, 4, 1, 32}, {"DETAIL_MAX_PER_CYCLE", &c.DetailMax, 25, 1, 1000}, {"DETAIL_CONCURRENCY", &c.Concurrency, 2, 1, 8}, {"MAX_PENDING_RECORDS", &c.MaxPending, 10000, 1000, 100000}} {
		*v.dst, err = boundedInt(lookup, prefix+v.name, v.def, v.min, v.max)
		if err != nil {
			return c, err
		}
	}
	if c.Concurrency > c.DetailMax {
		return c, fmt.Errorf("%s concurrency exceeds detail budget", prefix)
	}
	return c, nil
}
