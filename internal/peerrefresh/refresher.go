// Package peerrefresh performs on-demand seeder/leecher refreshes for specific torrents.
//
// For each info hash it runs a short iterative DHT lookup towards the hash (BEP 5 get_peers with
// BEP 33 scrape=1), starting from the closest nodes in the crawler's routing table. The seeders/peers
// bloom filters returned by every responding node are unioned, which gives a better estimate than the
// crawler's single-node scrape, and the result is written to both torrents_torrent_sources (dht) and the
// denormalised torrent_contents seeders/leechers columns that search, sorting and filtering read.
package peerrefresh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"sync"
	"time"

	"github.com/bitmagnet-io/bitmagnet/internal/database/dao"
	"github.com/bitmagnet-io/bitmagnet/internal/lazy"
	"github.com/bitmagnet-io/bitmagnet/internal/model"
	"github.com/bitmagnet-io/bitmagnet/internal/protocol"
	"github.com/bitmagnet-io/bitmagnet/internal/protocol/dht/client"
	"github.com/bitmagnet-io/bitmagnet/internal/protocol/dht/ktable"
	"github.com/bits-and-blooms/bloom/v3"
	"gorm.io/gorm"
)

type (
	netipAddrPort = netip.AddrPort
	gormDB        = gorm.DB
)

const (
	// MaxInfoHashes caps one refresh request.
	MaxInfoHashes = 200
	// alpha is the number of nodes queried in parallel per lookup round.
	alpha = 8
	// maxRounds bounds the iterative lookup depth.
	maxRounds = 4
	// maxQueriesPerHash bounds total DHT queries per info hash.
	maxQueriesPerHash = 48
	// hashConcurrency is how many info hashes are looked up at once.
	hashConcurrency = 8
	// overallTimeout bounds one refresh request.
	overallTimeout = 90 * time.Second
)

var ErrTooMany = fmt.Errorf("at most %d torrents can be refreshed at once", MaxInfoHashes)

type Result struct {
	InfoHash       protocol.ID
	NodesQueried   int
	NodesResponded int
	// NodesWithPeers counts responding nodes that returned BEP 33 scrape data for the hash.
	NodesWithPeers int
	SeedersBefore  model.NullUint
	LeechersBefore model.NullUint
	Seeders        model.NullUint
	Leechers       model.NullUint
	Err            error
}

type Refresher interface {
	Refresh(ctx context.Context, infoHashes []protocol.ID) ([]Result, error)
}

func New(kTable ktable.Table, dhtClient lazy.Lazy[client.Client], d *dao.Query) Refresher {
	return &refresher{kTable: kTable, client: dhtClient, dao: d}
}

type refresher struct {
	kTable ktable.Table
	client lazy.Lazy[client.Client]
	dao    *dao.Query
}

// minRespondedForEmpty is how many nodes must answer a converged lookup before "no node has scrape
// data" is trusted as "no peers" (0/0) rather than reported as an inconclusive lookup.
const minRespondedForEmpty = 8

// scraper is the subset of the DHT client used here (eases testing).
type scraper interface {
	GetPeersScrapeLookup(ctx context.Context, addr netipAddrPort, infoHash protocol.ID) (client.GetPeersScrapeLookupResult, error)
}

func (r *refresher) Refresh(ctx context.Context, infoHashes []protocol.ID) ([]Result, error) {
	infoHashes = dedupe(infoHashes)
	if len(infoHashes) == 0 {
		return nil, nil
	}

	if len(infoHashes) > MaxInfoHashes {
		return nil, ErrTooMany
	}

	cl, err := r.client.Get()
	if err != nil {
		return nil, fmt.Errorf("DHT client unavailable: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, overallTimeout)
	defer cancel()

	before, err := r.currentCounts(ctx, infoHashes)
	if err != nil {
		return nil, err
	}

	results := make([]Result, len(infoHashes))
	sem := make(chan struct{}, hashConcurrency)

	var wg sync.WaitGroup

	for i, h := range infoHashes {
		wg.Add(1)

		go func(i int, h protocol.ID) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			seeds := make([]candidate, 0, 8)
			for _, n := range r.kTable.GetClosestNodes(h) {
				seeds = append(seeds, candidate{id: n.ID(), addr: n.Addr()})
			}

			res := lookup(ctx, cl, h, seeds, func(id protocol.ID, addr netip.AddrPort) {
				r.kTable.BatchCommand(ktable.PutNode{
					ID:      id,
					Addr:    addr,
					Options: []ktable.NodeOption{ktable.NodeResponded()},
				})
			})
			if b, ok := before[h]; ok {
				res.SeedersBefore, res.LeechersBefore = b[0], b[1]
			}

			results[i] = res
		}(i, h)
	}

	wg.Wait()

	if err := r.persist(ctx, results); err != nil {
		return results, err
	}

	return results, nil
}

type candidate struct {
	id   protocol.ID
	addr netipAddrPort
}

// lookup runs a bounded iterative get_peers+scrape lookup towards h and unions the bloom filters.
func lookup(
	ctx context.Context,
	s scraper,
	h protocol.ID,
	seeds []candidate,
	onResponded func(id protocol.ID, addr netip.AddrPort),
) Result {
	res := Result{InfoHash: h}

	known := map[netipAddrPort]candidate{}
	for _, c := range seeds {
		known[c.addr] = c
	}

	if len(known) == 0 {
		res.Err = errors.New("no DHT nodes known yet (is the crawler running?)")
		return res
	}

	queried := map[netipAddrPort]struct{}{}

	var (
		bfSeeders, bfPeers *bloom.BloomFilter
		mu                 sync.Mutex
	)

	for round := 0; round < maxRounds && res.NodesQueried < maxQueriesPerHash; round++ {
		if ctx.Err() != nil {
			break
		}

		batch := closestUnqueried(known, queried, h, min(alpha, maxQueriesPerHash-res.NodesQueried))
		if len(batch) == 0 {
			break
		}

		var wg sync.WaitGroup

		discovered := make([]candidate, 0)

		for _, c := range batch {
			queried[c.addr] = struct{}{}
			res.NodesQueried++

			wg.Add(1)

			go func(c candidate) {
				defer wg.Done()

				sr, err := s.GetPeersScrapeLookup(ctx, c.addr, h)
				if err != nil {
					return
				}

				if onResponded != nil {
					onResponded(sr.ID, c.addr)
				}

				mu.Lock()
				defer mu.Unlock()

				res.NodesResponded++

				if sr.BfSeeders != nil && sr.BfPeers != nil {
					res.NodesWithPeers++
					bfSeeders = union(bfSeeders, sr.BfSeeders)
					bfPeers = union(bfPeers, sr.BfPeers)
				}

				for _, n := range sr.Nodes {
					discovered = append(discovered, candidate{id: n.ID, addr: n.Addr})
				}
			}(c)
		}

		wg.Wait()

		for _, d := range discovered {
			if _, ok := known[d.addr]; !ok {
				known[d.addr] = d
			}
		}
	}

	switch {
	case res.NodesResponded == 0:
		res.Err = fmt.Errorf("none of the %d DHT nodes queried responded", res.NodesQueried)
	case res.NodesWithPeers > 0:
		res.Seeders = model.NewNullUint(uint(bfSeeders.ApproximatedSize()))
		res.Leechers = model.NewNullUint(uint(bfPeers.ApproximatedSize()))
	case res.NodesResponded >= minRespondedForEmpty:
		// the lookup reached the hash's neighbourhood and nobody is tracking peers: it's dead
		res.Seeders = model.NewNullUint(0)
		res.Leechers = model.NewNullUint(0)
	default:
		res.Err = fmt.Errorf("inconclusive: only %d of %d DHT nodes responded and none had peers",
			res.NodesResponded, res.NodesQueried)
	}

	return res
}

func union(acc *bloom.BloomFilter, f *bloom.BloomFilter) *bloom.BloomFilter {
	if acc == nil {
		return f.Copy()
	}

	if err := acc.Merge(f); err != nil {
		// incompatible filter (should not happen: all BEP 33 filters share m/k); ignore it
		return acc
	}

	return acc
}

func closestUnqueried(
	known map[netipAddrPort]candidate,
	queried map[netipAddrPort]struct{},
	target protocol.ID,
	n int,
) []candidate {
	out := make([]candidate, 0, len(known))

	for addr, c := range known {
		if _, ok := queried[addr]; !ok {
			out = append(out, c)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return lessDistance(out[i].id, out[j].id, target)
	})

	if len(out) > n {
		out = out[:n]
	}

	return out
}

// lessDistance reports whether a is XOR-closer to target than b.
func lessDistance(a, b, target protocol.ID) bool {
	var da, db [20]byte
	for i := range target {
		da[i] = a[i] ^ target[i]
		db[i] = b[i] ^ target[i]
	}

	return bytes.Compare(da[:], db[:]) < 0
}

func dedupe(hashes []protocol.ID) []protocol.ID {
	seen := make(map[protocol.ID]struct{}, len(hashes))
	out := make([]protocol.ID, 0, len(hashes))

	for _, h := range hashes {
		if _, ok := seen[h]; !ok {
			seen[h] = struct{}{}
			out = append(out, h)
		}
	}

	return out
}

// currentCounts returns the seeders/leechers currently shown in search (torrent_contents).
func (r *refresher) currentCounts(ctx context.Context, hashes []protocol.ID) (map[protocol.ID][2]model.NullUint, error) {
	var rows []struct {
		InfoHash protocol.ID
		Seeders  model.NullUint
		Leechers model.NullUint
	}

	if err := r.dao.TorrentContent.UnderlyingDB().WithContext(ctx).Raw(
		`SELECT info_hash, max(seeders) AS seeders, max(leechers) AS leechers
		   FROM torrent_contents WHERE info_hash IN ? GROUP BY info_hash`,
		hashes,
	).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("reading current counts: %w", err)
	}

	out := make(map[protocol.ID][2]model.NullUint, len(rows))
	for _, row := range rows {
		out[row.InfoHash] = [2]model.NullUint{row.Seeders, row.Leechers}
	}

	return out, nil
}

// persist writes successful results to the dht source row and re-derives torrent_contents counts
// (max over sources, matching model.Torrent.Seeders/Leechers used by the processor).
func (r *refresher) persist(ctx context.Context, results []Result) error {
	ok := make([]Result, 0, len(results))
	for _, res := range results {
		if res.Err == nil {
			ok = append(ok, res)
		}
	}

	if len(ok) == 0 {
		return nil
	}

	db := r.dao.TorrentContent.UnderlyingDB().WithContext(ctx)

	return db.Transaction(func(tx *gormDB) error {
		hashes := make([]protocol.ID, 0, len(ok))

		for _, res := range ok {
			hashes = append(hashes, res.InfoHash)

			if err := tx.Exec(
				`INSERT INTO torrents_torrent_sources (source, info_hash, seeders, leechers, created_at, updated_at)
				 SELECT 'dht', ?, ?, ?, now(), now()
				  WHERE EXISTS (SELECT 1 FROM torrents WHERE info_hash = ?)
				 ON CONFLICT (source, info_hash) DO UPDATE
				   SET seeders = EXCLUDED.seeders, leechers = EXCLUDED.leechers, updated_at = now()`,
				res.InfoHash, res.Seeders, res.Leechers, res.InfoHash,
			).Error; err != nil {
				return fmt.Errorf("saving source counts: %w", err)
			}
		}

		if err := tx.Exec(
			`UPDATE torrent_contents tc
			    SET seeders = agg.seeders, leechers = agg.leechers
			   FROM (SELECT info_hash, max(seeders) AS seeders, max(leechers) AS leechers
			           FROM torrents_torrent_sources WHERE info_hash IN ? GROUP BY info_hash) agg
			  WHERE tc.info_hash = agg.info_hash`,
			hashes,
		).Error; err != nil {
			return fmt.Errorf("updating search counts: %w", err)
		}

		return nil
	})
}
