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

// Package crypto provides authenticated encryption for the few values this
// service must persist but must never store in the clear — currently the
// product-consumption OAuth2 client secret and the two subscription secret
// keys (see internal/repository/project_consumption_repo.go).
//
// This is deliberately the smallest thing that works. It is not a key
// management system: the key arrives as configuration, and there is exactly
// one of them. The ciphertext carries a version byte so that a later move to
// an external KMS, or a second key during rotation, can be told apart from
// what this package writes today without a schema change or a backfill.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// keyLen is the required key size in bytes. AES-256 only — a shorter key is
// rejected rather than stretched, so a misconfigured deployment fails at
// startup instead of silently encrypting with less strength than intended.
const keyLen = 32

// versionAESGCM tags ciphertext produced by aesGCMCodec. Stored as the first
// byte of every value so a future codec can coexist with existing rows.
const versionAESGCM byte = 1

// ErrNoKey reports that no encryption key was configured. Callers that need a
// codec should treat this as fatal at startup rather than falling back to
// storing plaintext.
var ErrNoKey = errors.New("crypto: no encryption key configured")

// SecretCodec encrypts and decrypts values that are persisted as BYTEA.
//
// Implementations must be safe for concurrent use and must authenticate the
// ciphertext — Decrypt is required to fail on tampering rather than return
// damaged plaintext.
type SecretCodec interface {
	// Encrypt returns the sealed form of plaintext. Calling it twice with the
	// same input produces different output; do not use the result for equality
	// comparison or as an index key.
	Encrypt(plaintext string) ([]byte, error)
	// Decrypt reverses Encrypt. It returns an error if the value was produced
	// by a codec this one does not recognise, or if it has been altered.
	Decrypt(ciphertext []byte) (string, error)
}

// aesGCMCodec is the AES-256-GCM implementation of SecretCodec.
type aesGCMCodec struct {
	aead cipher.AEAD
}

// KeyFromBase64 decodes a standard-base64 encryption key and checks its
// length. This is the expected configuration format — a raw 32-byte key is not
// safe to carry in an environment variable.
func KeyFromBase64(encoded string) ([]byte, error) {
	if encoded == "" {
		return nil, ErrNoKey
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		// The key itself is never included in the error — this string reaches
		// startup logs.
		return nil, errors.New("crypto: encryption key is not valid base64")
	}
	if len(key) != keyLen {
		return nil, fmt.Errorf("crypto: encryption key must decode to %d bytes, got %d", keyLen, len(key))
	}
	return key, nil
}

// NewAESGCMCodec constructs a SecretCodec over the given 32-byte key.
func NewAESGCMCodec(key []byte) (SecretCodec, error) {
	if len(key) != keyLen {
		return nil, fmt.Errorf("crypto: encryption key must be %d bytes, got %d", keyLen, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new GCM: %w", err)
	}
	return &aesGCMCodec{aead: aead}, nil
}

// Encrypt implements SecretCodec. The layout is
//
//	[1]byte version | [NonceSize]byte nonce | ciphertext+tag
//
// with the version byte authenticated as additional data, so a value cannot be
// relabelled as some other codec's output without failing the tag check.
func (c *aesGCMCodec) Encrypt(plaintext string) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("crypto: read nonce: %w", err)
	}

	out := make([]byte, 0, 1+len(nonce)+len(plaintext)+c.aead.Overhead())
	out = append(out, versionAESGCM)
	out = append(out, nonce...)
	return c.aead.Seal(out, nonce, []byte(plaintext), out[:1]), nil
}

// Decrypt implements SecretCodec.
func (c *aesGCMCodec) Decrypt(ciphertext []byte) (string, error) {
	nonceSize := c.aead.NonceSize()
	if len(ciphertext) < 1+nonceSize+c.aead.Overhead() {
		return "", errors.New("crypto: ciphertext is too short")
	}
	if ciphertext[0] != versionAESGCM {
		return "", fmt.Errorf("crypto: unsupported ciphertext version %d", ciphertext[0])
	}

	nonce := ciphertext[1 : 1+nonceSize]
	plaintext, err := c.aead.Open(nil, nonce, ciphertext[1+nonceSize:], ciphertext[:1])
	if err != nil {
		// Deliberately not wrapped: the underlying error carries no useful
		// detail and "message authentication failed" in a log invites someone
		// to treat tampering as a transient fault.
		return "", errors.New("crypto: decrypt failed")
	}
	return string(plaintext), nil
}
