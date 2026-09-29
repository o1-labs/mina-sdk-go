package itn

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
)

// Key is an ed25519 key that signs requests to a daemon's ITN GraphQL
// server. The daemon accepts a request only when the key's public half is
// in its --itn-keys list.
//
// Keys are exchanged as standard base64 with padding: the private key as
// its 32-byte seed (the format of mina-perf-testing's orchestrator key and
// fetcher_sk), and the public key as the 32-byte value for --itn-keys.
type Key struct {
	sk ed25519.PrivateKey
}

// KeyFromBase64 loads a key from the base64 encoding of its 32-byte seed.
// Surrounding white space is ignored.
func KeyFromBase64(seedB64 string) (Key, error) {
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(seedB64))
	if err != nil {
		return Key{}, &InvalidKeyError{Reason: "not base64: " + err.Error()}
	}
	if len(seed) != ed25519.SeedSize {
		return Key{}, &InvalidKeyError{
			Reason: fmt.Sprintf("expected %d bytes, got %d", ed25519.SeedSize, len(seed)),
		}
	}
	return KeyFromSeed(seed), nil
}

// KeyFromSeed creates a key from its 32-byte seed. It panics if the seed
// does not have ed25519.SeedSize bytes.
func KeyFromSeed(seed []byte) Key {
	return Key{sk: ed25519.NewKeyFromSeed(seed)}
}

// GenerateKey creates a new random key.
func GenerateKey() (Key, error) {
	_, sk, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Key{}, err
	}
	return Key{sk: sk}, nil
}

// Base64 returns the base64 encoding of the key's 32-byte seed; the inverse
// of KeyFromBase64.
func (k Key) Base64() string {
	return base64.StdEncoding.EncodeToString(k.sk.Seed())
}

// PublicKeyBase64 returns the base64 public key, as the daemon expects it in
// --itn-keys.
func (k Key) PublicKeyBase64() string {
	return base64.StdEncoding.EncodeToString(k.sk.Public().(ed25519.PublicKey))
}

// String shows the public key only, so that a key never ends up in a log.
func (k Key) String() string {
	return "itn.Key{public: " + k.PublicKeyBase64() + "}"
}

// GoString is String, for %#v.
func (k Key) GoString() string { return k.String() }

func (k Key) signBase64(msg []byte) string {
	return base64.StdEncoding.EncodeToString(ed25519.Sign(k.sk, msg))
}
