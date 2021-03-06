package oniontransport

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/yawning/bulb/utils/pkcs1"
	"os"
	"path"
	"testing"
)

var key string

func TestMain(m *testing.M) {
	setup()
	retCode := m.Run()
	teardown()
	os.Exit(retCode)
}

func setup() {
	key, _ = createHiddenServiceKey()
}

func teardown() {
	os.RemoveAll(path.Join("./", key+".onion_key"))
}

func TestIsValidOnionMultiAddr(t *testing.T) {
	// Test valid
	validAddr, err := ma.NewMultiaddr("/onion3/7ozqnhf4lksxulre3qnclbmyzcg4qmkt4uww7wjtq457wlwhalolmgyd:4003")
	if err != nil {
		t.Fatal(err)
	}
	valid := IsValidOnionMultiAddr(validAddr)
	if !valid {
		t.Fatal("IsValidMultiAddr failed")
	}

	// Test wrong protocol
	invalidAddr, err := ma.NewMultiaddr("/ip4/0.0.0.0/tcp/4001")
	if err != nil {
		t.Fatal(err)
	}
	valid = IsValidOnionMultiAddr(invalidAddr)
	if valid {
		t.Fatal("IsValidMultiAddr failed")
	}
}

func Test_loadKeys(t *testing.T) {
	tpt := &OnionTransport{keysDir: "./"}
	keys, err := tpt.loadKeys()
	if err != nil {
		t.Fatal(err)
	}
	tpt.keys = keys
	k, ok := tpt.keys[key]
	if !ok {
		t.Fatal("Failed to correctly load keys")
	}
	id, err := pkcs1.OnionAddr(&k.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if id != key {
		t.Fatal("Failed to correctly load keys")
	}
}

func createHiddenServiceKey() (string, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return "", err
	}
	id, err := pkcs1.OnionAddr(&priv.PublicKey)
	if err != nil {
		return "", err
	}

	f, err := os.Create(id + ".onion_key")
	if err != nil {
		return "", err
	}
	defer f.Close()

	privKeyBytes, err := pkcs1.EncodePrivateKeyDER(priv)
	if err != nil {
		return "", err
	}

	block := pem.Block{Type: "RSA PRIVATE KEY", Bytes: privKeyBytes}
	err = pem.Encode(f, &block)
	if err != nil {
		return "", err
	}
	return id, nil
}

func Test_testTorConnectivity(t *testing.T) {

	onion, err := NewOnionTransport("",nil,"",nil,true)

	if err != nil {
		t.Fatal(err)
	}

	ma,err := ma.NewMultiaddr("/ip4/0.0.0.0/tcp/8090")

	if err != nil {
		t.Fatal(err)
	}

	_, err = onion.Listen(ma)

	if err != nil {
		t.Fatal(err)
	}
}
