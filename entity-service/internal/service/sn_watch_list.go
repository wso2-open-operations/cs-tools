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

package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// Watch lists on case create/update (and incident create) arrive either as
// watcher emails — the Customer Portal / Ballerina contract — or as platform
// user UUIDs from CSM. The backing service's write payloads are not keyed by
// id at all: those three payloads declare the watch list as email addresses,
// and the incident update payload declares it as usernames. Forwarding UUIDs
// verbatim is silently accepted upstream and drops every watcher, so each id
// is resolved to the identity value its target payload actually declares
// before the write goes out. Emails are already the declared shape and are
// forwarded as-is.
//
// snWatchListIdentity holds both values from the one lookup, so a caller can
// pick the one its payload needs without a second round trip.
type snWatchListIdentity struct {
	Email    string
	UserName string
}

// resolveWatchListIdentities resolves watch-list user UUIDs to their upstream
// identity values, preserving the caller's order. field names the request field
// being validated so a bad value produces the same class of validation error as
// every other id field on the same request.
//
// An id that resolves to no user is a validation error rather than a silent
// omission: a dropped watcher is invisible to the caller, who sees a successful
// write and no watcher.
func resolveWatchListIdentities(
	ctx context.Context, client *integrationservice.Client, token, field string, ids []string,
) ([]snWatchListIdentity, error) {
	if err := validateUUIDs(field, ids); err != nil {
		return nil, err
	}

	out := make([]snWatchListIdentity, 0, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	// The backing user search caps a page at maxUserLimit, and the whole watch
	// list has to resolve in one page for the lookup to stay a single call.
	if len(ids) > maxUserLimit {
		return nil, &apierror.ValidationError{
			Msg: fmt.Sprintf("%s cannot contain more than %d values", field, maxUserLimit),
		}
	}

	// One search for the whole list rather than a lookup per id: the filter takes
	// a set of ids, and a watch list routinely carries several users.
	sysIDs := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		sysID := uuidToSysid(id)
		if _, dup := seen[sysID]; dup {
			continue
		}
		seen[sysID] = struct{}{}
		sysIDs = append(sysIDs, sysID)
	}

	payload := snUserSearchPayload{
		Filters:    snUserFilters{UserIDs: sysIDs},
		Pagination: snProjectPagination{Limit: len(sysIDs), Offset: 0},
	}

	raw, err := client.Post(ctx, "/users/search", token, payload)
	if err != nil {
		return nil, err
	}

	var snResp snUsersResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return nil, fmt.Errorf("sn resolve watch list: parse response: %w", err)
	}

	byID := make(map[string]snUser, len(snResp.Users))
	for _, u := range snResp.Users {
		byID[u.ID] = u
	}

	for _, id := range ids {
		u, ok := byID[uuidToSysid(id)]
		if !ok {
			return nil, &apierror.ValidationError{
				Msg: fmt.Sprintf("%s contains an unknown user id: %q", field, id),
			}
		}
		out = append(out, snWatchListIdentity{Email: u.Email, UserName: u.UserName})
	}

	return out, nil
}

// watchListEmails returns the email addresses the backing service's case
// create, case update, and incident create payloads declare. Incoming emails
// (Customer Portal) are forwarded in the caller's order with no lookup and
// no length cap (Ballerina; existing SN cases often exceed 50 watchers).
// Incoming platform UUIDs (CSM) are resolved to emails first — forwarding
// ids verbatim is silently accepted upstream and drops every watcher — and
// are capped at maxUserLimit because resolution is one /users/search page.
// A mixed list, or a value that is neither an email nor a UUID, is a
// validation error. The returned slice is non-nil and in the caller's order.
func watchListEmails(
	ctx context.Context, client *integrationservice.Client, token, field string, values []string,
) ([]string, error) {
	if len(values) == 0 {
		return []string{}, nil
	}

	allEmail := true
	allUUID := true
	var firstInvalid string
	for _, v := range values {
		isEmail := emailRE.MatchString(v)
		isUUID := uuidRE.MatchString(v)
		if !isEmail {
			allEmail = false
		}
		if !isUUID {
			allUUID = false
		}
		if !isEmail && !isUUID && firstInvalid == "" {
			firstInvalid = v
		}
	}

	if allEmail {
		// Emails are forwarded as-is (Ballerina / Customer Portal). The 50-item
		// cap below exists only because UUID resolution is a single
		// /users/search page; it must not reject an existing SN watch list that
		// is already larger than that page size.
		out := make([]string, len(values))
		copy(out, values)
		return out, nil
	}
	if !allUUID {
		if firstInvalid != "" {
			return nil, &apierror.ValidationError{
				Msg: fmt.Sprintf("%s contains invalid email: %q", field, firstInvalid),
			}
		}
		return nil, &apierror.ValidationError{
			Msg: fmt.Sprintf("%s items must all be email addresses or all be user identifiers", field),
		}
	}

	if len(values) > maxUserLimit {
		return nil, &apierror.ValidationError{
			Msg: fmt.Sprintf("%s cannot contain more than %d values", field, maxUserLimit),
		}
	}

	identities, err := resolveWatchListIdentities(ctx, client, token, field, values)
	if err != nil {
		return nil, err
	}

	emails := make([]string, 0, len(identities))
	for i, identity := range identities {
		if identity.Email == "" {
			return nil, &apierror.ValidationError{
				Msg: fmt.Sprintf("%s contains a user with no email address: %q", field, values[i]),
			}
		}
		emails = append(emails, identity.Email)
	}

	return emails, nil
}

// watchListUserNames resolves watch-list user UUIDs to usernames, for the
// payloads that declare the watch list that way. The returned slice is non-nil
// and in the caller's order, so an explicitly empty watch list still reaches the
// backing service as an empty list rather than being dropped.
func watchListUserNames(
	ctx context.Context, client *integrationservice.Client, token, field string, ids []string,
) ([]string, error) {
	identities, err := resolveWatchListIdentities(ctx, client, token, field, ids)
	if err != nil {
		return nil, err
	}

	userNames := make([]string, 0, len(identities))
	for i, identity := range identities {
		if identity.UserName == "" {
			return nil, &apierror.ValidationError{
				Msg: fmt.Sprintf("%s contains a user with no username: %q", field, ids[i]),
			}
		}
		userNames = append(userNames, identity.UserName)
	}

	return userNames, nil
}
