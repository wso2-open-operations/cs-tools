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

package util

import (
	"regexp"
	"testing"
)

func TestInitEmailRegex(t *testing.T) {
	// Test default pattern
	err := InitEmailRegex("")
	if err != nil {
		t.Fatalf("Failed to compile default regex: %v", err)
	}

	if emailRegex == nil {
		t.Fatal("emailRegex is nil after initialization")
	}

	// Test a simple valid email
	if !IsLikelyEmail("test@example.com") {
		t.Error("Default regex should match test@example.com")
	}
}

func TestEmailRegexCompilation(t *testing.T) {
	// Test that the refined simple default pattern compiles
	pattern := `^[\p{L}0-9!#$'%*+=?^_{|}~&-]+(?:\.[\p{L}0-9!#$'%*+=?^_{|}~&-]+)*@[\p{L}0-9.\-_]+\.[a-zA-Z]{2,10}$`

	_, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("Default pattern failed to compile: %v", err)
	}
}
