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

package holidays

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

// fakeServer serves /token and the API path; apiStatus lets a test answer 401
// to the first API call.
func fakeServer(t *testing.T, api http.HandlerFunc) (*httptest.Server, *int32) {
	t.Helper()
	var tokens int32
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "client_credentials" || r.Form.Get("client_id") != "id" {
			http.Error(w, "bad grant", http.StatusBadRequest)
			return
		}
		atomic.AddInt32(&tokens, 1)
		_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":3600}`))
	})
	mux.HandleFunc("/", api)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &tokens
}

func creds(srv *httptest.Server) Credentials {
	return Credentials{TokenURL: srv.URL + "/token", ClientID: "id", ClientSecret: "secret"}
}

func TestCalendarClient_Holidays(t *testing.T) {
	srv, _ := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/holidays" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		q := r.URL.Query()
		if q.Get("countries") != "Sri Lanka" || q.Get("startTimeMin") != "2026-01-01T00:00:00Z" || q.Get("startTimeMax") != "2026-12-31T00:00:00Z" {
			t.Errorf("unexpected query %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[{"country":"Sri Lanka","holidays":[
			{"id":"a","kind":"calendar#event","summary":"Tamil Thai Pongal Day","start":{"date":"2026-01-14"},"end":{"date":"2026-01-15"}},
			{"id":"b","summary":"Sinhala and Tamil New Year","start":{"date":"2026-04-13"},"end":{"date":"2026-04-15"}},
			{"id":"c","summary":"timed","start":{"datetime":"2026-05-01T10:00:00Z"},"end":{"datetime":"2026-05-01T11:00:00Z"}}
		]}]`))
	})
	c := NewCalendarClient(srv.URL+"/", creds(srv))
	got, err := c.Holidays(context.Background(), "Sri Lanka",
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Holidays: %v", err)
	}
	want := []Holiday{
		{Date: "2026-01-14", Title: "Tamil Thai Pongal Day"},
		{Date: "2026-04-13", Title: "Sinhala and Tamil New Year"},
		{Date: "2026-04-14", Title: "Sinhala and Tamil New Year"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestCalendarClient_EmptyAndErrors(t *testing.T) {
	status := http.StatusOK
	srv, _ := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`[]`))
	})
	c := NewCalendarClient(srv.URL, creds(srv))
	got, err := c.Holidays(context.Background(), "India", time.Now(), time.Now())
	if err != nil || len(got) != 0 {
		t.Fatalf("empty: got %v, %v", got, err)
	}
	status = http.StatusBadGateway
	if _, err := c.Holidays(context.Background(), "India", time.Now(), time.Now()); err == nil {
		t.Fatal("502: want an error")
	}
}

func TestCalendarClient_RefreshesTokenOn401(t *testing.T) {
	var calls int32
	srv, tokens := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`[]`))
	})
	c := NewCalendarClient(srv.URL, creds(srv))
	if _, err := c.Holidays(context.Background(), "India", time.Now(), time.Now()); err != nil {
		t.Fatalf("Holidays: %v", err)
	}
	if atomic.LoadInt32(tokens) != 2 || atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("tokens=%d calls=%d, want 2 and 2", *tokens, calls)
	}
}

func TestExpandEvents_EndNotAfterStartIsOneDay(t *testing.T) {
	ev := calendarEvent{Summary: " Poya "}
	ev.Start.Date = "2026-06-29"
	ev.End.Date = "2026-06-29"
	got := expandEvents([]calendarEvent{ev})
	if len(got) != 1 || got[0] != (Holiday{Date: "2026-06-29", Title: "Poya"}) {
		t.Fatalf("got %+v", got)
	}
}

func TestEmployeeClient_Location(t *testing.T) {
	srv, _ := fakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var req graphQLRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		switch req.Variables["email"] {
		case "lk@example.com":
			_, _ = w.Write([]byte(`{"data":{"employee":{"location":" Sri Lanka "}}}`))
		case "none@example.com":
			_, _ = w.Write([]byte(`{"data":{"employee":{"location":null}}}`))
		case "err@example.com":
			_, _ = w.Write([]byte(`{"data":{"employee":null},"errors":[{"message":"boom"}]}`))
		default:
			_, _ = w.Write([]byte(`{"data":{"employee":null}}`))
		}
	})
	c := NewEmployeeClient(srv.URL+"/graphql", creds(srv))
	ctx := context.Background()
	if loc, err := c.Location(ctx, "lk@example.com"); err != nil || loc != "Sri Lanka" {
		t.Fatalf("lk: %q, %v", loc, err)
	}
	if loc, err := c.Location(ctx, "none@example.com"); err != nil || loc != "" {
		t.Fatalf("none: %q, %v", loc, err)
	}
	if _, err := c.Location(ctx, "missing@example.com"); !errors.Is(err, ErrEmployeeNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if _, err := c.Location(ctx, "err@example.com"); err == nil || errors.Is(err, ErrEmployeeNotFound) {
		t.Fatalf("graphql error: %v", err)
	}
}
