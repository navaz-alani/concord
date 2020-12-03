package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"sync"

	"github.com/navaz-alani/concord/core"
)

// Target names the Crypto extension reserves.
const (
	TargetKeyExchangeServer = "crypto.kex-cs"
	TargetKeyExchangeClient = "crypto.kex-cc"
)

// Metadata keys for Crypto extension
const (
	// KeyNoCipher is a metadata key, which when set to "true" in a packet causes
	// the Crypto extension to skip that packet.
	KeyNoCrypto = "_no_crypto"
)

// Curve is the elliptic curve used by Crypto.
var Curve = elliptic.P256()

// PublicKey is the structure of the public key used by Crypto, and its clients.
// The curve used needs to be the same for all clients.
type PublicKey struct {
	X *big.Int `json:"x"`
	Y *big.Int `json:"y"`
}

type keyStore struct {
	mu      sync.RWMutex // mu protects `ketSent`
	keySent bool         // indicates whether or not this key has been shared
	shared  *big.Int
	public  *PublicKey
}

func (ks *keyStore) getKeySent() bool {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.keySent
}

func (ks *keyStore) setKeySent(s bool) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	ks.keySent = s
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
		if pk, err := ecdsa.GenerateKey(Curve, rand.Reader); err != nil {
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

func (cr *Crypto) computeSharedKey(pk *PublicKey) *big.Int {
	x, _ := Curve.ScalarMult(pk.X, pk.Y, cr.privKey.D.Bytes())
	return x
}

// Extend installs Crypto onto the given Processor's pipeline.
func (cr *Crypto) Extend(kind string, target core.Processor) error {
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

// encryptTransport is the data pipeline BufferTransform which encrypts packet
// data based on the destination of the packet. If a key exchange with the
// destination has not been performed, then the transform will be the identity
// transform (will do nothing to the contents of the buffer).
func (cr *Crypto) encryptTransport(ctx *core.TransformContext, buff []byte) []byte {
	switch ctx.Pkt.Meta().Get(KeyNoCrypto) {
	case "true", "t", "yes", "y", "1":
		return buff
	}
	if k, ok := cr.getKeyStore(ctx.Pkt.Dest()); !ok {
		return buff
	} else if !k.getKeySent() { // ctx.Dest does not have public key for server yet...
		// Here, it is being assumed that the first outgoing packet to `ctx.Dest` is
		// the public key of the server. This implies that the connection should be
		// used atomically when the key-exchange is being performed i.e. no other
		// packets should be going between the server and `ctx.Dest` from the time
		// that the key exchange request is sent to when the server has sent a
		// response.
		k.setKeySent(true)
		return buff
	} else if ciphertext, err := encryptAES(k.shared.Bytes(), buff); err != nil {
		ctx.Stat = core.CodeStopError
		ctx.Msg = "encryption error: " + err.Error()
		return buff
	} else {
		return ciphertext
	}
}

// decryptTransport is the data pipeline BufferTransform which decrypts packet
// data based on the sender of the packet. If a key exchange with the
// sender has not been performed, then the transform will be the identity
// transform (will do nothing to the contents of the buffer).
func (cr *Crypto) decryptTransport(ctx *core.TransformContext, buff []byte) []byte {
	if k, ok := cr.getKeyStore(ctx.From); !ok {
		return buff
	} else if decrypted, err := decryptAES(k.shared.Bytes(), buff); err != nil {
		// the payload could not be encrypted - do not know yet... so this
		// decryption error may not really be a processing error.
		return buff
	} else {
		return decrypted
	}
}
