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

package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
)

func sign(secret string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

func TestVerifySignature(t *testing.T) {
	const secret = "s3cr3t"
	body := []byte(`{"action":"labeled"}`)

	tests := []struct {
		name   string
		secret string
		body   []byte
		header string
		wantOK bool
	}{
		{"a correct signature verifies", secret, body, sign(secret, body), true},
		{"a wrong secret does not", secret, body, sign("other", body), false},
		// The signature covers the RAW bytes, so re-encoding the JSON breaks
		// it even when the object is equivalent. This is why the body must be
		// forwarded byte for byte rather than re-marshalled.
		{"the same JSON re-encoded does not", secret, []byte(`{ "action" : "labeled" }`), sign(secret, body), false},
		{"a missing header does not", secret, body, "", false},
		{"a sha1 header is refused", secret, body, "sha1=" + hex.EncodeToString(body), false},
		{"non-hex is refused", secret, body, "sha256=zzzz", false},
		// Refuse rather than skip: an unconfigured deployment must not accept
		// everything that reaches it.
		{"an empty secret refuses everything", "", body, sign(secret, body), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifySignature(tc.secret, tc.body, tc.header)
			if tc.wantOK && err != nil {
				t.Fatalf("expected the signature to verify, got %v", err)
			}
			if !tc.wantOK && err == nil {
				t.Fatal("expected the signature to be refused")
			}
		})
	}
}

// Every rejection must be indistinguishable: a caller learning WHICH check
// failed is a caller being helped to forge one.
func TestVerifySignatureRejectionsAreIndistinguishable(t *testing.T) {
	const secret = "s3cr3t"
	body := []byte(`{}`)
	for _, header := range []string{"", "sha1=aa", "sha256=zz", sign("other", body)} {
		if err := VerifySignature(secret, body, header); !errors.Is(err, ErrInvalidSignature) {
			t.Errorf("header %q: got %v, want ErrInvalidSignature", header, err)
		}
	}
}
