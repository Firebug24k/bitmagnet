package gqlmodel

import (
	"errors"
	"time"

	"github.com/99designs/gqlgen/graphql"
	q "github.com/bitmagnet-io/bitmagnet/internal/database/query"
	"github.com/bitmagnet-io/bitmagnet/internal/database/search"
	"github.com/bitmagnet-io/bitmagnet/internal/gql/gqlmodel/gen"
)

// maxPublishedWithinDays caps the relative first-seen window (~100 years).
const maxPublishedWithinDays = 36500

var errInvalidRange = errors.New("invalid filter range: values must be non-negative")

func omittableValue[T any](o graphql.Omittable[*T]) *T {
	if v, ok := o.ValueOK(); ok && v != nil {
		return v
	}

	return nil
}

func intRangeCriteria(
	column search.TorrentContentRangeColumn,
	r graphql.Omittable[*gen.IntRangeInput],
) (q.Criteria, error) {
	rng := omittableValue(r)
	if rng == nil {
		return nil, nil
	}

	var gte, lte *int64

	if v := omittableValue(rng.Gte); v != nil {
		if *v < 0 {
			return nil, errInvalidRange
		}

		x := int64(*v)
		gte = &x
	}

	if v := omittableValue(rng.Lte); v != nil {
		if *v < 0 {
			return nil, errInvalidRange
		}

		x := int64(*v)
		lte = &x
	}

	return search.TorrentContentRangeCriteria(column, gte, lte), nil
}

func torrentContentFilterCriteria(input gen.TorrentContentFiltersInput, now time.Time) ([]q.Criteria, error) {
	var criteria []q.Criteria

	add := func(c q.Criteria) {
		if c != nil {
			criteria = append(criteria, c)
		}
	}

	if rng := omittableValue(input.PublishedAt); rng != nil {
		add(search.TorrentContentRangeCriteria(
			search.TorrentContentRangePublishedAt,
			omittableValue(rng.Gte),
			omittableValue(rng.Lte),
		))
	}

	if days := omittableValue(input.PublishedWithinDays); days != nil && *days > 0 {
		d := min(*days, maxPublishedWithinDays)
		since := now.Add(-time.Duration(d) * 24 * time.Hour)
		add(search.TorrentContentRangeCriteria(search.TorrentContentRangePublishedAt, &since, nil))
	}

	for _, r := range []struct {
		column search.TorrentContentRangeColumn
		value  graphql.Omittable[*gen.IntRangeInput]
	}{
		{search.TorrentContentRangeSeeders, input.Seeders},
		{search.TorrentContentRangeLeechers, input.Leechers},
		{search.TorrentContentRangeSize, input.Size},
		{search.TorrentContentRangeFilesCount, input.FilesCount},
	} {
		c, err := intRangeCriteria(r.column, r.value)
		if err != nil {
			return nil, err
		}

		add(c)
	}

	return criteria, nil
}
