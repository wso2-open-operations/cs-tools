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

import "fmt"

// supportGroupSetSQL is the set of groups an incident may be created in when
// the caller names the group itself: every active "group" that is the support
// group (service.support_group_id) of at least one service. POST /incidents'
// check of an explicit assignmentGroupId (IsSupportGroup) and POST
// /groups/search's supportGroupsOnly filter (SearchSupportGroups) both read
// this one statement, so the picker can never offer a group the create would
// refuse.
//
// "group".is_active is nullable (the sync leaves it NULL on some rows); NULL
// counts as active, the same convention "user".is_active follows everywhere
// in this service. Only an explicit FALSE removes a group from the set.
//
// It selects g.id, g.name, g.parent_id under the alias g; callers add their
// own conditions after it with AND.
const supportGroupSetSQL = `
	SELECT g.id, g.name, g.parent_id
	FROM "group" g
	WHERE g.is_active IS NOT FALSE
	  AND EXISTS (SELECT 1 FROM service s WHERE s.support_group_id = g.id)`

// incidentAssignableGroupSQL is the set of groups an incident may be created
// in when the caller names the group itself: supportGroupSetSQL plus every
// active "group" whose id is in the configured allowlist
// (INCIDENT_EXTRA_ASSIGNMENT_GROUP_IDS), for groups that own no service. The
// create check (IncidentRepository.IsSupportGroup) and the picker
// (GroupRepository.SearchSupportGroups) both build it here, so the picker can
// never offer a group the create would refuse. The allowlisted group must be
// active too, with the same convention as supportGroupSetSQL (NULL counts as
// active).
//
// extraIDsParam is the 1-based position of a uuid[] bind parameter holding the
// allowlist (an empty array, never NULL, when there is none). Same columns and
// alias as supportGroupSetSQL. Other consumers of supportGroupSetSQL (the
// case-alert route options) deliberately do not use this.
func incidentAssignableGroupSQL(extraIDsParam int) string {
	return fmt.Sprintf(`
	SELECT g.id, g.name, g.parent_id
	FROM "group" g
	WHERE g.is_active IS NOT FALSE
	  AND (EXISTS (SELECT 1 FROM service s WHERE s.support_group_id = g.id)
	       OR g.id = ANY($%d::uuid[]))`, extraIDsParam)
}

// incidentExtraGroupSQL is the allowlist part of incidentAssignableGroupSQL on
// its own: every active "group" whose id is in the configured allowlist
// (INCIDENT_EXTRA_ASSIGNMENT_GROUP_IDS), whether or not it supports a service.
// An empty allowlist matches nothing. Same bind parameter, columns, alias and
// active convention as incidentAssignableGroupSQL.
func incidentExtraGroupSQL(extraIDsParam int) string {
	return fmt.Sprintf(`
	SELECT g.id, g.name, g.parent_id
	FROM "group" g
	WHERE g.is_active IS NOT FALSE
	  AND g.id = ANY($%d::uuid[])`, extraIDsParam)
}

// extraGroupIDsArg is the bind value for incidentAssignableGroupSQL's
// allowlist parameter: never nil, which would bind NULL.
func extraGroupIDsArg(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}

// ServiceSupportGroup is a service as the incident create reads it: its name
// and its support group. Found is false when no service has the id; GroupID
// is "" when the service has no support group.
type ServiceSupportGroup struct {
	Found       bool
	ServiceName string
	GroupID     string
	GroupName   string
}
