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

package spec

// TriggerKind is how a flow starts. The original design modelled exactly one
// ("event"). Seven are in live use on the WSO2 instance.
type TriggerKind string

const (
	TriggerRecordCreate         TriggerKind = "record_create"
	TriggerRecordUpdate         TriggerKind = "record_update"
	TriggerRecordCreateOrUpdate TriggerKind = "record_create_or_update"
	TriggerDaily                TriggerKind = "daily"
	TriggerWeekly               TriggerKind = "weekly"
	// TriggerEmail is inbound mail creating or updating a record — a whole
	// intake path, not a variation on an event.
	TriggerEmail TriggerKind = "email"
	// TriggerRunOnce is a one-off migration trigger.
	TriggerRunOnce TriggerKind = "run_once"
	// TriggerSLATask fires on SLA lifecycle. Found late: the two busiest
	// triggered flows on the instance (Default SLA flow at 2761 runs, SLA
	// notification and escalation flow at 1736) both use it.
	TriggerSLATask TriggerKind = "sla_task"
)

// RunTrigger controls RE-firing, and getting it wrong is a correctness bug
// rather than a tuning detail.
//
// ServiceNow distinguishes "run on every update where the condition holds"
// from "run only when the condition NEWLY becomes true". Treating the second
// as the first re-sends a notification on every subsequent update — exactly
// the duplicate-send class of failure this engine exists to remove.
//
// Evaluating RunTriggerUniqueChanges requires durable per-entity state: the
// engine must remember whether the condition already held last time it saw
// this record.
type RunTrigger string

const (
	// RunTriggerEvery fires whenever the condition holds. ServiceNow "every".
	RunTriggerEvery RunTrigger = "every"
	// RunTriggerUniqueChanges fires only on the transition false -> true.
	RunTriggerUniqueChanges RunTrigger = "unique_changes"
	// RunTriggerAlways fires regardless of whether the condition changed.
	RunTriggerAlways RunTrigger = "always"
)

// Trigger is what starts a flow.
type Trigger struct {
	Kind TriggerKind `json:"kind"`
	// EntityType is the record type, for the record_* kinds.
	EntityType string `json:"entityType,omitempty"`
	// RunTrigger defaults to RunTriggerEvery when empty.
	RunTrigger RunTrigger `json:"runTrigger,omitempty"`
	// Cron is required for daily/weekly. A 5- or 6-field expression, matching
	// operations/csm-scheduled-tasks' registry.Task.Schedule — scheduled flows
	// register there as sub-crons rather than getting a second scheduler.
	Cron string `json:"cron,omitempty"`
	// Condition gates the trigger before any node runs.
	Condition *Condition `json:"condition,omitempty"`
}

// Flow is a complete, publishable specification.
type Flow struct {
	Key     string  `json:"key"`
	Trigger Trigger `json:"trigger"`
	Nodes   []Node  `json:"nodes"`
}
