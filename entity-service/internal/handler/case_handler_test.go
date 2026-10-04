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
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// stubCaseService embeds the service.CaseService interface so tests only need
// to implement the method(s) under test; any call to an unimplemented method
// panics on the nil embedded interface, which is fine since these tests never
// reach them.
type stubCaseService struct {
	service.CaseService
	createAttachmentCalled bool
	createAttachmentResp   domain.CreateAttachmentResponse
	createAttachmentErr    error

	searchTagsCalled bool
	searchTagsReq    domain.SearchTagsRequest
	searchTagsTags   []domain.Tag
	searchTagsErr    error

	getAttachmentCalled bool
	getAttachmentID     string
	getAttachmentResp   domain.AttachmentDetails
	getAttachmentErr    error

	updateAttachmentCalled bool
	updateAttachmentReq    domain.UpdateAttachmentRequest
	updateAttachmentResp   domain.UpdateAttachmentResponse
	updateAttachmentErr    error

	addCaseTagCalled   bool
	addCaseTagCaseID   string
	addCaseTagLabel    string
	addCaseTagResp     domain.Tag
	addCaseTagErr      error
	addCaseTagAsCalled bool
	addCaseTagAsCaseID string
	addCaseTagAsLabel  string
	addCaseTagAsActor  string
	addCaseTagAsResp   domain.Tag
	addCaseTagAsErr    error

	createCaseCommentCalled bool
	createCaseCommentReq    domain.CreateCaseCommentRequest
	createCaseCommentResp   domain.CreateCaseCommentResponse
	createCaseCommentErr    error

	createCaseCommentAsCalled bool
	createCaseCommentAsReq    domain.CreateCaseCommentRequest
	createCaseCommentAsActor  string
	createCaseCommentAsResp   domain.CreateCaseCommentResponse
	createCaseCommentAsErr    error
}

func (s *stubCaseService) AddCaseTag(_ context.Context, caseID, label string) (domain.Tag, error) {
	s.addCaseTagCalled = true
	s.addCaseTagCaseID = caseID
	s.addCaseTagLabel = label
	return s.addCaseTagResp, s.addCaseTagErr
}

func (s *stubCaseService) AddCaseTagAs(_ context.Context, caseID, label, actorEmail string) (domain.Tag, error) {
	s.addCaseTagAsCalled = true
	s.addCaseTagAsCaseID = caseID
	s.addCaseTagAsLabel = label
	s.addCaseTagAsActor = actorEmail
	return s.addCaseTagAsResp, s.addCaseTagAsErr
}

func (s *stubCaseService) CreateCaseComment(_ context.Context, req domain.CreateCaseCommentRequest) (domain.CreateCaseCommentResponse, error) {
	s.createCaseCommentCalled = true
	s.createCaseCommentReq = req
	return s.createCaseCommentResp, s.createCaseCommentErr
}

func (s *stubCaseService) CreateCaseCommentAs(_ context.Context, req domain.CreateCaseCommentRequest, actorEmail string) (domain.CreateCaseCommentResponse, error) {
	s.createCaseCommentAsCalled = true
	s.createCaseCommentAsReq = req
	s.createCaseCommentAsActor = actorEmail
	return s.createCaseCommentAsResp, s.createCaseCommentAsErr
}

func (s *stubCaseService) GetAttachmentByID(_ context.Context, attachmentID string) (domain.AttachmentDetails, error) {
	s.getAttachmentCalled = true
	s.getAttachmentID = attachmentID
	return s.getAttachmentResp, s.getAttachmentErr
}

func (s *stubCaseService) UpdateAttachment(_ context.Context, req domain.UpdateAttachmentRequest) (domain.UpdateAttachmentResponse, error) {
	s.updateAttachmentCalled = true
	s.updateAttachmentReq = req
	return s.updateAttachmentResp, s.updateAttachmentErr
}

func (s *stubCaseService) CreateCaseAttachment(_ context.Context, _ domain.CreateAttachmentRequest) (domain.CreateAttachmentResponse, error) {
	s.createAttachmentCalled = true
	return s.createAttachmentResp, s.createAttachmentErr
}

// attachmentRequestBody builds a valid CreateAttachmentRequest JSON body whose
// base64 "file" field is padded to approximately targetBytes total size, so
// tests can exercise the size cap without depending on a real file.
func attachmentRequestBody(t *testing.T, targetBytes int) []byte {
	t.Helper()
	// Reserve room for the JSON envelope; pad only the base64 payload.
	const envelopeOverhead = 512
	payloadBytes := targetBytes - envelopeOverhead
	if payloadBytes < 0 {
		payloadBytes = 0
	}
	raw := make([]byte, payloadBytes)
	encoded := base64.StdEncoding.EncodeToString(raw)

	req := domain.CreateAttachmentRequest{
		ReferenceID:   "11111111-1111-1111-1111-111111111111",
		ReferenceType: domain.ReferenceTypeCase,
		Name:          "test-file.bin",
		Type:          "application/octet-stream",
		File:          encoded,
	}
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return body
}

// TestCreateCaseAttachment_UnderNewLimit_OverOldLimit verifies a body larger
// than the old generic 1 MiB cap, but under the new 15 MiB attachment cap,
// is no longer rejected at the decode stage (this is the QA regression: a
// 2 MB attachment used to fail with "request body too large").
func TestCreateCaseAttachment_UnderNewLimit_OverOldLimit(t *testing.T) {
	body := attachmentRequestBody(t, 2<<20) // ~2 MiB, matches QA's repro size.
	if int64(len(body)) <= maxRequestBodySize {
		t.Fatalf("test body (%d bytes) must exceed the old 1 MiB limit (%d) to be meaningful", len(body), maxRequestBodySize)
	}
	if int64(len(body)) >= maxAttachmentBodySize {
		t.Fatalf("test body (%d bytes) must stay under the new attachment limit (%d)", len(body), maxAttachmentBodySize)
	}

	stub := &stubCaseService{
		createAttachmentResp: domain.CreateAttachmentResponse{Message: "created"},
	}
	h := NewCaseHandler(stub, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/attachments", strings.NewReader(string(body)))
	rec := httptest.NewRecorder()

	h.CreateCaseAttachment(rec, req)

	if !stub.createAttachmentCalled {
		t.Fatalf("expected CreateCaseAttachment to reach the service layer, got status %d body %q", rec.Code, rec.Body.String())
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestCreateCaseAttachment_OverNewLimit verifies a body over the new 15 MiB
// attachment cap is still rejected, with the attachment-specific message.
func TestCreateCaseAttachment_OverNewLimit(t *testing.T) {
	body := attachmentRequestBody(t, 16<<20) // ~16 MiB, over the 15 MiB cap.

	stub := &stubCaseService{}
	h := NewCaseHandler(stub, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/attachments", strings.NewReader(string(body)))
	rec := httptest.NewRecorder()

	h.CreateCaseAttachment(rec, req)

	if stub.createAttachmentCalled {
		t.Fatalf("expected the request to be rejected before reaching the service layer")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), attachmentTooLargeMsg) {
		t.Fatalf("expected the attachment-specific too-large message, got: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "request body too large") {
		t.Fatalf("expected the generic message to be replaced by the attachment-specific one, got: %s", rec.Body.String())
	}
}

// searchTagsCalled/searchTagsReq capture what the handler passes down, so the
// tests below can assert the JSON body is decoded into the documented shape.
func (s *stubCaseService) SearchTags(_ context.Context, req domain.SearchTagsRequest) ([]domain.Tag, error) {
	s.searchTagsCalled = true
	s.searchTagsReq = req
	return s.searchTagsTags, s.searchTagsErr
}

// TestSearchTags_DecodesFiltersSearchQuery pins the request shape: the query
// lives under filters.searchQuery and limit is top level. Asserting on the
// decoded request here is safe only because the raw wire format one layer down
// is separately pinned in the service package's tests.
func TestSearchTags_DecodesFiltersSearchQuery(t *testing.T) {
	stub := &stubCaseService{searchTagsTags: []domain.Tag{{ID: "t1", Label: "micro-gw"}}}
	h := NewCaseHandler(stub, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/tags/search",
		strings.NewReader(`{"filters":{"searchQuery":"micro"},"limit":20}`))
	rec := httptest.NewRecorder()

	h.SearchTags(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !stub.searchTagsCalled {
		t.Fatalf("expected SearchTags to reach the service layer")
	}
	if stub.searchTagsReq.Filters.SearchQuery != "micro" {
		t.Fatalf("SearchQuery = %q, want micro", stub.searchTagsReq.Filters.SearchQuery)
	}
	if stub.searchTagsReq.Limit != 20 {
		t.Fatalf("Limit = %d, want 20", stub.searchTagsReq.Limit)
	}
	if !strings.Contains(rec.Body.String(), `"tags"`) {
		t.Fatalf("response must keep the {tags:[...]} envelope, got: %s", rec.Body.String())
	}
}

// TestSearchTags_RejectsLegacyQueryParamShape proves the old GET contract is
// gone: `q` is now an unknown field and the decoder rejects it outright.
func TestSearchTags_RejectsLegacyQueryParamShape(t *testing.T) {
	stub := &stubCaseService{}
	h := NewCaseHandler(stub, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/tags/search", strings.NewReader(`{"q":"micro"}`))
	rec := httptest.NewRecorder()

	h.SearchTags(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for the legacy q key, got %d: %s", rec.Code, rec.Body.String())
	}
	if stub.searchTagsCalled {
		t.Fatalf("expected the request to be rejected before the service layer")
	}
}

func TestSearchTags_RejectsOutOfRangeLimit(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{"negative", `{"limit":-1}`},
		{"over max", `{"limit":101}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubCaseService{}
			h := NewCaseHandler(stub, nil, nil)

			req := httptest.NewRequest(http.MethodPost, "/tags/search", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()

			h.SearchTags(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
			if stub.searchTagsCalled {
				t.Fatalf("expected the request to be rejected before the service layer")
			}
		})
	}
}

// TestSearchTags_EmptyBodyIsValid keeps the "list all known tags" behaviour the
// old GET had when q and limit were both omitted.
func TestSearchTags_EmptyBodyIsValid(t *testing.T) {
	stub := &stubCaseService{}
	h := NewCaseHandler(stub, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/tags/search", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	h.SearchTags(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !stub.searchTagsCalled {
		t.Fatalf("expected SearchTags to reach the service layer")
	}
	if stub.searchTagsReq.Filters.SearchQuery != "" || stub.searchTagsReq.Limit != 0 {
		t.Fatalf("expected a zero request, got %#v", stub.searchTagsReq)
	}
}

// TestSearchTagsQuery_MatchesPostRequest is the guarantee the deprecated GET
// alias exists for: `GET /tags/search?q=micro&limit=5` must reach the service
// layer as the exact same domain.SearchTagsRequest as
// `POST /tags/search {"filters":{"searchQuery":"micro"},"limit":5}`. Because the
// upstream wire body is derived from that request and separately pinned against
// raw bytes in the service package's tests, request equality here means the two
// forms are byte-identical on the wire too.
func TestSearchTagsQuery_MatchesPostRequest(t *testing.T) {
	postStub := &stubCaseService{}
	postReq := httptest.NewRequest(http.MethodPost, "/tags/search",
		strings.NewReader(`{"filters":{"searchQuery":"micro"},"limit":5}`))
	postRec := httptest.NewRecorder()
	NewCaseHandler(postStub, nil, nil).SearchTags(postRec, postReq)
	if postRec.Code != http.StatusOK {
		t.Fatalf("POST: expected 200, got %d: %s", postRec.Code, postRec.Body.String())
	}

	getStub := &stubCaseService{}
	getReq := httptest.NewRequest(http.MethodGet, "/tags/search?q=micro&limit=5", nil)
	getRec := httptest.NewRecorder()
	NewCaseHandler(getStub, nil, nil).SearchTagsQuery(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET: expected 200, got %d: %s", getRec.Code, getRec.Body.String())
	}

	if !getStub.searchTagsCalled {
		t.Fatalf("expected the GET alias to reach the service layer")
	}
	if getStub.searchTagsReq != postStub.searchTagsReq {
		t.Fatalf("GET alias produced %#v, want the POST's %#v", getStub.searchTagsReq, postStub.searchTagsReq)
	}
	if getRec.Body.String() != postRec.Body.String() {
		t.Fatalf("GET alias response %s, want the POST's %s", getRec.Body.String(), postRec.Body.String())
	}
}

// TestSearchTagsQuery_NoParams keeps the "list all known tags" behaviour when q
// and limit are both omitted.
func TestSearchTagsQuery_NoParams(t *testing.T) {
	stub := &stubCaseService{}
	rec := httptest.NewRecorder()

	NewCaseHandler(stub, nil, nil).SearchTagsQuery(rec, httptest.NewRequest(http.MethodGet, "/tags/search", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !stub.searchTagsCalled {
		t.Fatalf("expected SearchTags to reach the service layer")
	}
	if (stub.searchTagsReq != domain.SearchTagsRequest{}) {
		t.Fatalf("expected a zero request, got %#v", stub.searchTagsReq)
	}
}

// TestGetAttachment_PassesPathIDToService pins the GET /attachments/{id} handler's
// wiring: the path value must reach the service layer as-is (the service layer
// owns UUID validation), and the response body must round-trip the service result.
func TestGetAttachment_PassesPathIDToService(t *testing.T) {
	const attachmentID = "11111111-1111-1111-1111-111111111111"
	stub := &stubCaseService{getAttachmentResp: domain.AttachmentDetails{ID: attachmentID, Name: "logs.txt"}}
	h := NewCaseHandler(stub, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/attachments/"+attachmentID, nil)
	req.SetPathValue("id", attachmentID)
	rec := httptest.NewRecorder()

	h.GetAttachmentByID(rec, req)

	if !stub.getAttachmentCalled {
		t.Fatalf("expected GetAttachmentByID to reach the service layer")
	}
	if stub.getAttachmentID != attachmentID {
		t.Fatalf("service saw id %q, want %q", stub.getAttachmentID, attachmentID)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "logs.txt") {
		t.Fatalf("expected response body to contain the attachment name, got: %s", rec.Body.String())
	}
}

// TestUpdateAttachment_DecodesBodyAndPathID pins the PATCH /attachments/{id}
// handler's wiring: the path id populates req.AttachmentID (the request body has
// no id field of its own -- see domain.UpdateAttachmentRequest's `json:"-"` tag),
// and the JSON body fields decode into the rest of the request.
func TestUpdateAttachment_DecodesBodyAndPathID(t *testing.T) {
	const attachmentID = "11111111-1111-1111-1111-111111111111"
	stub := &stubCaseService{
		updateAttachmentResp: domain.UpdateAttachmentResponse{Message: "updated"},
	}
	h := NewCaseHandler(stub, nil, nil)

	body := `{"referenceId":"22222222-2222-2222-2222-222222222222","referenceType":"deployment","name":"renamed.txt"}`
	req := httptest.NewRequest(http.MethodPatch, "/attachments/"+attachmentID, strings.NewReader(body))
	req.SetPathValue("id", attachmentID)
	rec := httptest.NewRecorder()

	h.UpdateAttachment(rec, req)

	if !stub.updateAttachmentCalled {
		t.Fatalf("expected UpdateAttachment to reach the service layer, got status %d body %q", rec.Code, rec.Body.String())
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if stub.updateAttachmentReq.AttachmentID != attachmentID {
		t.Fatalf("AttachmentID = %q, want %q (from the URL path, not the body)", stub.updateAttachmentReq.AttachmentID, attachmentID)
	}
	if stub.updateAttachmentReq.ReferenceID != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("ReferenceID = %q, want the decoded body value", stub.updateAttachmentReq.ReferenceID)
	}
	if stub.updateAttachmentReq.ReferenceType != domain.ReferenceTypeDeployment {
		t.Fatalf("ReferenceType = %q, want %q", stub.updateAttachmentReq.ReferenceType, domain.ReferenceTypeDeployment)
	}
	if stub.updateAttachmentReq.Name == nil || *stub.updateAttachmentReq.Name != "renamed.txt" {
		t.Fatalf("Name = %v, want renamed.txt", stub.updateAttachmentReq.Name)
	}
}

// TestUpdateAttachment_RejectsMalformedBody proves a body the decoder cannot
// parse is rejected before the service layer runs.
func TestUpdateAttachment_RejectsMalformedBody(t *testing.T) {
	stub := &stubCaseService{}
	h := NewCaseHandler(stub, nil, nil)

	req := httptest.NewRequest(http.MethodPatch, "/attachments/x", strings.NewReader(`{"name":`))
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()

	h.UpdateAttachment(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if stub.updateAttachmentCalled {
		t.Fatalf("expected the request to be rejected before the service layer")
	}
}

// TestSearchTagsQuery_RejectsBadLimit covers the two limit values the alias must
// refuse: one it cannot parse, and ones outside the range the POST enforces. The
// bounds check is shared code, so this also proves the alias runs through it.
func TestSearchTagsQuery_RejectsBadLimit(t *testing.T) {
	for _, tc := range []struct {
		name string
		url  string
	}{
		{"non-numeric", "/tags/search?q=micro&limit=abc"},
		{"negative", "/tags/search?q=micro&limit=-1"},
		{"over max", "/tags/search?q=micro&limit=101"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubCaseService{}
			rec := httptest.NewRecorder()

			NewCaseHandler(stub, nil, nil).SearchTagsQuery(rec, httptest.NewRequest(http.MethodGet, tc.url, nil))

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
			if stub.searchTagsCalled {
				t.Fatalf("expected the request to be rejected before the service layer")
			}
		})
	}
}

// addCaseTagRequest builds a POST /cases/{id}/tags request, optionally
// carrying an actorEmail in the body and/or a real x-user-id-token header --
// the latter is only observed by the handler when the request is routed
// through middleware.UserIDToken, as runAddCaseTag below does.
func addCaseTagRequest(t *testing.T, caseID string, actorEmail *string, userIDToken string) *http.Request {
	t.Helper()
	body := domain.AddCaseTagRequest{Label: "micro-gw", ActorEmail: actorEmail}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/tags", strings.NewReader(string(raw)))
	req.SetPathValue("id", caseID)
	if userIDToken != "" {
		req.Header.Set("x-user-id-token", userIDToken)
	}
	return withM2MClient(req)
}

// runAddCaseTag drives h.AddCaseTag through middleware.UserIDToken, exactly
// as routes.go wires the real mux, so req.Header's x-user-id-token (if any)
// actually reaches the handler via the request context rather than being
// silently ignored.
func runAddCaseTag(h *CaseHandler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	middleware.UserIDToken(http.HandlerFunc(h.AddCaseTag)).ServeHTTP(rec, req)
	return rec
}

const testAddCaseTagCaseID = "11111111-1111-1111-1111-111111111111"

// TestAddCaseTag_AllowlistedActorEmail_NoToken covers the M2M path this
// endpoint was added for: an allowlisted actorEmail with no x-user-id-token
// must succeed and call AddCaseTagAs (not AddCaseTag) with the exact case
// ID, label, and actor email supplied. The allowlist and the request use
// different casing to prove the comparison is case-insensitive.
func TestAddCaseTag_AllowlistedActorEmail_NoToken(t *testing.T) {
	stub := &stubCaseService{addCaseTagAsResp: domain.Tag{ID: "t1", Label: "micro-gw"}}
	h := NewCaseHandler(stub, []string{"UMT-Service@Example.com"}, testM2MClients)

	actor := "umt-service@example.com"
	req := addCaseTagRequest(t, testAddCaseTagCaseID, &actor, "")
	rec := runAddCaseTag(h, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if !stub.addCaseTagAsCalled {
		t.Fatalf("expected AddCaseTagAs to be called")
	}
	if stub.addCaseTagCalled {
		t.Fatalf("expected AddCaseTag (token-resolved path) NOT to be called")
	}
	if stub.addCaseTagAsCaseID != testAddCaseTagCaseID {
		t.Fatalf("expected caseID %q, got %q", testAddCaseTagCaseID, stub.addCaseTagAsCaseID)
	}
	if stub.addCaseTagAsLabel != "micro-gw" {
		t.Fatalf("expected label %q, got %q", "micro-gw", stub.addCaseTagAsLabel)
	}
	if stub.addCaseTagAsActor != actor {
		t.Fatalf("expected actorEmail %q, got %q", actor, stub.addCaseTagAsActor)
	}
}

// TestAddCaseTag_NonAllowlistedActorEmail_Rejected covers an actorEmail that
// is not on the configured allowlist: it must be rejected with 403 directly
// from the handler, never reaching the service layer at all.
func TestAddCaseTag_NonAllowlistedActorEmail_Rejected(t *testing.T) {
	stub := &stubCaseService{}
	h := NewCaseHandler(stub, []string{"umt-service@example.com"}, testM2MClients)

	actor := "someone-else@example.com"
	req := addCaseTagRequest(t, testAddCaseTagCaseID, &actor, "")
	rec := runAddCaseTag(h, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if stub.addCaseTagCalled || stub.addCaseTagAsCalled {
		t.Fatalf("expected the request to be rejected before the service layer")
	}
}

// TestAddCaseTag_ActorEmailAndToken_Rejected covers the mutual-exclusion
// rule: a request carrying both a real x-user-id-token and an actorEmail is
// a 400, not a silent pick of one over the other.
func TestAddCaseTag_ActorEmailAndToken_Rejected(t *testing.T) {
	stub := &stubCaseService{}
	h := NewCaseHandler(stub, []string{"umt-service@example.com"}, testM2MClients)

	actor := "umt-service@example.com"
	req := addCaseTagRequest(t, testAddCaseTagCaseID, &actor, "a-real-token")
	rec := runAddCaseTag(h, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if stub.addCaseTagCalled || stub.addCaseTagAsCalled {
		t.Fatalf("expected the request to be rejected before the service layer")
	}
}

// TestAddCaseTag_NeitherActorEmailNorToken_UnchangedBehavior pins that the
// pre-existing path is untouched: no actorEmail and no token must still
// reach CaseService.AddCaseTag (which itself 401s via resolveActor when
// there's no token -- exercised in case_service_test.go, not duplicated
// here) rather than AddCaseTagAs.
func TestAddCaseTag_NeitherActorEmailNorToken_UnchangedBehavior(t *testing.T) {
	stub := &stubCaseService{addCaseTagErr: &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}}
	h := NewCaseHandler(stub, nil, nil)

	req := addCaseTagRequest(t, testAddCaseTagCaseID, nil, "")
	rec := runAddCaseTag(h, req)

	if !stub.addCaseTagCalled {
		t.Fatalf("expected AddCaseTag to be called (unchanged existing behavior)")
	}
	if stub.addCaseTagAsCalled {
		t.Fatalf("expected AddCaseTagAs NOT to be called")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func createCaseCommentRequest(t *testing.T, caseID string, actorEmail *string, userIDToken string) *http.Request {
	t.Helper()
	body := domain.CreateCaseCommentRequest{Type: domain.CommentTypeComment, Content: "hello", ActorEmail: actorEmail}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/comments", strings.NewReader(string(raw)))
	req.SetPathValue("id", caseID)
	if userIDToken != "" {
		req.Header.Set("x-user-id-token", userIDToken)
	}
	return withM2MClient(req)
}

// runCreateCaseComment drives h.CreateCaseComment through
// middleware.UserIDToken, exactly as routes.go wires the real mux, so
// req.Header's x-user-id-token (if any) actually reaches the handler via the
// request context rather than being silently ignored.
func runCreateCaseComment(h *CaseHandler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	middleware.UserIDToken(http.HandlerFunc(h.CreateCaseComment)).ServeHTTP(rec, req)
	return rec
}

const testCreateCaseCommentCaseID = "22222222-2222-2222-2222-222222222222"

// TestCreateCaseComment_AllowlistedActorEmail_NoToken covers the M2M path
// this endpoint was added for: an allowlisted actorEmail with no
// x-user-id-token must succeed and call CreateCaseCommentAs (not
// CreateCaseComment) with the exact request and actor email supplied. The
// allowlist and the request use different casing to prove the comparison is
// case-insensitive.
func TestCreateCaseComment_AllowlistedActorEmail_NoToken(t *testing.T) {
	stub := &stubCaseService{createCaseCommentAsResp: domain.CreateCaseCommentResponse{Message: "Comment created successfully"}}
	h := NewCaseHandler(stub, []string{"UMT-Service@Example.com"}, testM2MClients)

	actor := "umt-service@example.com"
	req := createCaseCommentRequest(t, testCreateCaseCommentCaseID, &actor, "")
	rec := runCreateCaseComment(h, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if !stub.createCaseCommentAsCalled {
		t.Fatalf("expected CreateCaseCommentAs to be called")
	}
	if stub.createCaseCommentCalled {
		t.Fatalf("expected CreateCaseComment (token-resolved path) NOT to be called")
	}
	if stub.createCaseCommentAsReq.CaseID != testCreateCaseCommentCaseID {
		t.Fatalf("expected caseID %q, got %q", testCreateCaseCommentCaseID, stub.createCaseCommentAsReq.CaseID)
	}
	if stub.createCaseCommentAsReq.Content != "hello" {
		t.Fatalf("expected content %q, got %q", "hello", stub.createCaseCommentAsReq.Content)
	}
	if stub.createCaseCommentAsActor != actor {
		t.Fatalf("expected actorEmail %q, got %q", actor, stub.createCaseCommentAsActor)
	}
}

// TestCreateCaseComment_NonAllowlistedActorEmail_Rejected covers an
// actorEmail that is not on the configured allowlist: it must be rejected
// with 403 directly from the handler, never reaching the service layer at
// all.
func TestCreateCaseComment_NonAllowlistedActorEmail_Rejected(t *testing.T) {
	stub := &stubCaseService{}
	h := NewCaseHandler(stub, []string{"umt-service@example.com"}, testM2MClients)

	actor := "someone-else@example.com"
	req := createCaseCommentRequest(t, testCreateCaseCommentCaseID, &actor, "")
	rec := runCreateCaseComment(h, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if stub.createCaseCommentCalled || stub.createCaseCommentAsCalled {
		t.Fatalf("expected the request to be rejected before the service layer")
	}
}

// TestCreateCaseComment_ActorEmailAndToken_Rejected covers the mutual-
// exclusion rule: a request carrying both a real x-user-id-token and an
// actorEmail is a 400, not a silent pick of one over the other.
func TestCreateCaseComment_ActorEmailAndToken_Rejected(t *testing.T) {
	stub := &stubCaseService{}
	h := NewCaseHandler(stub, []string{"umt-service@example.com"}, testM2MClients)

	actor := "umt-service@example.com"
	req := createCaseCommentRequest(t, testCreateCaseCommentCaseID, &actor, "a-real-token")
	rec := runCreateCaseComment(h, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if stub.createCaseCommentCalled || stub.createCaseCommentAsCalled {
		t.Fatalf("expected the request to be rejected before the service layer")
	}
}

// TestCreateCaseComment_NeitherActorEmailNorToken_UnchangedBehavior pins
// that the pre-existing path is untouched: no actorEmail and no token must
// still reach CaseService.CreateCaseComment (which itself 401s via
// x-user-id-token resolution when there's no token -- exercised in
// case_service_test.go, not duplicated here) rather than
// CreateCaseCommentAs.
func TestCreateCaseComment_NeitherActorEmailNorToken_UnchangedBehavior(t *testing.T) {
	stub := &stubCaseService{createCaseCommentErr: &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}}
	h := NewCaseHandler(stub, nil, nil)

	req := createCaseCommentRequest(t, testCreateCaseCommentCaseID, nil, "")
	rec := runCreateCaseComment(h, req)

	if !stub.createCaseCommentCalled {
		t.Fatalf("expected CreateCaseComment to be called (unchanged existing behavior)")
	}
	if stub.createCaseCommentAsCalled {
		t.Fatalf("expected CreateCaseCommentAs NOT to be called")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

// testM2MClient is the internal client id the M2M actorEmail tests present;
// testM2MClients is the allow-list the handlers under test are built with.
const testM2MClient = "m2m-client"

var testM2MClients = map[string]bool{testM2MClient: true}

// withM2MClient stamps req with a validated identity carrying testM2MClient,
// as auth.Middleware would for an x-jwt-assertion naming it.
func withM2MClient(req *http.Request) *http.Request {
	return withClientID(req, testM2MClient)
}

func withClientID(req *http.Request, clientID string) *http.Request {
	return req.WithContext(auth.WithIdentity(req.Context(), auth.Identity{Validated: true, ClientID: clientID}))
}

// TestActorEmail_RequiresAnAllowlistedInternalClient pins that an
// allow-listed service-account email is not enough on its own: the request
// must also come from an allow-listed internal client, on both endpoints that
// accept an actorEmail.
func TestActorEmail_RequiresAnAllowlistedInternalClient(t *testing.T) {
	actor := "umt-service@example.com"
	for name, clientID := range map[string]string{
		"no client credential": "",
		"unknown client":       "some-other-client",
	} {
		t.Run("tag/"+name, func(t *testing.T) {
			stub := &stubCaseService{}
			h := NewCaseHandler(stub, []string{actor}, testM2MClients)
			rec := runAddCaseTag(h, withClientID(addCaseTagRequest(t, testAddCaseTagCaseID, &actor, ""), clientID))
			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
			}
			if stub.addCaseTagCalled || stub.addCaseTagAsCalled {
				t.Fatal("expected the request to be rejected before the service layer")
			}
		})
		t.Run("comment/"+name, func(t *testing.T) {
			stub := &stubCaseService{}
			h := NewCaseHandler(stub, []string{actor}, testM2MClients)
			rec := runCreateCaseComment(h, withClientID(createCaseCommentRequest(t, testCreateCaseCommentCaseID, &actor, ""), clientID))
			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
			}
			if stub.createCaseCommentAsActor != "" {
				t.Fatal("expected the request to be rejected before the service layer")
			}
		})
	}
}

// contentCaseService serves one attachment's bytes and type.
type contentCaseService struct {
	service.CaseService
	content     []byte
	contentType string
}

func (s contentCaseService) GetCaseAttachmentContent(context.Context, string) ([]byte, string, error) {
	return s.content, s.contentType, nil
}

// TestGetCaseAttachmentContent_DownloadHeaders pins the headers every download
// carries: the allow-listed (or fallback) type, attachment disposition, and
// nosniff so the browser does not second-guess that type.
func TestGetCaseAttachmentContent_DownloadHeaders(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"text/plain; charset=utf-8", "text/plain"},
		{"text/html", "application/octet-stream"},
	} {
		h := NewCaseHandler(contentCaseService{content: []byte("hello"), contentType: tc.in}, nil, nil)
		req := httptest.NewRequest(http.MethodGet, "/attachments/a-1/content", nil)
		req.SetPathValue("id", "a-1")
		rec := httptest.NewRecorder()
		h.GetCaseAttachmentContent(rec, req)

		if got := rec.Header().Get("Content-Type"); got != tc.want {
			t.Errorf("%s: Content-Type = %q, want %q", tc.in, got, tc.want)
		}
		if got := rec.Header().Get("Content-Disposition"); got != "attachment" {
			t.Errorf("%s: Content-Disposition = %q, want attachment", tc.in, got)
		}
		if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s: X-Content-Type-Options = %q, want nosniff", tc.in, got)
		}
	}
}
