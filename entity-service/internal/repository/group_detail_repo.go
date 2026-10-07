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

package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/db"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// GroupDetailRepository reads one "group" row together with its members -- the
// page a user lands on when they open an approval stage's assignment group,
// like ServiceNow's group form and its "Group Members" tab.
//
// This is deliberately separate from GroupRepository, whose SearchGroups lists
// the `team` registry (the group picker). Ids from the approvals response
// (approval_stage.assignment_group_id) are "group" ids, and a group's members
// are team_member rows keyed by group_id -- see entity-service CLAUDE.md,
// "team_member.group_id is the real column for this".
type GroupDetailRepository interface {
	// GetGroupDetail returns the group and its active INTERNAL members ordered by name,
	// or a NotFoundError when no "group" row has this id. A group with no
	// members is not an error: Members is empty (never nil).
	GetGroupDetail(ctx context.Context, groupID string) (domain.GroupDetail, error)
}

type groupDetailRepo struct {
	db db.Pool
}

// NewGroupDetailRepository constructs a GroupDetailRepository backed by the
// given connection pool.
func NewGroupDetailRepository(db db.Pool) GroupDetailRepository {
	return &groupDetailRepo{db: db}
}

// namedPoolGroups are the groups the approval pools resolve BY NAME rather than
// by id: the CAB and ECAB stages' own groups and the Devops Approval peer
// fallback (resolveApprovalPool / resolvePeerPool -> namedGroup). Every other
// group an approval stage points at is a change's assigned group, whose pools
// read team_member.group_id = <the group's id> only (groupMemberIDs).
var namedPoolGroups = map[string]bool{
	domain.CABApprovalGroupName:          true,
	domain.ECABApprovalGroupName:         true,
	domain.PeerApprovalFallbackGroupName: true,
}

// groupDetailMembersSQL lists the distinct active INTERNAL users who are members
// of the group, one row per user, in name order.
//
// WHO IS A MEMBER is exactly who the approval pools provision from, so the list
// a user sees is the list of people who can actually be asked to approve
// (apart from per-change exclusions such as the creator), and there are two
// shapes of pool:
//
//   - an assigned group (the Peer and Review stages) reads
//     team_member.group_id = <the group's id> and nothing else
//     (groupMemberIDs) -- so does this, for any group not listed below;
//   - the CAB / ECAB / Devops Approval groups are resolved by NAME
//     (namedGroup): anyone whose team_member.group_id points at a "group" of
//     that name, or whose team_member.team_id points at a `team` of that name
//     (which is also how the CR-notice flow addresses these audiences) -- so
//     for those names does this ($3 = true).
//
// A team of the same name as an ordinary assigned group therefore does not add
// people to that group's page: its members are not in the peer pool either.
//
// $1 is the group id, $2 its name (NULL when the row has none), $3 whether the
// group is one of namedPoolGroups.
//
// Like every pool (internalApproverIDs), only an active user ("user".is_active,
// NULL counting as active, as in user_repo.go) whose user_type is INTERNAL is
// listed: a customer who happens to sit in a staff group, an inactive user or a
// user with no derivable type could never be provisioned, so showing them would
// promise an approver the stage does not have. A user holding several
// membership rows is one member, and a "lead" row on any of them makes them a
// lead.
//
// The display name falls back from "user".name to first + last name to the
// email, so a member is never listed blank.
const groupDetailMembersSQL = `
	SELECT u.id::text,
	       COALESCE(NULLIF(TRIM(u.name), ''),
	                NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), ''),
	                NULLIF(TRIM(u.email), ''),
	                '') AS member_name,
	       u.email,
	       u.user_type::text,
	       CASE WHEN BOOL_OR(tm.role = 'lead') THEN 'lead' ELSE 'member' END AS member_role
	FROM team_member tm
	JOIN "user" u ON u.id = tm.user_id
	WHERE (
	        tm.group_id = $1::uuid
	     OR ($3::boolean AND $2::text IS NOT NULL AND (
	            tm.group_id IN (SELECT g2.id FROM "group" g2 WHERE g2.name = $2::text)
	         OR tm.team_id  IN (SELECT t.id  FROM team    t  WHERE t.name  = $2::text)))
	      )
	  AND u.user_type = 'INTERNAL'::user_type_enum
	  AND COALESCE(u.is_active, TRUE)
	GROUP BY u.id, u.name, u.first_name, u.last_name, u.email, u.user_type
	ORDER BY LOWER(COALESCE(NULLIF(TRIM(u.name), ''),
	                        NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), ''),
	                        NULLIF(TRIM(u.email), ''),
	                        '')),
	         u.id`

// GetGroupDetail implements GroupDetailRepository.
func (r *groupDetailRepo) GetGroupDetail(ctx context.Context, groupID string) (domain.GroupDetail, error) {
	var (
		detail      domain.GroupDetail
		name        *string
		managerID   *string
		managerName *string
	)
	err := r.db.QueryRow(ctx, `
		SELECT g.id::text, g.name, g.description, g.group_email, m.id::text,
		       COALESCE(NULLIF(TRIM(m.name), ''),
		                NULLIF(TRIM(CONCAT_WS(' ', m.first_name, m.last_name)), ''),
		                NULLIF(TRIM(m.email), ''))
		FROM "group" g
		LEFT JOIN "user" m ON m.id = g.manager_id
		WHERE g.id = $1::uuid`, groupID).
		Scan(&detail.ID, &name, &detail.Description, &detail.Email, &managerID, &managerName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.GroupDetail{}, &apierror.NotFoundError{Msg: "group not found"}
		}
		return domain.GroupDetail{}, fmt.Errorf("get group: %w", err)
	}
	if name != nil {
		detail.Name = *name
	}
	if managerID != nil {
		ref := &domain.GroupManagerRef{ID: *managerID}
		if managerName != nil {
			ref.Name = *managerName
		}
		detail.Manager = ref
	}

	resolvedByName := name != nil && namedPoolGroups[*name]
	rows, err := r.db.Query(ctx, groupDetailMembersSQL, groupID, name, resolvedByName)
	if err != nil {
		return domain.GroupDetail{}, fmt.Errorf("list group members: %w", err)
	}
	defer rows.Close()

	detail.Members = []domain.GroupMember{}
	for rows.Next() {
		var m domain.GroupMember
		if err := rows.Scan(&m.ID, &m.Name, &m.Email, &m.UserType, &m.Role); err != nil {
			return domain.GroupDetail{}, fmt.Errorf("scan group member: %w", err)
		}
		detail.Members = append(detail.Members, m)
	}
	if err := rows.Err(); err != nil {
		return domain.GroupDetail{}, fmt.Errorf("iterate group members: %w", err)
	}
	detail.Total = len(detail.Members)
	return detail, nil
}
