module paidpiper.com/go-libp2p-onion-transport

require (
	github.com/cretz/bine v0.1.0
	github.com/libp2p/go-buffer-pool v0.0.2 // indirect
	github.com/libp2p/go-libp2p-core v0.2.3
	github.com/libp2p/go-libp2p-mplex v0.2.1 // indirect
	github.com/libp2p/go-libp2p-testing v0.1.0 // indirect
	github.com/libp2p/go-libp2p-transport v0.1.0
	github.com/libp2p/go-libp2p-transport-upgrader v0.1.1
	github.com/multiformats/go-multiaddr v0.3.0
	github.com/multiformats/go-multiaddr-net v0.2.0
	github.com/whyrusleeping/mafmt v1.2.8
	github.com/yawning/bulb v0.0.0-20170405033506-85d80d893c3d
	golang.org/x/crypto v0.0.0-20190923035154-9ee001bba392
	golang.org/x/net v0.0.0-20190620200207-3b0461eec859

)

replace github.com/multiformats/go-multiaddr => ../go-multiaddr

replace github.com/multiformats/go-multiaddr-net => ../go-multiaddr-net

go 1.12
