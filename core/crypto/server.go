package core

import (
	"encoding/json"

	"github.com/navaz-alani/concord/core"
	"github.com/navaz-alani/concord/packet"
)

func (cr *Crypto) installOnServer(p core.Processor) error {
	p.PacketProcessor().AddCallback(TargetKeyExchangeServer, cr.keyExchangeServer)
	p.PacketProcessor().AddCallback(TargetKeyExchangeClient, cr.keyExchangeClient)
	return nil
}

func (cr *Crypto) keyExchangeServer(ctx *core.TargetCtx, pw packet.Writer) {
	// get public key from packet
	var pk PublicKey
	if err := json.Unmarshal(ctx.Pkt.Data(), &pk); err != nil {
		ctx.Stat = core.CodeStopError
		ctx.Msg = "malformed packet"
		return
	}
	// store client shared & public keys
	cr.setKeyStore(ctx.From, &keyStore{
		public: &pk,
		shared: cr.computeSharedKey(&pk),
	})
	// write svr public key to response packet
	pw.Meta().Add(KeyNoCrypto, "true")
	pw.Write(cr.publicKey)
	ctx.Stat = core.CodeStopCloseSend
}

func (cr *Crypto) keyExchangeClient(ctx *core.TargetCtx, pw packet.Writer) {
	var otherClient struct {
		IP string `json:"ip"`
	}
	if err := json.Unmarshal(ctx.Pkt.Data(), &otherClient); err != nil {
		ctx.Stat = core.CodeStopError
		ctx.Msg = "malformed packet"
		return
	}
	if keys, ok := cr.getKeyStore(otherClient.IP); !ok {
		ctx.Stat = core.CodeStopError
		ctx.Msg = "client non-existent"
	} else {
		otherClientPubKey, _ := json.Marshal(keys.public)
		pw.Meta().Add(KeyNoCrypto, "true")
		pw.Write(otherClientPubKey)
	}
}
