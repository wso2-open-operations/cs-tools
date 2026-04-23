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

package service

import (
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/config"
)

func TestIsInternalUser(t *testing.T) {
	// Create a minimal service instance for testing
	s := &IdPService{
		cfg: &config.Config{InternalUserSuffix: "@wso2.com"},
	}

	tests := []struct {
		name     string
		username string
		want     bool
	}{
		{
			name:     "Internal user with @wso2.com",
			username: "john.doe@wso2.com",
			want:     true,
		},
		{
			name:     "External user with different domain",
			username: "jane.smith@example.com",
			want:     false,
		},
		{
			name:     "Empty string",
			username: "",
			want:     false,
		},
		{
			name:     "No @ symbol",
			username: "username",
			want:     false,
		},
		{
			name:     "Multiple @ symbols",
			username: "user@name@wso2.com",
			want:     true,
		},
		{
			name:     "Case sensitive - lowercase",
			username: "user@wso2.com",
			want:     true,
		},
		{
			name:     "Case sensitive - uppercase domain",
			username: "user@WSO2.COM",
			want:     false,
		},
		{
			name:     "Subdomain",
			username: "user@mail.wso2.com",
			want:     false, // HasSuffix checks exact suffix, not domain matching
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.isInternalUser(tt.username)
			if got != tt.want {
				t.Errorf("isInternalUser(%q) = %v, want %v", tt.username, got, tt.want)
			}
		})
	}
}
