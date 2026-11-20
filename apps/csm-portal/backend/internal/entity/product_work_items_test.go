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

package entity

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

func TestProductWorkItemClientMethods(t *testing.T) {
	const (
		caseID = "11111111-1111-1111-1111-111111111111"
		itemID = "22222222-2222-2222-2222-222222222222"
	)
	type seen struct {
		method, rawPath, query, token, ctype string
		body                                 string
	}
	var got seen
	status := http.StatusOK
	respBody := `{"ok":true}`
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"t","token_type":"Bearer","expires_in":3600}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = seen{r.Method, r.URL.EscapedPath(), r.URL.RawQuery, r.Header.Get("x-user-id-token"), r.Header.Get("Content-Type"), string(b)}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := NewCustomerEntityClient(CustomerEntityConfig{BaseURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "a", ClientSecret: "b"})
	ctx := WithUserIDToken(context.Background(), "user-token")

	const body = `{"a": 1,  "b":"x"}`
	tests := []struct {
		name     string
		call     func() ([]byte, error)
		method   string
		path     string
		wantBody string
	}{
		{"create", func() ([]byte, error) { return c.CreateCaseProductWorkItem(ctx, caseID, []byte(body)) }, "POST", "/cases/" + caseID + "/product-work-items", body},
		{"list", func() ([]byte, error) { return c.ListCaseProductWorkItems(ctx, caseID) }, "GET", "/cases/" + caseID + "/product-work-items", ""},
		{"get", func() ([]byte, error) { return c.GetProductWorkItem(ctx, itemID) }, "GET", "/product-work-items/" + itemID, ""},
		{"search", func() ([]byte, error) { return c.SearchProductWorkItems(ctx, []byte(body)) }, "POST", "/product-work-items/search", body},
		{"link", func() ([]byte, error) { return c.LinkCaseProductWorkItem(ctx, caseID, []byte(body)) }, "POST", "/cases/" + caseID + "/product-work-items/links", body},
		{"unlink", func() ([]byte, error) { return c.UnlinkCaseProductWorkItem(ctx, caseID, itemID) }, "DELETE", "/cases/" + caseID + "/product-work-items/" + itemID + "/link", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, respBody = http.StatusOK, `{"ok":true}`
			got = seen{}
			out, err := tc.call()
			if err != nil {
				t.Fatal(err)
			}
			if got.method != tc.method || got.rawPath != tc.path || got.query != "" {
				t.Errorf("request = %s %s ?%s, want %s %s", got.method, got.rawPath, got.query, tc.method, tc.path)
			}
			if got.body != tc.wantBody {
				t.Errorf("body = %q, want %q", got.body, tc.wantBody)
			}
			if (tc.wantBody != "") != (got.ctype == "application/json") {
				t.Errorf("Content-Type = %q for body %q", got.ctype, tc.wantBody)
			}
			if got.token != "user-token" {
				t.Errorf("x-user-id-token = %q", got.token)
			}
			if string(out) != `{"ok":true}` {
				t.Errorf("response = %q", out)
			}
			// 201 succeeds, 204 succeeds with no body, 409 is an *apierror.Error with its body.
			status, respBody = http.StatusCreated, `{"c":1}`
			if out, err := tc.call(); err != nil || string(out) != `{"c":1}` {
				t.Errorf("201: %q, %v", out, err)
			}
			status, respBody = http.StatusNoContent, ``
			if out, err := tc.call(); err != nil || len(out) != 0 {
				t.Errorf("204: %q, %v", out, err)
			}
			status, respBody = http.StatusConflict, `{"message":"m"}`
			_, err = tc.call()
			var apiErr *apierror.Error
			if !errors.As(err, &apiErr) || apiErr.StatusCode != 409 || apiErr.Body != respBody {
				t.Errorf("409: err = %v", err)
			}
		})
	}

	t.Run("ids are path-escaped", func(t *testing.T) {
		status, respBody = http.StatusOK, `{}`
		if _, err := c.GetProductWorkItem(ctx, "a/b?c"); err != nil {
			t.Fatal(err)
		}
		if got.rawPath != "/product-work-items/a%2Fb%3Fc" || got.query != "" {
			t.Errorf("path = %q query = %q", got.rawPath, got.query)
		}
		if _, err := c.UnlinkCaseProductWorkItem(ctx, "x/y", "z?w"); err != nil {
			t.Fatal(err)
		}
		if got.rawPath != "/cases/x%2Fy/product-work-items/z%3Fw/link" {
			t.Errorf("path = %q", got.rawPath)
		}
	})
}

func TestProductWorkItemWriteBackClientMethods(t *testing.T) {
	const (
		itemID = "22222222-2222-2222-2222-222222222222"
		cmdID  = "33333333-3333-3333-3333-333333333333"
	)
	var method, rawPath, key, token, body string
	status := http.StatusAccepted
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"t","token_type":"Bearer","expires_in":3600}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		method, rawPath, key, token, body = r.Method, r.URL.EscapedPath(), r.Header.Get("Idempotency-Key"), r.Header.Get("x-user-id-token"), string(b)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"command":{}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := NewCustomerEntityClient(CustomerEntityConfig{BaseURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "a", ClientSecret: "b"})
	ctx := WithUserIDToken(context.Background(), "user-token")

	calls := map[string]func(k string) (int, []byte, error){
		"/product-work-items/" + itemID + "/comments": func(k string) (int, []byte, error) {
			return c.CommentProductWorkItem(ctx, itemID, []byte(`{"body":"x"}`), k)
		},
		"/product-work-items/" + itemID + "/board-status": func(k string) (int, []byte, error) {
			return c.SetProductWorkItemBoardStatus(ctx, itemID, []byte(`{"body":"x"}`), k)
		},
		"/product-work-items/" + itemID + "/close": func(k string) (int, []byte, error) {
			return c.CloseProductWorkItem(ctx, itemID, []byte(`{"body":"x"}`), k)
		},
		"/product-work-items/" + itemID + "/commands/" + cmdID + "/retry": func(k string) (int, []byte, error) {
			return c.RetryProductWorkItemCommand(ctx, itemID, cmdID, []byte(`{"body":"x"}`), k)
		},
	}
	for path, call := range calls {
		for _, st := range []int{202, 200} {
			status = st
			code, out, err := call("k-1")
			if err != nil || code != st || string(out) != `{"command":{}}` {
				t.Fatalf("%s: %d %q %v", path, code, out, err)
			}
			if method != "POST" || rawPath != path || key != "k-1" || token != "user-token" || body != `{"body":"x"}` {
				t.Errorf("%s: sent %s %s key=%q token=%q body=%q", path, method, rawPath, key, token, body)
			}
		}
		status = 202
		if _, _, err := call(""); err != nil || key != "" {
			t.Errorf("%s: empty key sent %q (%v)", path, key, err)
		}
		status = http.StatusConflict
		_, _, err := call("")
		var apiErr *apierror.Error
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 409 {
			t.Errorf("%s: 409 err = %v", path, err)
		}
	}
}
