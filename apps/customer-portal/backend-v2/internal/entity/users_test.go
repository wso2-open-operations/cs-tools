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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func patchMeBody(t *testing.T, req PatchUserMeRequest) map[string]any {
	t.Helper()
	var got map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"t","token_type":"Bearer","expires_in":3600}`))
	})
	mux.HandleFunc("PATCH /users/me", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &got); err != nil {
			t.Errorf("decode: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"ok","user":{"id":"u"}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := NewClient(Config{BaseURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "c", ClientSecret: "s"})
	if _, err := c.PatchMe(context.Background(), req); err != nil {
		t.Fatalf("PatchMe: %v", err)
	}
	return got
}

func TestPatchMe_TimeZoneOnlyOmitsPhoneKey(t *testing.T) {
	tz := "Asia/Colombo"
	got := patchMeBody(t, PatchUserMeRequest{TimeZone: &tz})
	if _, has := got["phone"]; has {
		t.Fatalf("phone key sent: %v", got)
	}
	if got["timeZone"] != tz {
		t.Fatalf("body = %v", got)
	}
}

func TestPatchMe_PhoneOnlyOmitsTimeZoneAndKeepsEmptyPhone(t *testing.T) {
	empty := ""
	got := patchMeBody(t, PatchUserMeRequest{Phone: &empty})
	if _, has := got["timeZone"]; has {
		t.Fatalf("timeZone key sent: %v", got)
	}
	if v, has := got["phone"]; !has || v != "" {
		t.Fatalf("empty phone must be sent, body = %v", got)
	}
}
