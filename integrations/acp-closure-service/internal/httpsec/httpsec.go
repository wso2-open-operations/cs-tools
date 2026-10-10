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

// Package httpsec holds the transport-safety checks shared by this
// component's two OAuth2 HTTP clients (internal/entity for
// csm-integration-service, internal/emailservice for the email service).
// Both clients send a real client secret on their token request and a bearer
// token on every API call, so both need the same guards; keeping them in one
// place is what stops the two clients drifting apart again (the entity client
// once went without these checks after the email client got them).
package httpsec

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
)

// RequireHTTPS rejects a URL that doesn't use the https scheme — name is the
// config field being checked, used only to make the error message point at
// the right place. A loopback host (127.0.0.1, ::1, localhost) is exempt even
// over plain http: that traffic never leaves the machine, so the
// cleartext-interception risk this check exists for doesn't apply — and it's
// exactly what every client test uses via httptest.NewServer, which only
// ever binds to loopback.
func RequireHTTPS(name, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse %s %q: %w", name, rawURL, err)
	}
	// "https://" alone parses cleanly and would pass a scheme-only check,
	// then fail on every request with a far less obvious error.
	if u.Hostname() == "" {
		return fmt.Errorf("%s has no host: %q", name, rawURL)
	}
	if u.Scheme == "https" || isLoopback(u.Hostname()) {
		return nil
	}
	return fmt.Errorf("%s must use https, got %q", name, rawURL)
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// RefuseRedirects is an http.Client CheckRedirect func that never follows a
// redirect. Needed on both the token client (which POSTs the client secret)
// and the API client (oauth2.Transport reattaches the bearer token to every
// request it processes, including a followed redirect to another host). The
// 3xx response is returned to the caller as-is instead.
func RefuseRedirects(req *http.Request, via []*http.Request) error {
	return http.ErrUseLastResponse
}

// ReadBounded reads all of r, refusing to read more than max bytes. It
// returns an error when the body is larger, instead of buffering an
// arbitrarily large response: both clients read whole response bodies into
// memory, so without a cap a buggy or compromised upstream could exhaust the
// task's memory (security assessment 2026-10-08, CWE-400). Error bodies are
// already capped separately, at 256 bytes.
func ReadBounded(r io.Reader, max int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("response body exceeds %d bytes", max)
	}
	return data, nil
}
