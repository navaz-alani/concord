package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"math/big"
	"sync"

	"github.com/navaz-alani/concord/client"
	"github.com/navaz-alani/concord/packet"
)

const (
	TargetServerPublicKey = "e2e.svr-pub"
	TargetClientPublicKey = "e2e.cli-pub"
)

var curve = elliptic.P256()

type publicKey struct {
	X *big.Int `json:"x"`
	Y *big.Int `json:"y"`
}

type sharedSecret *big.Int

// E2ECrypto is a Server extension which installs end-to-end encryption
// capabilities on the server. Specifically, it adds public-key accessor and
// handshake targets to the server. It then intercepts data read/written over
// the connection and decrypts/encrypts it respectively.
type E2ECrypto struct {
	ServerExtensionInstaller
	mu        sync.RWMutex
	privKey   *ecdsa.PrivateKey
	publicKey []byte
	keyCache  map[string]sharedSecret
}

func NewE2ECrypto(privKey *ecdsa.PrivateKey) (*E2ECrypto, error) {
	if privKey == nil {
		if pk, err := ecdsa.GenerateKey(curve, rand.Reader); err != nil {
			return nil, err
		} else {
			privKey = pk
		}
	}
	publicKey, _ := json.Marshal(publicKey{
		X: privKey.PublicKey.X,
		Y: privKey.PublicKey.Y,
	})
	e2e := &E2ECrypto{
		mu:        sync.RWMutex{},
		privKey:   privKey,
		publicKey: publicKey,
	}
	return e2e, nil
}

func (e2e *E2ECrypto) sendServerPubKey(ctx *ServerCtx, pw packet.Writer) {
	pw.Write(e2e.publicKey)
}

func (e2e *E2ECrypto) getClientPubKey(ctx *ServerCtx, pw packet.Writer) {
	// get public key from client
	var pk publicKey
	if err := json.Unmarshal(ctx.Pkt.Data(), &pk); err != nil {
		ctx.Stat = -1
		ctx.Msg = "malformed packet"
		return
	}
	clientPublicKey := &ecdsa.PublicKey{
		Curve: curve,
		X:     pk.X,
		Y:     pk.Y,
	}
	sk, _ := clientPublicKey.Curve.ScalarMult(pk.X, pk.Y, e2e.privKey.D.Bytes())
	e2e.keyCache[ctx.From] = sk
}

func (e2e *E2ECrypto) InstallServer(s Server) error {
	// install targets onto server
	s.AddTargetCallback(TargetClientPublicKey, e2e.getClientPubKey)
	s.AddTargetCallback(TargetServerPublicKey, e2e.sendServerPubKey)
	return nil
}

func (e2e *E2ECrypto) InstallClient(c client.Client) error {
	// TODO
	return nil
}
