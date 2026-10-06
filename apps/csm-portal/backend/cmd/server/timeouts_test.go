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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package main

import (
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadTimeoutsDefaults(t *testing.T) {
	got, err := loadTimeouts(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	want := timeouts{RESTRead: 60 * time.Second, RESTWrite: 60 * time.Second, EntityService: 60 * time.Second}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestLoadTimeoutsOverride(t *testing.T) {
	got, err := loadTimeouts(env(map[string]string{
		"REST_READ_TIMEOUT":      "90s",
		"REST_WRITE_TIMEOUT":     "2m",
		"ENTITY_SERVICE_TIMEOUT": "100s",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := timeouts{RESTRead: 90 * time.Second, RESTWrite: 2 * time.Minute, EntityService: 100 * time.Second}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestLoadTimeoutsInvalid(t *testing.T) {
	for _, tc := range []struct{ key, val string }{
		{"REST_READ_TIMEOUT", "abc"},
		{"REST_READ_TIMEOUT", "30"},
		{"REST_WRITE_TIMEOUT", "0s"},
		{"REST_WRITE_TIMEOUT", "-5s"},
		{"ENTITY_SERVICE_TIMEOUT", "0"},
		{"ENTITY_SERVICE_TIMEOUT", "-1m"},
	} {
		_, err := loadTimeouts(env(map[string]string{tc.key: tc.val}))
		if err == nil {
			t.Errorf("%s=%q: expected error", tc.key, tc.val)
			continue
		}
		if !strings.Contains(err.Error(), tc.key) {
			t.Errorf("%s=%q: error %q does not name the variable", tc.key, tc.val, err)
		}
	}
}
