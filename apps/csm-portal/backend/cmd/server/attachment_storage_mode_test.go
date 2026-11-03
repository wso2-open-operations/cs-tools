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

// TestBlobAttachmentRoutesDoNotConflict registers the backend-mode patterns next
// to the case-attachment ones: net/http.ServeMux panics at registration on an
// ambiguous pair, which would otherwise only show when the server starts.
func TestBlobAttachmentRoutesDoNotConflict(t *testing.T) {
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	mux := http.NewServeMux()
	for _, p := range []string{
		"POST /attachments/upload",
		"PUT /attachments/upload/{id}/chunks/{index}",
		"GET /attachments/upload/{id}/chunks",
		"POST /attachments/upload/{id}/confirm",
		"POST /attachments/delete",
		"GET /attachments/download/{id}/content",
		"GET /attachments/inline/{id}/content",
		"GET /attachments/{id}/content",
		"GET /attachments/{id}",
		"PATCH /attachments/{id}",
		"DELETE /attachments/{id}",
		"POST /attachments/search",
		"POST /attachments",
		"POST /attachments/{id}/share",
	} {
		mux.HandleFunc(p, h)
	}
}
