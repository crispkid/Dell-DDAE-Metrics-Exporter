package config

import (
	"testing"
	"time"
)

// TEST-DDAE-10-001: disabled, parent, timing and retention boundaries.
func TestBackfillConfiguration(t *testing.T) {
	env := map[string]string{}
	lookup := func(k string) (string, bool) { v, ok := env[k]; return v, ok }
	if c, e := loadBackfill(lookup, "QUERY_BACKFILL_", false, 5*time.Second, 720*time.Hour); e != nil || c.Enabled {
		t.Fatal(c, e)
	}
	env["QUERY_BACKFILL_ENABLED"] = "true"
	if _, e := loadBackfill(lookup, "QUERY_BACKFILL_", false, 5*time.Second, 720*time.Hour); e == nil {
		t.Fatal("disabled parent accepted")
	}
	c, e := loadBackfill(lookup, "QUERY_BACKFILL_", true, 5*time.Second, 720*time.Hour)
	if e != nil || c.Lookback != 24*time.Hour || c.MaxPages != 4 || c.DetailMax != 25 {
		t.Fatal(c, e)
	}
	for k, v := range map[string]string{"LOOKBACK": "721h", "OVERLAP": "25h", "CYCLE_TIMEOUT": "30s", "DETAIL_CONCURRENCY": "9", "MAX_PENDING_RECORDS": "999"} {
		env["QUERY_BACKFILL_"+k] = v
		if _, e := loadBackfill(lookup, "QUERY_BACKFILL_", true, 5*time.Second, 720*time.Hour); e == nil {
			t.Fatal(k)
		}
		delete(env, "QUERY_BACKFILL_"+k)
	}
	if _, e := loadBackfill(lookup, "QUERY_BACKFILL_", true, 5*time.Second, 12*time.Hour); e == nil {
		t.Fatal("retention overrun")
	}
}

func TestBackfillAllBounds(t *testing.T) {
	for _, tc := range []struct{ key, value string }{{"LOOKBACK", "59m"}, {"LOOKBACK", "721h"}, {"OVERLAP", "500ms"}, {"OVERLAP", "61m"}, {"INTERVAL", "4s"}, {"INTERVAL", "61m"}, {"CYCLE_TIMEOUT", "5s"}, {"CYCLE_TIMEOUT", "30s"}, {"RESCAN_INTERVAL", "20s"}, {"RESCAN_INTERVAL", "25h"}, {"MAX_PAGES_PER_CYCLE", "0"}, {"MAX_PAGES_PER_CYCLE", "33"}, {"DETAIL_MAX_PER_CYCLE", "0"}, {"DETAIL_MAX_PER_CYCLE", "1001"}, {"DETAIL_CONCURRENCY", "0"}, {"DETAIL_CONCURRENCY", "9"}, {"MAX_PENDING_RECORDS", "999"}, {"MAX_PENDING_RECORDS", "100001"}, {"LOOKBACK", "invalid"}} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			env := map[string]string{"QUERY_BACKFILL_ENABLED": "true", "QUERY_BACKFILL_" + tc.key: tc.value}
			lookup := func(k string) (string, bool) { v, ok := env[k]; return v, ok }
			if _, e := loadBackfill(lookup, "QUERY_BACKFILL_", true, 5*time.Second, 720*time.Hour); e == nil {
				t.Fatal("invalid setting accepted")
			}
		})
	}
	values, e := decodeYAML([]byte("version: 1\nmonitoring:\n  queries:\n    backfill:\n      enabled: true\n      lookback: 48h\n      max_pages_per_cycle: 8\n"))
	if e != nil {
		t.Fatal(e)
	}
	env := func(k string) (string, bool) {
		if k == "QUERY_BACKFILL_MAX_PAGES_PER_CYCLE" {
			return "2", true
		}
		return "", false
	}
	c, e := loadBackfill(layeredLookup(values, env), "QUERY_BACKFILL_", true, 5*time.Second, 720*time.Hour)
	if e != nil || c.Lookback != 48*time.Hour || c.MaxPages != 2 {
		t.Fatal(c, e)
	}
}
