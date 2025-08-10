package crypto

import (
	"encoding/base64"
	"testing"
)

func TestAEAD_EncryptDecrypt_RoundTrip(t *testing.T) {
	keyB64 := "dGVzdGtleXRlc3RrZXl0ZXN0a2V5dGVzdGtleTEyMw==" // base64(32 bytes?)
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(key) != 32 {
		// ensure length 32; if not, construct one
		key = make([]byte, 32)
		for i := range key {
			key[i] = byte(i)
		}
	}

	aead, err := NewAEADCipher(key)
	if err != nil {
		t.Fatalf("NewAEADCipher: %v", err)
	}

	plaintext := []byte("hello secret world")
	aad := []byte("namespace|name|version")

	ct, err := aead.Encrypt(plaintext, aad)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if string(ct) == string(plaintext) {
		t.Fatalf("ciphertext equals plaintext")
	}

	pt, err := aead.Decrypt(ct, aad)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(pt) != string(plaintext) {
		t.Fatalf("roundtrip mismatch: %q != %q", pt, plaintext)
	}
}

func TestAEAD_Decrypt_BadAAD(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(42)
	}
	aead, err := NewAEADCipher(key)
	if err != nil {
		t.Fatal(err)
	}

	ct, err := aead.Encrypt([]byte("data"), []byte("aad"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := aead.Decrypt(ct, []byte("wrong")); err == nil {
		t.Fatalf("expected auth error with wrong AAD")
	}
}

func TestAEAD_NonceReusePrevention(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	aead, err := NewAEADCipher(key)
	if err != nil {
		t.Fatal(err)
	}

	plaintext := []byte("test data")
	aad := []byte("test aad")

	// First encryption should succeed
	ct1, err := aead.Encrypt(plaintext, aad)
	if err != nil {
		t.Fatal(err)
	}

	// Second encryption should succeed (different nonce due to random generation)
	ct2, err := aead.Encrypt(plaintext, aad)
	if err != nil {
		t.Fatal(err)
	}

	// Both should decrypt successfully
	pt1, err := aead.Decrypt(ct1, aad)
	if err != nil {
		t.Fatal(err)
	}
	if string(pt1) != string(plaintext) {
		t.Fatalf("first decryption failed: %q != %q", pt1, plaintext)
	}

	pt2, err := aead.Decrypt(ct2, aad)
	if err != nil {
		t.Fatal(err)
	}
	if string(pt2) != string(plaintext) {
		t.Fatalf("second decryption failed: %q != %q", pt2, plaintext)
	}

	// Verify nonces are different (ciphertexts should be different)
	if string(ct1) == string(ct2) {
		t.Fatal("nonces should be different, ciphertexts should not be identical")
	}
}

func TestAEAD_InvalidMAC(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	aead, err := NewAEADCipher(key)
	if err != nil {
		t.Fatal(err)
	}

	plaintext := []byte("test data")
	aad := []byte("test aad")

	ct, err := aead.Encrypt(plaintext, aad)
	if err != nil {
		t.Fatal(err)
	}

	// Corrupt the ciphertext (change a byte in the middle)
	if len(ct) > 12 { // Ensure we have ciphertext beyond nonce
		ct[20] ^= 0x01 // Flip a bit in the ciphertext
	}

	// Decryption should fail with corrupted ciphertext
	if _, err := aead.Decrypt(ct, aad); err == nil {
		t.Fatal("expected decryption to fail with corrupted ciphertext")
	}
}

func TestAEAD_InvalidKeyLength(t *testing.T) {
	// Test with invalid key lengths
	invalidKeys := [][]byte{
		nil,
		[]byte{},
		make([]byte, 16), // Too short
		make([]byte, 64), // Too long
	}

	for _, key := range invalidKeys {
		if _, err := NewAEADCipher(key); err == nil {
			t.Fatalf("expected error with key length %d, got nil", len(key))
		}
	}
}
