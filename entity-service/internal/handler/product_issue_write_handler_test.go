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
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type fakeWriteService struct {
	replay             bool
	err                error
	id, key, commandID string
	comment            domain.AddProductIssueCommentRequest
	status             domain.SetProductIssueBoardStatusRequest
	called             string
}

func (f *fakeWriteService) AddComment(_ context.Context, id, key string, req domain.AddProductIssueCommentRequest) (domain.AddProductIssueCommentResponse, bool, error) {
	f.called, f.id, f.key, f.comment = "comment", id, key, req
	return domain.AddProductIssueCommentResponse{Command: domain.GitHubCommand{ID: "c1", Kind: "ADD_COMMENT", Status: "PENDING"}}, f.replay, f.err
}
func (f *fakeWriteService) SetBoardStatus(_ context.Context, id, key string, req domain.SetProductIssueBoardStatusRequest) (domain.GitHubCommand, bool, error) {
	f.called, f.id, f.key, f.status = "board", id, key, req
	return domain.GitHubCommand{ID: "c2", Kind: "SET_BOARD_STATUS", Status: "PENDING"}, f.replay, f.err
}
func (f *fakeWriteService) Close(_ context.Context, id, key string) (domain.GitHubCommand, bool, error) {
	f.called, f.id, f.key = "close", id, key
	return domain.GitHubCommand{ID: "c3", Kind: "SET_STATE", Status: "PENDING"}, f.replay, f.err
}
func (f *fakeWriteService) Retry(_ context.Context, id, commandID string) (domain.GitHubCommand, error) {
	f.called, f.id, f.commandID = "retry", id, commandID
	return domain.GitHubCommand{ID: commandID, Status: "PENDING"}, f.err
}

func writeReq(h http.HandlerFunc, method, path, body string, hdr map[string]string, pv map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	for k, v := range pv {
		r.SetPathValue(k, v)
	}
	w := httptest.NewRecorder()
	h(w, r)
	return w
}

func TestProductIssueWriteHandler_StatusAndForwarding(t *testing.T) {
	svc := &fakeWriteService{}
	h := NewProductIssueWriteHandler(svc)
	idem := map[string]string{"Idempotency-Key": "key-1"}
	id := map[string]string{"id": "wi-1"}

	w := writeReq(h.AddComment, "POST", "/x", `{"body":"hi"}`, idem, id)
	if w.Code != http.StatusAccepted || svc.called != "comment" || svc.id != "wi-1" || svc.key != "key-1" || svc.comment.Body != "hi" || !strings.Contains(w.Body.String(), `"command"`) {
		t.Fatalf("comment: %d %+v %s", w.Code, svc, w.Body.String())
	}
	svc.replay = true
	if w = writeReq(h.AddComment, "POST", "/x", `{"body":"hi"}`, idem, id); w.Code != http.StatusOK {
		t.Errorf("comment replay: %d", w.Code)
	}
	svc.replay = false

	w = writeReq(h.SetBoardStatus, "POST", "/x", `{"status":"WAITING_ON_PRODUCT_TEAM"}`, idem, id)
	if w.Code != http.StatusAccepted || svc.called != "board" || svc.status.Status != "WAITING_ON_PRODUCT_TEAM" || svc.key != "key-1" {
		t.Fatalf("board: %d %+v", w.Code, svc)
	}
	svc.replay = true
	if w = writeReq(h.SetBoardStatus, "POST", "/x", `{"status":"WAITING_ON_PRODUCT_TEAM"}`, idem, id); w.Code != http.StatusOK {
		t.Errorf("board replay: %d", w.Code)
	}
	svc.replay = false

	// Close takes an empty body or {}.
	for _, body := range []string{"", "{}"} {
		w = writeReq(h.CloseProductWorkItem, "POST", "/x", body, idem, id)
		if w.Code != http.StatusAccepted || svc.called != "close" {
			t.Errorf("close %q: %d", body, w.Code)
		}
	}
	svc.replay = true
	if w = writeReq(h.CloseProductWorkItem, "POST", "/x", "", idem, id); w.Code != http.StatusOK {
		t.Errorf("close replay: %d", w.Code)
	}
	svc.replay = false

	w = writeReq(h.RetryCommand, "POST", "/x", "", nil, map[string]string{"id": "wi-1", "commandId": "cmd-9"})
	if w.Code != http.StatusAccepted || svc.commandID != "cmd-9" || svc.id != "wi-1" {
		t.Errorf("retry: %d %+v", w.Code, svc)
	}
}

func TestProductIssueWriteHandler_BadBodies(t *testing.T) {
	svc := &fakeWriteService{}
	h := NewProductIssueWriteHandler(svc)
	for name, tc := range map[string]struct {
		fn   http.HandlerFunc
		body string
	}{
		"comment missing body":  {h.AddComment, ``},
		"comment unknown field": {h.AddComment, `{"body":"x","extra":1}`},
		"comment not json":      {h.AddComment, `nope`},
		"board unknown field":   {h.SetBoardStatus, `{"status":"WAITING_ON_PRODUCT_TEAM","x":1}`},
		"board missing body":    {h.SetBoardStatus, ``},
		"close with unknown":    {h.CloseProductWorkItem, `{"x":1}`},
		"retry with unknown":    {h.RetryCommand, `{"x":1}`},
	} {
		if w := writeReq(tc.fn, "POST", "/x", tc.body, nil, map[string]string{"id": "i", "commandId": "c"}); w.Code != http.StatusBadRequest || svc.called != "" {
			t.Errorf("%s: %d called=%q", name, w.Code, svc.called)
		}
	}
}

func TestProductIssueWriteHandler_RefusalEnvelopes(t *testing.T) {
	svc := &fakeWriteService{}
	h := NewProductIssueWriteHandler(svc)
	for _, code := range []string{"link_inactive", "not_waiting_on_cs", "command_pending", "no_tracking_board", "already_closed", "not_failed"} {
		svc.err = &apierror.ConflictError{Msg: "refused", Code: code}
		w := writeReq(h.CloseProductWorkItem, "POST", "/x", "", nil, map[string]string{"id": "i"})
		if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), `"errorCode":"`+code+`"`) || !strings.Contains(w.Body.String(), `"message":"refused"`) {
			t.Errorf("%s: %d %s", code, w.Code, w.Body.String())
		}
	}
	svc.err = &apierror.ValidationError{Msg: "Only WAITING_ON_PRODUCT_TEAM can be set.", Code: apierror.CodeUnsupportedStatus}
	w := writeReq(h.SetBoardStatus, "POST", "/x", `{"status":"RESOLVED"}`, nil, map[string]string{"id": "i"})
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `"errorCode":"unsupported_status"`) {
		t.Errorf("unsupported: %d %s", w.Code, w.Body.String())
	}
	svc.err = &apierror.NotFoundError{Msg: "product issue not found"}
	if w = writeReq(h.AddComment, "POST", "/x", `{"body":"x"}`, nil, map[string]string{"id": "i"}); w.Code != http.StatusNotFound {
		t.Errorf("404: %d", w.Code)
	}
}
