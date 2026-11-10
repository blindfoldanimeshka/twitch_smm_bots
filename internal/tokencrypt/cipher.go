// Package tokencrypt encrypts and decrypts sensitive values (Twitch tokens) at
// rest using AES-256-GCM. The ciphertext is returned as base64 of
// nonce||ciphertext||tag and is safe to store in a text column.
package tokencrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// keySize is the required length in bytes of the raw encryption key.
const keySize = 32

// Cipher encrypts and decrypts single values with a fixed AES-256-GCM key.
// It is safe for concurrent use.
type Cipher struct {
	aead cipher.AEAD
}

// NewCipher builds a Cipher from a base64-encoded key. The decoded key must be
// exactly 32 bytes (TOKEN_ENCRYPTION_KEY); an empty, malformed or wrong-length
// key is rejected. The error never contains the key material.
func NewCipher(keyBase64 string) (*Cipher, error) {
	if keyBase64 == "" {
		return nil, errors.New("tokencrypt: encryption key is empty (set TOKEN_ENCRYPTION_KEY)")
	}

	raw, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, fmt.Errorf("tokencrypt: decode encryption key: %w", err)
	}
	if len(raw) != keySize {
		return nil, fmt.Errorf("tokencrypt: encryption key must be %d bytes, got %d", keySize, len(raw))
	}

	block, err := aes.NewCipher(raw)
	if err != nil {
		return nil, fmt.Errorf("tokencrypt: create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("tokencrypt: create gcm: %w", err)
	}

	return &Cipher{aead: aead}, nil
}

// Encrypt seals plain under a fresh random nonce and returns
// base64(nonce||ciphertext||tag). Encrypting the same value twice yields
// different output.
func (c *Cipher) Encrypt(plain string) (string, error) {
	if c == nil || c.aead == nil {
		return "", errors.New("tokencrypt: cipher is not initialised")
	}

	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("tokencrypt: generate nonce: %w", err)
	}

	sealed := c.aead.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt reverses Encrypt. Any integrity failure (wrong key, tampered or
// truncated ciphertext) returns an error that never exposes the plaintext.
func (c *Cipher) Decrypt(enc string) (string, error) {
	if c == nil || c.aead == nil {
		return "", errors.New("tokencrypt: cipher is not initialised")
	}

	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", fmt.Errorf("tokencrypt: decode ciphertext: %w", err)
	}

	nonceSize := c.aead.NonceSize()
	// The shortest valid payload is nonce||tag (an empty plaintext still carries
	// a tag).
	if len(raw) < nonceSize+c.aead.Overhead() {
		return "", errors.New("tokencrypt: ciphertext too short")
	}

	plain, err := c.aead.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("tokencrypt: decrypt: %w", err)
	}

	return string(plain), nil
}
