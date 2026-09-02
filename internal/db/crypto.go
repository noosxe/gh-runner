package db

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

var (
	// ErrInvalidKey is returned when the encryption key is not 32 bytes (AES-256).
	ErrInvalidKey = errors.New("invalid AES-256 key: must be exactly 32 bytes")

	// ErrInvalidCiphertext is returned when the ciphertext is too short or malformed.
	ErrInvalidCiphertext = errors.New("invalid ciphertext: payload too short or malformed")

	// ErrAuthenticationFailed is returned when AES-GCM tag verification fails (wrong key or tampered data).
	ErrAuthenticationFailed = errors.New("decryption failed: wrong key or tampered ciphertext")

	// ErrNoEncryptor is returned when a credential helper is called but no encryption key was configured.
	ErrNoEncryptor = errors.New("database encryptor is not configured: encryption key required")
)

// Encryptor provides AES-256-GCM authenticated encryption and decryption for sensitive credential columns
// (auth_profiles.private_key_encrypted, token_encrypted) per docs/05 §5.
type Encryptor struct {
	key []byte
}

// NewEncryptor creates a new AES-256-GCM encryptor using a 32-byte key derived via HKDF (RUN-9).
func NewEncryptor(key []byte) (*Encryptor, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("%w: got %d bytes", ErrInvalidKey, len(key))
	}
	k := make([]byte, 32)
	copy(k, key)
	return &Encryptor{key: k}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM with a cryptographically secure random nonce.
// Returns base64(nonce || ciphertext_with_tag). Plaintext and keys are never logged.
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("creating cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded AES-256-GCM ciphertext string.
// If the key is incorrect or ciphertext has been altered, ErrAuthenticationFailed is returned.
// Plaintext is never logged.
func (e *Encryptor) Decrypt(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("%w: base64 decode: %v", ErrInvalidCiphertext, err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("creating cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize+gcm.Overhead() {
		return "", ErrInvalidCiphertext
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrAuthenticationFailed
	}
	return string(plaintext), nil
}

// EncryptNullString encrypts a sql.NullString, returning valid ciphertext NullString or invalid NullString if input is null/empty.
func (e *Encryptor) EncryptNullString(ns sql.NullString) (sql.NullString, error) {
	if !ns.Valid || ns.String == "" {
		return sql.NullString{Valid: false}, nil
	}
	enc, err := e.Encrypt(ns.String)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: enc, Valid: true}, nil
}

// DecryptNullString decrypts a sql.NullString containing ciphertext.
func (e *Encryptor) DecryptNullString(ns sql.NullString) (sql.NullString, error) {
	if !ns.Valid || ns.String == "" {
		return sql.NullString{Valid: false}, nil
	}
	dec, err := e.Decrypt(ns.String)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: dec, Valid: true}, nil
}

// ConstantTimeCompare compares two strings in constant time to protect against timing attacks.
func ConstantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// Encrypt is a package-level helper that encrypts plaintext with the given 32-byte AES key.
func Encrypt(key []byte, plaintext string) (string, error) {
	enc, err := NewEncryptor(key)
	if err != nil {
		return "", err
	}
	return enc.Encrypt(plaintext)
}

// Decrypt is a package-level helper that decrypts ciphertext with the given 32-byte AES key.
func Decrypt(key []byte, ciphertext string) (string, error) {
	enc, err := NewEncryptor(key)
	if err != nil {
		return "", err
	}
	return enc.Decrypt(ciphertext)
}

// DecryptedAuthProfile holds an AuthProfile with its sensitive credentials decrypted.
type DecryptedAuthProfile struct {
	AuthProfile
	PrivateKey string
	Token      string
}

// CreateEncryptedAuthProfile encrypts privateKey and token before inserting into auth_profiles table.
func (d *DB) CreateEncryptedAuthProfile(ctx context.Context, name, authMethod string, appID sql.NullInt64, rawPrivateKey, rawToken string) (AuthProfile, error) {
	if d.encryptor == nil {
		return AuthProfile{}, ErrNoEncryptor
	}
	encPriv, err := d.encryptor.EncryptNullString(sql.NullString{String: rawPrivateKey, Valid: rawPrivateKey != ""})
	if err != nil {
		return AuthProfile{}, fmt.Errorf("encrypting private key: %w", err)
	}
	encTok, err := d.encryptor.EncryptNullString(sql.NullString{String: rawToken, Valid: rawToken != ""})
	if err != nil {
		return AuthProfile{}, fmt.Errorf("encrypting token: %w", err)
	}

	return d.CreateAuthProfile(ctx, CreateAuthProfileParams{
		Name:                name,
		AuthMethod:          authMethod,
		AppID:               appID,
		PrivateKeyEncrypted: encPriv,
		TokenEncrypted:      encTok,
	})
}

// UpdateEncryptedAuthProfile updates an existing auth_profile, re-encrypting credentials.
func (d *DB) UpdateEncryptedAuthProfile(ctx context.Context, id int64, name, authMethod string, appID sql.NullInt64, rawPrivateKey, rawToken string) (AuthProfile, error) {
	if d.encryptor == nil {
		return AuthProfile{}, ErrNoEncryptor
	}
	encPriv, err := d.encryptor.EncryptNullString(sql.NullString{String: rawPrivateKey, Valid: rawPrivateKey != ""})
	if err != nil {
		return AuthProfile{}, fmt.Errorf("encrypting private key: %w", err)
	}
	encTok, err := d.encryptor.EncryptNullString(sql.NullString{String: rawToken, Valid: rawToken != ""})
	if err != nil {
		return AuthProfile{}, fmt.Errorf("encrypting token: %w", err)
	}

	return d.UpdateAuthProfile(ctx, UpdateAuthProfileParams{
		Name:                name,
		AuthMethod:          authMethod,
		AppID:               appID,
		PrivateKeyEncrypted: encPriv,
		TokenEncrypted:      encTok,
		ID:                  id,
	})
}

// GetDecryptedAuthProfileById retrieves an auth profile by ID and decrypts its private key and token.
func (d *DB) GetDecryptedAuthProfileById(ctx context.Context, id int64) (*DecryptedAuthProfile, error) {
	if d.encryptor == nil {
		return nil, ErrNoEncryptor
	}
	profile, err := d.GetAuthProfileById(ctx, id)
	if err != nil {
		return nil, err
	}

	privKey, err := d.encryptor.Decrypt(profile.PrivateKeyEncrypted.String)
	if err != nil {
		return nil, fmt.Errorf("decrypting private key for profile %q: %w", profile.Name, err)
	}
	token, err := d.encryptor.Decrypt(profile.TokenEncrypted.String)
	if err != nil {
		return nil, fmt.Errorf("decrypting token for profile %q: %w", profile.Name, err)
	}

	return &DecryptedAuthProfile{
		AuthProfile: profile,
		PrivateKey:  privKey,
		Token:       token,
	}, nil
}

// GetDecryptedAuthProfileByName retrieves an auth profile by name and decrypts its private key and token.
func (d *DB) GetDecryptedAuthProfileByName(ctx context.Context, name string) (*DecryptedAuthProfile, error) {
	if d.encryptor == nil {
		return nil, ErrNoEncryptor
	}
	profile, err := d.GetAuthProfileByName(ctx, name)
	if err != nil {
		return nil, err
	}

	privKey, err := d.encryptor.Decrypt(profile.PrivateKeyEncrypted.String)
	if err != nil {
		return nil, fmt.Errorf("decrypting private key for profile %q: %w", profile.Name, err)
	}
	token, err := d.encryptor.Decrypt(profile.TokenEncrypted.String)
	if err != nil {
		return nil, fmt.Errorf("decrypting token for profile %q: %w", profile.Name, err)
	}

	return &DecryptedAuthProfile{
		AuthProfile: profile,
		PrivateKey:  privKey,
		Token:       token,
	}, nil
}
