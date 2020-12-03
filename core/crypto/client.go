package core

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/json"
	"fmt"

	"github.com/navaz-alani/concord/client"
	"github.com/navaz-alani/concord/core"
	"github.com/navaz-alani/concord/packet"
)

func (cr *Crypto) installOnClient(p core.Processor) error {
	return nil
}

// IsKeyExchanged reports whether or not a successful handshake has been
// performed with the given address.
func (cr *Crypto) IsKeyExchanged(addr string) bool {
	_, ok := cr.getKeyStore(addr)
	return ok
}

// ConfigureClient secures the given client by installing the Crypto extension
// on it (to which a pointer is returned). It then performs a key exchange with
// svrAddr. If successful, the connection between the server and the returned
// client will be secure i.e. packets sent between the server and the client
// will be encrypted with AES, using a shared key generated using ECDH. The
// `pkt` parameter will be used to compose the key exchange packet with the
// server (ownership of `pkt` is assumed by ConfigureClient).
func ConfigureClient(client client.Client, svrAddr string, pkt packet.Packet) (*Crypto, error) {
	// generate private key
	privKey, err := ecdsa.GenerateKey(Curve, rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("public key gen fail: %s", err.Error())
	}
	// initialize Crypto extension
	cr, err := NewCrypto(privKey)
	if err != nil {
		return nil, fmt.Errorf("Crypto extension error: %s", err.Error())
	}
	// perform key-exchange with server
	if err := cr.ServerKEx(client, svrAddr, pkt); err != nil {
		return nil, fmt.Errorf("handshake error: %s", err.Error())
	}
	// install extension on client pipelines to provide transport encryption
	if err = cr.Extend("client", client); err != nil {
		return nil, fmt.Errorf("Crypto install err: %s", err.Error())
	}
	return cr, nil
}

// ConfigureKeyExClientPkt writes the configuration (target, metadata, body,
// etc) for a client-client key-exchange with the given address to the packet.
// The packet being written to must be new and after a call to this method, the
// packet's Write method should not be used. Also, this method will clear pw's
// underlying Packet.
func (cr *Crypto) ConfigureKeyExClientPkt(addr string, pw packet.Writer) {
	pw.Clear()
	pw.Meta().Add(packet.KeyTarget, TargetKeyExchangeClient)
	pw.Write([]byte(fmt.Sprintf(`{"ip":"%s"}`, addr)))
	pw.Close()
}

// ConfigureKeyExServerPkt writes the configuration (target, metadata, body, etc)
// for a client-server key-exchange. The packet being written to must be new and
// after a call to this method, the packet's Write method should not be used.
// Also, this method will clear pw's underlying packet.
func (cr *Crypto) ConfigureKeyExServerPkt(pw packet.Writer) {
	pw.Clear()
	pw.Meta().Add(packet.KeyTarget, TargetKeyExchangeServer)
	pw.Write(cr.publicKey)
	pw.Close()
}

// EncryptFor encrypts the given data for the given address using the
// corresponding shared key. If there is no shared key, or there is an
// encryption error, the error returned will be non-nil and reflect this.
//
// It is used in end-to-end encryption to encrypt outgoing payloads, being
// server-relayed to `addr`. To generate a shared key with `addr`, a
// client-client key exchange has to be performed.
func (cr *Crypto) EncryptFor(addr string, data []byte) ([]byte, error) {
	if ks, ok := cr.getKeyStore(addr); !ok {
		return nil, fmt.Errorf("keys not exchanged")
	} else if encrypted, err := encryptAES(ks.shared.Bytes(), data); err != nil {
		return nil, fmt.Errorf("encryption error: " + err.Error())
	} else {
		return encrypted, nil
	}
}

// DecryptFrom decrypts the given data using the shared key with `addr. The
// returned error is non-nil if there is no shared key between with the address
// and if there is an error in the decryption process.
//
// It is used in end-to-end encryption to decrypt incoming payloads, which have
// been server-relayed from the given address. To generate a shared key with
// `addr`, a client-client key exchange has to be performed.
func (cr *Crypto) DecryptFrom(addr string, data []byte) ([]byte, error) {
	if ks, ok := cr.getKeyStore(addr); !ok {
		return nil, fmt.Errorf("keys not exchanged")
	} else if decrypted, err := decryptAES(ks.shared.Bytes(), data); err != nil {
		return nil, fmt.Errorf("encryption error: " + err.Error())
	} else {
		return decrypted, nil
	}
}

// EncryptE2E performs encryption on the packet's data for end-to-end encryption
// during transit to the packet's destination. The packet is modified so that
// its Data method returns the encrypted data. Clients should use this to
// encrypt their application data. Note that E2E does not imply that the entire
// packet is encrypted since the packet metadata will still possibly be plain
// text. To get full-packet encryption, clients should install Crypto on their
// `client` handle, and perform a key-exchange with they servers they interact
// with to get full-packet encryption between the client and the servers.
//
// Note that before this function can work, it needs a shared key with the
// destination to which the packet is destined. If a key exchange has been
// successfully performed, then there will most likely be no errors.
func (cr *Crypto) EncryptE2E(to string, pkt packet.Packet) error {
	if encrypted, err := cr.EncryptFor(to, pkt.Data()); err != nil {
		return fmt.Errorf("e2e encrypt error: %s", err.Error())
	} else {
		writer := pkt.Writer()
		writer.Clear()
		writer.Write(encrypted)
		writer.Close()
		return nil
	}
}

// DecryptE2E performs decryption on end-to-end encrypted packet data from the
// specified sender. The packet is modified so that its Data method returns the
// decrypted data. Clients should use this to decrypt packets.
//
// Note that before this function can work, it needs a shared key with the
// sender of the packet. If a key exchange has been successfully performed, then
// there will most likely be no errors.
func (cr *Crypto) DecryptE2E(sender string, pkt packet.Packet) error {
	if decrypted, err := cr.DecryptFrom(sender, pkt.Data()); err != nil {
		return fmt.Errorf("e2e decrypt error: %s", err.Error())
	} else {
		writer := pkt.Writer()
		writer.Clear()
		writer.Write(decrypted)
		writer.Close()
		return nil
	}
}

// ProcessKeyExResp processes the response to a key-exchange with the given
// address (server address if server and client address otherwise).
func (cr *Crypto) ProcessKeyExResp(addr string, resp packet.Packet) error {
	var pk PublicKey
	if err := json.Unmarshal(resp.Data(), &pk); err != nil {
		return fmt.Errorf("packet decode error: " + err.Error())
	}
	// store key
	cr.setKeyStore(addr, &keyStore{
		keySent: true,
		public:  &pk,
		shared:  cr.computeSharedKey(&pk),
	})
	return nil
}

// ServerKEx performs a key exchange between the client and the server at
// svrAddr. If successful, there will then exist a shared key between `svrAddr`
// and `client`. A side effect of this is that if the Crypto extension is
// installed on `client`, then all packets whose Dest method returns `svrAddr`,
// will have their content encrypted in transit. This is a full-packet
// encryption (metadata and data), in contrast to end-to-end encryption which is
// just a data encrytion i.e. the packet's metadata will still possibly be in
// plain text.
func (cr *Crypto) ServerKEx(client client.Client, svrAddr string, pkt packet.Packet) error {
	cr.ConfigureKeyExServerPkt(pkt.Writer())
	kexResp := make(chan packet.Packet)
	client.Send(pkt, kexResp)
	if err := cr.ProcessKeyExResp(svrAddr, <-kexResp); err != nil {
		return fmt.Errorf("handshake error: %s", err.Error())
	}
	return nil
}

// ClientKEx performs a key exchange between the client and the client at
// clientAddr. If successful, there will then exist a shared key between
// `clientAddr` (remote) and `client` (local). This shared key can be used for
// end-to-end encryption with `clientAddr` with the {Encrypt,Decrypt}E2E
// methods.
func (cr *Crypto) ClientKEx(client client.Client, clientAddr string, pkt packet.Packet) error {
	cr.ConfigureKeyExClientPkt(clientAddr, pkt.Writer())
	respChan := make(chan packet.Packet)
	client.Send(pkt, respChan)
	if err := cr.ProcessKeyExResp(clientAddr, <-respChan); err != nil {
		return fmt.Errorf("client-kex ferror: %s", err.Error())
	}
	return nil
}
