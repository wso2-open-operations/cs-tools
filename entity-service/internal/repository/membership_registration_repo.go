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

package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// InvitedMembership is one not-yet-accepted project_contact row, reduced to
// the two Salesforce ids the registration flip needs: the membership's own
// (project_contact.sf_id) and its contact's (account_contact.sf_id, the
// Salesforce Contact the lockout flag lives on).
type InvitedMembership struct {
	MembershipSfID string
	ContactSfID    string
}

// MembershipRegistrationRepository reads what POST /users/me/memberships/register has to act on:
// the caller's memberships that Salesforce still considers un-accepted.
//
// Schema prerequisite, the same one the Salesforce membership ingest carries:
// the sf_id columns on project_contact and account_contact come from the
// csm-sync migration 0076, which is not in this repo's migrations/.
type MembershipRegistrationRepository interface {
	// InvitedMembershipsByEmail returns the caller's project_contact rows in
	// an un-accepted state (INVITED / RE-INVITED) that carry both Salesforce
	// ids. A row missing either id cannot be flipped in Salesforce at all, so
	// it is left out rather than returned for a call that would fail; no rows
	// is an empty slice, never an error -- the overwhelmingly common case,
	// since this runs on every profile load.
	InvitedMembershipsByEmail(ctx context.Context, email string) ([]InvitedMembership, error)
}

type registrationRepo struct {
	db *pgxpool.Pool
}

// NewMembershipRegistrationRepository constructs a MembershipRegistrationRepository backed by the pool.
func NewMembershipRegistrationRepository(db *pgxpool.Pool) MembershipRegistrationRepository {
	return &registrationRepo{db: db}
}

// InvitedMembershipsByEmail implements MembershipRegistrationRepository. project_contact
// is matched on its own email (case-insensitively), the same join
// access_repo.go's RegisteredProjectIDs uses for the mirror-image state.
func (r *registrationRepo) InvitedMembershipsByEmail(ctx context.Context, email string) ([]InvitedMembership, error) {
	rows, err := r.db.Query(ctx, `
		SELECT pc.sf_id, ac.sf_id
		FROM project_contact pc
		JOIN account_contact ac ON ac.id = pc.account_contact_id
		WHERE LOWER(pc.email) = LOWER($1)
		  AND pc.state::TEXT = ANY($2::text[])
		  AND NULLIF(TRIM(pc.sf_id), '') IS NOT NULL
		  AND NULLIF(TRIM(ac.sf_id), '') IS NOT NULL
		ORDER BY pc.sf_id`,
		email, []string{domain.MembershipStateInvited, domain.MembershipStateReInvited})
	if err != nil {
		return nil, fmt.Errorf("register memberships: invited memberships by email: %w", err)
	}
	defer rows.Close()

	out := []InvitedMembership{}
	for rows.Next() {
		var m InvitedMembership
		if err := rows.Scan(&m.MembershipSfID, &m.ContactSfID); err != nil {
			return nil, fmt.Errorf("register memberships: scan invited membership: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("register memberships: iterate invited memberships: %w", err)
	}
	return out, nil
}
