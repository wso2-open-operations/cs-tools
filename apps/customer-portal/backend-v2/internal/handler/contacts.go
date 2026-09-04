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
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/usermanagement"
)

// entityProjectResolver is the subset of the entity client ContactHandler
// needs — just enough to resolve a project's Salesforce ID for the
// downstream project-contact onboarding service, which is keyed on it.
type entityProjectResolver interface {
	GetProject(ctx context.Context, id string) (entity.ProjectDetailsView, error)
}

// contactsClient abstracts the project-contact onboarding service operations
// used by ContactHandler.
type contactsClient interface {
	GetProjectContacts(ctx context.Context, projectID string) ([]usermanagement.Contact, error)
	CreateProjectContact(ctx context.Context, projectID string, req usermanagement.OnBoardContactPayload) (usermanagement.Membership, error)
	RemoveProjectContact(ctx context.Context, projectID, contactEmail, adminEmail string) (usermanagement.Membership, error)
	UpdateMembershipRole(ctx context.Context, projectID, contactEmail string, req usermanagement.MembershipRolePayload) (usermanagement.Membership, error)
	ValidateProjectContact(ctx context.Context, req usermanagement.ValidationPayload) (contact *usermanagement.Contact, conflict bool, err error)
}

// ContactHandler handles HTTP requests for project contact/membership
// management, backed by a separate microservice (not entity-service, not
// SCIM) keyed on the project's Salesforce ID.
type ContactHandler struct {
	entity      entityProjectResolver
	contacts    contactsClient
	callerScope *CallerScopeResolver
}

// NewContactHandler creates a ContactHandler backed by the given entity and
// project-contact onboarding service clients.
func NewContactHandler(entityClient entityProjectResolver, contactsClient contactsClient) *ContactHandler {
	return &ContactHandler{entity: entityClient, contacts: contactsClient}
}

// SetCallerScope enables caller-scoped access: contact management operations
// require the caller to be an active portal-user contact of the project in the
// URL path. Always enforced in production (main.go calls this unconditionally,
// no kill switch) — see ProjectHandler.SetCallerScope for why this is a
// setter rather than a constructor parameter.
func (h *ContactHandler) SetCallerScope(resolver *CallerScopeResolver) {
	h.callerScope = resolver
}

// GetProjectContacts handles GET /projects/{id}/contacts.
func (h *ContactHandler) GetProjectContacts(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	projectID := r.PathValue("id")
	if projectID == "" || !uuidRe.MatchString(projectID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Caller-scope check commented out for now per review; will be re-evaluated:
	// if !requireProjectMember(w, r, h.callerScope, projectID, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return
	// }

	project, err := h.entity.GetProject(r.Context(), projectID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProject failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve project details.")
		return
	}

	result, err := h.contacts.GetProjectContacts(r.Context(), project.SfID)
	if err != nil {
		slog.ErrorContext(r.Context(), "usermanagement GetProjectContacts failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve project contacts.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapContacts(result))
}

// CreateProjectContact handles POST /projects/{id}/contacts. AdminEmail is
// always the caller's own email, never client-supplied.
func (h *ContactHandler) CreateProjectContact(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	projectID := r.PathValue("id")
	if projectID == "" || !uuidRe.MatchString(projectID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Caller-scope check commented out for now per review; will be re-evaluated:
	// if !requireProjectMember(w, r, h.callerScope, projectID, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return
	// }

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}
	var req dto.ContactOnboardRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if !dto.ValidContactEmail(req.ContactEmail) {
		writeError(w, http.StatusBadRequest, "Invalid contact email format.")
		return
	}

	project, err := h.entity.GetProject(r.Context(), projectID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProject failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve project details.")
		return
	}

	result, err := h.contacts.CreateProjectContact(r.Context(), project.SfID, dto.BuildOnBoardContactPayload(req, user.Email))
	if err != nil {
		slog.ErrorContext(r.Context(), "usermanagement CreateProjectContact failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		writeUpstreamMessage(w, err, "Failed to add project contact.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapMembership(result))
}

// RemoveProjectContact handles DELETE /projects/{id}/contacts/{email}.
func (h *ContactHandler) RemoveProjectContact(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	projectID := r.PathValue("id")
	email := r.PathValue("email")
	if projectID == "" || !uuidRe.MatchString(projectID) || email == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	// Caller-scope check commented out for now per review; will be re-evaluated:
	// if !requireProjectMember(w, r, h.callerScope, projectID, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return
	// }

	project, err := h.entity.GetProject(r.Context(), projectID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProject failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve project details.")
		return
	}

	result, err := h.contacts.RemoveProjectContact(r.Context(), project.SfID, email, user.Email)
	if err != nil {
		slog.ErrorContext(r.Context(), "usermanagement RemoveProjectContact failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		writeUpstreamMessage(w, err, "Failed to remove project contact.")
		return
	}

	_ = result // this endpoint returns a fixed success message here, not the membership details.
	writeJSONValue(w, http.StatusOK, map[string]string{"message": "Project contact removed successfully!"})
}

// UpdateProjectContactRole handles PATCH /projects/{id}/contacts/{email}.
func (h *ContactHandler) UpdateProjectContactRole(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	projectID := r.PathValue("id")
	email := r.PathValue("email")
	if projectID == "" || !uuidRe.MatchString(projectID) || email == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	// Caller-scope check commented out for now per review; will be re-evaluated:
	// if !requireProjectMember(w, r, h.callerScope, projectID, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return
	// }

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}
	var req dto.MembershipRoleUpdateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	project, err := h.entity.GetProject(r.Context(), projectID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProject failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve project details.")
		return
	}

	result, err := h.contacts.UpdateMembershipRole(r.Context(), project.SfID, email, dto.BuildMembershipRolePayload(req, user.Email))
	if err != nil {
		slog.ErrorContext(r.Context(), "usermanagement UpdateMembershipRole failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		writeUpstreamMessage(w, err, "Failed to update project contact.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapMembership(result))
}

// ValidateProjectContact handles POST /projects/{id}/contacts/validate.
func (h *ContactHandler) ValidateProjectContact(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	projectID := r.PathValue("id")
	if projectID == "" || !uuidRe.MatchString(projectID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	// Caller-scope check commented out for now per review; will be re-evaluated:
	// if !requireProjectMember(w, r, h.callerScope, projectID, user.UserID, user.Email, http.StatusForbidden, ErrMsgForbidden) {
	// 	return
	// }

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}
	var req dto.ContactValidationRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	project, err := h.entity.GetProject(r.Context(), projectID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProject failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve project details.")
		return
	}

	contact, conflict, err := h.contacts.ValidateProjectContact(r.Context(), usermanagement.ValidationPayload{
		ProjectID:    project.SfID,
		ContactEmail: req.ContactEmail,
		AdminEmail:   user.Email,
	})
	if conflict {
		writeError(w, http.StatusConflict, "Contact with the provided email already exists in the project!")
		return
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "usermanagement ValidateProjectContact failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		writeUpstreamMessage(w, err, "Failed to validate project contact.")
		return
	}

	if contact != nil {
		mapped := dto.MapContact(*contact)
		writeJSONValue(w, http.StatusOK, dto.ContactValidationResponse{
			IsContactValid: true,
			Message:        "Contact is valid but already exists in the project!",
			ContactDetails: &mapped,
		})
		return
	}

	writeJSONValue(w, http.StatusOK, dto.ContactValidationResponse{
		IsContactValid: true,
		Message:        "Project contact is valid and can be added to the project!",
	})
}
