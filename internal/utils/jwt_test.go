package utils

import (
	"testing"
	"time"
)

const testJWTSecret = "synthetic-jwt-secret-with-at-least-32-bytes"

func TestGenerateAndParseTokenPreservesIdentityAndExpiration(t *testing.T) {
	merchantID := uint(731)
	token, err := GenerateToken(testJWTSecret, 42, "customer@example.test", "customer", &merchantID, 1)
	if err != nil {
		t.Fatalf("generate synthetic token: %v", err)
	}

	claims, err := ParseToken(testJWTSecret, token)
	if err != nil {
		t.Fatalf("parse synthetic token: %v", err)
	}
	if claims.UserID != 42 || claims.Identity != "customer@example.test" || claims.Role != "customer" {
		t.Fatalf("identity claims did not round-trip: %#v", claims)
	}
	if claims.MerchantID == nil || *claims.MerchantID != merchantID {
		t.Fatalf("merchant claim did not round-trip: %#v", claims.MerchantID)
	}
	if claims.ExpiresAt == nil || !claims.ExpiresAt.Time.After(time.Now()) {
		t.Fatalf("token must contain a future expiration: %#v", claims.ExpiresAt)
	}
	if claims.IssuedAt == nil || claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time) < time.Hour-time.Second || claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time) > time.Hour+time.Second {
		t.Fatalf("unexpected one-hour token lifetime: issued=%v expires=%v", claims.IssuedAt, claims.ExpiresAt)
	}
}

func TestParseTokenRejectsExpiredTokenAndWrongSigningSecret(t *testing.T) {
	expired, err := GenerateToken(testJWTSecret, 42, "customer@example.test", "customer", nil, -1)
	if err != nil {
		t.Fatalf("generate expired synthetic token: %v", err)
	}
	if _, err := ParseToken(testJWTSecret, expired); err == nil {
		t.Fatal("expired JWT unexpectedly parsed successfully")
	}

	valid, err := GenerateToken(testJWTSecret, 42, "customer@example.test", "customer", nil, 1)
	if err != nil {
		t.Fatalf("generate valid synthetic token: %v", err)
	}
	if _, err := ParseToken("different-synthetic-signing-secret", valid); err == nil {
		t.Fatal("JWT signed with a different secret unexpectedly parsed successfully")
	}
}

func TestParseTokenRejectsMalformedToken(t *testing.T) {
	for _, token := range []string{"", "not-a-jwt", "a.b.c"} {
		t.Run(token, func(t *testing.T) {
			if _, err := ParseToken(testJWTSecret, token); err == nil {
				t.Fatalf("malformed token %q unexpectedly parsed successfully", token)
			}
		})
	}
}
