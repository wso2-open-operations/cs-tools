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

// Package alertpayload defines the JSON shape actually persisted in
// alert_buffer.payload: the mapped csmclient.CreateIncidentRequest a
// worker eventually POSTs to csm-integration-service, plus the subset of
// the original inbound alert's own fields internal/worker needs at
// delivery time for incident-grouping (Source/UniqueIdentifier to look up
// an earlier alert's already-open incident for the same underlying
// condition; Service/MetricName/AlertStatus to record on the resulting
// alert-incident-mapping row).
//
// This is its own tiny package, rather than living in internal/handler
// (which builds it) or internal/worker (which reads it back), so neither
// package has to import the other just for this shared wire shape.
package alertpayload

import "github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/csmclient"

// Payload is the full JSON persisted as alert_buffer.payload.
// csmclient.CreateIncidentRequest is embedded at the top level (not nested
// under a key) so the exact fields csm-integration-service expects on
// POST /incidents are, byte for byte, what internal/worker sends —
// json.Unmarshal(row.Payload, &p) then p.CreateIncidentRequest is passed
// straight through to CreateIncident, with no re-mapping.
type Payload struct {
	csmclient.CreateIncidentRequest

	// Source is the alerting tool's own identifier (e.g. "azure",
	// "site24x7"), carried through unmapped — it's half of the
	// incident-grouping match key (see internal/worker's grouping check).
	Source string `json:"source"`
	// UniqueIdentifier is the alerting tool's own correlation id for the
	// underlying condition — e.g. what ties a "firing" alert to its later
	// "resolved" alert for the same condition. Optional: grouping is skipped
	// entirely for a row where this is empty (nothing to group against, and
	// nothing for a later alert to group onto this one by).
	UniqueIdentifier string `json:"uniqueIdentifier,omitempty"`
	// Service and MetricName are recorded on the alert-incident-mapping row
	// this alert ends up attached to, for later operator visibility. They
	// play no role in the grouping lookup itself — Source+UniqueIdentifier
	// is the sole match key.
	Service    string `json:"service,omitempty"`
	MetricName string `json:"metricName,omitempty"`
	// AlertStatus is this alert's derived FIRING/RESOLVED state — see
	// internal/handler.deriveAlertStatus's doc comment for how it's derived
	// from AlertRequest.Severity (the inbound AlertRequest carries no
	// explicit status/state field of its own today).
	AlertStatus string `json:"alertStatus"`
}
