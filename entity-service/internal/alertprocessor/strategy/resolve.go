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

package strategy

import "github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/severity"

// ResolveStrategy turns a resolving alert into an OK note on its open incident, and drops it when there is none.
type ResolveStrategy struct{}

// Name implements Strategy.
func (ResolveStrategy) Name() string { return "resolve" }

// Decide implements Strategy.
func (ResolveStrategy) Decide(in Input) (Decision, bool) {
	if !severity.IsResolving(in.Event.Alert.Severity) {
		return Decision{}, false
	}
	if !in.HasOpenIncident() {
		return Decision{Outcome: Ignore, Reason: "resolving alert with no open incident"}, true
	}
	return Decision{Outcome: Resolve, Reason: "resolving alert"}, true
}
