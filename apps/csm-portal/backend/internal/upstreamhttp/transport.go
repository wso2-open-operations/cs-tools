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

// Package upstreamhttp holds the one HTTP transport every OAuth2-authenticated
// upstream client in this backend shares.
package upstreamhttp

import (
	"net/http"
	"time"
)

// Transport is the connection pool shared by every upstream client. Before it
// existed each client fell back to http.DefaultTransport, whose
// MaxIdleConnsPerHost of 2 closes all but two idle connections to a host after
// each burst, so concurrent calls to the entity service paid a fresh TLS
// handshake per request. Pass it as the Transport of the http.Client stored
// under oauth2.HTTPClient: oauth2's own Transport then uses it as its Base for
// API calls as well as token fetches.
var Transport http.RoundTripper = newTransport()

func newTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 200
	t.MaxIdleConnsPerHost = 50
	t.IdleConnTimeout = 90 * time.Second
	return t
}

// TokenClient returns the http.Client an oauth2 token source should use: the
// shared Transport with the given overall timeout.
func TokenClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: Transport}
}
