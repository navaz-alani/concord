package internal

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"sync"

	"github.com/navaz-alani/concord/internal"
)

// Target names the Crypto extension reserves.
const (
	TargetKeyExchangeServer = "crypto.kex-cs"
	TargetKeyExchangeClient = "crypto.kex-cc"
)

var curve = elliptic.P256()

// PublicKey is the structure of the public key used by Crypto, and its clients.
// The curve used needs to be the same for all clients.
type PublicKey struct {
	X *big.Int `json:"x"`
	Y *big.Int `json:"y"`
}

type keyStore struct {
	shared *big.Int
	public PublicKey
}

// Crypto is a cyrptographic extension for a Server/Client. It provides, mainly,
// the ability to share keys and establish shared secrets. It uses the ECDSA
// algorithm with (currenly) NIST P-256 for the curve.
//
// Before installation onto a Processor, a key exchange to
// the server will need to be performed. Due to the transforms that it installs
// on the in/out data pipelines on a client/server, these pipelines will fail if
// there is no shared key to the packet destination (when sending) or source
// (when receiving). To perform a server key exchange, create a packet and use
// the Crypto.ConfigureKeyExServerPkt method to configure it to target a server
// key exchange and then use Cryto.ProcessKeyExServer method to process the
// response, which when successful, will add the key to the internal key store,
// after which the pipelines will be unblocked.
type Crypto struct {
	mu        sync.RWMutex // mu protects the `keys` field
	keys      map[string]*keyStore
	privKey   *ecdsa.PrivateKey
	publicKey []byte
}

func NewCrypto(privKey *ecdsa.PrivateKey) (*Crypto, error) {
	if privKey == nil {
		if pk, err := ecdsa.GenerateKey(curve, rand.Reader); err != nil {
			return nil, err
		} else {
			privKey = pk
		}
	}
	publicKey, err := json.Marshal(PublicKey{
		X: privKey.PublicKey.X,
		Y: privKey.PublicKey.Y,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode public key: " + err.Error())
	}
	cr := &Crypto{
		mu:        sync.RWMutex{},
		keys:      make(map[string]*keyStore),
		privKey:   privKey,
		publicKey: publicKey,
	}
	return cr, nil
}

// Extend installs Crypto onto the given Processor's pipeline.
func (cr *Crypto) Extend(kind string, target internal.Processor) error {
	// install transport layer encryption & decryption buffer transforms
	// for both server and client kinds
	target.DataProcessor().AddTransform("_in_", cr.decryptTransport)
	target.DataProcessor().AddTransform("_out_", cr.encryptTransport)
	switch kind {
	case "server":
		if err := cr.installOnServer(target); err != nil {
			return fmt.Errorf("error installing on server: " + err.Error())
		}
	case "client":
		if err := cr.installOnClient(target); err != nil {
			return fmt.Errorf("error installing on client: " + err.Error())
		}
	default:
		return fmt.Errorf("unknown processor kind: \"" + kind + "\"")
	}
	return nil
}

// setKeyStore sets the keyStore for the given address in the shared
// interal `keys` map, by first acquiring the mutex for write.
func (cr *Crypto) setKeyStore(addr string, ks *keyStore) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.keys[addr] = ks
}

// getKeyStore obtains the keyStore for the given address from the shared
// interal `keys` map, by first acquiring the mutex for read.
func (cr *Crypto) getKeyStore(addr string) (*keyStore, bool) {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	ks, ok := cr.keys[addr]
	return ks, ok
}

func (cr *Crypto) encryptTransport(ctx *internal.TransformContext, buff []byte) []byte {
	if k, ok := cr.getKeyStore(ctx.Dest); !ok {
		ctx.Stat = internal.CodeStopError
		ctx.Msg = "no shared key to encrypt data"
		return buff
	} else if ciphertext, err := encryptAES(k.shared.Bytes(), buff); err != nil {
		ctx.Stat = internal.CodeStopError
		ctx.Msg = "encryption error: " + err.Error()
		return buff
	} else {
		return ciphertext
	}
}

func (cr *Crypto) decryptTransport(ctx *internal.TransformContext, buff []byte) []byte {
	if k, ok := cr.getKeyStore(ctx.From); !ok {
		ctx.Stat = internal.CodeStopNoop
		ctx.Msg = "no shared key to decrypt data"
		return buff
	} else if decrypted, err := decryptAES(k.shared.Bytes(), buff); err != nil {
		ctx.Stat = internal.CodeStopError
		ctx.Msg = "decryption error: " + err.Error()
		return buff
	} else {
		return decrypted
	}
}
