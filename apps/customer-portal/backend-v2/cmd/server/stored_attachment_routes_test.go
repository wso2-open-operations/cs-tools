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
	"net/http"
	"testing"
)

// TestStoredAttachmentRoutesDoNotConflictWithTheCaseAttachmentRoutes registers
// the stored-attachment patterns next to every existing /attachments pattern:
// net/http.ServeMux (Go 1.22+) panics at registration on an ambiguous pair, which
// would otherwise only show up when the server starts.
func TestStoredAttachmentRoutesDoNotConflictWithTheCaseAttachmentRoutes(t *testing.T) {
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	mux := http.NewServeMux()
	for _, p := range []string{
		"POST /attachments/download",
		"POST /attachments/upload",
		"POST /attachments/upload/{id}/confirm",
		"POST /attachments/delete",
		"PUT /attachments/upload/{id}/chunks/{index}",
		"GET /attachments/upload/{id}/chunks",
		"GET /attachments/download/{id}/content",
		"GET /attachments/inline/{id}/content",
		"GET /attachments/{id}/content",
		"GET /attachments/{id}",
		"DELETE /attachments/{id}",
		"GET /attachments/{id}/case",
		"POST /attachments/search",
		"POST /attachments",
	} {
		mux.HandleFunc(p, h)
	}
}

func TestParseAttachmentStorageMode(t *testing.T) {
	for in, want := range map[string]string{"": "sftpgo", " Backend ": "backend", "SFTPGO": "sftpgo"} {
		if got, err := parseAttachmentStorageMode(in); err != nil || got != want {
			t.Errorf("%q = %q, %v", in, got, err)
		}
	}
	if _, err := parseAttachmentStorageMode("s3"); err == nil {
		t.Error("an unknown mode must be refused")
	}
}
