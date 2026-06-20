package offercodes

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

// RandomGenerator mints canonical offer codes from cryptographically secure
// randomness: 60 bits (12 × 5) drawn from 8 random bytes, each 5-bit group
// mapped through the Crockford alphabet.
type RandomGenerator struct{}

// NewRandomGenerator returns a crypto/rand-backed generator.
func NewRandomGenerator() RandomGenerator { return RandomGenerator{} }

// Generate returns a fresh canonical 12-character code. It errors only if the
// system CSPRNG is unavailable.
func (RandomGenerator) Generate() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	value := binary.LittleEndian.Uint64(b[:])

	var buf [canonicalLength]byte
	for i := canonicalLength - 1; i >= 0; i-- {
		buf[i] = alphabet[value&0x1F]
		value >>= 5
	}
	return string(buf[:]), nil
}
