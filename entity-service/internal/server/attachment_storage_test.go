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

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestAttachmentRequestTimeout pins which routes get a longer limit than the
// ordinary REQUEST_TIMEOUT: only the ones that move a file, and only with the
// method that does.
func TestAttachmentRequestTimeout(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		want         time.Duration
		long         bool
	}{
		{http.MethodPut, "/attachments/upload/abc/chunks/3", chunkRequestTimeout, true},
		{http.MethodGet, "/attachments/download/abc/content", downloadRequestTimeout, true},
		{http.MethodGet, "/attachments/inline/abc/content", inlineImageRequestTimeout, true},
		{http.MethodGet, "/attachments/upload/abc/chunks", 0, false},
		{http.MethodPost, "/attachments/upload/abc/confirm", 0, false},
		{http.MethodPost, "/attachments/upload", 0, false},
		{http.MethodPost, "/attachments/delete", 0, false},
		{http.MethodPost, "/attachments/uploads/cleanup", 0, false},
		{http.MethodPut, "/cases/abc/chunks/3", 0, false},
		{http.MethodPost, "/attachments/download/abc/content", 0, false},
	} {
		got, long := attachmentRequestTimeout(httptest.NewRequest(tc.method, tc.path, nil))
		if long != tc.long || got != tc.want {
			t.Errorf("%s %s = %v, %v; want %v, %v", tc.method, tc.path, got, long, tc.want, tc.long)
		}
	}
}
