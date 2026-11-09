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

// incidentAutocloseKey carries the incident auto-close's system user and
// threshold on a context, for the one UpdateIncident call it makes per incident.
type incidentAutocloseKey struct{}

type incidentAutocloseMark struct {
	email string
	days  int
}

// withIncidentAutoclose marks ctx as the incident auto-close closing an
// incident as the given system user, as withAutoclosureActor does for case
// auto-closure. UpdateIncident then:
//
//   - attributes the close (updated_by, closed_by_id, the activity entry and
//     the task cascade's notes) to the system user, since a scheduled task has
//     no x-user-id-token to resolve;
//   - closes the incident only while it is still due after days days
//     (re-checked under the row lock), and without the resolution code and
//     notes a manual close needs.
//
// It is unexported on purpose: only IncidentAutocloseService, in this
// package, can set it, so no HTTP request can ask for either exemption.
func withIncidentAutoclose(ctx context.Context, email string, days int) context.Context {
	return context.WithValue(ctx, incidentAutocloseKey{}, incidentAutocloseMark{email: email, days: days})
}

// incidentAutocloseFromContext returns what withIncidentAutoclose set, if any.
func incidentAutocloseFromContext(ctx context.Context) (email string, days int, ok bool) {
	m, ok := ctx.Value(incidentAutocloseKey{}).(incidentAutocloseMark)
	if !ok || m.email == "" {
		return "", 0, false
	}
	return m.email, m.days, true
}
