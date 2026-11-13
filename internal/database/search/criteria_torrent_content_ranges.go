package search

import (
	"fmt"
	"time"

	"github.com/bitmagnet-io/bitmagnet/internal/database/query"
	"github.com/bitmagnet-io/bitmagnet/internal/maps"
	"github.com/bitmagnet-io/bitmagnet/internal/model"
)

// TorrentContentRangeColumn is a numeric/time column of torrent_contents that supports range filtering.
type TorrentContentRangeColumn string

const (
	TorrentContentRangePublishedAt TorrentContentRangeColumn = "published_at"
	TorrentContentRangeSeeders     TorrentContentRangeColumn = "seeders"
	TorrentContentRangeLeechers    TorrentContentRangeColumn = "leechers"
	TorrentContentRangeSize        TorrentContentRangeColumn = "size"
	TorrentContentRangeFilesCount  TorrentContentRangeColumn = "files_count"
)

func (c TorrentContentRangeColumn) valid() bool {
	switch c {
	case TorrentContentRangePublishedAt, TorrentContentRangeSeeders, TorrentContentRangeLeechers,
		TorrentContentRangeSize, TorrentContentRangeFilesCount:
		return true
	}

	return false
}

// TorrentContentRangeCriteria constrains a torrent_contents column to an inclusive range.
// Either bound may be nil. Rows where the column is NULL never match a bounded range.
func TorrentContentRangeCriteria[T int64 | time.Time](
	column TorrentContentRangeColumn,
	gte *T,
	lte *T,
) query.Criteria {
	if !column.valid() || (gte == nil && lte == nil) {
		return nil
	}

	// column is from a closed whitelist above, so interpolation is safe; values are bound parameters.
	col := fmt.Sprintf("%s.%s", model.TableNameTorrentContent, column)
	sql := ""
	args := make([]interface{}, 0, 2)

	if gte != nil {
		sql = col + " >= ?"
		args = append(args, *gte)
	}

	if lte != nil {
		if sql != "" {
			sql += " AND "
		}

		sql += col + " <= ?"
		args = append(args, *lte)
	}

	return query.RawCriteria{
		Query: sql,
		Args:  args,
		Joins: maps.NewInsertMap[string, struct{}](),
	}
}
