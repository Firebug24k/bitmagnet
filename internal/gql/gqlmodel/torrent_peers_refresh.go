package gqlmodel

import (
	"github.com/bitmagnet-io/bitmagnet/internal/model"
	"github.com/bitmagnet-io/bitmagnet/internal/peerrefresh"
	"github.com/bitmagnet-io/bitmagnet/internal/protocol"
)

type TorrentPeersRefreshResult struct {
	InfoHash       protocol.ID
	Ok             bool
	Error          *string
	NodesQueried   int
	NodesResponded int
	NodesWithPeers int
	SeedersBefore  model.NullUint
	LeechersBefore model.NullUint
	Seeders        model.NullUint
	Leechers       model.NullUint
}

func NewTorrentPeersRefreshResults(results []peerrefresh.Result) []TorrentPeersRefreshResult {
	out := make([]TorrentPeersRefreshResult, 0, len(results))

	for _, r := range results {
		item := TorrentPeersRefreshResult{
			InfoHash:       r.InfoHash,
			Ok:             r.Err == nil,
			NodesQueried:   r.NodesQueried,
			NodesResponded: r.NodesResponded,
			NodesWithPeers: r.NodesWithPeers,
			SeedersBefore:  r.SeedersBefore,
			LeechersBefore: r.LeechersBefore,
			Seeders:        r.Seeders,
			Leechers:       r.Leechers,
		}

		if r.Err != nil {
			msg := r.Err.Error()
			item.Error = &msg
		}

		out = append(out, item)
	}

	return out
}
