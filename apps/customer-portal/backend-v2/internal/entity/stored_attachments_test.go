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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
)

func storedTestClient(t *testing.T, routes map[string]http.HandlerFunc) *Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"t","token_type":"Bearer","expires_in":3600}`))
	})
	for p, h := range routes {
		mux.HandleFunc(p, h)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return NewClient(Config{BaseURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "c", ClientSecret: "s"})
}

func TestStoredAttachments_BodiesAreRelayedVerbatimWithTheCallersHeaders(t *testing.T) {
	var gotBody, gotUser, gotCorr, gotAuth string
	record := func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody, gotUser, gotCorr, gotAuth = string(b), r.Header.Get("x-user-id-token"), r.Header.Get("X-CSM-Correlation-ID"), r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"relayed":true}`))
	}
	c := storedTestClient(t, map[string]http.HandlerFunc{
		"POST /attachments/upload":              record,
		"POST /attachments/upload/{id}/confirm": record,
		"POST /attachments/delete":              func(w http.ResponseWriter, r *http.Request) { record(w, r); w.WriteHeader(http.StatusNoContent) },
	})
	ctx := WithCorrelationID(WithUserIDToken(context.Background(), "tok"), "corr-1")
	body := []byte(`{"type":"work_item","ids":["a"]}`)

	out, err := c.InitiateStoredAttachmentUpload(ctx, body)
	if err != nil || string(out) != `{"relayed":true}` || gotBody != string(body) || gotUser != "tok" || gotCorr != "corr-1" || !strings.HasPrefix(gotAuth, "Bearer ") {
		t.Errorf("upload: err=%v out=%s body=%q user=%q corr=%q auth=%q", err, out, gotBody, gotUser, gotCorr, gotAuth)
	}
	if _, err := c.ConfirmStoredAttachmentUpload(ctx, "abc", body); err != nil {
		t.Errorf("confirm: %v", err)
	}
	if err := c.DeleteStoredAttachment(ctx, body); err != nil {
		t.Errorf("delete: %v", err)
	}
}

func TestStoredAttachments_UpstreamStatusIsKept(t *testing.T) {
	c := storedTestClient(t, map[string]http.HandlerFunc{
		"POST /attachments/upload": func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"code":429,"message":"too many"}`, http.StatusTooManyRequests)
		},
	})
	_, err := c.InitiateStoredAttachmentUpload(context.Background(), []byte(`{}`))
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTooManyRequests {
		t.Errorf("err = %v, want an upstream 429", err)
	}
}
