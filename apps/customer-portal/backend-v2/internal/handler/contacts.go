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
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/usermanagement"
)

// entityWriteTimeout bounds an entity-service membership write that runs on a
// context detached from the request (see entityWriteContext). It sits above
// the entity client's own 25-second timeout, so in practice that timeout
// fires first; this one only guarantees the detached call cannot run forever.
const entityWriteTimeout = 30 * time.Second

// entityWriteContext returns the context an entity-service membership write
// runs on: the request's values (user token, correlation id) without its
// cancellation, and with its own deadline. A write updates Salesforce and
// then commits Postgres, so abandoning it halfway because the admin closed
// the tab or the connection dropped would leave Salesforce written and the
// database not. Letting it finish is the safer outcome in every case.
func entityWriteContext(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(r.Context()), entityWriteTimeout)
}

// invitationProcessingResponse is the 202 body CreateProjectContact returns
// when the portal stopped waiting before entity-service answered.
type invitationProcessingResponse struct {
	Message string `json:"message"`
	Status  string `json:"status"`
}

// invitationStatusProcessing is invitationProcessingResponse.Status: the
// webapp keeps the row pending and refreshes the list until it appears.
const invitationStatusProcessing = "PROCESSING"

// membershipStateDeactivated is the membership state that no longer grants
// anything, compared case-insensitively against the contact list's status.
const membershipStateDeactivated = "DEACTIVATED"

// entityProjectResolver is the subset of the entity client ContactHandler
// needs — just enough to resolve a project's Salesforce ID for the
// downstream project-contact onboarding service, which is keyed on it.
type entityProjectResolver interface {
	GetProject(ctx context.Context, id string) (entity.ProjectDetailsView, error)
}

// membershipsClient is the entity-service side of the same operations. It
// replaces contactsClient after cutover: entity-service updates Postgres and
// Salesforce in one transaction, so the portal no longer needs a second
// service that writes Salesforce on its own. Keyed on the project UUID, not
// the project's Salesforce Id.
type membershipsClient interface {
	CreateProjectMembership(ctx context.Context, projectID string, req entity.CreateProjectMembershipRequest) (entity.ProjectMembership, error)
	UpdateProjectMembershipRoles(ctx context.Context, projectID, email string, req entity.UpdateProjectMembershipRolesRequest) (entity.ProjectMembership, error)
	DeactivateProjectMembership(ctx context.Context, projectID, email string) error
	ResendProjectMembershipInvitation(ctx context.Context, projectID, email string) error
	ListProjectContacts(ctx context.Context, projectID string) ([]entity.ProjectContact, error)
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
	memberships membershipsClient
	// portalContactsEnabled is CSM_MIGRATION_PORTAL_CONTACTS_ENABLED. On, the
	// contact list, the admin check and every write below go to
	// entity-service and the CSM database; off, they go to the pre-cutover
	// onboarding service exactly as before. Keeping both paths is what makes
	// the cutover reversible without a redeploy of anything but this flag.
	portalContactsEnabled bool
}

// NewContactHandler creates a ContactHandler backed by the given entity and
// project-contact onboarding service clients. memberships may be nil when
// portalContacts is false, which is the pre-cutover shape.
func NewContactHandler(entityClient entityProjectResolver, contactsClient contactsClient, memberships membershipsClient, portalContacts bool) *ContactHandler {
	return &ContactHandler{
		entity:                entityClient,
		contacts:              contactsClient,
		memberships:           memberships,
		portalContactsEnabled: portalContacts && memberships != nil,
	}
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

	if h.portalContactsEnabled {
		contacts, err := h.memberships.ListProjectContacts(r.Context(), projectID)
		if err != nil {
			slog.ErrorContext(r.Context(), "entity ListProjectContacts failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
			mapUpstreamError(w, err, "Failed to retrieve project contacts.")
			return
		}
		writeJSONValue(w, http.StatusOK, dto.MapEntityProjectContacts(contacts))
		return
	}

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

	if h.portalContactsEnabled {
		if !h.requireProjectAdmin(w, r, user, projectID) {
			return
		}
		wctx, cancel := entityWriteContext(r)
		defer cancel()
		membership, err := h.memberships.CreateProjectMembership(wctx, projectID, dto.BuildCreateProjectMembershipRequest(req))
		if errors.Is(err, context.DeadlineExceeded) {
			// The portal stopped waiting, not entity-service: it keeps going
			// and commits, so reporting a failure here would tell the admin
			// an invitation failed when it is about to exist. A retry would
			// then be refused as a duplicate. 202 tells the webapp to keep
			// the row pending and refresh the list until it shows up.
			slog.WarnContext(r.Context(), "entity CreateProjectMembership still running when the portal stopped waiting", "userID", user.UserID, "projectID", projectID)
			writeJSONValue(w, http.StatusAccepted, invitationProcessingResponse{
				Message: "The invitation is still being processed.",
				Status:  invitationStatusProcessing,
			})
			return
		}
		if err != nil {
			slog.ErrorContext(r.Context(), "entity CreateProjectMembership failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
			writeUpstreamMessage(w, err, "Failed to add project contact.")
			return
		}
		writeJSONValue(w, http.StatusOK, dto.MapEntityMembership(membership))
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

	if h.portalContactsEnabled {
		if !h.requireProjectAdmin(w, r, user, projectID) {
			return
		}
		wctx, cancel := entityWriteContext(r)
		defer cancel()
		if err := h.memberships.DeactivateProjectMembership(wctx, projectID, email); err != nil {
			slog.ErrorContext(r.Context(), "entity DeactivateProjectMembership failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
			writeUpstreamMessage(w, err, "Failed to remove project contact.")
			return
		}
		writeJSONValue(w, http.StatusOK, map[string]string{"message": "Project contact removed successfully!"})
		return
	}

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

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}
	var req dto.MembershipRoleUpdateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	if h.portalContactsEnabled {
		if !h.requireProjectAdmin(w, r, user, projectID) {
			return
		}
		wctx, cancel := entityWriteContext(r)
		defer cancel()
		membership, err := h.memberships.UpdateProjectMembershipRoles(wctx, projectID, email,
			entity.UpdateProjectMembershipRolesRequest{Roles: dto.RolesFromRoleUpdateRequest(req)})
		if err != nil {
			slog.ErrorContext(r.Context(), "entity UpdateProjectMembershipRoles failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
			writeUpstreamMessage(w, err, "Failed to update project contact.")
			return
		}
		writeJSONValue(w, http.StatusOK, dto.MapEntityMembership(membership))
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

// ResendProjectContactInvitation handles
// POST /projects/{id}/contacts/{email}/resend-invitation.
//
// There is no pre-cutover equivalent, so unlike the three writes above this
// has no fallback branch: with CSM_MIGRATION_PORTAL_CONTACTS_ENABLED off it
// answers 404, the same answer entity-service itself would give, rather than
// pretending to have resent something.
//
// entity-service republishes the invitation with a resend marker, which makes
// csm-notification-service bypass its duplicate-invitation guard and send the
// reminder wording. That guard exists to stop an accidental second
// invitation, and a requested resend is by definition not that.
func (h *ContactHandler) ResendProjectContactInvitation(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	if !h.portalContactsEnabled {
		writeError(w, http.StatusNotFound, "Resending invitations is not available.")
		return
	}

	projectID := r.PathValue("id")
	email := r.PathValue("email")
	if projectID == "" || !uuidRe.MatchString(projectID) || email == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	if !h.requireProjectAdmin(w, r, user, projectID) {
		return
	}

	wctx, cancel := entityWriteContext(r)
	defer cancel()
	if err := h.memberships.ResendProjectMembershipInvitation(wctx, projectID, email); err != nil {
		slog.ErrorContext(r.Context(), "entity ResendProjectMembershipInvitation failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		writeUpstreamMessage(w, err, "Failed to resend the invitation.")
		return
	}

	writeJSONValue(w, http.StatusOK, map[string]string{"message": "Invitation resent successfully!"})
}

// requireProjectAdmin is the authorization every entity-service write needs
// before it is sent. It writes the error response itself and reports whether
// the caller may go ahead.
//
// entity-service deliberately does not make this decision: it only checks
// that the caller is an allow-listed internal client, so whether this
// particular user may change this particular project's contacts is the
// portal's call. The pre-cutover path got it from the onboarding service,
// which checked the AdminEmail it was sent. Here the caller must hold the
// ADMIN project role on an active membership of the project, read from the
// same database contact list the settings page shows. The list is fetched
// by project UUID, so the answer does not depend on how entity-service
// scopes the portal's other reads.
func (h *ContactHandler) requireProjectAdmin(w http.ResponseWriter, r *http.Request, user *middleware.UserInfo, projectID string) bool {
	contacts, err := h.memberships.ListProjectContacts(r.Context(), projectID)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ListProjectContacts failed", "userID", user.UserID, "projectID", projectID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve project contacts.")
		return false
	}

	if !isActiveProjectAdmin(contacts, user.Email) {
		slog.WarnContext(r.Context(), "project contact write refused: caller is not an admin of the project", "userID", user.UserID, "projectID", projectID)
		writeError(w, http.StatusForbidden, ErrMsgForbidden)
		return false
	}
	return true
}

// isActiveProjectAdmin reports whether email holds the ADMIN project role on
// a membership that has not been deactivated. Email comparison ignores case,
// since Salesforce does not preserve the casing the address was typed in.
func isActiveProjectAdmin(contacts []entity.ProjectContact, email string) bool {
	email = strings.TrimSpace(email)
	if email == "" {
		return false
	}
	for _, c := range contacts {
		if !strings.EqualFold(strings.TrimSpace(c.Email), email) || !dto.IsProjectAdmin(c) {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(c.RegistrationState), membershipStateDeactivated) {
			continue
		}
		return true
	}
	return false
}
