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

// Package spec defines the flow specification: the tree an author edits and
// the engine compiles. It is pure data plus validation — no I/O, no execution.
//
// The node vocabulary is not invented. It comes from counting what ServiceNow
// actually uses across 209 Flow Designer flows (63 distinct action types) and
// six live legacy workflows (40 activity types) — see
// docs/servicenow-discovery/RESULTS.md. Anything absent from that evidence is
// absent here.
package spec

// NodeType is the discriminator on Node. Every value maps to something
// observed in the ServiceNow estate.
type NodeType string

const (
	// --- data ---

	// NodeLookup fetches records into run scope. The single most-used action
	// in the estate (89 uses), and one the original design omitted entirely:
	// it assumed a flow reacts to an event with its snapshot already in hand,
	// when in reality flows constantly query OTHER records.
	NodeLookup NodeType = "lookup"
	// NodeSetVars writes to run scope. ServiceNow "Set Values" (1297 uses in
	// the legacy engine alone).
	NodeSetVars NodeType = "set_vars"

	// --- mutation ---

	NodePatchEntity  NodeType = "patch_entity"  // Update Record (76)
	NodeCreateEntity NodeType = "create_entity" // Create Record / Create Case (54)
	NodeDeleteEntity NodeType = "delete_entity" // Delete Record / Attachment (6)

	// --- output ---

	NodeNotify   NodeType = "notify"    // Send Email (60)
	NodeLog      NodeType = "log"       // Log (13)
	NodeCallHTTP NodeType = "call_http" // outbound, allow-listed hosts only

	// --- control flow ---
	//
	// The original design modelled a flow as a FLAT list of actions. That
	// cannot express what the estate contains: WSO2.Notify On Call Members V3
	// is 12 actions to 28 logic steps, and the Twilio P1 flows are 1 action to
	// 16. Control flow is the majority of real flows, not an embellishment.

	NodeIf       NodeType = "if"       // If / Else If / Else (1241)
	NodeForEach  NodeType = "for_each" // For Each
	NodeUntil    NodeType = "until"    // Do the following until
	NodeParallel NodeType = "parallel" // Parallel Flow Launcher / Branch / Join

	// --- time ---

	// NodeWait is the MOST-USED primitive in the whole estate: "Wait for
	// condition" appears 1686 times in the legacy engine, ahead of Set Values,
	// Approval Action and If. Durable waits are how this automation is
	// written, not a late-phase feature.
	NodeWait        NodeType = "wait"
	NodeStartTimer  NodeType = "start_timer" // Timer (71)
	NodeCancelTimer NodeType = "cancel_timer"

	// --- human ---

	NodeRequestApproval NodeType = "request_approval" // Approval Action (1295)

	// --- composition ---

	NodeRunFlow NodeType = "run_flow" // Workflow / subflow call (144)
)

// Node is one step in the flow tree. Container types (if, for_each, until,
// parallel) hold Children; everything else is a leaf.
//
// ID must be stable and unique within a version: it is half of the
// (event_id, node_id) idempotency key, and it survives compilation into the
// instruction list. Renaming one is a semantic change, not a cosmetic one —
// which is why published versions are immutable.
type Node struct {
	ID   string   `json:"id"`
	Type NodeType `json:"type"`

	// Children are the body of a container node. Nil for leaves.
	Children []Node `json:"children,omitempty"`
	// Else is the alternate branch of an `if`. Nil otherwise.
	Else []Node `json:"else,omitempty"`

	// Condition gates an `if`, and is the cancel test on a `wait`.
	Condition *Condition `json:"condition,omitempty"`

	// Over is the collection a for_each iterates, as a reference expression
	// (e.g. "$.vars.watchers"). As names the per-iteration binding, read back
	// through "$.loop.<As>".
	Over string `json:"over,omitempty"`
	As   string `json:"as,omitempty"`

	// Params carries type-specific fields — entity, template, channel, to,
	// duration, fields. Kept generic so adding an action is one registry entry
	// rather than a change to this struct.
	Params map[string]any `json:"params,omitempty"`

	// UI is presentation metadata for the visual builder: position on the
	// canvas, collapsed state. The engine ignores it entirely. It lives in the
	// spec so the builder needs no second store to keep in sync.
	UI map[string]any `json:"ui,omitempty"`
}

// IsContainer reports whether this node type holds children.
func (n Node) IsContainer() bool {
	switch n.Type {
	case NodeIf, NodeForEach, NodeUntil, NodeParallel, NodeWait:
		return true
	default:
		return false
	}
}
