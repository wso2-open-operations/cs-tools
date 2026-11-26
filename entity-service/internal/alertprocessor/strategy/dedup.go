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

import "time"

// DedupStrategy folds an alert into its fingerprint's open incident when it was received within Window of that incident's first alert, before or after it.
type DedupStrategy struct {
	Window time.Duration
}

// Name implements Strategy.
func (DedupStrategy) Name() string { return "dedup" }

// Decide implements Strategy.
func (d DedupStrategy) Decide(in Input) (Decision, bool) {
	if !in.HasOpenIncident() {
		return Decision{}, false
	}
	// Both sides: an alert received well before the incident's first alert, such as a retried event, is not part of it.
	if elapsed := in.Event.ReceivedOn.Sub(in.Group.FirstSeenOn); elapsed >= d.Window || elapsed <= -d.Window {
		return Decision{}, false
	}
	return Decision{Outcome: Duplicate, Reason: "same fingerprint inside the dedup window"}, true
}
