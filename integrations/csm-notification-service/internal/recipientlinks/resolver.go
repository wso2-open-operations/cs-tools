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

// Package recipientlinks resolves, for a list of notification recipients,
// which portal's case link each one should get in their email: a customer
// contact should never be handed a CSM-portal link they can't access (and
// vice versa isn't the point, but keeping the audiences separate is). This
// is a per-recipient decision, not a per-event one — the same
// case.comment_added notification can go to both a customer watcher and an
// internal CSM watcher at once, each needing a different link — so
// internal/dispatch calls this to resolve links, then groups recipients by
// the link they resolved to, before rendering/sending anything (see
// Dispatcher.groupByLink). This does not resolve *who* to notify — every
// case.* payload still carries its own caller-supplied Recipients list, see
// internal/events' package doc — only which link a given recipient gets.
//
// Note the customer portal has no comment-permalink fragment handling today
// (only the CSM portal's frontend reads location.hash to scroll to a
// comment) — a comment-specific fragment on a customer-portal link is
// simply inert there, not an error.
package recipientlinks

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/entity"
)

// entityClient abstracts entity.CustomerEntityClient's user-role lookup for
// testability.
type entityClient interface {
	SearchUsersByEmail(ctx context.Context, emails []string) ([]entity.UserRoleInfo, error)
}

// wso2EmailDomain is WSO2's own corporate domain — mirrors
// apps/csm-portal/backend's own wso2EmailDomain constant (see that
// package's user_external_account.go), used the same way here: linkFor's
// fallback classifier when a recipient's roles (from the same
// SearchUsersByEmail response ResolveLinks already calls) match neither
// CustomerRoles nor CSMRoles.
const wso2EmailDomain = "wso2.com"

// Config holds the role classification and portal base URLs Resolver needs.
// CustomerRoles and CSMRoles need not be exhaustive of every role in the
// system — see ResolveLinks' doc comment for what happens when a
// recipient's roles match neither list.
type Config struct {
	CustomerRoles   []string
	CSMRoles        []string
	CustomerBaseURL string
	CSMBaseURL      string
}

// Resolver resolves recipient emails to portal-appropriate case links.
type Resolver struct {
	entity        entityClient
	customerRoles map[string]bool
	csmRoles      map[string]bool
	customerBase  string
	csmBase       string
}

// New constructs a Resolver.
func New(entity entityClient, cfg Config) *Resolver {
	return &Resolver{
		entity:        entity,
		customerRoles: toSet(cfg.CustomerRoles),
		csmRoles:      toSet(cfg.CSMRoles),
		customerBase:  cfg.CustomerBaseURL,
		csmBase:       cfg.CSMBaseURL,
	}
}

func toSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, v := range values {
		set[v] = true
	}
	return set
}

// RecipientLink pairs a recipient's email with the case link resolved for
// their role.
type RecipientLink struct {
	Email    string
	CaseLink string
	// UserID is the recipient's entity-service user id, "" when entity-service
	// has no record for this email (linkFor's "not found" fallback path) —
	// dispatch logs this instead of the email address itself when a
	// notification actually sends, so a delivery is traceable without a raw
	// address ever appearing in logs.
	UserID string
}

// ResolveLinks looks up each of emails' roles via entity-service and
// returns the case link appropriate to each: a recipient whose roles
// include one from Config.CustomerRoles gets the customer portal's link
// (<CustomerBaseURL>/projects/{projectID}/support/cases/{caseID}); anyone
// else gets the CSM portal's link (<CSMBaseURL>/cases/{caseID}).
//
// A recipient whose roles match neither CustomerRoles nor CSMRoles — or
// whom entity-service has no record for at all — falls back to their email
// domain instead: wso2EmailDomain gets the CSM portal link, anything else
// gets the customer portal link, and a warning is logged either way. The
// role lists are operator-curated and may not be exhaustive, and
// incomplete/test entity-service data frequently leaves roles empty — an
// email domain is always present.
//
// The returned links are the bare case link only — appending a
// comment-specific anchor/fragment is internal/dispatch's job (see
// commentLinkFor there), since that varies by event type and this package
// only knows about portal audiences.
func (r *Resolver) ResolveLinks(ctx context.Context, emails []string, projectID, caseID string) ([]RecipientLink, error) {
	if len(emails) == 0 {
		return nil, nil
	}

	users, err := r.entity.SearchUsersByEmail(ctx, emails)
	if err != nil {
		return nil, fmt.Errorf("recipientlinks: search users: %w", err)
	}

	// Keyed lowercase: entity-service and the caller's own recipient list
	// don't necessarily agree on email casing, and an email address is the
	// same address regardless of case.
	byEmail := make(map[string]entity.UserRoleInfo, len(users))
	for _, u := range users {
		byEmail[strings.ToLower(u.Email)] = u
	}

	links := make([]RecipientLink, 0, len(emails))
	for _, email := range emails {
		user, found := byEmail[strings.ToLower(email)]
		links = append(links, RecipientLink{
			Email:    email,
			CaseLink: r.linkFor(ctx, user, found, email, projectID, caseID),
			UserID:   user.ID,
		})
	}
	return links, nil
}

// linkFor takes the recipient's email only for its own domain comparison
// against wso2EmailDomain — never logged, same PII reasoning as everywhere
// else in this package (an email address in the logs is what this repo's
// own convention disallows). caseID identifies which notification a
// warning belongs to without identifying who it's about.
//
// Classification order: role first (CustomerRoles/CSMRoles — from the same
// SearchUsersByEmail response ResolveLinks already has, the operator-
// curated, most specific signal); when a recipient's roles match neither
// list — including when entity-service has no record for them at all —
// their email domain decides instead: wso2EmailDomain gets the CSM portal
// link, anything else gets the customer portal link. A domain is always
// present, unlike roles, which incomplete/test entity-service data
// frequently leaves empty.
func (r *Resolver) linkFor(ctx context.Context, user entity.UserRoleInfo, found bool, email, projectID, caseID string) string {
	isCustomer := false
	switch {
	case found && r.matchesAny(user.Roles, r.customerRoles):
		isCustomer = true
	case found && r.matchesAny(user.Roles, r.csmRoles):
		// isCustomer already false.
	default:
		isCustomer = !strings.EqualFold(emailDomain(email), wso2EmailDomain)
		slog.WarnContext(ctx, "recipientlinks: recipient's roles did not resolve a portal; used email domain as a fallback",
			"caseID", caseID, "found", found, "isCustomer", isCustomer)
	}

	if isCustomer {
		return fmt.Sprintf("%s/projects/%s/support/cases/%s", r.customerBase, url.PathEscape(projectID), url.PathEscape(caseID))
	}
	return fmt.Sprintf("%s/cases/%s", r.csmBase, url.PathEscape(caseID))
}

// CSMLink builds the CSM portal's case link directly, without any recipient
// or role lookup — for a notification with no per-recipient audience to
// resolve against (e.g. dispatch's case.created Google Chat alert, which
// posts once to a shared internal Chat space rather than per-recipient), the
// CSM portal is always the right audience; there's no customer-facing
// equivalent of that Chat space to route to instead.
func (r *Resolver) CSMLink(caseID string) string {
	return fmt.Sprintf("%s/cases/%s", r.csmBase, url.PathEscape(caseID))
}

// ChangeRequestLink builds the link in a change-request approval notice.
//
// Not CSMLink: that returns "<csmBase>/cases/<id>", and a change request is not
// a case — the CSM portal serves it at /operations/change-requests/<id>, and
// the customer portal nests its own copy under the project. The audience on the
// notice picks between them, which is also why it must: an internal approver
// linked into the customer portal lands somewhere they have no reason to be,
// and a customer linked into the CSM portal lands somewhere they cannot go at
// all.
//
// A customer notice with no project falls back to the CSM link rather than
// building "/projects//operations/..." — a wrong link a recipient can recognise
// beats a malformed one.
func (r *Resolver) ChangeRequestLink(audience, changeRequestID, projectID string) string {
	if audience == "customer" && projectID != "" {
		return fmt.Sprintf("%s/projects/%s/operations/change-requests/%s",
			r.customerBase, url.PathEscape(projectID), url.PathEscape(changeRequestID))
	}
	return fmt.Sprintf("%s/operations/change-requests/%s", r.csmBase, url.PathEscape(changeRequestID))
}

// IncidentLink builds the CSM portal's incident link directly — the same
// no-recipient reasoning as CSMLink, applied to incident.created's Google
// Chat alert. A publisher only supplies the incident's own identity
// (EntityID); this service is the one that knows the CSM portal's base URL
// and builds the "Open in Portal" button target itself, the same way it
// already does for case.created rather than trusting a caller-supplied
// link — see dispatch.handleIncidentCreated.
func (r *Resolver) IncidentLink(incidentID string) string {
	return fmt.Sprintf("%s/operations/incidents/%s", r.csmBase, url.PathEscape(incidentID))
}

// emailDomain returns the part of email after its last "@", lowercased —
// "" for an address with no "@" at all, which wso2EmailDomain will simply
// never match (see linkFor).
func emailDomain(email string) string {
	i := strings.LastIndex(email, "@")
	if i < 0 {
		return ""
	}
	return strings.ToLower(email[i+1:])
}

func (r *Resolver) matchesAny(roles []string, set map[string]bool) bool {
	for _, role := range roles {
		if set[role] {
			return true
		}
	}
	return false
}
