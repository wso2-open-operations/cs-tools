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

package httpsec

import (
	"strings"
	"testing"
)

// TestRequireHTTPS covers the startup URL check both OAuth2 clients use.
// Anything that would send the client secret or bearer token in cleartext
// is rejected, and so is a URL with no host at all: "https://" parses
// cleanly and passes a scheme-only check, but every request made with it
// fails later with a far less obvious error than a clear rejection here.
func TestRequireHTTPS(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "https with host", url: "https://csm-integration.example/token", wantErr: false},
		{name: "http loopback IP", url: "http://127.0.0.1:8080/token", wantErr: false},
		{name: "http localhost", url: "http://localhost:8080", wantErr: false},
		{name: "http remote host", url: "http://csm-integration.example/token", wantErr: true},
		{name: "https without host", url: "https://", wantErr: true},
		{name: "https path but no host", url: "https:///token", wantErr: true},
		{name: "empty", url: "", wantErr: true},
		{name: "unparseable", url: "https://%zz", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RequireHTTPS("TokenURL", tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("RequireHTTPS(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

// TestRequireHTTPS_ErrorOmitsURL: callers log the error, so it must never
// carry userinfo or a secret query parameter from the configured URL, nor
// wrap a parse error that embeds it.
func TestRequireHTTPS_ErrorOmitsURL(t *testing.T) {
	const secret = "s3cr3t-value"
	tests := []struct {
		name string
		url  string
	}{
		{name: "http remote with userinfo", url: "http://user:" + secret + "@csm-integration.example/token"},
		{name: "http remote with query secret", url: "http://csm-integration.example/token?client_secret=" + secret},
		{name: "no host with query secret", url: "https:///token?client_secret=" + secret},
		{name: "unparseable with secret", url: "https://%zz/" + secret},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RequireHTTPS("TokenURL", tt.url)
			if err == nil {
				t.Fatalf("RequireHTTPS(%q) = nil, want error", tt.url)
			}
			if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "csm-integration.example") {
				t.Errorf("error %q leaks the configured URL", err)
			}
			if !strings.Contains(err.Error(), "TokenURL") {
				t.Errorf("error %q should name the config field", err)
			}
		})
	}
}
