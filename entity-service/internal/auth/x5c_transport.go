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

package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// x5cStrippingTransport removes the "x5c" certificate chain from every key in
// a JWKS response before it reaches the jwkset parser. Verification only needs
// the public key material ("n"/"e", or the EC equivalents); jwkset
// unconditionally parses "x5c" as X.509 certificates, and Asgardeo publishes
// certs with a negative serial number that Go's x509 parser has rejected since
// Go 1.23, which would otherwise make the whole JWK Set fail to load. Mirrors
// apps/csm-portal/backend's middleware of the same name.
type x5cStrippingTransport struct {
	base http.RoundTripper
}

func (t *x5cStrippingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return resp, err
	}

	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read JWKS response body: %w", err)
	}

	var jwks struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal(body, &jwks); err != nil {
		// Not a JWKS document we can sanitize; hand back the original body untouched.
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}

	for _, key := range jwks.Keys {
		delete(key, "x5c")
	}
	sanitized, err := json.Marshal(jwks)
	if err != nil {
		return nil, fmt.Errorf("marshal sanitized JWKS: %w", err)
	}

	resp.Body = io.NopCloser(bytes.NewReader(sanitized))
	resp.ContentLength = int64(len(sanitized))
	resp.Header.Set("Content-Length", fmt.Sprint(len(sanitized)))
	return resp, nil
}
