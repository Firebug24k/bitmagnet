package client

import (
	"context"
	"net/netip"

	"github.com/bitmagnet-io/bitmagnet/internal/protocol"
	"github.com/bits-and-blooms/bloom/v3"
)

type Client interface {
	Ping(ctx context.Context, addr netip.AddrPort) (PingResult, error)
	FindNode(ctx context.Context, addr netip.AddrPort, target protocol.ID) (FindNodeResult, error)
	GetPeers(ctx context.Context, addr netip.AddrPort, infoHash protocol.ID) (GetPeersResult, error)
	GetPeersScrape(ctx context.Context, addr netip.AddrPort, infoHash protocol.ID) (GetPeersScrapeResult, error)
	// GetPeersScrapeLookup is GetPeersScrape for iterative lookups: a response without BEP 33 bloom
	// filters (the normal answer from nodes that only know closer nodes) is not an error.
	GetPeersScrapeLookup(ctx context.Context, addr netip.AddrPort, infoHash protocol.ID) (GetPeersScrapeLookupResult, error)
	SampleInfoHashes(ctx context.Context, addr netip.AddrPort, target protocol.ID) (SampleInfoHashesResult, error)
}

type PingResult struct {
	ID protocol.ID
}

type FindNodeResult struct {
	ID    protocol.ID
	Nodes []NodeInfo
}

type GetPeersResult struct {
	ID     protocol.ID
	Values []netip.AddrPort
	Nodes  []NodeInfo
}

type GetPeersScrapeResult struct {
	ID        protocol.ID
	Values    []netip.AddrPort
	Nodes     []NodeInfo
	BfPeers   bloom.BloomFilter
	BfSeeders bloom.BloomFilter
}

type GetPeersScrapeLookupResult struct {
	ID     protocol.ID
	Values []netip.AddrPort
	Nodes  []NodeInfo
	// nil when the node returned no scrape data
	BfPeers   *bloom.BloomFilter
	BfSeeders *bloom.BloomFilter
}

type SampleInfoHashesResult struct {
	ID       protocol.ID
	Samples  []protocol.ID
	Nodes    []NodeInfo
	Num      int
	Interval int
}

type NodeInfo struct {
	ID   protocol.ID
	Addr netip.AddrPort
}
