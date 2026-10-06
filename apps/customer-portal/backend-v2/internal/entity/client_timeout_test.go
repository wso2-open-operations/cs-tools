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

package entity

import (
	"testing"
	"time"
)

func TestNewClient_Timeout(t *testing.T) {
	for name, tc := range map[string]struct {
		in   time.Duration
		want time.Duration
	}{
		"unset uses default":    {0, 60 * time.Second},
		"negative uses default": {-time.Second, 60 * time.Second},
		"override":              {90 * time.Second, 90 * time.Second},
	} {
		t.Run(name, func(t *testing.T) {
			c := NewClient(Config{BaseURL: "http://localhost", Timeout: tc.in})
			if c.http.Timeout != tc.want {
				t.Fatalf("got %s, want %s", c.http.Timeout, tc.want)
			}
		})
	}
}
