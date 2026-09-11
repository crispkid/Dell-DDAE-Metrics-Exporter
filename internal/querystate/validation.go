package querystate

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/queryclient"
	bolt "go.etcd.io/bbolt"
)

var errCorrupt = errors.New("query persistent state is corrupt")

const maxEventBytes = 64 << 10
const maxDurationSeconds = float64(9007199254740991) / 1000

// Check duplicate keys before typed decoding: encoding/json otherwise silently
// accepts the last value. Depth and caller byte limits bound validation work.
func scanJSON(d *json.Decoder, depth int) error {
	if depth > 16 {
		return errCorrupt
	}
	token, err := d.Token()
	if err != nil {
		return errCorrupt
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return errCorrupt
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return errCorrupt
			}
			seen[name] = true
			if err := scanJSON(d, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := scanJSON(d, depth+1); err != nil {
				return err
			}
		}
	default:
		return errCorrupt
	}
	end, err := d.Token()
	if err != nil || delim == '{' && end != json.Delim('}') || delim == '[' && end != json.Delim(']') {
		return errCorrupt
	}
	return nil
}

func strictObject(data []byte, limit int, out any, required, optional string) (map[string]json.RawMessage, error) {
	if len(data) == 0 || len(data) > limit || !utf8.Valid(data) {
		return nil, errCorrupt
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if scanJSON(d, 0) != nil {
		return nil, errCorrupt
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, errCorrupt
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil || fields == nil {
		return nil, errCorrupt
	}
	allowed := map[string]bool{}
	for _, name := range strings.Fields(required) {
		value, ok := fields[name]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, errCorrupt
		}
		allowed[name] = true
	}
	for _, name := range strings.Fields(optional) {
		allowed[name] = true
	}
	for name := range fields {
		if !allowed[name] {
			return nil, errCorrupt
		}
	}
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		return nil, errCorrupt
	}
	return fields, nil
}

func validText(value string, required bool) bool {
	return (!required || value != "") && len(value) <= 1024 && utf8.ValidString(value) && !strings.ContainsAny(value, "\x00\r\n")
}

func validateEvent(e queryclient.Event, source string) error {
	if e.SourceInstance != source || !queryclient.ValidID(e.QueryID) || !validText(e.User, true) || !validText(e.Source, false) || e.State == "" || queryclient.State(e.State) != e.State || e.Submitted.IsZero() || e.Observed.IsZero() {
		return errCorrupt
	}
	if e.Completed != nil && (e.Completed.IsZero() || e.Completed.Before(e.Submitted)) {
		return errCorrupt
	}
	if queryclient.Terminal(e.State) && (e.Completed == nil || e.Elapsed == nil) {
		return errCorrupt
	}
	for _, value := range []*float64{e.Elapsed, e.Queued, e.Execution, e.CPU} {
		if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 || *value > maxDurationSeconds) {
			return errCorrupt
		}
	}
	return nil
}

func decodeEvent(data []byte, source string) (queryclient.Event, error) {
	var e queryclient.Event
	_, err := strictObject(data, maxEventBytes, &e, "source_instance query_id user state submitted_at observed_at", "source completed_at elapsed_seconds queued_seconds execution_seconds cpu_seconds")
	if err != nil {
		return e, err
	}
	return e, validateEvent(e, source)
}

func eventHash(e queryclient.Event) ([32]byte, error) {
	e.Observed = time.Time{}
	data, err := json.Marshal(e)
	return sha256.Sum256(data), err
}

func decodeCheckpoint(key, data []byte) (checkpoint, error) {
	var cp checkpoint
	fields, err := strictObject(data, 2048, &cp, "Final Hash LastFetched Time", "")
	if err != nil || !queryclient.ValidID(string(key)) || cp.LastFetched.IsZero() || cp.Time.IsZero() || cp.Hash == [32]byte{} {
		return cp, errCorrupt
	}
	// A fixed Go array decoder accepts too few/many entries; require all 32 bytes.
	var hash []json.RawMessage
	if json.Unmarshal(fields["Hash"], &hash) != nil || len(hash) != 32 {
		return cp, errCorrupt
	}
	for _, value := range hash {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return cp, errCorrupt
		}
	}
	return cp, nil
}

func decodeStats(data []byte) (Stats, error) {
	var a Stats
	fields, err := strictObject(data, maxEventBytes, &a, "Revision Completed Histograms Pending Bytes Floor", "")
	if err != nil || a.Revision == 0 || a.Completed == nil || a.Histograms == nil || a.Pending < 0 || a.Bytes < 0 {
		return a, errCorrupt
	}
	for state := range a.Completed {
		if !queryclient.Terminal(state) {
			return a, errCorrupt
		}
	}
	var counters map[string]json.RawMessage
	if json.Unmarshal(fields["Completed"], &counters) != nil {
		return a, errCorrupt
	}
	for _, value := range counters {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return a, errCorrupt
		}
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(fields["Histograms"], &raw) != nil {
		return a, errCorrupt
	}
	for key, h := range a.Histograms {
		name, state, ok := strings.Cut(key, ":")
		if !ok || (name != "elapsed" && name != "execution" && name != "queued") || !queryclient.Terminal(state) {
			return a, errCorrupt
		}
		histogramFields, err := strictObject(raw[key], 2048, &h, "Count Sum Buckets", "")
		if err != nil {
			return a, err
		}
		if len(h.Buckets) != len(Bounds) || math.IsNaN(h.Sum) || math.IsInf(h.Sum, 0) || h.Sum < 0 || h.Count == 0 && h.Sum != 0 || h.Count > a.Completed[state] {
			return a, errCorrupt
		}
		var buckets []json.RawMessage
		if json.Unmarshal(histogramFields["Buckets"], &buckets) != nil {
			return a, errCorrupt
		}
		for _, value := range buckets {
			if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return a, errCorrupt
			}
		}
		var previous uint64
		for _, count := range h.Buckets {
			if count < previous || count > h.Count {
				return a, errCorrupt
			}
			previous = count
		}
	}
	return a, nil
}

func (s *Store) validateRecord(tx *bolt.Tx, key, payload []byte, floor time.Time) (queryclient.Event, error) {
	if len(key) != 8 || binary.BigEndian.Uint64(key) == 0 || binary.BigEndian.Uint64(key) > tx.Bucket(events).Sequence() {
		return queryclient.Event{}, errCorrupt
	}
	e, err := decodeEvent(payload, s.options.Source)
	if err != nil {
		return e, err
	}
	// Retention may legitimately remove checkpoints while retaining older pending
	// events. Only a terminal record within the stored floor has a locked match.
	if queryclient.Terminal(e.State) && !e.Completed.Before(floor) {
		cp, err := decodeCheckpoint([]byte(e.QueryID), tx.Bucket(checkpoints).Get([]byte(e.QueryID)))
		if err != nil {
			return e, err
		}
		hash, err := eventHash(e)
		if err != nil || !cp.Final || cp.Hash != hash || !cp.Time.Equal(*e.Completed) || !cp.LastFetched.Equal(e.Observed) {
			return e, errCorrupt
		}
	}
	return e, nil
}

func (s *Store) validate(tx *bolt.Tx) error {
	for _, name := range [][]byte{meta, checkpoints, events} {
		if tx.Bucket(name) == nil {
			return errCorrupt
		}
	}
	m := tx.Bucket(meta)
	if string(m.Get([]byte("version"))) != "1" || string(m.Get([]byte("source"))) != s.options.Source {
		return errors.New("query state schema or source mismatch")
	}
	a, err := readStats(tx)
	if err != nil {
		return err
	}
	if err := tx.Bucket(checkpoints).ForEach(func(k, v []byte) error { _, err := decodeCheckpoint(k, v); return err }); err != nil {
		return err
	}
	count, size := 0, int64(0)
	if err := tx.Bucket(events).ForEach(func(k, v []byte) error {
		if _, err := s.validateRecord(tx, k, v, a.Floor); err != nil {
			return err
		}
		if size > math.MaxInt64-int64(len(v)) || count == math.MaxInt {
			return errCorrupt
		}
		count++
		size += int64(len(v))
		return nil
	}); err != nil {
		return err
	}
	if count != a.Pending || size != a.Bytes {
		return errCorrupt
	}
	return nil
}
