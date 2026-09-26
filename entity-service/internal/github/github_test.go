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

package github

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseIssueURL(t *testing.T) {
	ok := map[string]Issue{
		"https://github.com/wso2/choreo/issues/42":   {"wso2", "choreo", 42},
		"https://github.com/SParaparan/cr-test/issues/635": {"SParaparan", "cr-test", 635},
		// Enterprise host, same shape.
		"https://github.acme.internal/org/repo/issues/1": {"org", "repo", 1},
	}
	for raw, want := range ok {
		got, err := ParseIssueURL(raw)
		if err != nil {
			t.Errorf("ParseIssueURL(%q) = %v", raw, err)
			continue
		}
		if got != want {
			t.Errorf("ParseIssueURL(%q) = %+v, want %+v", raw, got, want)
		}
	}

	// Each of these the ServiceNow regex would have accepted or mangled.
	bad := []string{
		"",
		"not a url",
		"ftp://github.com/o/r/issues/1",
		"https://github.com/wso2/choreo/pull/42",      // a PR, not an issue
		"https://github.com/wso2/choreo/issues/",      // no number
		"https://github.com/wso2/choreo/issues/abc",   // not a number
		"https://github.com/wso2/choreo/issues/0",     // issues are 1-based
		"https://github.com/wso2/choreo/issues/42/x",  // trailing segment
		"https://github.com/choreo/issues/42",         // no owner
	}
	for _, raw := range bad {
		if got, err := ParseIssueURL(raw); err == nil {
			t.Errorf("ParseIssueURL(%q) = %+v, want an error", raw, got)
		}
	}
}

func sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifySignature(t *testing.T) {
	body := []byte(`{"action":"labeled"}`)
	const secret = "s3cret"

	if err := VerifySignature(secret, body, sign(secret, body)); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}

	bad := map[string]string{
		"empty":            "",
		"no prefix":        hex.EncodeToString([]byte("whatever")),
		"not hex":          "sha256=zzzz",
		"wrong secret":     sign("other", body),
		"wrong body":       sign(secret, []byte(`{"action":"closed"}`)),
		"sha1 downgrade":   "sha1=" + hex.EncodeToString([]byte("x")),
	}
	for name, header := range bad {
		t.Run(name, func(t *testing.T) {
			if err := VerifySignature(secret, body, header); !errors.Is(err, ErrInvalidSignature) {
				t.Fatalf("got %v, want ErrInvalidSignature", err)
			}
		})
	}

	// No secret must refuse, never pass.
	if err := VerifySignature("", body, sign(secret, body)); err == nil {
		t.Fatal("an unconfigured secret accepted a webhook")
	}
}

// The failure must not describe itself. Anyone who could tell "malformed" from
// "wrong" -- or read the expected value -- could forge.
func TestVerifySignature_RevealsNothing(t *testing.T) {
	body := []byte(`{"a":1}`)
	expected := sign("real", body)
	err := VerifySignature("real", body, sign("guess", body))
	if err == nil {
		t.Fatal("want an error")
	}
	if strings.Contains(err.Error(), strings.TrimPrefix(expected, "sha256=")) {
		t.Fatal("the error leaked the expected signature")
	}
}

func testClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewClient(Config{BaseURL: srv.URL, Token: "t"})
}

func TestCreateComment(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if r.URL.Path != "/repos/wso2/choreo/issues/42/comments" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer t" {
			t.Errorf("auth = %q", got)
		}
		var in map[string]string
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in["body"] != "hello" {
			t.Errorf("body = %q", in["body"])
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Comment{ID: 1, Body: "hello"})
	})
	got, err := c.CreateComment(context.Background(), Issue{"wso2", "choreo", 42}, "hello")
	if err != nil {
		t.Fatalf("CreateComment: %v", err)
	}
	if got.ID != 1 {
		t.Fatalf("id = %d", got.ID)
	}
}

func TestCreateComment_RejectsEmpty(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not have called GitHub")
	})
	if _, err := c.CreateComment(context.Background(), Issue{"o", "r", 1}, "   "); err == nil {
		t.Fatal("want an error for an empty comment")
	}
}

// A comment thread longer than one page must not silently truncate.
func TestListComments_Paginates(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		out := []Comment{}
		if page == "1" {
			for i := 0; i < 100; i++ {
				out = append(out, Comment{ID: int64(i)})
			}
		} else if page == "2" {
			out = append(out, Comment{ID: 100})
		}
		_ = json.NewEncoder(w).Encode(out)
	})
	got, err := c.ListComments(context.Background(), Issue{"o", "r", 1})
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	if len(got) != 101 {
		t.Fatalf("got %d comments, want 101", len(got))
	}
}

// Removing a label that is not there is the caller's desired end state.
func TestRemoveLabel_404IsSuccess(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Label does not exist"}`))
	})
	if err := c.RemoveLabel(context.Background(), Issue{"o", "r", 1}, "gone"); err != nil {
		t.Fatalf("RemoveLabel: %v", err)
	}
}

func TestError_RateLimited(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message":"rate limited"}`))
	})
	err := c.SetState(context.Background(), Issue{"o", "r", 1}, StateClosed)
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("got %v, want *Error", err)
	}
	if !apiErr.RateLimited() {
		t.Error("RateLimited() = false")
	}
	if apiErr.RetryAfter.Seconds() != 30 {
		t.Errorf("RetryAfter = %v", apiErr.RetryAfter)
	}
}

func TestSetState_RejectsUnknown(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not have called GitHub")
	})
	if err := c.SetState(context.Background(), Issue{"o", "r", 1}, State("reopened")); err == nil {
		t.Fatal("want an error")
	}
}

func TestIncompleteIssueRejected(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not have called GitHub")
	})
	if _, err := c.CreateComment(context.Background(), Issue{Owner: "o"}, "x"); err == nil {
		t.Fatal("want an error for an incomplete issue")
	}
}
