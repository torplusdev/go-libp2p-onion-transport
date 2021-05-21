package oniontransport

import (
	"context"
	"testing"

	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

func TestCanConnectToOnionService(t *testing.T) {

	// Test valid
	//http://7ozqnhf4lksxulre3qnclbmyzcg4qmkt4uww7wjtq457wlwhalolmgyd.onion:8080/ipfs/QmfM2r8seH2GiRaC4esTjeraXEachRt8ZsSeGaWTPLyMoG

	validAddr, err := ma.NewMultiaddr("/onion3/7ozqnhf4lksxulre3qnclbmyzcg4qmkt4uww7wjtq457wlwhalolmgyd:4001")
	if err != nil {
		t.Fatal(err)
	}

	onion, err := NewOnionTransport("", "", "", "", nil, "", nil, true, true)

	if err != nil {
		t.Fatal(err)
	}
	var ctx = context.Background()
	var peerId = peer.ID("Qmc9FmqhnHGNJJNkg89PKFYh9kSNU6KegKqmpizpbF2S2d")
	onion.Dial(ctx, validAddr, peerId)

}
