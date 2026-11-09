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

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
)

func TestReadInlineImage_AsksForTheImageOfTheRecord(t *testing.T) {
	var path, query, auth string
	token := newCustomerTokenServer(t)
	defer token.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, query, auth = r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("png"))
	}))
	defer api.Close()
	c := newTestCustomerClient(t, token, api)

	img, err := c.ReadInlineImage(context.Background(), "img-1", "case-1")
	if err != nil {
		t.Fatalf("ReadInlineImage: %v", err)
	}
	defer img.Body.Close()
	if b, _ := io.ReadAll(img.Body); string(b) != "png" || img.ContentType != "image/png" || img.ContentLength != 3 {
		t.Errorf("body = %q, type = %q, length = %d", b, img.ContentType, img.ContentLength)
	}
	if path != "/attachments/inline/img-1/content" || query != "workItemId=case-1" {
		t.Errorf("request = %s?%s", path, query)
	}
	if auth != "Bearer test-token" {
		t.Errorf("Authorization = %q, want this service's own token", auth)
	}
}

// Without the record, entity-service would not filter the image at all.
func TestReadInlineImage_NeedsTheRecord(t *testing.T) {
	called := false
	token := newCustomerTokenServer(t)
	defer token.Close()
	api := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer api.Close()
	if _, err := newTestCustomerClient(t, token, api).ReadInlineImage(context.Background(), "img-1", ""); err == nil || called {
		t.Errorf("err = %v, called = %v: a read with no record must not leave this process", err, called)
	}
}

func TestReadInlineImage_RefusalsKeepOnlyTheStatus(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusGone, http.StatusServiceUnavailable} {
		token := newCustomerTokenServer(t)
		api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"message":"image of customer X"}`))
		}))
		_, err := newTestCustomerClient(t, token, api).ReadInlineImage(context.Background(), "img-1", "case-1")
		var apiErr *apierror.Error
		if !errors.As(err, &apiErr) || apiErr.StatusCode != status || apiErr.Body == `{"message":"image of customer X"}` {
			t.Errorf("status %d: err = %v, want only the status kept", status, err)
		}
		token.Close()
		api.Close()
	}
}
