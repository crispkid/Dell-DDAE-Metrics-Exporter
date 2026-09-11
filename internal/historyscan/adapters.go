package historyscan

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/ddae"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/historystate"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/logstate"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/queryclient"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/querystate"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/serviceability"
	"time"
)

// Identity pins progress to the source and API contract without persisting credentials.
func Identity(source, origin, path string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(source+"\x00"+origin+"\x00"+path+"\x00v1")))
}

type logAPI interface {
	ServiceabilityLogWindow(context.Context, time.Time, time.Time) (ddae.ServiceabilityLogList, error)
	ServiceabilityLogDetail(context.Context, string) (ddae.ServiceabilityLogDetail, error)
	CloseIdleConnections()
}
type logSource struct {
	client logAPI
	store  *logstate.Store
	source string
}

func NewLogSource(cfg config.Config, store *logstate.Store) (Source, error) {
	c, err := ddae.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &logSource{c, store, cfg.SourceInstance}, nil
}
func (s *logSource) Close() { s.client.CloseIdleConnections() }
func logPage(list ddae.ServiceabilityLogList) (Page, error) {
	p := Page{}
	if list.Malformed || list.TotalRecords == nil || *list.TotalRecords < 0 || list.Threshold == nil || *list.Threshold <= 0 || int64(len(list.Results)) > *list.Threshold || int64(len(list.Results)) > *list.TotalRecords {
		return p, errors.New("history log list metadata invalid")
	}
	seen := map[string]bool{}
	for _, i := range list.Results {
		if ddae.ValidateServiceabilityLogID(i.ID) != nil || seen[i.ID] || i.UpdatedOn == nil {
			return Page{}, errors.New("history log identity invalid")
		}
		at, err := time.Parse(time.RFC3339Nano, *i.UpdatedOn)
		if err != nil {
			return Page{}, errors.New("history log timestamp invalid")
		}
		seen[i.ID] = true
		p.Items = append(p.Items, historystate.Item{ID: i.ID, Marker: at.UTC().Format(time.RFC3339Nano), When: at})
	}
	p.Complete = int64(len(p.Items)) == *list.TotalRecords && int64(len(p.Items)) < *list.Threshold
	return p, nil
}
func (s *logSource) List(ctx context.Context, w historystate.Window) (Page, error) {
	l, err := s.client.ServiceabilityLogWindow(ctx, w.Start, w.End)
	if err != nil {
		return Page{}, err
	}
	return logPage(l)
}
func (s *logSource) Record(ctx context.Context, i historystate.Item) error {
	d, err := s.client.ServiceabilityLogDetail(ctx, i.ID)
	if err != nil {
		return err
	}
	if d.UpdatedOn == nil {
		return errors.New("history log detail timestamp missing")
	}
	at, err := time.Parse(time.RFC3339Nano, *d.UpdatedOn)
	if err != nil || at.Before(i.When) {
		return errors.New("history log detail predates list")
	}
	now := time.Now().UTC()
	event, err := serviceability.BuildEvent(s.source, i.ID, d, now)
	if err != nil {
		return err
	}
	_, err = s.store.Enqueue(event, at.UTC().Format(time.RFC3339Nano), now)
	var full logstate.FullError
	if errors.As(err, &full) {
		return historystate.ErrFull
	}
	if err != nil {
		return errors.Join(historystate.ErrCorrupt, err)
	}
	return nil
}

type queryAPI interface {
	Scope(context.Context) error
	HistoryWindow(context.Context, time.Time, time.Time) ([]byte, error)
	Get(context.Context, string) ([]byte, error)
	CloseIdleConnections()
}
type querySource struct {
	client queryAPI
	store  *querystate.Store
	source string
	max    int
}

func NewQuerySource(cfg config.Config, store *querystate.Store) (Source, error) {
	c, err := queryclient.New(cfg.Query, cfg.AllowInsecureTLS)
	if err != nil {
		return nil, err
	}
	return &querySource{c, store, cfg.SourceInstance, cfg.Query.MaxHistory}, nil
}
func (s *querySource) Close() { s.client.CloseIdleConnections() }
func queryPage(body []byte, max int) (Page, error) {
	items, err := queryclient.DecodeHistory(body, max)
	if err != nil {
		return Page{}, err
	}
	p := Page{Complete: len(items) < 1000}
	for _, i := range items {
		p.Items = append(p.Items, historystate.Item{ID: i.ID, When: i.Created})
	}
	return p, nil
}
func (s *querySource) List(ctx context.Context, w historystate.Window) (Page, error) {
	if err := s.client.Scope(ctx); err != nil {
		return Page{}, err
	}
	body, err := s.client.HistoryWindow(ctx, w.Start, w.End)
	if err != nil {
		return Page{}, err
	}
	return queryPage(body, s.max)
}
func (s *querySource) Record(ctx context.Context, i historystate.Item) error {
	e, err := queryclient.FetchDetail(ctx, s.client, i.ID, s.source)
	if err != nil {
		return err
	}
	err = s.store.Record(e)
	if errors.Is(err, querystate.ErrExpired) {
		return ErrExpired
	}
	if errors.Is(err, querystate.ErrFull) {
		return historystate.ErrFull
	}
	if err != nil {
		return errors.Join(historystate.ErrCorrupt, err)
	}
	return nil
}
