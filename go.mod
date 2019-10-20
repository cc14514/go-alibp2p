module github.com/cc14514/go-alibp2p

require (
	github.com/btcsuite/btcd v0.0.0-20190213025234-306aecffea32
	github.com/cc14514/go-cookiekit v0.0.0-20181212102238-6a04bd7258bb
	github.com/cc14514/go-lightrpc v0.0.0-20191009082400-f2eba654f95d
	github.com/libp2p/go-buffer-pool v0.0.2
	github.com/libp2p/go-libp2p v0.1.0
	github.com/libp2p/go-libp2p-circuit v0.1.0
	github.com/libp2p/go-libp2p-connmgr v0.1.0
	github.com/libp2p/go-libp2p-core v0.0.1
	github.com/libp2p/go-libp2p-kad-dht v0.1.0
	github.com/libp2p/go-libp2p-pnet v0.1.0
	github.com/multiformats/go-multiaddr v0.0.4
	github.com/urfave/cli v1.22.1
	gopkg.in/karalabe/cookiejar.v2 v2.0.0-20150724131613-8dcd6a7f4951 // indirect
)

go 1.13

replace github.com/libp2p/go-libp2p-circuit => ./libs/go-libp2p-circuit

replace github.com/libp2p/go-libp2p-kad-dht => ./libs/go-libp2p-kad-dht

replace github.com/libp2p/go-libp2p => ./libs/go-libp2p
