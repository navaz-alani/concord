package internal

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"io"
	"math/big"
	"sync"

	"github.com/navaz-alani/concord/packet"
)

// Target names the Crypto extension reserves.
const (
	TargetKeyExchangeClientServer = "crypto.kex-cs"
	TargetKeyExchangeClientClient = "crypto.kex-cc"
)

var curve = elliptic.P256()

// PublicKey is the structure of the public key used by Crypto, and its clients.
// The curve used needs to be the same for all clients.
type PublicKey struct {
	X *big.Int `json:"x"`
	Y *big.Int `json:"y"`
}

type sharedSecret *big.Int

type keyStore struct {
	shared sharedSecret
	public PublicKey
}

// Crypto is a cyrptographic extension for a Server/Client. It provides, mainly,
// the ability to share keys and establish shared secrets. It uses the ECDSA
// algorithm with (currenly)
type Crypto struct {
	mu        sync.RWMutex
	privKey   *ecdsa.PrivateKey
	publicKey []byte
	keys      map[string]*keyStore
}

func NewCrypto(privKey *ecdsa.PrivateKey) (*Crypto, error) {
	if privKey == nil {
		if pk, err := ecdsa.GenerateKey(curve, rand.Reader); err != nil {
			return nil, err
		} else {
			privKey = pk
		}
	}
	publicKey, _ := json.Marshal(PublicKey{
		X: privKey.PublicKey.X,
		Y: privKey.PublicKey.Y,
	})
	cr := &Crypto{
		mu:        sync.RWMutex{},
		privKey:   privKey,
		publicKey: publicKey,
	}
	return cr, nil
}

func (cr *Crypto) keyExchangeClientClient(ctx *TargetCtx, pw packet.Writer) {
	var otherClient struct {
		IP string `json:"ip"`
	}
	if err := json.Unmarshal(ctx.Pkt.Data(), &otherClient); err != nil {
		ctx.Stat = -1
		ctx.Msg = "malformed packet"
		return
	}
	cr.mu.RLock()
	keys, ok := cr.keys[otherClient.IP]
	cr.mu.RUnlock()
	if !ok {
		ctx.Stat = -1
		ctx.Msg = "client non-existent"
	} else {
		otherClientPubKey, _ := json.Marshal(keys.public)
		pw.Write(otherClientPubKey)
	}
}

func (cr *Crypto) keyExchangeClientServer(ctx *TargetCtx, pw packet.Writer) {
	// get public key from packet
	var pk PublicKey
	if err := json.Unmarshal(ctx.Pkt.Data(), &pk); err != nil {
		ctx.Stat = -1
		ctx.Msg = "malformed packet"
		return
	}
	// compute shared key
	sharedKey, _ := ecdsa.PublicKey{
		Curve: curve,
		X:     pk.X,
		Y:     pk.Y,
	}.Curve.ScalarMult(pk.X, pk.Y, cr.privKey.D.Bytes())
	// store client shared & public keys
	cr.mu.Lock()
	cr.keys[ctx.From] = &keyStore{
		shared: sharedKey,
		public: pk,
	}
	cr.mu.Unlock()
	// write svr public key to response packet
	pw.Write(cr.publicKey)
}

func (cr *Crypto) encrypt(ctx *TransformContext, buff []byte) ([]byte, error) {
	var key *big.Int
	cr.mu.RLock()
	k, ok := cr.keys[ctx.Dest]
	cr.mu.RUnlock()
	if ok {
		key = k.shared
	} else {
		ctx.Stat = CodeStopError
		ctx.Msg = "no shared key to encrypt data"
		return buff, nil
	}

	block, err := aes.NewCipher(key.Bytes())
	if err != nil {
		ctx.Stat = CodeStopError
		ctx.Msg = "cipher init fail: " + err.Error()
		return buff, nil
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		ctx.Stat = CodeStopError
		ctx.Msg = "gcm init fail: " + err.Error()
		return buff, nil
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		ctx.Stat = CodeStopError
		ctx.Msg = "nonce gen fail: " + err.Error()
		return buff, nil
	}

	return aesGCM.Seal(nonce, nonce, buff, nil), nil
}

func (cr *Crypto) decrypt(ctx *TransformContext, buff []byte) ([]byte, error) {
	var key *big.Int
	cr.mu.RLock()
	k, ok := cr.keys[ctx.Dest]
	cr.mu.RUnlock()
	if ok {
		key = k.shared
	} else {
		ctx.Stat = CodeStopNoop
		return buff, nil
	}

	block, err := aes.NewCipher(key.Bytes())
	if err != nil {
		ctx.Stat = CodeStopError
		ctx.Msg = "cipher init fail: " + err.Error()
		return buff, nil
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		ctx.Stat = CodeStopError
		ctx.Msg = "gcm init fail: " + err.Error()
		return buff, nil
	}

	nonceSize := aesGCM.NonceSize()
	nonce, ciphertext := buff[:nonceSize], buff[nonceSize:]

	if decrypted, err := aesGCM.Open(nil, nonce, ciphertext, nil); err != nil {
		ctx.Stat = CodeStopError
		ctx.Msg = "decrypt fail: " + err.Error()
		return buff, nil
	} else {
		return decrypted, nil
	}
}
