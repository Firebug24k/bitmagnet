package peerrefresh

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"

	"github.com/bitmagnet-io/bitmagnet/internal/protocol"
	"github.com/bitmagnet-io/bitmagnet/internal/protocol/dht"
	"github.com/bitmagnet-io/bitmagnet/internal/protocol/dht/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func idWithFirstByte(b byte) protocol.ID {
	var id protocol.ID
	id[0] = b

	return id
}

func addr(i int) netip.AddrPort {
	return netip.AddrPortFrom(netip.AddrFrom4([4]byte{10, 0, 0, byte(i)}), 6881)
}

func filterWith(n, offset int) dht.ScrapeBloomFilter {
	var f dht.ScrapeBloomFilter
	for i := 0; i < n; i++ {
		f.AddIP(net.IPv4(192, 168, byte((i+offset)/250), byte((i+offset)%250)).To4())
	}

	return f
}

// fakeDHT: node i (addr(i)) answers with seeders filter seeds[i] and refers to nodes next[i].
type fakeDHT struct {
	seeds   map[netip.AddrPort]dht.ScrapeBloomFilter
	next    map[netip.AddrPort][]client.NodeInfo
	down    map[netip.AddrPort]bool
	queries []netip.AddrPort
}

func (f *fakeDHT) GetPeersScrape(_ context.Context, a netip.AddrPort, _ protocol.ID) (client.GetPeersScrapeResult, error) {
	f.queries = append(f.queries, a)
	if f.down[a] {
		return client.GetPeersScrapeResult{}, errors.New("timeout")
	}

	sd := f.seeds[a]
	var pe dht.ScrapeBloomFilter

	return client.GetPeersScrapeResult{
		ID:        protocol.RandomNodeID(),
		Nodes:     f.next[a],
		BfSeeders: *sd.ToBloomFilter(),
		BfPeers:   *pe.ToBloomFilter(),
	}, nil
}

func TestLookupUnionsFiltersAcrossRounds(t *testing.T) {
	t.Parallel()

	target := idWithFirstByte(0x00)
	f := &fakeDHT{
		seeds: map[netip.AddrPort]dht.ScrapeBloomFilter{
			addr(1): filterWith(10, 0),  // far seed node knows 10 seeders
			addr(2): filterWith(10, 5),  // overlaps 5 of them
			addr(3): filterWith(20, 30), // closer node discovered in round 2
		},
		next: map[netip.AddrPort][]client.NodeInfo{
			addr(1): {{ID: idWithFirstByte(0x01), Addr: addr(3)}},
		},
		down: map[netip.AddrPort]bool{},
	}

	seeds := []candidate{
		{id: idWithFirstByte(0x80), addr: addr(1)},
		{id: idWithFirstByte(0x90), addr: addr(2)},
	}

	var responded int
	res := lookup(context.Background(), f, target, seeds, func(protocol.ID, netip.AddrPort) { responded++ })

	require.NoError(t, res.Err)
	assert.Equal(t, 3, res.NodesQueried)
	assert.Equal(t, 3, res.NodesResponded)
	assert.Equal(t, 3, responded)
	require.True(t, res.Seeders.Valid)
	// 35 distinct IPs in total (0..14 and 30..49); bloom estimate should be close
	assert.InDelta(t, 35, float64(res.Seeders.Uint), 4)
	assert.True(t, res.Leechers.Valid)
	assert.Equal(t, uint(0), res.Leechers.Uint)
}

func TestLookupAllDown(t *testing.T) {
	t.Parallel()

	f := &fakeDHT{down: map[netip.AddrPort]bool{addr(1): true}}
	res := lookup(context.Background(), f, idWithFirstByte(0), []candidate{{id: idWithFirstByte(1), addr: addr(1)}}, nil)
	require.Error(t, res.Err)
	assert.False(t, res.Seeders.Valid)
}

func TestLookupNoSeeds(t *testing.T) {
	t.Parallel()

	res := lookup(context.Background(), &fakeDHT{}, idWithFirstByte(0), nil, nil)
	require.Error(t, res.Err)
}

func TestClosestUnqueriedOrdersByXorDistance(t *testing.T) {
	t.Parallel()

	target := idWithFirstByte(0x10)
	known := map[netip.AddrPort]candidate{
		addr(1): {id: idWithFirstByte(0xF0), addr: addr(1)},
		addr(2): {id: idWithFirstByte(0x11), addr: addr(2)},
		addr(3): {id: idWithFirstByte(0x30), addr: addr(3)},
	}
	got := closestUnqueried(known, map[netip.AddrPort]struct{}{addr(2): {}}, target, 5)
	require.Len(t, got, 2)
	assert.Equal(t, addr(3), got[0].addr)
	assert.Equal(t, addr(1), got[1].addr)
}

func TestDedupe(t *testing.T) {
	t.Parallel()

	a, b := idWithFirstByte(1), idWithFirstByte(2)
	assert.Equal(t, []protocol.ID{a, b}, dedupe([]protocol.ID{a, b, a}))
}
