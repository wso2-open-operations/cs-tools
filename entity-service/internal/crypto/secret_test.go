// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package crypto

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

// testKey is a fixed all-zero key. Test-only: never a default anywhere in
// non-test code, which is why NewAESGCMCodec has no zero-value fallback.
var testKey = make([]byte, keyLen)

func newTestCodec(t *testing.T) SecretCodec {
	t.Helper()
	codec, err := NewAESGCMCodec(testKey)
	if err != nil {
		t.Fatalf("NewAESGCMCodec: %v", err)
	}
	return codec
}

func TestAESGCM_RoundTrip(t *testing.T) {
	codec := newTestCodec(t)

	for _, plaintext := range []string{
		"",
		"consumer-secret",
		strings.Repeat("x", 4096),
		"unicode ✓ and \x00 embedded NUL",
	} {
		sealed, err := codec.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Encrypt(%q): %v", plaintext, err)
		}
		got, err := codec.Decrypt(sealed)
		if err != nil {
			t.Fatalf("Decrypt(%q): %v", plaintext, err)
		}
		if got != plaintext {
			t.Fatalf("round trip: got %q, want %q", got, plaintext)
		}
	}
}

// TestAESGCM_CiphertextDoesNotContainPlaintext is the property that actually
// matters for the migration: these values reach a database column, and a
// reviewer must be able to rule out that the secret is sitting there in the
// clear.
func TestAESGCM_CiphertextDoesNotContainPlaintext(t *testing.T) {
	codec := newTestCodec(t)
	const secret = "super-secret-consumer-key-value"

	sealed, err := codec.Encrypt(secret)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if bytes.Contains(sealed, []byte(secret)) {
		t.Fatal("ciphertext contains the plaintext")
	}
}

// TestAESGCM_NonDeterministic guards the nonce actually varying. Identical
// output for identical input would leak which projects share a credential.
func TestAESGCM_NonDeterministic(t *testing.T) {
	codec := newTestCodec(t)

	first, err := codec.Encrypt("same input")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	second, err := codec.Encrypt("same input")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("two encryptions of the same plaintext produced identical ciphertext")
	}
}

func TestAESGCM_DetectsTampering(t *testing.T) {
	codec := newTestCodec(t)

	sealed, err := codec.Encrypt("consumer-secret")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	tests := map[string]func([]byte) []byte{
		"flipped byte in body": func(b []byte) []byte {
			out := bytes.Clone(b)
			out[len(out)-1] ^= 0xFF
			return out
		},
		"flipped byte in nonce": func(b []byte) []byte {
			out := bytes.Clone(b)
			out[2] ^= 0xFF
			return out
		},
		"relabelled version byte": func(b []byte) []byte {
			out := bytes.Clone(b)
			out[0] = 9
			return out
		},
		"truncated": func(b []byte) []byte {
			return b[:len(b)-1]
		},
		"empty": func([]byte) []byte {
			return nil
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := codec.Decrypt(mutate(sealed)); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}

// TestAESGCM_WrongKeyFails covers the rotation mistake — pointing a deployment
// at a different key must fail loudly rather than return damaged plaintext.
func TestAESGCM_WrongKeyFails(t *testing.T) {
	codec := newTestCodec(t)
	sealed, err := codec.Encrypt("consumer-secret")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	otherKey := bytes.Repeat([]byte{0x01}, keyLen)
	other, err := NewAESGCMCodec(otherKey)
	if err != nil {
		t.Fatalf("NewAESGCMCodec: %v", err)
	}
	if _, err := other.Decrypt(sealed); err == nil {
		t.Fatal("decrypting with a different key succeeded")
	}
}

// TestAESGCM_DecryptErrorOmitsPlaintext checks the error text is safe to log.
func TestAESGCM_DecryptErrorOmitsPlaintext(t *testing.T) {
	codec := newTestCodec(t)
	const secret = "secret-value-that-must-not-leak"

	sealed, err := codec.Encrypt(secret)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	sealed[len(sealed)-1] ^= 0xFF

	_, err = codec.Decrypt(sealed)
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error text leaks the plaintext: %v", err)
	}
}

func TestKeyFromBase64(t *testing.T) {
	valid := base64.StdEncoding.EncodeToString(testKey)

	t.Run("valid", func(t *testing.T) {
		key, err := KeyFromBase64(valid)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(key) != keyLen {
			t.Fatalf("got %d bytes, want %d", len(key), keyLen)
		}
	})

	t.Run("empty reports ErrNoKey", func(t *testing.T) {
		if _, err := KeyFromBase64(""); !errors.Is(err, ErrNoKey) {
			t.Fatalf("got %v, want ErrNoKey", err)
		}
	})

	t.Run("not base64", func(t *testing.T) {
		if _, err := KeyFromBase64("not base64!!"); err == nil {
			t.Fatal("expected an error")
		}
	})

	// A short key must be rejected outright rather than padded or stretched —
	// otherwise a deployment can silently run with less strength than intended.
	t.Run("wrong length", func(t *testing.T) {
		short := base64.StdEncoding.EncodeToString(make([]byte, 16))
		if _, err := KeyFromBase64(short); err == nil {
			t.Fatal("expected an error for a 16-byte key")
		}
	})

	// The key must never appear in an error that reaches startup logs.
	t.Run("error omits the key", func(t *testing.T) {
		short := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0xAB}, 16))
		_, err := KeyFromBase64(short)
		if err == nil {
			t.Fatal("expected an error")
		}
		if strings.Contains(err.Error(), short) {
			t.Fatalf("error text leaks the key: %v", err)
		}
	})
}
