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
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
)

// salesEntityClient abstracts the sales-side entity GraphQL operations used
// by SplUserScanHandler.
type salesEntityClient interface {
	GetContactByEmail(ctx context.Context, email string) (*entity.Contact, error)
	GetSubscriptionByKey(ctx context.Context, subscriptionKey string) (*entity.Subscription, error)
}

// entityScanClient is the subset of internal/entity.CustomerEntityClient
// SplUserScanHandler's backing-system-side checks need. Replaces the old CS-side
// entity GraphQL service (internal/entity/cs.go, removed): that GraphQL
// service turned out to be the backing system itself behind a second, parallel
// integration, not an independent data source, so there was nothing to gain
// from keeping it once this handler could resolve the same data through the
// entity service every other CS Portal handler already uses.
type entityScanClient interface {
	SearchUsers(ctx context.Context, body []byte) ([]byte, error)
	SearchProjects(ctx context.Context, body []byte) ([]byte, error)
	ResendProjectContactInvitation(ctx context.Context, projectID, email string) ([]byte, error)
}

type entitySearchUsersFilters struct {
	Emails []string `json:"emails"`
}

type entitySearchUsersRequest struct {
	Pagination entityPagination         `json:"pagination"`
	Filters    entitySearchUsersFilters `json:"filters"`
}

// entityScanUserView is the subset of entity-service's user-search result
// this handler needs. LockedOut is populated only when entity-service's own
// CUSTOMER_ENTITY_DATA_SOURCE is "servicenow" (its own first-party
// The backing system integration, domain.SNUser) -- the "postgres" data source
// (domain.User) has no such column, so LockedOut silently reads false
// there. "servicenow" is this file's documented default (see
// CUSTOMER_ENTITY_DATA_SOURCE's own .env.example comment).
type entityScanUserView struct {
	Email     string `json:"email"`
	LockedOut bool   `json:"lockedOut"`
}

type entitySearchUsersResponse struct {
	Users []entityScanUserView `json:"users"`
}

// The membership-type values a sales-side Contact's Memberships[i].Type may
// carry — mirrors Ballerina modules/types.bal's MembershipType enum
// (CUSTOMER = "OWN CONTACT", PARTNER = "PARTNER CONTACT").
const (
	membershipTypeCustomer = "OWN CONTACT"
	membershipTypePartner  = "PARTNER CONTACT"
)

// accountClassificationPartner mirrors Ballerina constants:PARTNER, an
// Account.Classification value.
const accountClassificationPartner = "Partner"

// projectStateOpen mirrors Ballerina modules/types.bal's ProjectState
// enum's OPEN value, compared against Project.WSO2ClosureState.
const projectStateOpen = "Open"

// scanSystem* mirror Ballerina modules/types.bal's System enum.
const (
	scanSystemSalesforce = "Salesforce"
	scanSystemServicenow = "Servicenow"
)

// UserScanRequest is the request body for POST /scan-user — mirrors
// Ballerina modules/types.bal's UserScanPayload.
type UserScanRequest struct {
	Email           string `json:"email"`
	SubscriptionKey string `json:"subscriptionKey"`
	IsPartner       bool   `json:"isPartner"`
	// ResendInvitation opts in to re-sending a locked-out contact's project
	// invitation. Without it the scan is read-only and only reports the
	// locked-out state, so repeating a diagnostic does not e-mail the contact
	// each time.
	ResendInvitation bool `json:"resendInvitation,omitempty"`
}

// SplScanInformation is additional detail attached to a ScanResult —
// mirrors Ballerina modules/types.bal's Information. Fields are omitted from
// the JSON response when unset, matching Ballerina's optional (?) fields.
type SplScanInformation struct {
	Issue         string `json:"issue,omitempty"`
	Solution      string `json:"solution,omitempty"`
	Documentation string `json:"documentation,omitempty"`
}

// ScanResult is one row of a scan-user system's results — mirrors
// Ballerina modules/types.bal's ScanResult.
type ScanResult struct {
	Order       int                `json:"order"`
	Label       string             `json:"label"`
	State       bool               `json:"state"`
	Information SplScanInformation `json:"information"`
}

// ScanResponse groups one system's ScanResult rows — mirrors Ballerina
// modules/types.bal's Response (renamed to avoid colliding with this
// package's own response.go helpers).
type ScanResponse struct {
	System       string       `json:"system"`
	SystemResult []ScanResult `json:"systemResult"`
}

// scanErrorBody mirrors Ballerina types:AppServerErrorResponse's body
// shape, used only for the upstream-lookup-failure branches of scan-user
// (distinct from every other /spl/* endpoint's ErrMsg* fallback, since this
// endpoint's Ballerina source returns its own bespoke message per lookup
// rather than a generic one).
type scanErrorBody struct {
	Message string `json:"message"`
}

func writeScanError(w http.ResponseWriter, message string) {
	writeJSONValue(w, http.StatusInternalServerError, scanErrorBody{Message: message})
}

// Static Information values ported verbatim from
// modules/constants/constants.bal — copy text, including the linked
// documentation URLs, must not be paraphrased.
var (
	infoContactNotFound = SplScanInformation{
		Issue:    "The contact not found in Salesforce.",
		Solution: "We need to add the contact in Salesforce.",
		Documentation: "https://docs.google.com/document/d/1OyVjCjgcULEsq7idFeWAh6IFOVfEx0DrugvSMULd94Q/edit#heading=h." +
			"fw9vc1atxoc5",
	}
	infoSubscriptionNotFound = SplScanInformation{
		Issue:    "The subscription not found in the salesforce.",
		Solution: "We need to add the subscription in salesforce.",
		Documentation: "https://docs.google.com/document/d/1OyVjCjgcULEsq7idFeWAh6IFOVfEx0DrugvSMULd94Q/edit#heading=h." +
			"b9o03r3icco2",
	}
	infoMembershipNotFound = SplScanInformation{
		Issue:    "The contact is not associated with any subscription.",
		Solution: "We need to connect the project contact with the subscription.",
		Documentation: "https://docs.google.com/document/d/1OyVjCjgcULEsq7idFeWAh6IFOVfEx0DrugvSMULd94Q/edit#heading=h." +
			"rwkq1c6yzv6o",
	}
	infoMembershipNotFoundInSubscription = SplScanInformation{
		Issue:    "The contact is not associated with provided subscription.",
		Solution: "We need to check and bind the provided subscription with the contact.",
		Documentation: "https://docs.google.com/document/d/1OyVjCjgcULEsq7idFeWAh6IFOVfEx0DrugvSMULd94Q/edit#heading=h." +
			"rwkq1c6yzv6o",
	}
	infoInvalidMembership = SplScanInformation{
		Issue: "Partner is not added in the customer subscription.",
		Solution: "Partners should be in the partner account. The partner account should have a partner relationship with " +
			"the customer account. After that, partners should be added as partner contacts in the customer subscription.",
		Documentation: "https://docs.google.com/document/d/1OyVjCjgcULEsq7idFeWAh6IFOVfEx0DrugvSMULd94Q/edit#heading=h." +
			"rwkq1c6yzv6o",
	}
	infoInvalidCustomerMembership = SplScanInformation{
		Issue:    "Customer is added in the partner account.",
		Solution: "Customer should never be in the partner account. Customer should be added as a customer contact in the customer project.",
		Documentation: "https://docs.google.com/document/d/1OyVjCjgcULEsq7idFeWAh6IFOVfEx0DrugvSMULd94Q/edit#heading=h." +
			"lm8ke89ptede",
	}
	infoSubscriptionNotFoundInAccount = SplScanInformation{
		Issue:    "The contact is not added in the correct account.",
		Solution: "The contact and the subscription are in two different accounts. It's important to verify that the project is associated with the correct account.",
		Documentation: "https://docs.google.com/document/d/1OyVjCjgcULEsq7idFeWAh6IFOVfEx0DrugvSMULd94Q/edit#heading=h." +
			"b9o03r3icco2",
	}
	infoUserNotFound = SplScanInformation{
		Issue: "The user is not in the Servicenow.",
		Solution: "First check the relevant contact is there in the Salesforce or not. If not exists add the contact to " +
			"the Salesforce. Otherwise talk to Digi-Ops team.",
		Documentation: "https://docs.google.com/document/d/1OyVjCjgcULEsq7idFeWAh6IFOVfEx0DrugvSMULd94Q/edit#heading=h.rwkq1c6yzv6o",
	}
	infoUserLockedOutDocumentation = "https://docs.google.com/document/d/1OyVjCjgcULEsq7idFeWAh6IFOVfEx0DrugvSMULd94Q/edit#heading=h.rwkq1c6yzv6o"
	infoProjectNotFound            = SplScanInformation{
		Issue: "The project is not in the Servicenow.",
		Solution: "First check the relevant subscription is there in the Salesforce or not. If not exist add the " +
			"subscription the Salesforce. Otherwise talk to Digi-Ops team.",
		Documentation: "https://docs.google.com/document/d/1OyVjCjgcULEsq7idFeWAh6IFOVfEx0DrugvSMULd94Q/edit#heading=h.b9o03r3icco2",
	}
	infoUserNotFoundInProject = SplScanInformation{
		Issue:    "The project is not in the Servicenow. Due to that, Project contact is not in the Servicenow.",
		Solution: "First add the project and after that add the project contact.",
		Documentation: "https://docs.google.com/document/d/1OyVjCjgcULEsq7idFeWAh6IFOVfEx0DrugvSMULd94Q/edit#heading=h." +
			"rwkq1c6yzv6o",
	}
)

// SplUserScanHandler handles HTTP requests for the user-scan diagnostic
// tool, cross-referencing the sales-side (Salesforce) entity service and
// the entity service every other CS Portal handler already uses.
type SplUserScanHandler struct {
	sales       salesEntityClient
	entity      entityScanClient
	accessGuard *AccessGuard
}

// NewSplUserScanHandler creates a SplUserScanHandler backed by the given
// sales-side and entity clients. accessGuard enforces PermViewerAccess,
// SupportPortalLite's blanket audience gate.
func NewSplUserScanHandler(sales salesEntityClient, entityClient entityScanClient, accessGuard *AccessGuard) *SplUserScanHandler {
	return &SplUserScanHandler{sales: sales, entity: entityClient, accessGuard: accessGuard}
}

// ScanUser handles POST /scan-user — ported verbatim (business logic,
// copy text and documentation links included) from Ballerina service.bal's
// `post scan\-user` resource function. See that function for the
// authoritative behavior; comments below reference its structure.
func (h *SplUserScanHandler) ScanUser(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerWriteAccess(w, r, h.accessGuard)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return
	}

	var payload UserScanRequest
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	payload.Email = strings.TrimSpace(payload.Email)
	payload.SubscriptionKey = strings.TrimSpace(payload.SubscriptionKey)
	if payload.Email == "" || payload.SubscriptionKey == "" {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	ctx := r.Context()

	// ----- Salesforce-side: contact / subscription / membership -----

	contactResult := ScanResult{Order: 1, Label: "Contact Details"}
	subscriptionResult := ScanResult{Order: 2, Label: "Subscription Contact Details"}
	membershipResult := ScanResult{Order: 3, Label: "Membership Details"}

	contact, err := h.sales.GetContactByEmail(ctx, payload.Email)
	if err != nil {
		slog.ErrorContext(ctx, "sales entity GetContactByEmail failed", "userID", user.UserID, "err", summarizeErr(err))
		writeScanError(w, "Error occurred when retrieving contact information")
		return
	}

	subscription, err := h.sales.GetSubscriptionByKey(ctx, payload.SubscriptionKey)
	if err != nil {
		slog.ErrorContext(ctx, "sales entity GetSubscriptionByKey failed", "userID", user.UserID, "err", summarizeErr(err))
		writeScanError(w, "Error occurred when retrieving subscription information")
		return
	}

	if contact == nil {
		contactResult.Information = infoContactNotFound
		membershipResult.Information = infoMembershipNotFound
		if subscription == nil {
			subscriptionResult.Information = infoSubscriptionNotFound
		} else {
			subscriptionResult.State = true
		}
	} else {
		contactResult.State = true

		if len(contact.Memberships) == 0 {
			membershipResult.Information = infoMembershipNotFound
		}
		if subscription == nil {
			subscriptionResult.Information = infoSubscriptionNotFound
			membershipResult.Information = infoMembershipNotFound
		} else {
			subscriptionResult.State = true
			subscriptionID := subscription.ID

			var subscriptionMembership []entity.ContactMembership
			for _, m := range contact.Memberships {
				if m.SubscriptionID == subscriptionID {
					subscriptionMembership = append(subscriptionMembership, m)
				}
			}

			if len(subscriptionMembership) == 0 {
				membershipResult.Information = infoMembershipNotFoundInSubscription
			}
			contactAccountID := contact.Account.ID

			if contactAccountID != subscription.CustomerID {
				if !payload.IsPartner {
					subscriptionResult.Information = infoSubscriptionNotFoundInAccount
				}
			}

			if subscription.ID != "" && len(subscriptionMembership) > 0 {
				if contact.Account.Classification == accountClassificationPartner {
					if (!payload.IsPartner && subscriptionMembership[0].Type == membershipTypeCustomer) ||
						(payload.IsPartner && subscriptionMembership[0].Type == membershipTypePartner) {
						membershipResult.State = true
					} else {
						membershipResult.Information = infoInvalidMembership
					}
				} else {
					if !payload.IsPartner && subscriptionMembership[0].Type == membershipTypeCustomer {
						membershipResult.State = true
					} else {
						membershipResult.Information = infoInvalidCustomerMembership
					}
				}
			}
		}
	}

	salesResponse := []ScanResult{contactResult, subscriptionResult, membershipResult}

	// ----- entity-service side: user lock state / project closure state -----

	userStateResult := ScanResult{Order: 1, Label: "Accept the invitation"}
	projectStateResult := ScanResult{Order: 2, Label: "Project closure state"}

	entityUser, err := h.lookupScanUser(ctx, payload.Email)
	if err != nil {
		slog.ErrorContext(ctx, "entity SearchUsers failed", "userID", user.UserID, "err", summarizeErr(err))
		writeScanError(w, "Error occurred when retrieving user information")
		return
	}

	project, err := h.lookupProjectByKey(ctx, payload.SubscriptionKey)
	if err != nil {
		slog.ErrorContext(ctx, "entity SearchProjects failed", "userID", user.UserID, "err", summarizeErr(err))
		writeScanError(w, "Error occurred when retrieving project information")
		return
	}

	var projectID string
	if project == nil {
		projectStateResult.Information = infoProjectNotFound
		userStateResult.State = false
		userStateResult.Information = infoUserNotFoundInProject
	} else {
		projectID = project.ID
		closureState := derefStr(project.ClosureState)
		if closureState != projectStateOpen {
			projectStateResult.Information = SplScanInformation{
				Issue: "The project is not in open state. The project is in " + closureState +
					" state. The project should be in the Open state.",
				Solution: "Need to reopen this project for getting the uninterpreted support.",
			}
		} else {
			projectStateResult.State = true
		}

		if entityUser == nil {
			userStateResult.Information = infoUserNotFound
		} else if entityUser.LockedOut {
			// Resending is a side effect (an e-mail to the contact), so it
			// only happens when the caller asks for it with
			// resendInvitation; a plain scan only reports the state. There
			// is no existing invitation link to show instead (see
			// entityScanClient's own doc comment).
			info := SplScanInformation{
				Issue:         "The user didn't accept the invitation.",
				Documentation: infoUserLockedOutDocumentation,
			}
			if !payload.ResendInvitation {
				info.Solution = "Resend the invitation from the project's Contacts tab, or run the scan again with the invitation resend option."
			} else if _, err := h.entity.ResendProjectContactInvitation(ctx, projectID, payload.Email); err != nil {
				slog.WarnContext(ctx, "entity ResendProjectContactInvitation failed", "userID", user.UserID, "err", summarizeErr(err))
				info.Solution = "Could not resend the invitation automatically. Resend it manually from the project's Contacts tab."
			} else {
				info.Solution = "A fresh invitation email has been sent. Ask the user to check their inbox and accept it."
			}
			userStateResult.Information = info
		} else {
			userStateResult.State = true
		}
	}

	csResponse := []ScanResult{userStateResult, projectStateResult}

	writeJSONValue(w, http.StatusOK, []ScanResponse{
		{System: scanSystemSalesforce, SystemResult: salesResponse},
		{System: scanSystemServicenow, SystemResult: csResponse},
	})
}

// lookupScanUser resolves email to entity-service's user record via an
// exact-email search filter. Returns (nil, nil) when no user matches.
func (h *SplUserScanHandler) lookupScanUser(ctx context.Context, email string) (*entityScanUserView, error) {
	body, err := json.Marshal(entitySearchUsersRequest{
		Pagination: entityPagination{Limit: 1},
		Filters:    entitySearchUsersFilters{Emails: []string{email}},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal entity-service user search request: %w", err)
	}
	raw, err := h.entity.SearchUsers(ctx, body)
	if err != nil {
		return nil, err
	}
	var resp entitySearchUsersResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal entity-service user search response: %w", err)
	}
	if len(resp.Users) == 0 {
		return nil, nil
	}
	return &resp.Users[0], nil
}

// lookupProjectByKey resolves projectKey to entity-service's project record.
// SearchProjects' own searchQuery match is fuzzy (ILIKE against name/key/
// subscription_type), so this filters the results for an exact key match
// itself -- the same search-then-exact-match pattern used elsewhere in this
// package for a caller-facing key/number rather than entity-service's
// internal UUID (see e.g. reports_postgres.go's searchAllCases). Returns
// (nil, nil) when no project's key matches exactly.
func (h *SplUserScanHandler) lookupProjectByKey(ctx context.Context, projectKey string) (*entityProjectView, error) {
	body, err := json.Marshal(entitySearchProjectsRequest{
		Pagination:  entityPagination{Limit: 50},
		SearchQuery: projectKey,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal entity-service project search request: %w", err)
	}
	raw, err := h.entity.SearchProjects(ctx, body)
	if err != nil {
		return nil, err
	}
	var resp entitySearchProjectsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal entity-service project search response: %w", err)
	}
	for i := range resp.Projects {
		if resp.Projects[i].Key == projectKey {
			return &resp.Projects[i], nil
		}
	}
	return nil, nil
}
