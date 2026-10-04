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

package config

import (
	"strings"
	"testing"
)

func TestLoad_DBSSLModeDefaultsByHost(t *testing.T) {
	cases := []struct {
		host, explicit, want string
	}{
		{"localhost", "", "disable"},
		{"127.0.0.1", "", "disable"},
		{"postgres", "", "disable"}, // local compose service name
		{"db.backing-system.example.com", "", "verify-full"},
		{"db.backing-system.example.com", "require", "require"}, // an explicit value always wins
		{"localhost", "verify-full", "verify-full"},
	}
	for _, tc := range cases {
		t.Run(tc.host+"/"+tc.explicit, func(t *testing.T) {
			t.Setenv("DB_HOST", tc.host)
			t.Setenv("DB_SSLMODE", tc.explicit)
			if got := Load().DBSSLMode; got != tc.want {
				t.Errorf("DBSSLMode = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestConfig_DSN_NeverEmitsEmptySSLMode(t *testing.T) {
	c := baseValidConfig()
	c.DBHost, c.DBPort = "localhost", "5432"
	if dsn := c.DSN(); strings.Contains(dsn, "sslmode") {
		t.Errorf("DSN with no DBSSLMode = %q, want no sslmode parameter", dsn)
	}
	c.DBSSLMode = "verify-full"
	if dsn := c.DSN(); !strings.Contains(dsn, "sslmode=verify-full") {
		t.Errorf("DSN = %q, want sslmode=verify-full", dsn)
	}
}

func TestConfig_Validate_DBSSLMode(t *testing.T) {
	for _, mode := range []string{"", "disable", "allow", "prefer", "require", "verify-ca", "verify-full"} {
		c := baseValidConfig()
		c.DBSSLMode = mode
		if err := c.Validate(); err != nil {
			t.Errorf("DB_SSLMODE %q: unexpected error %v", mode, err)
		}
	}
	c := baseValidConfig()
	c.DBSSLMode = "requried"
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "DB_SSLMODE") {
		t.Errorf("DB_SSLMODE typo: err = %v, want an invalid DB_SSLMODE error", err)
	}
}
