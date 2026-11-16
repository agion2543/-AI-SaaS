package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
)

const encryptedStringV1Prefix = "v1:"

var (
	errInvalidAESSecret        = errors.New("AES secret must be at least 32 bytes")
	errMalformedCiphertext     = errors.New("malformed encrypted string")
	errUnsupportedCiphertext   = errors.New("unsupported encrypted string version")
	errCannotDecryptCiphertext = errors.New("unable to decrypt encrypted string")
)

func newAESGCM(secret string) (cipher.AEAD, error) {
	if len(secret) < 32 {
		return nil, errInvalidAESSecret
	}

	block, err := aes.NewCipher([]byte(secret[:32]))
	if err != nil {
		return nil, err
	}

	return cipher.NewGCM(block)
}

// EncryptString writes versioned ciphertexts with a fresh, random nonce.
func EncryptString(secret, plaintext string) (string, error) {
	gcm, err := newAESGCM(secret)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	payload := append(nonce, ciphertext...)
	return encryptedStringV1Prefix + base64.StdEncoding.EncodeToString(payload), nil
}

// DecryptString reads both versioned ciphertexts and legacy base64 ciphertexts.
// Legacy ciphertexts used the first GCM nonce-size bytes of the secret as nonce.
func DecryptString(secret, encoded string) (string, error) {
	gcm, err := newAESGCM(secret)
	if err != nil {
		return "", err
	}

	version, payload, hasVersion := strings.Cut(encoded, ":")
	if hasVersion {
		if version != "v1" {
			return "", errUnsupportedCiphertext
		}

		data, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return "", errMalformedCiphertext
		}
		if len(data) < gcm.NonceSize()+gcm.Overhead() {
			return "", errMalformedCiphertext
		}

		nonce := data[:gcm.NonceSize()]
		ciphertext := data[gcm.NonceSize():]
		plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			return "", errCannotDecryptCiphertext
		}
		return string(plaintext), nil
	}

	// Unversioned base64 is the historical format. Do not use this nonce for
	// encryption; it is retained only to read existing stored values.
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(ciphertext) < gcm.Overhead() {
		return "", errMalformedCiphertext
	}
	nonce := []byte(secret[:gcm.NonceSize()])
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errCannotDecryptCiphertext
	}
	return string(plaintext), nil
}
