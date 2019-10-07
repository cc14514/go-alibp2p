module github.com/cc14514/go-alibp2p

require (
	github.com/btcsuite/btcd v0.0.0-20190213025234-306aecffea32
	github.com/cc14514/go-lightrpc v0.0.0-20191006124726-a77c695f8c82
	github.com/libp2p/go-libp2p v0.1.0
	github.com/libp2p/go-libp2p-circuit v0.1.0
	github.com/libp2p/go-libp2p-core v0.0.1
	github.com/libp2p/go-libp2p-kad-dht v0.1.0
	github.com/libp2p/go-libp2p-pnet v0.1.0
	github.com/multiformats/go-multiaddr v0.0.4
	github.com/urfave/cli v1.22.1
)

go 1.13

replace github.com/libp2p/go-libp2p-circuit => ./libs/go-libp2p-circuit

replace github.com/libp2p/go-libp2p-kad-dht => ./libs/go-libp2p-kad-dht

replace github.com/libp2p/go-libp2p => ./libs/go-libp2p
