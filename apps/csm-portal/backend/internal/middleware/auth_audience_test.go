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

// Package middleware (internal test) so hasAnyAudience is accessible.
package middleware

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestHasAnyAudience(t *testing.T) {
	tests := []struct {
		name      string
		tokenAuds jwt.ClaimStrings
		expected  []string
		want      bool
	}{
		{
			name:      "exact match — single configured audience",
			tokenAuds: jwt.ClaimStrings{"api.example.com"},
			expected:  []string{"api.example.com"},
			want:      true,
		},
		{
			name:      "first of multiple configured audiences matches",
			tokenAuds: jwt.ClaimStrings{"api.example.com"},
			expected:  []string{"api.example.com", "other.example.com"},
			want:      true,
		},
		{
			name:      "second of multiple configured audiences matches",
			tokenAuds: jwt.ClaimStrings{"other.example.com"},
			expected:  []string{"api.example.com", "other.example.com"},
			want:      true,
		},
		{
			name:      "token carries multiple audiences, one matches",
			tokenAuds: jwt.ClaimStrings{"unrelated.example.com", "api.example.com"},
			expected:  []string{"api.example.com"},
			want:      true,
		},
		{
			name:      "no overlap between token and configured audiences",
			tokenAuds: jwt.ClaimStrings{"other.example.com"},
			expected:  []string{"api.example.com"},
			want:      false,
		},
		{
			name:      "empty token audiences",
			tokenAuds: jwt.ClaimStrings{},
			expected:  []string{"api.example.com"},
			want:      false,
		},
		{
			name:      "empty configured audiences",
			tokenAuds: jwt.ClaimStrings{"api.example.com"},
			expected:  []string{},
			want:      false,
		},
		{
			name:      "both empty",
			tokenAuds: jwt.ClaimStrings{},
			expected:  []string{},
			want:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasAnyAudience(tc.tokenAuds, tc.expected); got != tc.want {
				t.Errorf("hasAnyAudience(%v, %v) = %v, want %v",
					tc.tokenAuds, tc.expected, got, tc.want)
			}
		})
	}
}
