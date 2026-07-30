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
	"fmt"
	"io"
	"regexp"
	"strings"
)

var (
	// defaultEmailRegexPattern is the default pattern used for broad email address validation.
	// Supports accented letters (\p{L}), digits, and allowed special characters.
	defaultEmailRegexPattern = `^[\p{L}0-9!#$'%*+=?^_{|}~&-]+(?:\.[\p{L}0-9!#$'%*+=?^_{|}~&-]+)*@[\p{L}0-9.\-_]+\.[a-zA-Z]{2,10}$`

	// emailRegex is the compiled regular expression used for email validation.
	emailRegex *regexp.Regexp
)

// InitEmailRegex initializes the email regex with a custom pattern or uses the default.
func InitEmailRegex(pattern string) error {
	if pattern == "" {
		pattern = defaultEmailRegexPattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	emailRegex = re
	return nil
}

func init() {
	// Initialize with default pattern
	_ = InitEmailRegex("")
}

// SanitizeUsername replaces special characters in a username with underscores.
func SanitizeUsername(u string) string {
	return strings.NewReplacer("@", "_", ".", "_", "/", "_", "+", "_").Replace(u)
}

// IsLikelyEmail checks if a string broadly resembles an email address.
func IsLikelyEmail(s string) bool {
	if emailRegex == nil {
		return false
	}
	return emailRegex.MatchString(s)
}

// IsInternalUser checks if a username belongs to an internal user based on the configured suffix.
func IsInternalUser(username, suffix string) bool {
	if suffix == "" {
		return false
	}
	return strings.HasSuffix(username, suffix)
}

// ValidateFolderName checks if a folder name is valid (not empty and no illegal characters).
func ValidateFolderName(name string) error {
	if name == "" {
		return io.ErrShortBuffer // Reusing generic error for empty
	}
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("invalid folder name '%s': contains illegal characters (.., /, or \\)", name)
	}
	return nil
}
