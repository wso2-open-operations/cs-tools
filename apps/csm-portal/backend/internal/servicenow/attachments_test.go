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

package servicenow

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDownloadAttachment_ReturnsBodyAndHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/now/attachment/0123456789abcdef0123456789abcdef/file" {
			t.Errorf("path = %q, want /api/now/attachment/0123456789abcdef0123456789abcdef/file", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="report.pdf"`)
		_, _ = w.Write([]byte("%PDF-1.4 fake content"))
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, Username: "u", Password: "p"})

	body, ct, cd, err := c.DownloadAttachment(context.Background(), "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("DownloadAttachment returned error: %v", err)
	}
	if string(body) != "%PDF-1.4 fake content" {
		t.Errorf("body = %q", body)
	}
	if ct != "application/pdf" {
		t.Errorf("contentType = %q, want application/pdf", ct)
	}
	if cd == "" {
		t.Error("contentDisposition should be forwarded from upstream")
	}
}

func TestRequireCaseAttachment_AllowsCaseAttachment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/now/table/sys_attachment" {
			t.Errorf("path = %q, want /api/now/table/sys_attachment", r.URL.Path)
		}
		if got := r.URL.Query().Get("sysparm_query"); got != "sys_id=0123456789abcdef0123456789abcdef" {
			t.Errorf("sysparm_query = %q, want %q", got, "sys_id=0123456789abcdef0123456789abcdef")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(snAttachmentTableRefList{
			Result: []snAttachmentTableRef{{TableName: "sn_customerservice_case"}},
		})
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, Username: "u", Password: "p"})
	if err := c.RequireCaseAttachment(context.Background(), "0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatalf("RequireCaseAttachment returned error: %v", err)
	}
}

func TestRequireCaseAttachment_RejectsOtherTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(snAttachmentTableRefList{
			Result: []snAttachmentTableRef{{TableName: "incident"}},
		})
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, Username: "u", Password: "p"})
	err := c.RequireCaseAttachment(context.Background(), "0123456789abcdef0123456789abcdef")
	if !errors.Is(err, ErrAttachmentNotFound) {
		t.Fatalf("err = %v, want ErrAttachmentNotFound", err)
	}
}

func TestRequireCaseAttachment_RejectsMissingAttachment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(snAttachmentTableRefList{Result: nil})
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, Username: "u", Password: "p"})
	err := c.RequireCaseAttachment(context.Background(), "0123456789abcdef0123456789abcdef")
	if !errors.Is(err, ErrAttachmentNotFound) {
		t.Fatalf("err = %v, want ErrAttachmentNotFound", err)
	}
}

func TestRequireCaseAttachment_RejectsUnsafeID(t *testing.T) {
	c := NewClient(Config{BaseURL: "http://example.invalid", Username: "u", Password: "p"})
	err := c.RequireCaseAttachment(context.Background(), "att-1^OR active=true")
	var unsafe *ErrUnsafeQueryValue
	if !errors.As(err, &unsafe) {
		t.Fatalf("err = %v, want *ErrUnsafeQueryValue", err)
	}
}
