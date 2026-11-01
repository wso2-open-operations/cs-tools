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

import "context"

// autoclosureActorKey carries the system user's email on a context for the one
// UpdateCase call the auto-closure workflow makes when it closes a ticket.
type autoclosureActorKey struct{}

// withAutoclosureActor marks ctx as the auto-closure workflow closing a
// ticket as the given system user. It does two things UpdateCase would
// otherwise refuse or skip for a machine caller:
//
//   - it allows Awaiting Info -> Closed, a move no staff table offers (staff
//     must go through Waiting on WSO2) but which is exactly what the workflow
//     exists to do after the customer has been reminded twice;
//   - it attributes the close (closed_by_user_id and the activity entry) to the
//     system user, since a scheduled task has no x-user-id-token to resolve.
//
// It is unexported on purpose: only AutoclosureService, in this package, can
// set it, so no HTTP request can ask for either exemption.
func withAutoclosureActor(ctx context.Context, email string) context.Context {
	return context.WithValue(ctx, autoclosureActorKey{}, email)
}

// autoclosureActorFromContext returns the system user's email set by
// withAutoclosureActor, if any.
func autoclosureActorFromContext(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(autoclosureActorKey{}).(string)
	return email, ok && email != ""
}
