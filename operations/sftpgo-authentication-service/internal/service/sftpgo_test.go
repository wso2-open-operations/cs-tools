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

	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/util"
)

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
			err := util.ValidateFolderName(tt.folderName)
			if (err != nil) != tt.wantError {
				t.Errorf("util.ValidateFolderName(%q) error = %v, wantError %v", tt.folderName, err, tt.wantError)
			}
		})
	}
}
