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
package domain

// AuditTriggerProblem is one mismatch between audit_config and the audit
// triggers that exist on the tables.
type AuditTriggerProblem struct {
	Schema  string `json:"schema"`
	Table   string `json:"table"`
	Problem string `json:"problem"`
}

// AuditLogMaintenanceResponse is what one run of the audit log upkeep found.
// A run that finds nothing wrong has no health problems, no repaired triggers,
// no unresolved triggers and no step errors.
type AuditLogMaintenanceResponse struct {
	// PartitionsCreated is how many monthly partitions this run created.
	PartitionsCreated int `json:"partitionsCreated"`
	// PartitionsRetired are the names of the monthly partitions this run deleted
	// because they ended at least the retention period ago.
	PartitionsRetired []string `json:"partitionsRetired"`
	// DefaultRowsDeleted is how many rows older than the retention period this
	// run deleted from the catch-all partition.
	DefaultRowsDeleted int64 `json:"defaultRowsDeleted"`
	// HealthProblems are rows in the catch-all partition, or too few future months.
	HealthProblems []string `json:"healthProblems"`
	// RepairedTriggers were missing or disabled and have been put back. The
	// history had a gap while they were off.
	RepairedTriggers []AuditTriggerProblem `json:"repairedTriggers"`
	// UnresolvedTriggers are still wrong after the repair and need a person.
	UnresolvedTriggers []AuditTriggerProblem `json:"unresolvedTriggers"`
	// StepErrors names each step that could not run at all, in words that carry
	// no database detail. The full error is in the service log.
	StepErrors []string `json:"stepErrors"`
}
