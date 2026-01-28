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
	"testing"
)

func TestSanitizeUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		want     string
	}{
		{
			name:     "Email with @ and dot",
			username: "user@example.com",
			want:     "user_example_com",
		},
		{
			name:     "Email with subdomain",
			username: "john.doe@mail.example.com",
			want:     "john_doe_mail_example_com",
		},
		{
			name:     "Username with forward slash",
			username: "DEFAULT/user@example.com",
			want:     "DEFAULT_user_example_com",
		},
		{
			name:     "Username with plus",
			username: "user+tag@example.com",
			want:     "user_tag_example_com",
		},
		{
			name:     "Already sanitized",
			username: "user_example_com",
			want:     "user_example_com",
		},
		{
			name:     "Empty string",
			username: "",
			want:     "",
		},
		{
			name:     "Multiple special chars",
			username: "user@name.with+slash/test",
			want:     "user_name_with_slash_test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeUsername(tt.username)
			if got != tt.want {
				t.Errorf("SanitizeUsername(%q) = %q, want %q", tt.username, got, tt.want)
			}
		})
	}
}

func TestIsLikelyEmail(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "Valid email",
			input: "user@example.com",
			want:  true,
		},
		{
			name:  "Valid email with subdomain",
			input: "john.doe@mail.example.com",
			want:  true,
		},
		{
			name:  "Valid email with plus",
			input: "user+tag@example.com",
			want:  true,
		},
		{
			name:  "Valid email with special chars",
			input: "user!#$@example.com",
			want:  true,
		},
		{
			name:  "Valid email with accented letters",
			input: "josé@example.com",
			want:  true,
		},
		{
			name:  "No @ symbol",
			input: "username",
			want:  false,
		},
		{
			name:  "No domain",
			input: "user@",
			want:  false,
		},
		{
			name:  "No TLD",
			input: "user@example",
			want:  false,
		},
		{
			name:  "Empty string",
			input: "",
			want:  false,
		},
		{
			name:  "Multiple @ symbols",
			input: "user@@example.com",
			want:  false,
		},
		{
			name:  "Spaces in email",
			input: "user @example.com",
			want:  false,
		},
		{
			name:  "Valid with numbers",
			input: "user123@example123.com",
			want:  true,
		},
		{
			name:  "TLD too long (>10 chars)",
			input: "user@example.verylongtld",
			want:  false, // Regex restricts TLD length
		},
		{
			name:  "Starting with dot",
			input: ".user@example.com",
			want:  false, // Regex starts with non-dot atom
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsLikelyEmail(tt.input)
			if got != tt.want {
				t.Errorf("IsLikelyEmail(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateFolderName(t *testing.T) {
	tests := []struct {
		name       string
		folderName string
		wantError  bool
	}{
		{
			name:       "Valid folder name",
			folderName: "project1",
			wantError:  false,
		},
		{
			name:       "Valid folder with underscore",
			folderName: "project_folder_1",
			wantError:  false,
		},
		{
			name:       "Valid folder with hyphen",
			folderName: "project-folder-1",
			wantError:  false,
		},
		{
			name:       "Empty folder name",
			folderName: "",
			wantError:  true,
		},
		{
			name:       "Folder with parent directory traversal",
			folderName: "../etc",
			wantError:  true,
		},
		{
			name:       "Folder with double dots in middle",
			folderName: "folder..name",
			wantError:  true,
		},
		{
			name:       "Folder with forward slash",
			folderName: "folder/name",
			wantError:  true,
		},
		{
			name:       "Folder with backslash",
			folderName: "folder\\name",
			wantError:  true,
		},
		{
			name:       "Folder with leading slash",
			folderName: "/folder",
			wantError:  true,
		},
		{
			name:       "Folder with trailing slash",
			folderName: "folder/",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFolderName(tt.folderName)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateFolderName(%q) error = %v, wantError %v", tt.folderName, err, tt.wantError)
			}
		})
	}
}
