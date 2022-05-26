package oniontransport

import (
	"context"
	"fmt"
	"testing"
	"time"

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
	configPath := "/usr/local/etc/tor/torrc"
	onion, err := NewOnionTransport("", "", configPath, "", nil, "", nil, true, true)

	if err != nil {
		t.Fatal(err)
	}
	var ctx = context.Background()
	for i := 0; i < 10; i++ {
		var peerId = peer.ID("Qmc9FmqhnHGNJJNkg89PKFYh9kSNU6KegKqmpizpbF2S2d")
		connection, err := onion.Dial(ctx, validAddr, peerId)
		if err != nil {
			t.Error(err)
			fmt.Printf("error: %v\n", err)
			time.Sleep(4 * time.Second)
			continue
		}
		err = connection.Close()
		if err != nil {
			t.Error(err)
		}
		time.Sleep(4 * time.Second)
	}
}
