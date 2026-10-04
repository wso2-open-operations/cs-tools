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
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/directory"
)

// teamIDFilterLimit caps the teamIds filter. Each key expands to one group name
// on the membership query that resolves the filter to a user-id set, and that
// set then feeds an IN clause upstream that is silently truncated if it grows
// too long.
const teamIDFilterLimit = 50

// roleIDFilterLimit caps the roleIds filter, matching what the entity service
// enforced while it owned the role allow-list.
const roleIDFilterLimit = 20

// teamUUIDPattern matches this platform's canonical UUID text (lowercase or
// uppercase hex, 8-4-4-4-12 hyphenated). A teamIds entry in this shape is
// resolved against a team's backing group id rather than its registry key --
// the same UUID form accounts.creTeam.id/sreTeam.id already expose elsewhere.
var teamUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// userTeamRef is a team the user is a member of, as GET /users/{id} exposes it.
// ID is the registry team key, which is stable across environments -- unlike
// the underlying group id.
type userTeamRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Family string `json:"family,omitempty"`
}

// teamForGroups picks the caller's ABT team out of the groups they belong to.
//
// A user belongs to at most one registry team (confirmed), so the first match
// is authoritative and the rest of their groups are ordinary groups this does
// not care about. Nil when none of them is a registry team.
func (h *UsersHandler) teamForGroups(groups []entityGroupRef) *userTeamResponse {
	for _, g := range groups {
		team, ok := h.dir.TeamByGroupName(g.Name)
		if !ok {
			continue
		}
		return &userTeamResponse{
			TeamKey:  team.Key,
			TeamName: team.Name,
			Family:   string(team.Family),
		}
	}
	return nil
}

// withUserTeams adds the teams block to a GET /users/{id} response: the subset
// of the user's groups that are registry teams, keyed by registry key.
//
// The upstream response is re-encoded rather than string-patched, so a
// malformed one fails here instead of producing invalid JSON downstream. Field
// order is not preserved, which no JSON consumer depends on.
func (h *UsersHandler) withUserTeams(raw []byte) ([]byte, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode user profile: %w", err)
	}

	teams := []userTeamRef{}
	if rawGroups, ok := envelope["groups"]; ok {
		var groups []entityGroupRef
		if err := json.Unmarshal(rawGroups, &groups); err != nil {
			return nil, fmt.Errorf("decode user groups: %w", err)
		}
		for _, g := range groups {
			if team, ok := h.dir.TeamByGroupName(g.Name); ok {
				teams = append(teams, userTeamRef{
					ID:     team.Key,
					Name:   team.Name,
					Family: string(team.Family),
				})
			}
		}
	}

	encoded, err := json.Marshal(teams)
	if err != nil {
		return nil, fmt.Errorf("encode user teams: %w", err)
	}
	envelope["teams"] = encoded

	out, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("encode user profile: %w", err)
	}
	return out, nil
}

// resolveUserSearchFilters rewrites a POST /users/search body so the entity
// service can serve it without a team registry:
//
//   - roleIds is validated against the configured allow-list, which lives here
//     now. An unknown role is a caller mistake, and letting it through would
//     return a confidently empty page instead of an error.
//   - teamIds is replaced by the group names those teams resolve to. Each
//     entry may be either a registry teamKey or the platform UUID form of the
//     team's backing group id (the same shape accounts.creTeam.id/sreTeam.id
//     expose) -- both resolve to the same team. The entity service filters
//     membership by group name; only this layer knows which name either id
//     shape means.
//
// Every other field passes through untouched, and a body with no filters object
// is returned byte-for-byte as it arrived. The returned error is caller-facing:
// it names the offending value.
func (h *UsersHandler) resolveUserSearchFilters(body []byte) ([]byte, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		// Not an object (an array, a bare literal, ...). Nothing to rewrite;
		// let the entity service reject it as it always has.
		return body, nil
	}
	rawFilters, ok := envelope["filters"]
	if !ok {
		return body, nil
	}

	var filters map[string]json.RawMessage
	if err := json.Unmarshal(rawFilters, &filters); err != nil {
		return nil, callerMessageError(ErrMsgBadRequest)
	}

	if rawRoles, ok := filters["roleIds"]; ok {
		var roleIDs []string
		if err := json.Unmarshal(rawRoles, &roleIDs); err != nil {
			return nil, callerMessageError(ErrMsgBadRequest)
		}
		if len(roleIDs) > roleIDFilterLimit {
			return nil, fmt.Errorf("roleIds cannot contain more than %d values", roleIDFilterLimit)
		}
		for _, role := range roleIDs {
			if !h.dir.IsValidRole(role) {
				return nil, fmt.Errorf("roleIds contains invalid value: %s", role)
			}
		}
	}

	rawTeams, ok := filters["teamIds"]
	if !ok {
		return body, nil
	}
	var teamIDs []string
	if err := json.Unmarshal(rawTeams, &teamIDs); err != nil {
		return nil, callerMessageError(ErrMsgBadRequest)
	}
	if len(teamIDs) > teamIDFilterLimit {
		return nil, fmt.Errorf("teamIds cannot contain more than %d values", teamIDFilterLimit)
	}

	delete(filters, "teamIds")

	if len(teamIDs) > 0 {
		// Merge rather than overwrite: a caller may legitimately have supplied
		// groupNames of its own, and silently dropping them would narrow the
		// search without saying so.
		var groupNames []string
		if rawNames, ok := filters["groupNames"]; ok {
			if err := json.Unmarshal(rawNames, &groupNames); err != nil {
				return nil, callerMessageError(ErrMsgBadRequest)
			}
		}
		for _, key := range teamIDs {
			var team directory.Team
			var ok bool
			if teamUUIDPattern.MatchString(key) {
				// The platform UUID form of a team's backing group id -- the
				// same shape accounts.creTeam.id/sreTeam.id already expose.
				team, ok = h.dir.TeamByUUID(key)
			} else {
				team, ok = h.dir.TeamByKey(key)
			}
			if !ok {
				return nil, fmt.Errorf("teamIds contains unknown team: %s", key)
			}
			groupNames = append(groupNames, team.Name)
		}
		encoded, err := json.Marshal(groupNames)
		if err != nil {
			return nil, callerMessageError(ErrMsgInternal)
		}
		filters["groupNames"] = encoded
	}

	encodedFilters, err := json.Marshal(filters)
	if err != nil {
		return nil, callerMessageError(ErrMsgInternal)
	}
	envelope["filters"] = encodedFilters

	out, err := json.Marshal(envelope)
	if err != nil {
		return nil, callerMessageError(ErrMsgInternal)
	}
	return out, nil
}

// callerMessageError carries one of the fixed caller-facing messages
// (ErrMsgBadRequest, ErrMsgInternal) as an error value unchanged: SearchUsers
// writes err.Error() to the client, so those sentences keep their wording
// rather than Go's lower-case error-string style.
type callerMessageError string

func (e callerMessageError) Error() string { return string(e) }
