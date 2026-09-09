package historystate

import (
	"encoding/json"
	"fmt"
	bolt "go.etcd.io/bbolt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TEST-DDAE-10-004: durable replay, source pinning and failed transactions.
func TestBackfillStoreRecovery(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	at := time.Now().UTC().Truncate(time.Second)
	if e = s.Update("queries", "source-a", 1000, func(p *Progress) error { p.Windows = []Window{{Start: at.Add(-time.Hour), End: at}}; return nil }); e != nil {
		t.Fatal(e)
	}
	s.Close()
	s, e = Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	p, e := s.Load("queries", "source-a")
	if e != nil || len(p.Windows) != 1 {
		t.Fatal(p, e)
	}
	if _, e = s.Load("queries", "source-b"); e == nil {
		t.Fatal("source mismatch")
	}
	if e = s.Update("queries", "source-a", 1000, func(p *Progress) error { p.Windows = make([]Window, 4097); return nil }); e == nil {
		t.Fatal("unbounded windows")
	}
	p, e = s.Load("queries", "source-a")
	if e != nil || len(p.Windows) != 1 {
		t.Fatal("failed transaction changed progress")
	}
}

func TestBackfillCorruptionAndSchema(t *testing.T) {
	for _, mode := range []string{"schema", "checksum", "unknown-db"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			s, e := Open(dir)
			if e != nil {
				t.Fatal(e)
			}
			if mode == "unknown-db" {
				e = s.db.Update(func(tx *bolt.Tx) error { return tx.DeleteBucket(bucket) })
			} else {
				if e = s.Update("queries", "test", 1000, func(p *Progress) error { return nil }); e != nil {
					t.Fatal(e)
				}
				e = s.db.Update(func(tx *bolt.Tx) error {
					b := tx.Bucket(bucket)
					var v envelope
					if e := json.Unmarshal(b.Get([]byte("queries")), &v); e != nil {
						return e
					}
					if mode == "schema" {
						v.Version = 99
					} else {
						v.Hash = "invalid"
					}
					data, _ := json.Marshal(v)
					return b.Put([]byte("queries"), data)
				})
			}
			if e != nil {
				t.Fatal(e)
			}
			s.Close()
			before, e := os.ReadFile(filepath.Join(dir, "history-backfill.db"))
			if e != nil {
				t.Fatal(e)
			}
			s, e = Open(dir)
			if mode == "unknown-db" {
				if e == nil {
					s.Close()
					t.Fatal("unknown schema reset")
				}
			} else {
				if e != nil {
					t.Fatal(e)
				}
				if _, e = s.Load("queries", "test"); e == nil {
					t.Fatal("corruption accepted")
				}
				s.Close()
			}
			after, e := os.ReadFile(filepath.Join(dir, "history-backfill.db"))
			if e != nil || len(after) < len(before) {
				t.Fatal("state discarded", e)
			}
		})
	}
}
func TestBackfillPendingCapacity(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	e = s.Update("queries", "test", 1, func(p *Progress) error {
		p.Pending = []Item{{ID: "a", When: time.Now()}, {ID: "b", When: time.Now()}}
		return nil
	})
	if e != ErrFull {
		t.Fatal(e)
	}
	p, e := s.Load("queries", "test")
	if e != nil || len(p.Pending) != 0 {
		t.Fatal("failed transaction changed IDs")
	}
}

func TestBackfillLogicalByteLimitAndValidation(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	at := time.Now().UTC()
	e = s.Update("queries", "test", 100000, func(p *Progress) error {
		for n := 0; n < 33000; n++ {
			p.Pending = append(p.Pending, Item{ID: fmt.Sprint(n) + strings.Repeat("x", 1000), When: at})
		}
		return nil
	})
	if e != ErrFull {
		t.Fatal("byte ceiling", e)
	}
	p, e := s.Load("queries", "test")
	if e != nil || len(p.Pending) != 0 {
		t.Fatal("byte overflow committed")
	}
	for _, bad := range []Progress{{Source: "other"}, {Source: "test", Windows: []Window{{Start: at, End: at}}}, {Source: "test", Page: &Window{}}, {Source: "test", Pending: []Item{{ID: "a", When: at}, {ID: "a", When: at}}}} {
		if e = s.Update("queries", "test", 1000, func(p *Progress) error { *p = bad; return nil }); e == nil {
			t.Fatal("malformed progress accepted")
		}
	}
	if _, e = s.Load("unexpected", "test"); e == nil {
		t.Fatal("invalid key")
	}
	if e = s.Update("queries", "test", 0, func(*Progress) error { return nil }); e == nil {
		t.Fatal("invalid capacity")
	}
	if _, e = Open("relative"); e == nil {
		t.Fatal("relative path")
	}
}
