package github

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strconv"
	"time"
)

var (
	// ErrInvalidPrivateKey is returned when the PEM block is invalid or cannot be parsed as an RSA private key.
	ErrInvalidPrivateKey = errors.New("invalid RSA private key")
)

// ParseRSAPrivateKey parses a PEM-encoded PKCS#1 or PKCS#8 RSA private key.
func ParseRSAPrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("%w: no PEM block found", ErrInvalidPrivateKey)
	}

	// Try PKCS#1 (-----BEGIN RSA PRIVATE KEY-----)
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	// Try PKCS#8 (-----BEGIN PRIVATE KEY-----)
	keyInterface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		if rsaKey, ok := keyInterface.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
		return nil, fmt.Errorf("%w: PKCS#8 key is not RSA", ErrInvalidPrivateKey)
	}

	return nil, fmt.Errorf("%w: failed to parse private key: %v", ErrInvalidPrivateKey, err)
}

// GenerateAppJWT creates a signed RS256 JWT for GitHub App authentication (valid for 10 minutes).
func GenerateAppJWT(appID int64, key *rsa.PrivateKey, now time.Time) (string, error) {
	header := map[string]string{
		"alg": "RS256",
		"typ": "JWT",
	}
	headerBytes, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("marshalling JWT header: %w", err)
	}

	// GitHub recommends iat 60s in the past to account for clock drift, and exp max 10 minutes in future.
	claims := map[string]any{
		"iat": now.Add(-60 * time.Second).Unix(),
		"exp": now.Add(10 * time.Minute).Unix(),
		"iss": strconv.FormatInt(appID, 10),
	}
	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshalling JWT claims: %w", err)
	}

	encodedHeader := base64.RawURLEncoding.EncodeToString(headerBytes)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claimsBytes)
	signingInput := encodedHeader + "." + encodedClaims

	hash := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("signing JWT with RS256: %w", err)
	}

	encodedSig := base64.RawURLEncoding.EncodeToString(sig)
	return signingInput + "." + encodedSig, nil
}
