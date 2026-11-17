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

package main

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"4m", 4 * time.Minute, false},
		{"50s", 50 * time.Second, false},
		{" 90s ", 90 * time.Second, false},
		{"0", 0, false},
		{"0s", 0, false},
		{"abc", 0, true},
		{"", 0, true},
		{"10", 0, true},
		{"-5s", 0, true},
	}
	for _, tc := range tests {
		got, err := parseDuration(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("parseDuration(%q) err = %v, wantErr %v", tc.in, err, tc.wantErr)
			continue
		}
		if err == nil && got != tc.want {
			t.Errorf("parseDuration(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
	if _, err := parseDuration(defaultStreamMaxLifetime); err != nil {
		t.Errorf("default %q must parse: %v", defaultStreamMaxLifetime, err)
	}
}
