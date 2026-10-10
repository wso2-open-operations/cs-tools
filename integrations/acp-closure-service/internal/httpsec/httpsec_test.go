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

func TestReadBounded(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		max     int64
		wantErr bool
	}{
		{"under the limit", "abc", 5, false},
		{"exactly the limit", "abcde", 5, false},
		{"over the limit", "abcdef", 5, true},
		{"empty", "", 5, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadBounded(strings.NewReader(tt.body), tt.max)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadBounded() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && string(got) != tt.body {
				t.Errorf("ReadBounded() = %q, want %q", got, tt.body)
			}
		})
	}
}
