package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"go-web-gin-health/internal/model"
)

type Claims struct {
	UserID                 uint   `json:"user_id"`
	MerchantID             *uint  `json:"merchant_id,omitempty"`
	Identity               string `json:"identity"`
	Role                   string `json:"role"`
	UserCredentialVersion  string `json:"user_credential_version,omitempty"`
	AdminCredentialVersion string `json:"admin_credential_version,omitempty"`
	jwt.RegisteredClaims
}

// AdminCredentialVersion is a keyed digest of the current admin authentication
// and authorization state. It is safe to include in a signed token and changes
// immediately when the password hash, status, role assignment, role code, or
// role permissions change.
func AdminCredentialVersion(secret string, authVersion uint64, passwordHash, status string, roleID uint, roleCode, permissions string) string {
	state, _ := json.Marshal(struct {
		AuthVersion  uint64 `json:"auth_version"`
		PasswordHash string `json:"password_hash"`
		Status       string `json:"status"`
		RoleID       uint   `json:"role_id"`
		RoleCode     string `json:"role_code"`
		Permissions  string `json:"permissions"`
	}{
		AuthVersion:  authVersion,
		PasswordHash: passwordHash,
		Status:       status,
		RoleID:       roleID,
		RoleCode:     roleCode,
		Permissions:  permissions,
	})
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(state)
	return hex.EncodeToString(mac.Sum(nil))
}

// UserCredentialVersion binds a user token to the user's current credentials,
// status, role, and tenant. The keyed digest avoids exposing the password hash
// while making existing tokens stale as soon as that state changes.
func UserCredentialVersion(secret string, user model.User) string {
	role := user.Role
	if role == "" {
		role = "customer"
	}
	state, _ := json.Marshal(struct {
		UserID       uint   `json:"user_id"`
		Email        string `json:"email"`
		Phone        string `json:"phone"`
		PasswordHash string `json:"password_hash"`
		Status       string `json:"status"`
		Role         string `json:"role"`
		MerchantID   *uint  `json:"merchant_id"`
	}{
		UserID:       user.ID,
		Email:        user.Email,
		Phone:        user.Phone,
		PasswordHash: user.PasswordHash,
		Status:       user.Status,
		Role:         role,
		MerchantID:   user.MerchantID,
	})
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte("user-credential-v1\x00"))
	_, _ = mac.Write(state)
	return hex.EncodeToString(mac.Sum(nil))
}

func GenerateToken(secret string, userID uint, identity, role string, merchantID *uint, expireHours int) (string, error) {
	return generateToken(secret, userID, identity, role, merchantID, "", "", expireHours)
}

func GenerateUserToken(secret string, user model.User, expireHours int) (string, error) {
	role := user.Role
	if role == "" {
		role = "customer"
	}
	credentialVersion := UserCredentialVersion(secret, user)
	return generateToken(secret, user.ID, user.Phone, role, user.MerchantID, credentialVersion, "", expireHours)
}

func GenerateAdminToken(secret string, userID uint, identity, role string, credentialVersion string, expireHours int) (string, error) {
	return generateToken(secret, userID, identity, role, nil, "", credentialVersion, expireHours)
}

func generateToken(secret string, userID uint, identity, role string, merchantID *uint, userCredentialVersion, adminCredentialVersion string, expireHours int) (string, error) {
	claims := Claims{
		UserID:                 userID,
		MerchantID:             merchantID,
		Identity:               identity,
		Role:                   role,
		UserCredentialVersion:  userCredentialVersion,
		AdminCredentialVersion: adminCredentialVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseToken(secret, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return claims, nil
}
