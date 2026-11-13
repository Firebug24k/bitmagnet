package gqlmodel

import (
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql"
	q "github.com/bitmagnet-io/bitmagnet/internal/database/query"
	"github.com/bitmagnet-io/bitmagnet/internal/gql/gqlmodel/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

func rawOf(t *testing.T, c q.Criteria) q.RawCriteria {
	t.Helper()

	rc, ok := c.(q.RawCriteria)
	require.True(t, ok, "expected RawCriteria")

	return rc
}

func TestTorrentContentFilterCriteria_Empty(t *testing.T) {
	t.Parallel()

	c, err := torrentContentFilterCriteria(gen.TorrentContentFiltersInput{}, time.Now())
	require.NoError(t, err)
	assert.Empty(t, c)
}

func TestTorrentContentFilterCriteria_SeenWithinAndSeeders(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	c, err := torrentContentFilterCriteria(gen.TorrentContentFiltersInput{
		PublishedWithinDays: graphql.OmittableOf(ptr(7)),
		Seeders: graphql.OmittableOf(ptr(gen.IntRangeInput{
			Gte: graphql.OmittableOf(ptr(10)),
		})),
		Size: graphql.OmittableOf(ptr(gen.IntRangeInput{
			Gte: graphql.OmittableOf(ptr(1 << 30)),
			Lte: graphql.OmittableOf(ptr(5 << 30)),
		})),
	}, now)
	require.NoError(t, err)
	require.Len(t, c, 3)

	seen := rawOf(t, c[0])
	assert.Equal(t, "torrent_contents.published_at >= ?", seen.Query)
	assert.Equal(t, []interface{}{now.Add(-7 * 24 * time.Hour)}, seen.Args)

	seeders := rawOf(t, c[1])
	assert.Equal(t, "torrent_contents.seeders >= ?", seeders.Query)
	assert.Equal(t, []interface{}{int64(10)}, seeders.Args)

	size := rawOf(t, c[2])
	assert.Equal(t, "torrent_contents.size >= ? AND torrent_contents.size <= ?", size.Query)
	assert.Equal(t, []interface{}{int64(1 << 30), int64(5 << 30)}, size.Args)
}

func TestTorrentContentFilterCriteria_NullBoundsIgnored(t *testing.T) {
	t.Parallel()

	c, err := torrentContentFilterCriteria(gen.TorrentContentFiltersInput{
		Seeders:             graphql.OmittableOf[*gen.IntRangeInput](nil),
		Leechers:            graphql.OmittableOf(ptr(gen.IntRangeInput{})),
		PublishedWithinDays: graphql.OmittableOf(ptr(0)),
	}, time.Now())
	require.NoError(t, err)
	assert.Empty(t, c)
}

func TestTorrentContentFilterCriteria_NegativeRejected(t *testing.T) {
	t.Parallel()

	_, err := torrentContentFilterCriteria(gen.TorrentContentFiltersInput{
		Seeders: graphql.OmittableOf(ptr(gen.IntRangeInput{Gte: graphql.OmittableOf(ptr(-1))})),
	}, time.Now())
	require.Error(t, err)
}
