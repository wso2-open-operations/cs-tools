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

package incidentautoclose

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
	}))
	t.Cleanup(tokenSrv.Close)
	apiSrv := httptest.NewServer(handler)
	t.Cleanup(apiSrv.Close)
	c, err := NewClient(ClientConfig{BaseURL: apiSrv.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestClient_AutoClosePostsTheRequest(t *testing.T) {
	var path, auth string
	var body map[string]any
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path, auth = r.URL.Path, r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, _ = w.Write([]byte(`{"dryRun":true,"closed":[{"id":"1","number":"INC1","resolvedOn":"2026-10-01T00:00:00Z"}],` +
			`"failed":[{"id":"2","number":"INC2","error":"x"}],"more":true}`))
	})
	resp, err := c.AutoClose(context.Background(), Request{ResolvedForDays: 14, Limit: 200, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/incidents/autoclose" || auth != "Bearer test-token" {
		t.Errorf("path=%q auth=%q", path, auth)
	}
	if body["resolvedForDays"] != float64(14) || body["limit"] != float64(200) || body["dryRun"] != true {
		t.Errorf("body = %v", body)
	}
	if !resp.DryRun || !resp.More || len(resp.Closed) != 1 || resp.Closed[0].Number != "INC1" || resp.Failed[0].Number != "INC2" {
		t.Errorf("resp = %+v", resp)
	}
}

// entity-service's handler uses DisallowUnknownFields, so a key outside the
// agreed request {resolvedForDays, limit, dryRun} would fail every run with a
// 400. dryRun and limit are omitempty: a real run with the defaults sends only
// resolvedForDays.
func TestClient_SendsOnlyContractFields(t *testing.T) {
	allowed := map[string]bool{"resolvedForDays": true, "limit": true, "dryRun": true}
	for _, req := range []Request{
		{ResolvedForDays: 14},
		{ResolvedForDays: 14, Limit: PageSize},
		{ResolvedForDays: 14, Limit: PageSize, DryRun: true},
	} {
		var body map[string]any
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &body)
			_, _ = w.Write([]byte(`{"dryRun":false,"closed":[],"failed":[],"more":false}`))
		})
		if _, err := c.AutoClose(context.Background(), req); err != nil {
			t.Fatal(err)
		}
		for k := range body {
			if !allowed[k] {
				t.Errorf("request %+v sent %q, which is not in the endpoint's contract", req, k)
			}
		}
		if _, ok := body["dryRun"]; ok != req.DryRun {
			t.Errorf("request %+v: body = %v, want dryRun present only for a dry run", req, body)
		}
	}
}

func TestClient_NonSuccessIsAnAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"not supported on this data source"}`, http.StatusServiceUnavailable)
	})
	_, err := c.AutoClose(context.Background(), Request{ResolvedForDays: 14})
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("error = %v, want an *apierror.Error with status 503", err)
	}
}

func TestNewClient_RefusesInsecureURLs(t *testing.T) {
	if _, err := NewClient(ClientConfig{BaseURL: "http://entity.example.com", TokenURL: "https://sts.example.com/token"}); err == nil {
		t.Error("a plain-http base URL was accepted")
	}
	if _, err := NewClient(ClientConfig{BaseURL: "https://entity.example.com", TokenURL: "http://sts.example.com/token"}); err == nil {
		t.Error("a plain-http token URL was accepted")
	}
}
