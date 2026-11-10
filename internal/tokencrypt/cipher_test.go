package tokencrypt

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// secret is a stand-in for a Twitch access token: tests assert it never leaks
// into error messages.
const secret = "oauth2 AuthenticateUser~dX1aBC2efgHIJKLMNopqrst"

// keyB64 returns a base64-encoded random key of n raw bytes.
func keyB64(t *testing.T, n int) string {
	t.Helper()
	raw := make([]byte, n)
	_, err := io.ReadFull(rand.Reader, raw)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(raw)
}

func newCipher(t *testing.T) *Cipher {
	t.Helper()
	c, err := NewCipher(keyB64(t, 32))
	require.NoError(t, err)
	return c
}

func TestRoundTrip(t *testing.T) {
	c := newCipher(t)

	enc, err := c.Encrypt(secret)
	require.NoError(t, err)
	assert.NotEmpty(t, enc)
	assert.NotEqual(t, secret, enc)

	dec, err := c.Decrypt(enc)
	require.NoError(t, err)
	assert.Equal(t, secret, dec)
}

func TestRoundTripEmptyString(t *testing.T) {
	c := newCipher(t)

	enc, err := c.Encrypt("")
	require.NoError(t, err)

	dec, err := c.Decrypt(enc)
	require.NoError(t, err)
	assert.Equal(t, "", dec)
}

func TestRoundTripLongToken(t *testing.T) {
	c := newCipher(t)

	plain := make([]byte, 4096)
	_, err := io.ReadFull(rand.Reader, plain)
	require.NoError(t, err)
	want := string(plain)

	enc, err := c.Encrypt(want)
	require.NoError(t, err)

	dec, err := c.Decrypt(enc)
	require.NoError(t, err)
	assert.Equal(t, want, dec)
}

func TestEncryptUsesRandomNonce(t *testing.T) {
	c := newCipher(t)

	first, err := c.Encrypt(secret)
	require.NoError(t, err)
	second, err := c.Encrypt(secret)
	require.NoError(t, err)

	assert.NotEqual(t, first, second, "each encryption must use a fresh nonce")

	for _, enc := range []string{first, second} {
		dec, err := c.Decrypt(enc)
		require.NoError(t, err)
		assert.Equal(t, secret, dec)
	}
}

func TestWrongKeyFails(t *testing.T) {
	enc, err := newCipher(t).Encrypt(secret)
	require.NoError(t, err)

	other := newCipher(t)
	dec, err := other.Decrypt(enc)

	require.Error(t, err)
	assert.Empty(t, dec)
	assert.NotContains(t, err.Error(), secret)
}

func TestTamperedCiphertextFails(t *testing.T) {
	c := newCipher(t)

	enc, err := c.Encrypt(secret)
	require.NoError(t, err)

	raw, err := base64.StdEncoding.DecodeString(enc)
	require.NoError(t, err)

	// Flip a bit in the trailing tag/ciphertext region.
	raw[len(raw)-1] ^= 0x01
	_, err = c.Decrypt(base64.StdEncoding.EncodeToString(raw))
	require.Error(t, err)

	// Flip a bit in the nonce: authentication must still fail.
	raw2 := append([]byte(nil), raw...)
	raw2[0] ^= 0x01
	_, err = c.Decrypt(base64.StdEncoding.EncodeToString(raw2))
	require.Error(t, err)

	// Truncated payload (nonce only, no ciphertext/tag).
	nonceOnly := base64.StdEncoding.EncodeToString(raw[:12])
	_, err = c.Decrypt(nonceOnly)
	require.Error(t, err)
}

func TestDecryptMalformedInput(t *testing.T) {
	c := newCipher(t)

	for _, in := range []string{"", "not!!base64", "aGVsbG8", "AAAA"} {
		dec, err := c.Decrypt(in)
		require.Error(t, err, in)
		assert.Empty(t, dec)
	}
}

func TestNewCipherRejectsBadKeys(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"empty", ""},
		{"invalid base64", "not!!base64!!"},
		{"too short", keyB64(t, 16)},
		{"31 bytes", keyB64(t, 31)},
		{"33 bytes", keyB64(t, 33)},
		{"too long", keyB64(t, 64)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := NewCipher(tc.key)
			require.Error(t, err)
			assert.Nil(t, c)
		})
	}
}

func TestNewCipherAcceptsExactly32Bytes(t *testing.T) {
	c, err := NewCipher(keyB64(t, 32))
	require.NoError(t, err)
	require.NotNil(t, c)
}
