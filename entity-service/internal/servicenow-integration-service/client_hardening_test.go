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

package integrationservice

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// newHardeningServer serves a token endpoint that hands out "token-1",
// "token-2", ... and an API handler; it returns the client and the number of
// tokens issued so far.
func newHardeningServer(t *testing.T, tokenStatus int, api http.HandlerFunc) (*Client, *atomic.Int32) {
	t.Helper()
	var issued atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		if tokenStatus != http.StatusOK {
			w.WriteHeader(tokenStatus)
			return
		}
		n := issued.Add(1)
		fmt.Fprintf(w, `{"access_token":"token-%d","expires_in":3600}`, n)
	})
	mux.HandleFunc("/api/", api)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return New(srv.URL+"/api", ClientCredentialsConfig{TokenURL: srv.URL + "/token", ClientID: "id", ClientSecret: "secret"}), &issued
}

func TestClient_401RefreshesServiceTokenAndRetriesOnce(t *testing.T) {
	client, issued := newHardeningServer(t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		// The first service token has been revoked downstream.
		if r.Header.Get("Authorization") == "Bearer token-1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	got, err := client.Get(context.Background(), "/api/cases/1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != `{"ok":true}` || issued.Load() != 2 {
		t.Fatalf("got %s with %d tokens issued, want the retried response and 2 tokens", got, issued.Load())
	}
}

func TestClient_401AfterFreshTokenIsTheUserToken(t *testing.T) {
	var calls atomic.Int32
	client, issued := newHardeningServer(t, http.StatusOK, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	})
	_, err := client.Post(context.Background(), "/api/cases/1/comments", "user-token", map[string]string{"a": "b"})
	var ue *apierror.UnauthorizedError
	if !errors.As(err, &ue) || ue.Msg != "invalid or missing x-user-id-token" {
		t.Fatalf("err = %v, want the user-token UnauthorizedError", err)
	}
	if calls.Load() != 2 || issued.Load() != 2 {
		t.Fatalf("calls = %d tokens = %d, want exactly one retry with a fresh token", calls.Load(), issued.Load())
	}
}

func TestClient_RejectedCredentialsAreAGeneric503(t *testing.T) {
	client, _ := newHardeningServer(t, http.StatusUnauthorized, func(http.ResponseWriter, *http.Request) {
		t.Fatal("the API must not be called without a service token")
	})
	_, err := client.Get(context.Background(), "/api/cases/1", "")
	var sue *apierror.ServiceUnavailableError
	if !errors.As(err, &sue) {
		t.Fatalf("err = %v (%T), want ServiceUnavailableError", err, err)
	}
	if strings.Contains(strings.ToLower(sue.Msg), "secret") || strings.Contains(strings.ToLower(sue.Msg), "credential") {
		t.Fatalf("message %q leaks configuration detail to the caller", sue.Msg)
	}
}

func TestClient_OversizedResponseIsRefused(t *testing.T) {
	client, _ := newHardeningServer(t, http.StatusOK, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`"`))
		chunk := strings.Repeat("x", 1<<20)
		for i := 0; i < 17; i++ {
			_, _ = w.Write([]byte(chunk))
		}
		_, _ = w.Write([]byte(`"`))
	})
	_, err := client.Get(context.Background(), "/api/big", "")
	var de *apierror.DownstreamError
	if !errors.As(err, &de) {
		t.Fatalf("err = %v (%T), want DownstreamError for an oversized body", err, err)
	}
}

func TestClient_DownstreamValidationMessageIsKeptButLaundered(t *testing.T) {
	client, _ := newHardeningServer(t, http.StatusOK, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"[SN_ERR] ServiceNow rejected the update:\nsys_id is not a valid state for this GlideRecord"}`))
	})
	_, err := client.Patch(context.Background(), "/api/cases/1", "", map[string]string{"state": "x"})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v (%T), want ValidationError", err, err)
	}
	want := "the backing system rejected the update: id is not a valid state for this record"
	if ve.Msg != want {
		t.Fatalf("message = %q, want %q", ve.Msg, want)
	}
}

func TestLaunderDownstreamMessage_BoundsLength(t *testing.T) {
	got := launderDownstreamMessage(strings.Repeat("a", 1000))
	if len([]rune(got)) != maxDownstreamMessageRunes+3 || !strings.HasSuffix(got, "...") {
		t.Fatalf("length = %d, want %d with an ellipsis", len([]rune(got)), maxDownstreamMessageRunes+3)
	}
}
