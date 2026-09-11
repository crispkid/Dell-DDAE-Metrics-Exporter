package queryclient

import (
	"context"
	"time"
)

type detailGetter interface {
	Get(context.Context, string) ([]byte, error)
}

// FetchDetail keeps request ordering separate from response-time validation.
// Live polling and history backfill must use the same observation boundary.
func FetchDetail(ctx context.Context, client detailGetter, id, source string) (Event, error) {
	return fetchDetail(ctx, client, id, source, time.Now)
}

func fetchDetail(ctx context.Context, client detailGetter, id, source string, now func() time.Time) (Event, error) {
	if !ValidID(id) {
		return Event{}, ErrSchema
	}
	observed := now().UTC()
	body, err := client.Get(ctx, "/ui/api/insights/history/queries/"+id)
	if err != nil {
		return Event{}, err
	}
	return decodeDetail(body, id, source, observed, now().UTC())
}
