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
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityScheduleClient abstracts the entity service Team Schedule operations.
type entityScheduleClient interface {
	GetScheduleCatalogue(ctx context.Context) ([]byte, error)
	SearchScheduleAssignments(ctx context.Context, body []byte) ([]byte, error)
	SearchScheduleAbsences(ctx context.Context, body []byte) ([]byte, error)
	SearchScheduleEngagementAllocations(ctx context.Context, body []byte) ([]byte, error)
	DismissScheduleEngagementAllocation(ctx context.Context, body []byte) ([]byte, error)
	GetScheduleOnDuty(ctx context.Context, at string) ([]byte, error)

	// Lead edit. The portal proxies these unchanged -- entity-service is where
	// "may this person edit this team's rota" is decided.
	CreateScheduleAssignment(ctx context.Context, body []byte) ([]byte, error)
	UpdateScheduleAssignment(ctx context.Context, id string, body []byte) ([]byte, error)
	DeleteScheduleAssignment(ctx context.Context, id, note string) ([]byte, error)
	GetScheduleActivity(ctx context.Context, teamKey, from, to string) ([]byte, error)
	GetMyLeadTeams(ctx context.Context) ([]byte, error)
	ApplyScheduleRange(ctx context.Context, body []byte) ([]byte, error)
	ApplyScheduleAbsence(ctx context.Context, body []byte) ([]byte, error)
	DeleteScheduleAbsence(ctx context.Context, id, note string) ([]byte, error)
	CreateScheduleAbsenceKind(ctx context.Context, body []byte) ([]byte, error)
	DeleteScheduleAbsenceKind(ctx context.Context, code string) ([]byte, error)
	GetScheduleEditMarkers(ctx context.Context, from, to string) ([]byte, error)

	// Case Paging: who is on each tier of the paging chain, and one change to
	// it. entity-service decides who may change what.
	GetPagingChain(ctx context.Context, family string) ([]byte, error)
	UpdatePagingMember(ctx context.Context, id string, body []byte) ([]byte, error)

	// A paging-only phone number for a person with none on their profile,
	// kept by entity-service and never written to Asgardeo, and a test call
	// to it.
	PutPagingContact(ctx context.Context, userID string, body []byte) ([]byte, error)
	DeletePagingContact(ctx context.Context, userID string) ([]byte, error)
	TestPagingContact(ctx context.Context, userID string) ([]byte, error)
}

// ScheduleHandler handles the Team Schedule reads: who is working, when, and
// who is out of the rota.
type ScheduleHandler struct {
	entity entityScheduleClient
	// phones adds "hasProfilePhone" to the paging chain's members; nil leaves
	// the chain as entity-service sent it.
	phones *PagingPhoneChecker
}

// NewScheduleHandler creates a ScheduleHandler backed by the given entity client.
func NewScheduleHandler(entity entityScheduleClient) *ScheduleHandler {
	return &ScheduleHandler{entity: entity}
}

// WithPagingPhones makes GET /team-schedule/paging-chain say, per member,
// whether they have a mobile number on their profile: entity-service's answer
// where it gives one, else the same SCIM cache the readiness strip reads.
func (h *ScheduleHandler) WithPagingPhones(phones *PagingPhoneChecker) *ScheduleHandler {
	h.phones = phones
	return h
}

// readScheduleBody authenticates the caller and returns the request body,
// having checked it is valid JSON and within the size limit. Returns ok=false
// when it has already written the response.
func readScheduleBody(w http.ResponseWriter, r *http.Request) (body []byte, userID string, ok bool) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return nil, "", false
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return nil, "", false
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return nil, "", false
	}

	if !json.Valid(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return nil, "", false
	}
	return body, user.UserID, true
}

// GetScheduleCatalogue handles GET /team-schedule/catalogue.
func (h *ScheduleHandler) GetScheduleCatalogue(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	result, err := h.entity.GetScheduleCatalogue(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetScheduleCatalogue failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to load the schedule catalogue.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchScheduleAssignments handles POST /team-schedule/assignments/search.
func (h *ScheduleHandler) SearchScheduleAssignments(w http.ResponseWriter, r *http.Request) {
	body, userID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.SearchScheduleAssignments(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchScheduleAssignments failed", "userID", userID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search the schedule.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchScheduleAbsences handles POST /team-schedule/absences/search.
func (h *ScheduleHandler) SearchScheduleAbsences(w http.ResponseWriter, r *http.Request) {
	body, userID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.SearchScheduleAbsences(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchScheduleAbsences failed", "userID", userID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search schedule absences.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// SearchScheduleEngagementAllocations handles
// POST /team-schedule/engagement-allocations/search: the confirmed customer
// engagement allocations of the given engineers over a date window, shown
// read-only on the schedule. The body (userIds, from, to) is validated by
// entity-service and forwarded unchanged; the response is returned as sent.
func (h *ScheduleHandler) SearchScheduleEngagementAllocations(w http.ResponseWriter, r *http.Request) {
	body, userID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.SearchScheduleEngagementAllocations(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchScheduleEngagementAllocations failed", "userID", userID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search engagement allocations.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// DismissScheduleEngagementAllocation handles
// POST /team-schedule/engagement-allocations/dismiss: a lead or rota admin
// taking a customer engagement allocation (from the allocation app) off a
// person's roster for some days, or putting them back. Who may is the entity
// service's check, the same as marking a tag; its refusal is shown as written.
func (h *ScheduleHandler) DismissScheduleEngagementAllocation(w http.ResponseWriter, r *http.Request) {
	body, userID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.DismissScheduleEngagementAllocation(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity DismissScheduleEngagementAllocation failed", "userID", userID, "err", err)
		mapScheduleWriteError(w, err, "Failed to change the engagement allocation.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetScheduleOnDuty handles GET /team-schedule/on-duty[?at=RFC3339] -- who is
// responsible at this instant. The `at` parameter is passed through unchanged;
// the entity service validates it.
func (h *ScheduleHandler) GetScheduleOnDuty(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	result, err := h.entity.GetScheduleOnDuty(r.Context(), r.URL.Query().Get("at"))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetScheduleOnDuty failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to load who is on duty.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// CreateScheduleAssignment handles POST /team-schedule/assignments.
//
// The portal does not decide who may edit: entity-service checks that the
// caller leads the team the slot belongs to, and the body is passed through
// unchanged. PermWrite here only keeps the control out of the hands of someone
// who could not use it at all.
func (h *ScheduleHandler) CreateScheduleAssignment(w http.ResponseWriter, r *http.Request) {
	body, userID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.CreateScheduleAssignment(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateScheduleAssignment failed", "userID", userID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to add the assignment.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// UpdateScheduleAssignment handles PATCH /team-schedule/assignments/{id}.
func (h *ScheduleHandler) UpdateScheduleAssignment(w http.ResponseWriter, r *http.Request) {
	body, userID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.UpdateScheduleAssignment(r.Context(), r.PathValue("id"), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateScheduleAssignment failed", "userID", userID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to change the assignment.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// DeleteScheduleAssignment handles DELETE /team-schedule/assignments/{id}.
func (h *ScheduleHandler) DeleteScheduleAssignment(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	if _, err := h.entity.DeleteScheduleAssignment(r.Context(), r.PathValue("id"), r.URL.Query().Get("note")); err != nil {
		slog.ErrorContext(r.Context(), "entity DeleteScheduleAssignment failed", "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to remove the assignment.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetScheduleActivity handles GET /team-schedule/activity.
func (h *ScheduleHandler) GetScheduleActivity(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	q := r.URL.Query()
	result, err := h.entity.GetScheduleActivity(r.Context(), q.Get("teamKey"), q.Get("from"), q.Get("to"))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetScheduleActivity failed", "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to load the schedule history.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// pagingFamilies is the closed set of families the Case Paging chain is
// asked for by; anything else is refused here rather than forwarded.
var pagingFamilies = map[string]bool{"": true, "CRE": true, "SRE": true}

// GetPagingChain handles GET /team-schedule/paging-chain -- the Case Paging
// tab's read: who is on each tier, and what this user may change. The entity
// service decides the permissions; this only passes them through.
func (h *ScheduleHandler) GetPagingChain(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	family := r.URL.Query().Get("family")
	if !pagingFamilies[family] {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.GetPagingChain(r.Context(), family)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetPagingChain failed", "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to load the paging chain.")
		return
	}

	writeJSON(w, http.StatusOK, h.withProfilePhones(r.Context(), result))
}

// withProfilePhones sets "hasProfilePhone" on each member of a paging-chain
// response, leaving every other field as entity-service sent it.
// entity-service's own answer stands where it says true, and where it says
// false once the SCIM fallback is off; SCIM answers the rest. Best effort: a
// member whose lookup failed keeps what entity-service sent (no field from an
// older one), and a response that cannot be read is returned unchanged.
func (h *ScheduleHandler) withProfilePhones(ctx context.Context, raw []byte) []byte {
	if h.phones == nil {
		return raw
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return raw
	}
	var members []map[string]json.RawMessage
	if err := json.Unmarshal(top["members"], &members); err != nil || len(members) == 0 {
		return raw
	}

	emailOf := func(m map[string]json.RawMessage) string {
		var e string
		_ = json.Unmarshal(m["email"], &e)
		return normalizeEmail(e)
	}
	seen := make(map[string]bool)
	entityAnswer := make(map[string]bool)
	var emails []string
	for _, m := range members {
		e := emailOf(m)
		if e == "" {
			continue
		}
		if !seen[e] {
			seen[e] = true
			emails = append(emails, e)
		}
		var has bool
		if raw, ok := m["hasProfilePhone"]; ok && json.Unmarshal(raw, &has) == nil {
			entityAnswer[e] = entityAnswer[e] || has
		}
	}
	hasPhone, failed := h.phones.profilePhones(ctx, emails, entityAnswer)
	if len(failed) > 0 {
		slog.WarnContext(ctx, "paging chain: profile phone check unavailable for some people", "count", len(failed))
	}

	for _, m := range members {
		e := emailOf(m)
		if e == "" || failed[e] {
			continue
		}
		if hasPhone[e] {
			m["hasProfilePhone"] = json.RawMessage("true")
		} else {
			m["hasProfilePhone"] = json.RawMessage("false")
		}
	}
	enc, err := json.Marshal(members)
	if err != nil {
		return raw
	}
	top["members"] = enc
	out, err := json.Marshal(top)
	if err != nil {
		return raw
	}
	return out
}

// UpdatePagingMember handles PATCH /team-schedule/paging-chain/members/{id}:
// one change to who is on the Case Paging chain. The entity service checks who
// may make it, against the membership being changed; its refusal reason is
// kept, the same as a lead's edit to the rota.
func (h *ScheduleHandler) UpdatePagingMember(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	body, userID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}

	if status, msg := h.responderPhoneRefusal(r.Context(), id, body); status != 0 {
		writeErrorCode(w, status, msg, errCodeResponderNeedsProfilePhone)
		return
	}

	result, err := h.entity.UpdatePagingMember(r.Context(), id, body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdatePagingMember failed", "userID", userID, "err", err)
		mapScheduleWriteError(w, err, "Failed to change the paging chain.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// errCodeResponderNeedsProfilePhone names the refusal of a 1st, 2nd or 3rd
// responder with no phone number on their CSM Profile.
const errCodeResponderNeedsProfilePhone = "paging_responder_needs_profile_phone"

// pagingChainFamilies are the chains a membership is looked up in.
var pagingChainFamilies = []string{"CRE", "SRE"}

// responderPhoneRefusal decides whether a change that makes someone a 1st,
// 2nd or 3rd responder (Level 0, the people paged first) must be refused
// because they have no phone number on their CSM Profile: a responder nobody
// can call leaves the chain's first level empty in practice. It returns the
// status and message to answer with, or status 0 to let the change through.
//
// "Has a number" is the same answer the chain's own read gives (entity-
// service's "user".phone, then the SCIM fallback), so the page and this
// check never disagree. Clearing a responder (rank 0), every other change,
// and a membership the chains do not list are left to entity-service. An
// answer that cannot be had just now refuses the change rather than letting
// an unreachable responder in.
func (h *ScheduleHandler) responderPhoneRefusal(ctx context.Context, membershipID string, body []byte) (int, string) {
	var req struct {
		ResponderRank *int `json:"responderRank"`
	}
	if err := json.Unmarshal(body, &req); err != nil || req.ResponderRank == nil || *req.ResponderRank < 1 || *req.ResponderRank > 3 {
		return 0, ""
	}

	type chainMember struct {
		MembershipID    string `json:"membershipId"`
		Name            string `json:"name"`
		Email           string `json:"email"`
		HasProfilePhone *bool  `json:"hasProfilePhone"`
	}
	var member *chainMember
	for _, family := range pagingChainFamilies {
		raw, err := h.entity.GetPagingChain(ctx, family)
		if err != nil {
			slog.WarnContext(ctx, "paging responder check: could not read the chain", "family", family, "err", err)
			continue
		}
		var chain struct {
			Members []chainMember `json:"members"`
		}
		if json.Unmarshal(raw, &chain) != nil {
			continue
		}
		for i := range chain.Members {
			if strings.EqualFold(chain.Members[i].MembershipID, membershipID) {
				member = &chain.Members[i]
				break
			}
		}
		if member != nil {
			break
		}
	}
	if member == nil {
		return 0, ""
	}

	name := strings.TrimSpace(member.Name)
	if name == "" {
		name = member.Email
	}
	has := member.HasProfilePhone != nil && *member.HasProfilePhone
	email := normalizeEmail(member.Email)
	if !has && h.phones != nil && email != "" {
		entityAnswer := map[string]bool{}
		if member.HasProfilePhone != nil {
			entityAnswer[email] = *member.HasProfilePhone
		}
		hasPhone, failed := h.phones.profilePhones(ctx, []string{email}, entityAnswer)
		if failed[email] {
			return http.StatusServiceUnavailable, "Could not check whether " + name + " has a phone number on their CSM Profile just now. Try again."
		}
		has = hasPhone[email]
	} else if !has && member.HasProfilePhone == nil {
		// Nobody can say either way: entity-service is left to decide.
		return 0, ""
	}
	if has {
		return 0, ""
	}
	return http.StatusConflict, name + " can't be added to Level 0: a phone number should be included in their CSM Profile (avatar › Profile)"
}

// e164Re is a phone number in E.164: a +, a country code that does not start
// with 0, and 7 to 15 digits in all.
var e164Re = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)

// pagingContactUserID authenticates the caller and returns the {userId} path
// value, having checked it is a UUID. Returns ok=false when it has already
// written the response.
func pagingContactUserID(w http.ResponseWriter, r *http.Request) (userID, callerID string, ok bool) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return "", "", false
	}
	id := r.PathValue("userId")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return "", "", false
	}
	return id, user.UserID, true
}

// mapPagingContactError is mapScheduleWriteError, plus the test call's "too
// soon": a 429 is kept as a 429, with a fixed message, so the page can say to
// wait rather than that something broke.
func mapPagingContactError(w http.ResponseWriter, err error, fallbackMsg string) {
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusTooManyRequests {
		writeError(w, http.StatusTooManyRequests, "This number was tested less than 2 minutes ago. Try again shortly.")
		return
	}
	mapScheduleWriteError(w, err, fallbackMsg)
}

// PutPagingContact handles PUT /team-schedule/paging-contacts/{userId}: set a
// person's paging-only number. The number must be E.164; it is never logged.
func (h *ScheduleHandler) PutPagingContact(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := pagingContactUserID(w, r)
	if !ok {
		return
	}
	body, callerID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}
	var req struct {
		Phone *string `json:"phone"`
	}
	if err := json.Unmarshal(body, &req); err != nil || req.Phone == nil || !e164Re.MatchString(*req.Phone) {
		writeError(w, http.StatusBadRequest, "phone must be a number in E.164 form, e.g. +94771234567")
		return
	}
	// Rebuilt rather than forwarded, so nothing but the number reaches
	// entity-service.
	fwd, err := json.Marshal(map[string]string{"phone": *req.Phone})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to save the paging number.")
		return
	}

	result, err := h.entity.PutPagingContact(r.Context(), userID, fwd)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity PutPagingContact failed", "userID", callerID, "err", err)
		mapPagingContactError(w, err, "Failed to save the paging number.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// DeletePagingContact handles DELETE /team-schedule/paging-contacts/{userId}.
func (h *ScheduleHandler) DeletePagingContact(w http.ResponseWriter, r *http.Request) {
	userID, callerID, ok := pagingContactUserID(w, r)
	if !ok {
		return
	}
	if _, err := h.entity.DeletePagingContact(r.Context(), userID); err != nil {
		slog.ErrorContext(r.Context(), "entity DeletePagingContact failed", "userID", callerID, "err", err)
		mapPagingContactError(w, err, "Failed to remove the paging number.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// TestPagingContact handles POST /team-schedule/paging-contacts/{userId}/test:
// a test call to the person's paging number. 202 while the call is placed; the
// result lands on the paging chain's member as lastTestStatus.
func (h *ScheduleHandler) TestPagingContact(w http.ResponseWriter, r *http.Request) {
	userID, callerID, ok := pagingContactUserID(w, r)
	if !ok {
		return
	}
	result, err := h.entity.TestPagingContact(r.Context(), userID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity TestPagingContact failed", "userID", callerID, "err", err)
		mapPagingContactError(w, err, "Failed to start the test call.")
		return
	}
	if len(result) == 0 {
		result = []byte(`{"lastTestStatus":"pending"}`)
	}
	writeJSON(w, http.StatusAccepted, result)
}

// GetMyLeadTeams handles GET /team-schedule/my-lead-teams.
func (h *ScheduleHandler) GetMyLeadTeams(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	result, err := h.entity.GetMyLeadTeams(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetMyLeadTeams failed", "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to check your team permissions.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// mapScheduleWriteError is mapUpstreamErrorGeneric for a lead's edit to the
// rota, except that a refusal keeps entity-service's own reason. Its 403 and
// 409 messages are written for the lead -- "that engineer is not on
// americas...", "this person already has a window that overlaps this one..."
// -- and without them the picker could only say that nothing had saved,
// leaving a lead to guess why. Every other status stays generic, so nothing
// internal reaches the page.
func mapScheduleWriteError(w http.ResponseWriter, err error, fallbackMsg string) {
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusForbidden || apiErr.StatusCode == http.StatusConflict) {
		writeError(w, apiErr.StatusCode, upstreamErrorMessageStrict(apiErr.Body, fallbackMsg))
		return
	}
	mapUpstreamErrorGeneric(w, err, fallbackMsg)
}

// ApplyScheduleRange handles POST /team-schedule/assignments/apply.
func (h *ScheduleHandler) ApplyScheduleRange(w http.ResponseWriter, r *http.Request) {
	body, userID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.ApplyScheduleRange(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ApplyScheduleRange failed", "userID", userID, "err", err)
		mapScheduleWriteError(w, err, "Failed to change the rota.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ApplyScheduleAbsence handles POST /team-schedule/absences/apply.
func (h *ScheduleHandler) ApplyScheduleAbsence(w http.ResponseWriter, r *http.Request) {
	body, userID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.ApplyScheduleAbsence(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ApplyScheduleAbsence failed", "userID", userID, "err", err)
		mapScheduleWriteError(w, err, "Failed to change who is away.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// DeleteScheduleAbsence handles DELETE /team-schedule/absences/{id} -- the
// picker removing a whole span of leave or allocation in one click.
func (h *ScheduleHandler) DeleteScheduleAbsence(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	if _, err := h.entity.DeleteScheduleAbsence(r.Context(), r.PathValue("id"), r.URL.Query().Get("note")); err != nil {
		slog.ErrorContext(r.Context(), "entity DeleteScheduleAbsence failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to remove that leave or allocation.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CreateScheduleAbsenceKind handles POST /team-schedule/absence-kinds -- a
// lead adding a leave or allocation tag the catalogue does not have yet.
func (h *ScheduleHandler) CreateScheduleAbsenceKind(w http.ResponseWriter, r *http.Request) {
	body, userID, ok := readScheduleBody(w, r)
	if !ok {
		return
	}

	result, err := h.entity.CreateScheduleAbsenceKind(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateScheduleAbsenceKind failed", "userID", userID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to add the tag.")
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// DeleteScheduleAbsenceKind handles DELETE /team-schedule/absence-kinds/{code}
// -- a lead deleting a tag a lead added, once nothing uses it.
func (h *ScheduleHandler) DeleteScheduleAbsenceKind(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	if _, err := h.entity.DeleteScheduleAbsenceKind(r.Context(), r.PathValue("code")); err != nil {
		slog.ErrorContext(r.Context(), "entity DeleteScheduleAbsenceKind failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to delete the tag.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetScheduleEditMarkers handles GET /team-schedule/edit-markers.
func (h *ScheduleHandler) GetScheduleEditMarkers(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	q := r.URL.Query()
	result, err := h.entity.GetScheduleEditMarkers(r.Context(), q.Get("from"), q.Get("to"))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetScheduleEditMarkers failed", "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to load who changed the rota.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
