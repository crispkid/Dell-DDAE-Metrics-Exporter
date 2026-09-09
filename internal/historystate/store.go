package historystate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	bolt "go.etcd.io/bbolt"
	"os"
	"path/filepath"
	"time"
)

var ErrCorrupt = errors.New("history progress corrupt or incompatible")
var ErrFull = errors.New("history progress capacity reached")
var bucket = []byte("progress-v1")

type Window struct{ Start, End time.Time }
type Item struct {
	ID, Marker string
	When       time.Time
}
type Progress struct {
	Source                               string
	Lookback                             time.Duration
	Overlap                              time.Duration
	Start, End, LastCompleted, LastSweep time.Time
	Windows                              []Window
	Pending                              []Item
	Page                                 *Window
	Block                                string
}
type envelope struct {
	Version int
	Hash    string
	Data    json.RawMessage
}
type Store struct{ db *bolt.DB }

func Open(dir string) (*Store, error) {
	if !filepath.IsAbs(dir) {
		return nil, errors.New("history state needs absolute directory")
	}
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, errors.New("history directory unavailable")
	}
	path := filepath.Join(dir, "history-backfill.db")
	info, statErr := os.Lstat(path)
	if statErr != nil && !os.IsNotExist(statErr) {
		return nil, ErrCorrupt
	}
	existed := statErr == nil
	if existed && (!info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
		return nil, ErrCorrupt
	}
	db, e := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second})
	if e != nil {
		return nil, errors.New("history state unavailable")
	}
	if e = db.Update(func(tx *bolt.Tx) error {
		if existed && tx.Bucket(bucket) == nil {
			return ErrCorrupt
		}
		if err := tx.ForEach(func(name []byte, _ *bolt.Bucket) error {
			if !bytes.Equal(name, bucket) {
				return ErrCorrupt
			}
			return nil
		}); err != nil {
			return err
		}
		_, e := tx.CreateBucketIfNotExists(bucket)
		return e
	}); e != nil {
		db.Close()
		return nil, e
	}
	return &Store{db: db}, nil
}
func (s *Store) Close() error { return s.db.Close() }
func keyOK(k string) bool     { return k == "queries" || k == "serviceability_logs" }
func read(b *bolt.Bucket, key, source string) (Progress, error) {
	p := Progress{Source: source}
	if b == nil {
		return p, ErrCorrupt
	}
	v := b.Get([]byte(key))
	if v == nil {
		return p, nil
	}
	if len(v) > 32<<20 {
		return p, ErrCorrupt
	}
	var e envelope
	if json.Unmarshal(v, &e) != nil || e.Version != 1 {
		return p, ErrCorrupt
	}
	hash := sha256.Sum256(e.Data)
	if hex.EncodeToString(hash[:]) != e.Hash || json.Unmarshal(e.Data, &p) != nil || p.Source != source {
		return p, ErrCorrupt
	}
	canonical, err := json.Marshal(p)
	if err != nil || !bytes.Equal(canonical, e.Data) {
		return p, ErrCorrupt
	}
	if validate(p, 100000) != nil {
		return p, ErrCorrupt
	}
	return p, nil
}
func validate(p Progress, max int) error {
	if len(p.Windows) > 4096 || len(p.Pending) > max {
		return ErrFull
	}
	for _, w := range p.Windows {
		if w.Start.IsZero() || !w.End.After(w.Start) {
			return ErrCorrupt
		}
	}
	if p.Page != nil && (p.Page.Start.IsZero() || !p.Page.End.After(p.Page.Start)) {
		return ErrCorrupt
	}
	seen := map[string]bool{}
	for _, i := range p.Pending {
		if i.ID == "" || len(i.ID) > 1024 || len(i.Marker) > 128 || i.When.IsZero() || seen[i.ID] {
			return ErrCorrupt
		}
		seen[i.ID] = true
	}
	return nil
}
func (s *Store) Load(key, source string) (p Progress, e error) {
	if !keyOK(key) || source == "" {
		return p, ErrCorrupt
	}
	e = s.db.View(func(tx *bolt.Tx) error { var e error; p, e = read(tx.Bucket(bucket), key, source); return e })
	return
}
func (s *Store) Update(key, source string, max int, change func(*Progress) error) error {
	if !keyOK(key) || source == "" || max < 1 {
		return ErrCorrupt
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		p, e := read(b, key, source)
		if e != nil {
			return e
		}
		if e = change(&p); e != nil {
			return e
		}
		if p.Source != source {
			return ErrCorrupt
		}
		if e = validate(p, max); e != nil {
			return e
		}
		data, e := json.Marshal(p)
		if e != nil {
			return ErrCorrupt
		}
		h := sha256.Sum256(data)
		v, e := json.Marshal(envelope{1, hex.EncodeToString(h[:]), data})
		if e != nil {
			return e
		}
		total := len(v)
		if e = b.ForEach(func(k, x []byte) error {
			if !bytes.Equal(k, []byte(key)) {
				total += len(x)
			}
			return nil
		}); e != nil {
			return e
		}
		if total > 32<<20 {
			return ErrFull
		}
		return b.Put([]byte(key), v)
	})
}
