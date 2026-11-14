package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTorrentPeersUpdatedAt(t *testing.T) {
	t.Parallel()

	t1 := time.Date(2026, 2, 6, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

	assert.Nil(t, Torrent{}.PeersUpdatedAt())

	got := Torrent{Sources: []TorrentsTorrentSource{
		{Source: "dht", Seeders: NewNullUint(3), UpdatedAt: t1},
		{Source: "import", Seeders: NewNullUint(9), UpdatedAt: t2},
		// newest, but carries no seeder count -> ignored
		{Source: "other", UpdatedAt: t3},
	}}.PeersUpdatedAt()
	require.NotNil(t, got)
	assert.Equal(t, t2, *got)
}
