package tor

import (
	"fmt"
	"testing"
)

// readLines reads a whole file into memory
// and returns a slice of its lines.

func TestTorParcer(t *testing.T) {
	sp, cp, err := ReadTorConfigsLines("/usr/local/etc/tor/torrc")
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("SocksPort %v ControlPort %v", sp, cp)
}
