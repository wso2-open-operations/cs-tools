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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package salesentity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// The write half of the REST sales/sales-entity-service contract, used by the
// portal-driven membership writes (see service.ProjectMembershipWriteService).
//
// NONE of these endpoints is idempotent — sales-entity-service creates a new
// Salesforce record on every POST. Every caller must therefore search first
// and create only when the search came back empty. That rule is also what
// makes the "Salesforce succeeded, the commit did not" residue self-healing:
// the next attempt adopts the orphaned Salesforce record instead of making a
// second one.
const (
	contactsPath        = "/contacts"
	projectContactsPath = "/project-contacts"
)

// contactEmailSearchRequest is the POST /contacts/search body that keys on an
// address rather than an Id (idSearchRequest's sibling).
type contactEmailSearchRequest struct {
	Email string `json:"email"`
	Limit int    `json:"limit"`
}

// CreateContactInput is the POST /contacts body: the minimum Salesforce needs
// to create a Contact under an Account.
type CreateContactInput struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	// AccountID is the Salesforce Account Id the contact is created under.
	AccountID string `json:"accountId"`
	// IsCsIntegrationUser sets Is_Cs_Integration_User__c. A machine account
	// gets no Asgardeo identity and no invitation e-mail downstream, and
	// this is the only moment the caller can say so -- the flag is read back
	// off the contact everywhere after this.
	IsCsIntegrationUser bool `json:"isCsIntegrationUser,omitempty"`
}

// CreateProjectContactInput is the POST /project-contacts body. ProjectID is
// the Salesforce Subscription (project) Id, ContactID the Salesforce Contact
// Id, State a project_contact_state value ("INVITED", ...) and Role the raw
// Salesforce Role__c multi-picklist labels ("Portal user", "Admin", ...).
type CreateProjectContactInput struct {
	ProjectID string   `json:"projectId"`
	ContactID string   `json:"contactId"`
	State     string   `json:"state"`
	Role      []string `json:"role"`
}

// updateProjectContactRequest is the PATCH /project-contacts/{id} body. Both
// fields are optional; omitting one leaves that Salesforce field untouched.
type updateProjectContactRequest struct {
	State *string   `json:"state,omitempty"`
	Role  *[]string `json:"role,omitempty"`
}

// updateContactRequest is the PATCH /contacts/{id} body.
type updateContactRequest struct {
	LockoutStatus bool `json:"lockoutStatus"`
}

// SearchContactByEmail looks a Salesforce Contact up by address via
// POST /contacts/search {email}. found is false (with no error) when the
// search came back empty — unlike GetContact's by-Id lookup, "no such
// contact" is an ordinary, expected answer here: it is exactly what tells the
// caller to create one.
func (c *Client) SearchContactByEmail(ctx context.Context, email string) (Contact, bool, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return Contact{}, false, &apierror.ValidationError{Msg: "email is required to search for a Salesforce contact"}
	}
	var rows []Contact
	if err := c.searchWithRetry(ctx, contactSearchPath, contactEmailSearchRequest{Email: email, Limit: 1}, "contact", &rows); err != nil {
		return Contact{}, false, err
	}
	for _, ct := range rows {
		if ct.Email != nil && strings.EqualFold(strings.TrimSpace(*ct.Email), email) {
			return ct, true, nil
		}
	}
	// A row that does not actually carry the address we asked for is not a
	// match we may adopt — creating a duplicate is recoverable, writing a
	// membership for the wrong person is not.
	return Contact{}, false, nil
}

// SearchProjectContact looks a membership up by its (project, contact) pair
// via POST /project-contacts/search. found is false (no error) when there is
// none, the same "absence is an answer" contract as SearchContactByEmail.
func (c *Client) SearchProjectContact(ctx context.Context, projectSfID, contactSfID string) (ProjectContact, bool, error) {
	if strings.TrimSpace(projectSfID) == "" || strings.TrimSpace(contactSfID) == "" {
		return ProjectContact{}, false, &apierror.ValidationError{Msg: "projectId and contactId are required to search for a Salesforce membership"}
	}
	var rows []ProjectContact
	body := struct {
		ProjectID string `json:"projectId"`
		ContactID string `json:"contactId"`
		Limit     int    `json:"limit"`
	}{ProjectID: projectSfID, ContactID: contactSfID, Limit: 1}
	if err := c.searchWithRetry(ctx, projectContactSearchPath, body, "project contact", &rows); err != nil {
		return ProjectContact{}, false, err
	}
	for _, pc := range rows {
		if pc.Contact != nil && pc.Contact.ID != nil && salesforceIDEqual(*pc.Contact.ID, contactSfID) {
			return pc, true, nil
		}
	}
	return ProjectContact{}, false, nil
}

// CreateContact creates a Salesforce Contact (POST /contacts, 201) and
// returns it. Not idempotent — call SearchContactByEmail first.
func (c *Client) CreateContact(ctx context.Context, in CreateContactInput) (Contact, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if in.Email == "" {
		return Contact{}, &apierror.ValidationError{Msg: "email is required to create a Salesforce contact"}
	}
	if strings.TrimSpace(in.AccountID) == "" {
		return Contact{}, &apierror.ValidationError{Msg: "accountId is required to create a Salesforce contact"}
	}
	var out Contact
	if err := c.mutateWithRetry(ctx, http.MethodPost, contactsPath, in, "contact", &out); err != nil {
		return Contact{}, err
	}
	if out.ID == nil || strings.TrimSpace(*out.ID) == "" {
		return Contact{}, &apierror.ServiceUnavailableError{Msg: "salesentity: POST /contacts returned no contact id"}
	}
	return out, nil
}

// CreateProjectContact creates a Salesforce Project_Contact__c
// (POST /project-contacts, 201) and returns it. Not idempotent — call
// SearchProjectContact first.
func (c *Client) CreateProjectContact(ctx context.Context, in CreateProjectContactInput) (ProjectContact, error) {
	if strings.TrimSpace(in.ProjectID) == "" || strings.TrimSpace(in.ContactID) == "" {
		return ProjectContact{}, &apierror.ValidationError{Msg: "projectId and contactId are required to create a Salesforce membership"}
	}
	if in.Role == nil {
		in.Role = []string{}
	}
	var out ProjectContact
	if err := c.mutateWithRetry(ctx, http.MethodPost, projectContactsPath, in, "project contact", &out); err != nil {
		return ProjectContact{}, err
	}
	if strings.TrimSpace(out.ID) == "" {
		return ProjectContact{}, &apierror.ServiceUnavailableError{Msg: "salesentity: POST /project-contacts returned no membership id"}
	}
	return out, nil
}

// UpdateContactLockout sets the Contact's lockoutStatus
// (PATCH /contacts/{id}). The response body is not read: this call exists for
// its effect, and the caller already knows what it set.
func (c *Client) UpdateContactLockout(ctx context.Context, contactSfID string, lockedOut bool) error {
	if strings.TrimSpace(contactSfID) == "" {
		return &apierror.ValidationError{Msg: "contact id is required"}
	}
	return c.mutateWithRetry(ctx, http.MethodPatch, contactsPath+"/"+url.PathEscape(contactSfID),
		updateContactRequest{LockoutStatus: lockedOut}, "contact", nil)
}

// UpdateProjectContactState moves a membership to state
// (PATCH /project-contacts/{id}).
//
// That endpoint answers 200 with the record it re-read after the write, and
// 200 with an EMPTY BODY when its own re-read failed even though the write
// itself succeeded. An empty body is therefore a zero ProjectContact and no
// error: the write happened, we simply do not get the new row back.
func (c *Client) UpdateProjectContactState(ctx context.Context, membershipSfID, state string) (ProjectContact, error) {
	return c.updateProjectContact(ctx, membershipSfID, updateProjectContactRequest{State: &state})
}

// UpdateProjectContactRoles replaces a membership's Role__c picklist
// (PATCH /project-contacts/{id}), with the same empty-body contract as
// UpdateProjectContactState.
func (c *Client) UpdateProjectContactRoles(ctx context.Context, membershipSfID string, roles []string) (ProjectContact, error) {
	if roles == nil {
		roles = []string{}
	}
	return c.updateProjectContact(ctx, membershipSfID, updateProjectContactRequest{Role: &roles})
}

// UpdateProjectContact applies state and/or roles in one PATCH, so a change
// that moves both (a re-invitation that also changes the roles) is a single
// Salesforce write rather than two independently-failing ones.
func (c *Client) UpdateProjectContact(ctx context.Context, membershipSfID string, state *string, roles *[]string) (ProjectContact, error) {
	return c.updateProjectContact(ctx, membershipSfID, updateProjectContactRequest{State: state, Role: roles})
}

func (c *Client) updateProjectContact(ctx context.Context, membershipSfID string, body updateProjectContactRequest) (ProjectContact, error) {
	if strings.TrimSpace(membershipSfID) == "" {
		return ProjectContact{}, &apierror.ValidationError{Msg: "membership id is required"}
	}
	var out ProjectContact
	if err := c.mutateWithRetry(ctx, http.MethodPatch, projectContactsPath+"/"+url.PathEscape(membershipSfID), body, "project contact", &out); err != nil {
		return ProjectContact{}, err
	}
	return out, nil
}

// mutateWithRetry POSTs/PATCHes body to path, refreshing the token once on
// 401 — the same policy searchWithRetry applies to the read side.
func (c *Client) mutateWithRetry(ctx context.Context, method, path string, body any, what string, out any) error {
	err := c.mutate(ctx, method, path, body, what, out)
	if errors.Is(err, errCustomerUnauthorized) {
		c.invalidateToken()
		err = c.mutate(ctx, method, path, body, what, out)
		if errors.Is(err, errCustomerUnauthorized) {
			return &apierror.UnauthorizedError{Msg: "salesentity: " + path + " unauthorized"}
		}
	}
	return err
}

// mutate performs one authenticated write and decodes a 2xx JSON object into
// out (nil out discards the body). An empty 2xx body leaves out untouched —
// see UpdateProjectContactState for why that is a success, not a parse error.
// Status handling mirrors search, with one difference: a 404 here is a
// NotFoundError, because the record named in the path genuinely does not
// exist rather than being a search that has not caught up yet.
func (c *Client) mutate(ctx context.Context, method, path string, body any, what string, out any) error {
	token, err := c.accessToken(ctx)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("salesentity: marshal %s %s request: %w", method, path, err)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("salesentity: build %s %s request: %w", method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return err
		}
		return &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("salesentity: %s %s request: %v", method, path, err)}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("salesentity: read %s %s response: %w", method, path, err)
	}
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		if out == nil || len(bytes.TrimSpace(raw)) == 0 {
			return nil
		}
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("salesentity: parse %s %s response: %w", method, path, err)
		}
		return nil
	case resp.StatusCode == http.StatusUnauthorized:
		return errCustomerUnauthorized
	case resp.StatusCode == http.StatusNotFound:
		return &apierror.NotFoundError{Msg: "salesentity: " + what + " not found"}
	case resp.StatusCode >= 500:
		return &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("salesentity: %s %s returned %d", method, path, resp.StatusCode)}
	default:
		return &apierror.DownstreamError{Msg: fmt.Sprintf("salesentity rejected %s %s (status %d)", method, path, resp.StatusCode)}
	}
}
