module github.com/cc14514/go-alibp2p

require (
	github.com/btcsuite/btcd v0.20.1-beta
	github.com/cc14514/go-mux-transport v0.0.3-0.20200507033647-f8474a56dffe
	github.com/google/uuid v1.1.1
	github.com/hashicorp/golang-lru v0.5.4
	github.com/ipfs/go-cid v0.0.5
	github.com/ipfs/go-log v1.0.4
	github.com/libp2p/go-buffer-pool v0.0.2
	github.com/libp2p/go-libp2p v0.8.3
	github.com/libp2p/go-libp2p-circuit v0.2.2
	github.com/libp2p/go-libp2p-connmgr v0.2.1
	github.com/libp2p/go-libp2p-core v0.5.3
	github.com/libp2p/go-libp2p-discovery v0.4.0
	github.com/libp2p/go-libp2p-kad-dht v0.7.11
	github.com/libp2p/go-libp2p-mplex v0.2.3
	github.com/libp2p/go-libp2p-yamux v0.2.7
	github.com/multiformats/go-multiaddr v0.2.1
	github.com/multiformats/go-multihash v0.0.13
	github.com/tendermint/go-amino v0.0.0-20200130113325-59d50ef176f6
	golang.org/x/xerrors v0.0.0-20190717185122-a985d3407aa7
)

go 1.13

// TODO
//replace github.com/libp2p/go-libp2p-swarm => github.com/cc14514/go-libp2p-swarm v0.0.0-20200414101126-ec86bc27c764

// TODO
//replace github.com/libp2p/go-libp2p-kad-dht => github.com/cc14514/go-libp2p-kad-dht v0.0.0-20200416072228-916c63fc8591

// TODO : test
replace github.com/libp2p/go-libp2p => ../go-libp2p

// ok
replace github.com/libp2p/go-libp2p-circuit => github.com/cc14514/go-libp2p-circuit v0.0.3-0.20200507025847-712c27d4a1ca
