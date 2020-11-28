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
