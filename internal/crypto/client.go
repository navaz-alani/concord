package internal

import (
	"fmt"

	"github.com/navaz-alani/concord/internal"
	"github.com/navaz-alani/concord/packet"
)

func (cr *Crypto) installOnClient(p internal.Processor) error {
	return nil
}

// ConfigureKeyExClientPkt writes the configuration (target, metadata, body,
// etc) for a client-client key-exchange with the given address to the packet.
// The packet being written to must be new and after a call to this method, the
// packet's Write method should not be used.
func (cr *Crypto) ConfigureKeyExClientPkt(addr string, pw packet.Writer) {
	pw.Meta().Add(packet.KeyTarget, TargetKeyExchangeClient)
	pw.Write([]byte(fmt.Sprintf(`{"ip":"%s"}`, addr)))
	pw.Close()
}

// ConfigureKeyExServerPkt writes the configuration (target, metadata, body, etc)
// for a client-server key-exchange. The packet being written to must be new and
// after a call to this method, the packet's Write method should not be used.
func (cr *Crypto) ConfigureKeyExServerPkt(pw packet.Writer) {
	pw.Meta().Add(packet.KeyTarget, TargetKeyExchangeServer)
	pw.Write(cr.publicKey)
	pw.Close()
}

// EncryptFor encrypts the given data for the given address using the
// corresponding shared key. If there is no shared key, or there is an
// encryption error, the error returned will be non-nil an reflect this.
//
// It is used in end-to-end encryption to encrypt outgoing payloads, destined to
// the address. To generate a shared key with the address, a client-client key
// exchange has to be performed.
func (cr *Crypto) EncryptFor(addr string, data []byte) ([]byte, error) {
	if ks, ok := cr.getKeyStore(addr); !ok {
		return nil, fmt.Errorf("keys not exchanged")
	} else if encrypted, err := encryptAES(ks.shared.Bytes(), data); err != nil {
		return nil, fmt.Errorf("encryption error: " + err.Error())
	} else {
		return encrypted, nil
	}
}

// DecryptFrom decrypts the given data using the shared key with the given
// address. The returned error is non-nil if there is no shared key between with
// the address and if there is an error in the decryption process.
//
// It is used in end-to-end encryption to decrypt incoming payloads, from the
// given address. To generate a shared key with the address, a client-client key
// exchange has to be performed.
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
// encrypt packets.
//
// Note that before this function can work, it needs a shared key with the
// destination to which the packet is destined. If a key exchange has been
// successfully performed, then there will most likely be no errors.
func (cr *Crypto) EncryptE2E(pkt packet.Packet) error {
	if encrypted, err := cr.EncryptFor(pkt.Dest(), pkt.Data()); err != nil {
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
