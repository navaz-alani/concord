package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

func encryptAES(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return data, fmt.Errorf("cipher init fail: " + err.Error())
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return data, fmt.Errorf("gcm init fail: " + err.Error())
	}
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return data, fmt.Errorf("nonce gen fail: " + err.Error())
	}
	return aesGCM.Seal(nonce, nonce, data, nil), nil
}

func decryptAES(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return data, fmt.Errorf("cipher init fail: " + err.Error())
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return data, fmt.Errorf("gcm init fail: " + err.Error())
	}
	nonceSize := aesGCM.NonceSize()
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	if decrypted, err := aesGCM.Open(nil, nonce, ciphertext, nil); err != nil {
		return data, fmt.Errorf("decrypt fail: " + err.Error())
	} else {
		return decrypted, nil
	}
}
