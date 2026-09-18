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

package license

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Canonicalize renders a payload as the string the signature is computed over.
//
// Keys are sorted alphabetically and each key is concatenated directly with its
// value, with no separator of any kind — "clientId" + <clientId> + "clientSecret"
// + <clientSecret> + ... A string value is used as-is; anything else is JSON
// encoded first.
//
// Sorting is plain lexicographic, which is why secretKey10 and secretKey11 sort
// between secretKey1 and secretKey2 when the payload still carried the object
// form of secrets. With the flattened form that no longer arises, but the rule
// is unchanged.
//
// The absence of separators is load-bearing and fragile: it means the canonical
// string cannot be unambiguously parsed back, and that two different payloads
// can in principle produce the same string. It is reproduced here because the
// verifier in every deployed customer product already implements it.
func Canonicalize(payload map[string]any) (string, error) {
	keys := make([]string, 0, len(payload))
	for k := range payload {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k)
		switch v := payload[k].(type) {
		case string:
			sb.WriteString(v)
		default:
			encoded, err := json.Marshal(v)
			if err != nil {
				return "", fmt.Errorf("license: canonicalize %q: %w", k, err)
			}
			sb.Write(encoded)
		}
	}
	return sb.String(), nil
}

// Sign computes the licence signature: HMAC-SHA256 over the canonicalised
// payload, keyed on the primary secret key, returned base64-encoded.
//
// primaryKey arrives as the 64-character hex string the key is stored as; the
// MAC is keyed on the 32 raw bytes it decodes to. ServiceNow's own
// GlideCertificateEncryption.generateMac takes that key base64-encoded, but
// base64 is only how that API accepts the bytes — it is not part of the MAC
// input. Keying on the hex text, or on its base64 form, produces a signature no
// customer product will accept.
func Sign(payload map[string]any, primaryKey string) (string, error) {
	if len(primaryKey) != keyHexLen {
		return "", fmt.Errorf("license: primary key must be %d hex characters, got %d", keyHexLen, len(primaryKey))
	}
	raw, err := hex.DecodeString(primaryKey)
	if err != nil {
		return "", fmt.Errorf("license: primary key is not valid hex: %w", err)
	}

	canonical, err := Canonicalize(payload)
	if err != nil {
		return "", err
	}

	mac := hmac.New(sha256.New, raw)
	mac.Write([]byte(canonical))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

// Verify reports whether signature matches payload under primaryKey.
//
// Compares in constant time. This is the operation every deployed customer
// product performs on the licence it holds, so it is also the only honest test
// that this package's Sign is correct.
func Verify(payload map[string]any, primaryKey, signature string) (bool, error) {
	expected, err := Sign(payload, primaryKey)
	if err != nil {
		return false, err
	}
	return hmac.Equal([]byte(expected), []byte(signature)), nil
}
