// Package secrets provides encryption + decryption of stored automation token
// secrets using AES-256-GCM. The key is loaded from a file at startup.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
)

// Key is a symmetric encryption key for sealing stored token secrets.
type Key struct {
	bytes []byte
}

// LoadKey reads a 32-byte key from the file at path. If the file doesn't
// exist, it returns nil + no error — callers check for nil and degrade
// (token encryption disabled).
func LoadKey(path string) (*Key, error) {
	data, err := os.ReadFile(path) //nolint:gosec // operator-controlled key path
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read key file %s: %w", path, err)
	}
	if len(data) != 32 {
		return nil, fmt.Errorf("key file %s: expected 32 bytes, got %d", path, len(data))
	}
	return &Key{bytes: data}, nil
}

// Seal encrypts plaintext with AES-256-GCM. Returns nonce||ciphertext.
func (k *Key) Seal(plaintext []byte) ([]byte, error) {
	if k == nil {
		return nil, errors.New("secrets: key is nil")
	}
	block, err := aes.NewCipher(k.bytes)
	if err != nil {
		return nil, fmt.Errorf("aes new: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm new: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Open decrypts a Seal() output.
func (k *Key) Open(ciphertext []byte) ([]byte, error) {
	if k == nil {
		return nil, errors.New("secrets: key is nil")
	}
	block, err := aes.NewCipher(k.bytes)
	if err != nil {
		return nil, fmt.Errorf("aes new: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm new: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("secrets: ciphertext too short")
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}
