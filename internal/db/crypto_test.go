package db

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// TestCryptoRoundTrip verifies that plaintext encrypts and decrypts cleanly with the correct key.
func TestCryptoRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generating random key: %v", err)
	}

	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("NewEncryptor failed: %v", err)
	}

	samples := []string{
		"ghp_testpersonalaccesstoken1234567890",
		"-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA0...\n-----END RSA PRIVATE KEY-----",
		"gitea_token_secret_123456",
		"forgejo_token_abc_xyz",
		"unicode_secret_🔒_привет_世界",
	}

	for _, sample := range samples {
		ciphertext, err := enc.Encrypt(sample)
		if err != nil {
			t.Fatalf("Encrypt failed for sample: %v", err)
		}
		if ciphertext == sample {
			t.Fatal("ciphertext must not equal plaintext")
		}

		decrypted, err := enc.Decrypt(ciphertext)
		if err != nil {
			t.Fatalf("Decrypt failed: %v", err)
		}
		if decrypted != sample {
			t.Fatalf("round-trip mismatch: got %q, want %q", decrypted, sample)
		}
	}
}

// TestCryptoEmptyAndNullStrings verifies handling of empty and null inputs.
func TestCryptoEmptyAndNullStrings(t *testing.T) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)

	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	encEmpty, err := enc.Encrypt("")
	if err != nil || encEmpty != "" {
		t.Fatalf("Encrypt empty string should return empty string, got %q (err: %v)", encEmpty, err)
	}

	decEmpty, err := enc.Decrypt("")
	if err != nil || decEmpty != "" {
		t.Fatalf("Decrypt empty string should return empty string, got %q (err: %v)", decEmpty, err)
	}

	// NullString handling
	encNull, err := enc.EncryptNullString(sql.NullString{Valid: false})
	if err != nil || encNull.Valid {
		t.Fatalf("EncryptNullString with invalid NullString should return invalid NullString, got %+v", encNull)
	}

	decNull, err := enc.DecryptNullString(sql.NullString{Valid: false})
	if err != nil || decNull.Valid {
		t.Fatalf("DecryptNullString with invalid NullString should return invalid NullString, got %+v", decNull)
	}
}

// TestCryptoWrongKeyFailure verifies that decrypting with the wrong key returns ErrAuthenticationFailed.
func TestCryptoWrongKeyFailure(t *testing.T) {
	key1 := bytes.Repeat([]byte{0x01}, 32)
	key2 := bytes.Repeat([]byte{0x02}, 32)

	enc1, err := NewEncryptor(key1)
	if err != nil {
		t.Fatalf("NewEncryptor key1: %v", err)
	}
	enc2, err := NewEncryptor(key2)
	if err != nil {
		t.Fatalf("NewEncryptor key2: %v", err)
	}

	ciphertext, err := enc1.Encrypt("super-secret-token")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	_, err = enc2.Decrypt(ciphertext)
	if !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("expected ErrAuthenticationFailed on wrong key, got: %v", err)
	}
}

// TestCryptoTamperDetection verifies that any modification to the ciphertext or tag fails decryption.
func TestCryptoTamperDetection(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	ciphertext, err := enc.Encrypt("confidential-private-key")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		t.Fatalf("DecodeString: %v", err)
	}

	// Tamper with byte in the middle of ciphertext
	tampered := make([]byte, len(raw))
	copy(tampered, raw)
	tampered[len(tampered)/2] ^= 0xFF

	_, err = enc.Decrypt(base64.StdEncoding.EncodeToString(tampered))
	if !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("expected ErrAuthenticationFailed on tampered ciphertext, got: %v", err)
	}

	// Truncated payload
	truncated := raw[:len(raw)-10]
	_, err = enc.Decrypt(base64.StdEncoding.EncodeToString(truncated))
	if err == nil {
		t.Fatal("expected error on truncated ciphertext, got nil")
	}

	// Completely invalid base64
	_, err = enc.Decrypt("not-valid-base64!@#$%^")
	if !errors.Is(err, ErrInvalidCiphertext) {
		t.Fatalf("expected ErrInvalidCiphertext on invalid base64, got: %v", err)
	}
}

// TestCryptoKeyValidation verifies that only 32-byte keys are accepted for AES-256.
func TestCryptoKeyValidation(t *testing.T) {
	for _, size := range []int{0, 16, 24, 31, 33, 64} {
		_, err := NewEncryptor(bytes.Repeat([]byte{0xAA}, size))
		if !errors.Is(err, ErrInvalidKey) {
			t.Errorf("NewEncryptor with %d bytes: expected ErrInvalidKey, got: %v", size, err)
		}
	}
}

// TestCryptoNoPlaintextLeak verifies that error messages never contain plaintext secrets.
func TestCryptoNoPlaintextLeak(t *testing.T) {
	secret := "LEAK_TEST_SECRET_DO_NOT_EXPOSE"
	key := bytes.Repeat([]byte{0x77}, 32)
	enc, _ := NewEncryptor(key)

	ct, _ := enc.Encrypt(secret)

	// Attempt decrypt with wrong key
	wrongKey := bytes.Repeat([]byte{0x88}, 32)
	wrongEnc, _ := NewEncryptor(wrongKey)
	_, err := wrongEnc.Decrypt(ct)
	if err != nil && strings.Contains(err.Error(), secret) {
		t.Fatalf("error message leaked plaintext secret: %v", err)
	}
}

// TestCryptoConstantTimeCompare verifies constant-time string comparison helper.
func TestCryptoConstantTimeCompare(t *testing.T) {
	if !ConstantTimeCompare("secret_token_123", "secret_token_123") {
		t.Error("expected equal strings to return true")
	}
	if ConstantTimeCompare("secret_token_123", "secret_token_456") {
		t.Error("expected unequal strings to return false")
	}
	if ConstantTimeCompare("short", "longer_string") {
		t.Error("expected different-length strings to return false")
	}
}

// TestAuthProfileCredentialHelpersRoundTrip exercises DB-level encrypted auth profile methods.
func TestAuthProfileCredentialHelpersRoundTrip(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "crypto_profiles.db")
	aesKey := bytes.Repeat([]byte{0x33}, 32)

	database, err := Open(Options{
		Path:          dbPath,
		EncryptionKey: aesKey,
	})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	rawPrivKey := "-----BEGIN RSA PRIVATE KEY-----\nMIIEogIBAAKCAQEA..."
	rawToken := "ghp_secure_github_pat_token_999"

	// Create encrypted profile
	profile, err := database.CreateEncryptedAuthProfile(ctx, "prod-github-app", "github_app", sql.NullInt64{Int64: 4242, Valid: true}, rawPrivKey, rawToken)
	if err != nil {
		t.Fatalf("CreateEncryptedAuthProfile failed: %v", err)
	}

	// Verify that database row contains ciphertext, NOT plaintext
	rawRow, err := database.GetAuthProfileById(ctx, profile.ID)
	if err != nil {
		t.Fatalf("GetAuthProfileById failed: %v", err)
	}
	if rawRow.PrivateKeyEncrypted.String == rawPrivKey {
		t.Fatal("private key stored as plaintext in SQLite database!")
	}
	if rawRow.TokenEncrypted.String == rawToken {
		t.Fatal("token stored as plaintext in SQLite database!")
	}

	// Retrieve decrypted by ID
	decByID, err := database.GetDecryptedAuthProfileById(ctx, profile.ID)
	if err != nil {
		t.Fatalf("GetDecryptedAuthProfileById failed: %v", err)
	}
	if decByID.PrivateKey != rawPrivKey {
		t.Errorf("decrypted private key mismatch: got %q, want %q", decByID.PrivateKey, rawPrivKey)
	}
	if decByID.Token != rawToken {
		t.Errorf("decrypted token mismatch: got %q, want %q", decByID.Token, rawToken)
	}

	// Retrieve decrypted by Name
	decByName, err := database.GetDecryptedAuthProfileByName(ctx, "prod-github-app")
	if err != nil {
		t.Fatalf("GetDecryptedAuthProfileByName failed: %v", err)
	}
	if decByName.PrivateKey != rawPrivKey || decByName.Token != rawToken {
		t.Errorf("decrypted by name mismatch")
	}

	// Update encrypted profile
	updatedPrivKey := "-----BEGIN RSA PRIVATE KEY-----\nUPDATED_KEY..."
	updatedToken := "ghp_updated_pat_888"
	_, err = database.UpdateEncryptedAuthProfile(ctx, profile.ID, "prod-github-app-v2", "github_app", sql.NullInt64{Int64: 4242, Valid: true}, updatedPrivKey, updatedToken)
	if err != nil {
		t.Fatalf("UpdateEncryptedAuthProfile failed: %v", err)
	}

	decUpdated, err := database.GetDecryptedAuthProfileById(ctx, profile.ID)
	if err != nil {
		t.Fatalf("GetDecryptedAuthProfileById after update: %v", err)
	}
	if decUpdated.PrivateKey != updatedPrivKey || decUpdated.Token != updatedToken || decUpdated.Name != "prod-github-app-v2" {
		t.Errorf("unexpected updated profile: %+v", decUpdated)
	}
}

// TestAuthProfileHelperWithoutKeyFails verifies ErrNoEncryptor if DB has no encryption key.
func TestAuthProfileHelperWithoutKeyFails(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "no_crypto.db")

	database, err := Open(Options{Path: dbPath})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = database.Close() }()

	ctx := context.Background()

	_, err = database.CreateEncryptedAuthProfile(ctx, "test", "pat", sql.NullInt64{}, "", "secret")
	if !errors.Is(err, ErrNoEncryptor) {
		t.Fatalf("expected ErrNoEncryptor, got: %v", err)
	}

	_, err = database.GetDecryptedAuthProfileById(ctx, 1)
	if !errors.Is(err, ErrNoEncryptor) {
		t.Fatalf("expected ErrNoEncryptor, got: %v", err)
	}
}
