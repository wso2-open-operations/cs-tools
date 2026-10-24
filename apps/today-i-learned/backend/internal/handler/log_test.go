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

package handler

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/today-i-learned/backend/internal/middleware"
)

// captureLogs sends the service's own logger, which adds the caller's userID,
// to a buffer until the test ends.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	middleware.ConfigureLogger(&buf)
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestEveryWriteIsLogged(t *testing.T) {
	logs := captureLogs(t)
	api := newTestAPI(t)
	e := api.createUnsent(t)
	if w := api.do(t, mod, http.MethodPut, "/submissions/"+e.ID, `{"title":"New title","where":"Internal","what":"<p>Changed</p>"}`); w.Code != http.StatusOK {
		t.Fatalf("update: %d %s", w.Code, w.Body)
	}
	if w := api.do(t, mod, http.MethodPost, notifyPath(e.ID), ""); w.Code != http.StatusOK {
		t.Fatalf("notify: %d %s", w.Code, w.Body)
	}
	if w := api.do(t, jane, http.MethodDelete, "/submissions/"+e.ID, ""); w.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", w.Code, w.Body)
	}

	for _, want := range []string{
		`msg="entry created" entryID=` + e.ID + ` userID=u-jane`,
		`msg="entry updated" entryID=` + e.ID + ` byAuthor=false userID=u-mod`,
		`msg="entry sent to Google Chat" entryID=` + e.ID + ` userID=u-mod`,
		`msg="entry deleted" entryID=` + e.ID + ` byAuthor=true userID=u-jane`,
	} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("logs lack %q:\n%s", want, logs)
		}
	}
}

func TestAnUnchangedSaveIsNotLogged(t *testing.T) {
	api := newTestAPI(t)
	e := api.create(t, jane, validBody)
	logs := captureLogs(t)
	if w := api.do(t, jane, http.MethodPut, "/submissions/"+e.ID, `{"title":"T","where":"Internal","what":"<p>Learned X.</p>"}`); w.Code != http.StatusOK {
		t.Fatalf("update: %d %s", w.Code, w.Body)
	}
	if strings.Contains(logs.String(), "entry updated") {
		t.Fatalf("an unchanged save was logged:\n%s", logs)
	}
}
