package querystate

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/queryclient"
	bolt "go.etcd.io/bbolt"
)

var ErrFull = errors.New("query state capacity reached")
var ErrExpired = errors.New("query record predates retention floor")
var Bounds = []float64{.01, .05, .1, .5, 1, 5, 10, 30, 60, 300}
var meta = []byte("meta")
var checkpoints = []byte("checkpoints")
var events = []byte("events")

type Options struct {
	Dir, Source               string
	MaxEvents, MaxCheckpoints int
	MaxBytes                  int64
	Retention                 time.Duration
	Events                    bool
}
type Store struct {
	db      *bolt.DB
	options Options
}
type Histogram struct {
	Count   uint64
	Sum     float64
	Buckets []uint64
}
type Stats struct {
	Revision   uint64
	Completed  map[string]uint64
	Histograms map[string]Histogram
	Pending    int
	Bytes      int64
	Floor      time.Time
}
type checkpoint struct {
	Final       bool
	Hash        [32]byte
	LastFetched time.Time
	Time        time.Time
}
type Record struct {
	Sequence     uint64
	Key, Payload []byte
}

func emptyStats() Stats {
	return Stats{Completed: map[string]uint64{}, Histograms: map[string]Histogram{}}
}
func Open(o Options) (*Store, error) {
	if !filepath.IsAbs(o.Dir) || o.Source == "" || o.MaxBytes < 1 || o.MaxEvents < 1 || o.MaxCheckpoints < 1 || o.Retention <= 0 {
		return nil, errors.New("query state options invalid")
	}
	if err := os.MkdirAll(o.Dir, 0700); err != nil {
		return nil, errors.New("query state directory unavailable")
	}
	if err := os.Chmod(o.Dir, 0700); err != nil {
		return nil, errors.New("query state directory permissions failed")
	}
	path := filepath.Join(o.Dir, "query-events.db")
	if info, err := os.Lstat(path); err == nil && (!info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
		return nil, errors.New("query state must be regular file")
	}
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, errors.New("query state open failed")
	}
	s := &Store{db: db, options: o}
	err = db.Update(func(tx *bolt.Tx) error {
		m := tx.Bucket(meta)
		if m != nil {
			if string(m.Get([]byte("version"))) != "1" || string(m.Get([]byte("source"))) != o.Source {
				return errors.New("query state schema or source mismatch")
			}
			for _, name := range [][]byte{checkpoints, events} {
				if tx.Bucket(name) == nil {
					return errors.New("query state corrupt")
				}
			}
			return nil
		}
		for _, name := range [][]byte{meta, checkpoints, events} {
			if _, err := tx.CreateBucket(name); err != nil {
				return err
			}
		}
		m = tx.Bucket(meta)
		if err := m.Put([]byte("version"), []byte("1")); err != nil {
			return err
		}
		if err := m.Put([]byte("source"), []byte(o.Source)); err != nil {
			return err
		}
		return saveStats(tx, emptyStats())
	})
	if err == nil {
		err = os.Chmod(path, 0600)
	}
	if err != nil {
		db.Close()
		return nil, err
	}
	if err = s.Prune(time.Now()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}
func readStats(tx *bolt.Tx) (Stats, error) {
	s := emptyStats()
	b := tx.Bucket(meta).Get([]byte("stats"))
	if len(b) == 0 || json.Unmarshal(b, &s) != nil || s.Completed == nil || s.Histograms == nil || s.Pending < 0 || s.Bytes < 0 {
		return s, errors.New("query aggregate corrupt")
	}
	return s, nil
}
func saveStats(tx *bolt.Tx, s Stats) error {
	s.Revision++
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return tx.Bucket(meta).Put([]byte("stats"), b)
}
func (s *Store) Prune(now time.Time) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		a, err := readStats(tx)
		if err != nil {
			return err
		}
		floor := now.Add(-s.options.Retention)
		if floor.Before(a.Floor) {
			floor = a.Floor
		}
		a.Floor = floor
		c := tx.Bucket(checkpoints).Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var cp checkpoint
			if json.Unmarshal(v, &cp) != nil {
				return errors.New("query checkpoint corrupt")
			}
			if cp.Time.Before(floor) {
				if err := c.Delete(); err != nil {
					return err
				}
			}
		}
		return saveStats(tx, a)
	})
}
func (s *Store) Fetch(id string) (bool, time.Time, error) {
	var cp checkpoint
	err := s.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(checkpoints).Get([]byte(id))
		if v == nil {
			return nil
		}
		if json.Unmarshal(v, &cp) != nil {
			return errors.New("query checkpoint corrupt")
		}
		return nil
	})
	return cp.Final, cp.LastFetched, err
}
func (s *Store) Record(e queryclient.Event) error {
	if e.SourceInstance != s.options.Source || !queryclient.ValidID(e.QueryID) {
		return errors.New("query record identity invalid")
	}
	stable := e
	stable.Observed = time.Time{}
	canonical, err := json.Marshal(stable)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(canonical)
	payload, err := json.Marshal(e)
	if err != nil || len(payload) > 64<<10 {
		return errors.New("query event exceeds bounds")
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		a, err := readStats(tx)
		if err != nil {
			return err
		}
		when := e.Submitted
		if e.Completed != nil {
			when = *e.Completed
		}
		if when.Before(a.Floor) {
			return ErrExpired
		}
		b := tx.Bucket(checkpoints)
		v := b.Get([]byte(e.QueryID))
		var previous checkpoint
		if v != nil {
			if json.Unmarshal(v, &previous) != nil {
				return errors.New("query checkpoint corrupt")
			}
			if previous.Final || e.Observed.Before(previous.LastFetched) {
				return nil
			}
		}
		if v == nil && b.Stats().KeyN >= s.options.MaxCheckpoints {
			return ErrFull
		}
		changed := v == nil || previous.Hash != hash
		if changed && s.options.Events {
			if a.Pending >= s.options.MaxEvents || a.Bytes+int64(len(payload)) > s.options.MaxBytes {
				return ErrFull
			}
			eb := tx.Bucket(events)
			seq, err := eb.NextSequence()
			if err != nil {
				return err
			}
			key := make([]byte, 8)
			binary.BigEndian.PutUint64(key, seq)
			if err := eb.Put(key, payload); err != nil {
				return err
			}
			a.Pending++
			a.Bytes += int64(len(payload))
		}
		if queryclient.Terminal(e.State) {
			a.Completed[e.State]++
			for _, v := range []struct {
				name  string
				value *float64
			}{{"elapsed", e.Elapsed}, {"execution", e.Execution}, {"queued", e.Queued}} {
				if v.value == nil {
					continue
				}
				key := v.name + ":" + e.State
				h := a.Histograms[key]
				if h.Buckets == nil {
					h.Buckets = make([]uint64, len(Bounds))
				}
				if len(h.Buckets) != len(Bounds) {
					return errors.New("query histogram corrupt")
				}
				h.Count++
				h.Sum += *v.value
				for i, bound := range Bounds {
					if *v.value <= bound {
						h.Buckets[i]++
					}
				}
				a.Histograms[key] = h
			}
		}
		cp := checkpoint{queryclient.Terminal(e.State), hash, e.Observed, when}
		data, err := json.Marshal(cp)
		if err != nil {
			return err
		}
		if err = b.Put([]byte(e.QueryID), data); err != nil {
			return err
		}
		return saveStats(tx, a)
	})
}
func (s *Store) Stats() (Stats, error) {
	var a Stats
	err := s.db.View(func(tx *bolt.Tx) error {
		var err error
		a, err = readStats(tx)
		if err != nil {
			return err
		}
		if a.Pending != tx.Bucket(events).Stats().KeyN {
			return errors.New("query pending count corrupt")
		}
		return nil
	})
	return a, err
}
func (s *Store) Records(limit int) ([]Record, error) {
	if limit < 1 || limit > 100 {
		return nil, errors.New("query batch bound invalid")
	}
	var result []Record
	err := s.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(events).Cursor()
		for k, v := c.First(); k != nil && len(result) < limit; k, v = c.Next() {
			if len(k) != 8 {
				return errors.New("query sequence corrupt")
			}
			var e queryclient.Event
			if json.Unmarshal(v, &e) != nil || !queryclient.ValidID(e.QueryID) {
				return errors.New("query outbox corrupt")
			}
			key := sha256.Sum256([]byte(e.SourceInstance + "\x00" + e.QueryID))
			result = append(result, Record{binary.BigEndian.Uint64(k), append([]byte(nil), key[:]...), append([]byte(nil), v...)})
		}
		return nil
	})
	return result, err
}
func (s *Store) Ack(seq uint64) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(events)
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, seq)
		v := b.Get(key)
		if v == nil {
			return nil
		}
		a, err := readStats(tx)
		if err != nil {
			return err
		}
		a.Pending--
		a.Bytes -= int64(len(v))
		if a.Pending < 0 || a.Bytes < 0 {
			return errors.New("query outbox accounting corrupt")
		}
		if err := b.Delete(key); err != nil {
			return err
		}
		return saveStats(tx, a)
	})
}
func (s *Store) Close() error { return s.db.Close() }
