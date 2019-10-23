module github.com/cc14514/go-mux-transport

require (
	github.com/gogo/protobuf v1.2.1
	github.com/ipfs/go-log v0.0.1
	github.com/libp2p/go-buffer-pool v0.0.2
	github.com/libp2p/go-libp2p-blankhost v0.1.1
	github.com/libp2p/go-libp2p-core v0.0.1
	github.com/libp2p/go-libp2p-swarm v0.1.0
	github.com/libp2p/go-libp2p-transport-upgrader v0.1.1
	github.com/libp2p/go-tcp-transport v0.1.0
	github.com/multiformats/go-multiaddr v0.0.4
	github.com/multiformats/go-multiaddr-net v0.0.1
)

go 1.13

replace github.com/libp2p/go-tcp-transport => ../go-tcp-transport