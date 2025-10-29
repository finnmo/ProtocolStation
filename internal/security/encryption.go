package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"

	"golang.org/x/crypto/pbkdf2"
)

const (
	// Encryption prefix for encrypted values in config
	EncryptedPrefix = "ENC:"
	// Salt length for PBKDF2 key derivation
	saltLength = 32
	// Nonce length for GCM
	nonceLength = 12
	// PBKDF2 iterations for key derivation
	pbkdf2Iterations = 100000
)

// EncryptValue encrypts a plaintext value using AES-256-GCM with a master key.
// Returns the encrypted value in the format: "ENC:<base64-encoded-data>"
func EncryptValue(plaintext, masterKey string) (string, error) {
	if masterKey == "" {
		return "", fmt.Errorf("master key is required for encryption")
	}

	// Derive key from master password
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	key := pbkdf2.Key([]byte(masterKey), salt, pbkdf2Iterations, 32, sha256.New)

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, nonceLength)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	// Combine salt + nonce + ciphertext
	encryptedData := append(append(salt, nonce...), ciphertext...)

	// Encode and prefix
	encoded := base64.StdEncoding.EncodeToString(encryptedData)
	return EncryptedPrefix + encoded, nil
}

// DecryptValue decrypts an encrypted value.
// Returns the plaintext value.
func DecryptValue(encryptedValue, masterKey string) (string, error) {
	if masterKey == "" {
		return "", fmt.Errorf("master key is required for decryption")
	}

	// Check prefix
	if len(encryptedValue) < len(EncryptedPrefix) || encryptedValue[:len(EncryptedPrefix)] != EncryptedPrefix {
		return encryptedValue, nil // Not encrypted, return as-is
	}

	// Decode base64
	data, err := base64.StdEncoding.DecodeString(encryptedValue[len(EncryptedPrefix):])
	if err != nil {
		return "", fmt.Errorf("failed to decode encrypted value: %w", err)
	}

	if len(data) < saltLength+nonceLength {
		return "", fmt.Errorf("encrypted data too short")
	}

	// Extract components
	salt := data[:saltLength]
	nonce := data[saltLength : saltLength+nonceLength]
	ciphertext := data[saltLength+nonceLength:]

	// Derive key
	key := pbkdf2.Key([]byte(masterKey), salt, pbkdf2Iterations, 32, sha256.New)

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// GetMasterKey retrieves the master key from environment variable or file.
// Priority: BRIDGE_MASTER_KEY env var > master.key file
func GetMasterKey() (string, error) {
	// Try environment variable first
	if key := os.Getenv("BRIDGE_MASTER_KEY"); key != "" {
		return key, nil
	}

	// Try master.key file
	data, err := os.ReadFile("master.key")
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("master key not found: set BRIDGE_MASTER_KEY env var or create master.key file")
		}
		return "", fmt.Errorf("failed to read master.key: %w", err)
	}

	return string(data), nil
}

// DecryptConfigValue decrypts a config value if it's encrypted, otherwise returns it as-is.
func DecryptConfigValue(value string) (string, error) {
	if !IsEncrypted(value) {
		return value, nil
	}

	masterKey, err := GetMasterKey()
	if err != nil {
		return "", fmt.Errorf("failed to get master key: %w", err)
	}

	return DecryptValue(value, masterKey)
}

// IsEncrypted checks if a value is encrypted (starts with ENC: prefix).
func IsEncrypted(value string) bool {
	return len(value) >= len(EncryptedPrefix) && value[:len(EncryptedPrefix)] == EncryptedPrefix
}

