package cloudclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(Config{
		BaseURL:       srv.URL,
		OrgPath:       "/orgs/{orgId}",
		UserEmailPath: "/users/{userId}/email",
	})
}

func TestGetOrg_ReadsTheOrgRecord(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.EscapedPath() != "/orgs/org%2F1" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.EscapedPath())
		}
		_, _ = w.Write([]byte(`{"uuid":"org-1","friendlyName":"Acme","handle":"acme"}`))
	})
	org, err := c.GetOrg(context.Background(), "org/1")
	if err != nil || org.UUID != "org-1" || org.FriendlyName != "Acme" || org.Handle != "acme" {
		t.Fatalf("got %+v, %v", org, err)
	}
}

func TestGetUserEmail_ReadsTheEmailAndItsOrg(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.EscapedPath() != "/users/user-1/email" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.EscapedPath())
		}
		_, _ = w.Write([]byte(`{"userUuid":"user-1","orgUuid":"org-1","email":"jane@acme.io"}`))
	})
	u, err := c.GetUserEmail(context.Background(), "user-1")
	if err != nil || u.OrgUUID != "org-1" || u.Email != "jane@acme.io" {
		t.Fatalf("got %+v, %v", u, err)
	}
}

func TestStatusesMapToSentinels(t *testing.T) {
	for status, want := range map[int]error{
		http.StatusNotFound:        ErrNotFound,
		http.StatusTooManyRequests: ErrRateLimited,
	} {
		c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) })
		if _, err := c.GetOrg(context.Background(), "x"); !errors.Is(err, want) {
			t.Errorf("status %d: want %v, got %v", status, want, err)
		}
	}
}

func TestErrorsNeverCarryTheReplyBody(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"email":"secret@acme.io"}`))
	})
	_, err := c.GetUserEmail(context.Background(), "x")
	if err == nil || strings.Contains(err.Error(), "secret@acme.io") {
		t.Fatalf("error leaked the body or was nil: %v", err)
	}
}
