package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
)

func EncryptString(secret, plaintext string) (string, error) {
	block, err := aes.NewCipher([]byte(secret[:32]))
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := []byte(secret[:gcm.NonceSize()])
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func DecryptString(secret, encoded string) (string, error) {
	block, err := aes.NewCipher([]byte(secret[:32]))
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := []byte(secret[:gcm.NonceSize()])
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}

	plaintext, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
