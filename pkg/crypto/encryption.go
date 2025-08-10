package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// AEADCipher provides authenticated encryption with associated data (AEAD)
// using AES-256-GCM.
type AEADCipher struct {
	key []byte
}

// NewAEADCipher creates a new AEAD cipher with the provided 32-byte key.
func NewAEADCipher(key []byte) (*AEADCipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid key length: got %d, want 32", len(key))
	}
	keyCopy := make([]byte, len(key))
	copy(keyCopy, key)
	return &AEADCipher{key: keyCopy}, nil
}

// Encrypt encrypts plaintext with optional associated data (aad).
// It returns nonce||ciphertext, where nonce is 12 random bytes.
func (a *AEADCipher) Encrypt(plaintext, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// 12-byte nonce for GCM
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	sealed := gcm.Seal(nil, nonce, plaintext, aad)
	// prepend nonce
	out := make([]byte, 0, len(nonce)+len(sealed))
	out = append(out, nonce...)
	out = append(out, sealed...)
	return out, nil
}

// Decrypt decrypts data produced by Encrypt. Input must be nonce||ciphertext.
func (a *AEADCipher) Decrypt(data, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(data) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	nonce := data[:gcm.NonceSize()]
	ciphertext := data[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, aad)
}

// Zeroize attempts to clear key material from memory.
func (a *AEADCipher) Zeroize() {
	if a == nil || a.key == nil {
		return
	}
	for i := range a.key {
		a.key[i] = 0
	}
}
