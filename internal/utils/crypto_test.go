package utils

import (
	"encoding/base64"
	"strings"
	"testing"
)

const (
	testAESSecret = "0123456789abcdef0123456789abcdef"
	// This fixture was generated with testAESSecret and the legacy deterministic nonce.
	legacyCiphertextFixture = "9cyAWyCzuGuUfE7Y3XBcQCPysbh2sUoNTfe7bLudg2UEJeDlzLNgS+lWxtqIXA=="
	legacyPlaintextFixture  = "legacy payment gateway fixture"
)

func TestDecryptStringLegacyFixedNonceFixture(t *testing.T) {
	plaintext, err := DecryptString(testAESSecret, legacyCiphertextFixture)
	if err != nil {
		t.Fatalf("decrypt legacy fixture: %v", err)
	}
	if plaintext != legacyPlaintextFixture {
		t.Fatalf("unexpected plaintext: got %q, want %q", plaintext, legacyPlaintextFixture)
	}
}

func TestEncryptStringV1RoundTripAndRandomNonce(t *testing.T) {
	const plaintext = "synthetic gateway configuration"

	first, err := EncryptString(testAESSecret, plaintext)
	if err != nil {
		t.Fatalf("first encryption: %v", err)
	}
	second, err := EncryptString(testAESSecret, plaintext)
	if err != nil {
		t.Fatalf("second encryption: %v", err)
	}

	if !strings.HasPrefix(first, encryptedStringV1Prefix) || !strings.HasPrefix(second, encryptedStringV1Prefix) {
		t.Fatalf("new ciphertext must carry the %q version prefix", encryptedStringV1Prefix)
	}
	if first == second {
		t.Fatal("repeated encryption of identical plaintext produced identical ciphertext")
	}

	got, err := DecryptString(testAESSecret, first)
	if err != nil {
		t.Fatalf("decrypt new ciphertext: %v", err)
	}
	if got != plaintext {
		t.Fatalf("round trip mismatch: got %q, want %q", got, plaintext)
	}
}

func TestDecryptStringRejectsMalformedAndTamperedCiphertext(t *testing.T) {
	malformed := []string{
		"v1:not base64!",
		"v1:",
		"v2:AA==",
		"%%%",
	}
	for _, encoded := range malformed {
		t.Run(encoded, func(t *testing.T) {
			if _, err := DecryptString(testAESSecret, encoded); err == nil {
				t.Fatalf("expected malformed ciphertext %q to be rejected", encoded)
			}
		})
	}

	encoded, err := EncryptString(testAESSecret, "synthetic tamper target")
	if err != nil {
		t.Fatalf("encrypt tamper target: %v", err)
	}
	payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(encoded, encryptedStringV1Prefix))
	if err != nil {
		t.Fatalf("decode generated payload: %v", err)
	}
	payload[len(payload)-1] ^= 0x01
	tampered := encryptedStringV1Prefix + base64.StdEncoding.EncodeToString(payload)
	if _, err := DecryptString(testAESSecret, tampered); err == nil {
		t.Fatal("expected tampered ciphertext to be rejected")
	}
}

func TestEncryptStringAndDecryptStringRequireExactly32ByteSecret(t *testing.T) {
	for _, secret := range []string{"too-short", strings.Repeat("s", 31), strings.Repeat("s", 33), strings.Repeat("s", 64)} {
		if _, err := EncryptString(secret, "synthetic plaintext"); err == nil {
			t.Errorf("encryption with %d-byte secret must fail", len(secret))
		}
		if _, err := DecryptString(secret, legacyCiphertextFixture); err == nil {
			t.Errorf("decryption with %d-byte secret must fail", len(secret))
		}
	}
}

func TestDecryptStringRejectsDifferentKeyForLegacyAndVersionedData(t *testing.T) {
	versioned, err := EncryptString(testAESSecret, "synthetic key-mismatch fixture")
	if err != nil {
		t.Fatalf("create versioned fixture: %v", err)
	}
	wrongKey := "X" + testAESSecret[1:]
	for name, ciphertext := range map[string]string{
		"legacy":    legacyCiphertextFixture,
		"versioned": versioned,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecryptString(wrongKey, ciphertext); err == nil {
				t.Fatal("ciphertext decrypted with a different AES key")
			}
		})
	}
}
