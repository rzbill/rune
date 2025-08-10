package crypto

import "crypto/rand"

// RandomBytes returns n cryptographically-secure random bytes.
func RandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	return b, err
}
