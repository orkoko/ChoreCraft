package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Claims defines the fields stored in the session JWT.
type Claims struct {
	UserID       uuid.UUID `json:"user_id"`
	Role         string    `json:"role"`
	ChoreGroupID uuid.UUID `json:"choregroup_id"`
	Exp          int64     `json:"exp"`
}

// GenerateJWT creates a signed HS256 JWT containing the user session claims.
func GenerateJWT(userID, choregroupID uuid.UUID, role string, duration time.Duration, secret []byte) (string, error) {
	// Base64Url-encoded header
	headerJSON := `{"alg":"HS256","typ":"JWT"}`
	header := base64.RawURLEncoding.EncodeToString([]byte(headerJSON))

	// Claims object
	claims := Claims{
		UserID:       userID,
		Role:         role,
		ChoreGroupID: choregroupID,
		Exp:          time.Now().Add(duration).Unix(),
	}

	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(claimsBytes)

	// Signature computation (HMAC-SHA256)
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(header + "." + payload))
	sig := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	return header + "." + payload + "." + sig, nil
}

// VerifyJWT validates a JWT token and returns its decoded claims.
func VerifyJWT(token string, secret []byte) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	// Signature check
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(parts[0] + "." + parts[1]))
	expectedSig := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return nil, errors.New("invalid token signature")
	}

	// Payload decode
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	var claims Claims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, err
	}

	// Expiry check
	if time.Now().Unix() > claims.Exp {
		return nil, errors.New("token has expired")
	}

	return &claims, nil
}
