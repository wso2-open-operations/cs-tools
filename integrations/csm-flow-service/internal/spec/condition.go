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

// Operator is a comparison in a condition. Every one below was observed in a
// real ServiceNow encoded query on the WSO2 instance — see
// docs/servicenow-discovery/RESULTS.md §26. ServiceNow's spelling is noted
// against each so the porting translator has an unambiguous mapping.
type Operator string

const (
	OpEq          Operator = "eq"  // =
	OpNeq         Operator = "neq" // !=
	OpLt          Operator = "lt"  // <
	OpLte         Operator = "lte"
	OpGt          Operator = "gt"
	OpGte         Operator = "gte"
	OpIn          Operator = "in" // IN
	OpNotIn       Operator = "nin"
	OpExists      Operator = "exists"       // ISNOTEMPTY
	OpNotExists   Operator = "not_exists"   // ISEMPTY
	OpChanged     Operator = "changed"      // VALCHANGES
	OpChangedTo   Operator = "changed_to"   // CHANGESTO
	OpChangedFrom Operator = "changed_from" // CHANGESFROM

	// The three the original design lacked. All three appear in live triggers:
	//   account.u_owner.emailENDSWITHwso2.com
	//   recipientsLIKEnissaneupic-support@wso2.com
	//   sys_created_byNOT LIKE@wso2.com
	OpEndsWith    Operator = "ends_with"    // ENDSWITH
	OpStartsWith  Operator = "starts_with"  // STARTSWITH
	OpContains    Operator = "contains"     // LIKE
	OpNotContains Operator = "not_contains" // NOT LIKE

	// OpMatches is anchored, compiled at publish, and length-capped. Kept from
	// the original design; no embedded scripting (P4).
	OpMatches Operator = "matches"
)

// Condition is a boolean expression over the run context. It is a tree of
// groups (All/Any/Not) with comparisons at the leaves.
//
// ServiceNow's `^NQ` ("new query") is a top-level OR across whole condition
// groups — expressed here as Any containing several All groups. The porting
// translator must treat ^NQ as a group boundary, not as another ^OR.
type Condition struct {
	// Exactly one of these is set. Validate enforces it.
	All []Condition `json:"all,omitempty"`
	Any []Condition `json:"any,omitempty"`
	Not *Condition  `json:"not,omitempty"`

	// Leaf comparison.
	Field string   `json:"field,omitempty"`
	Op    Operator `json:"op,omitempty"`
	Value any      `json:"value,omitempty"`
}

// IsLeaf reports whether this is a comparison rather than a group.
func (c Condition) IsLeaf() bool {
	return len(c.All) == 0 && len(c.Any) == 0 && c.Not == nil
}

// validOperators is the closed set. An unknown operator is a publish-time
// error, never a runtime surprise: ServiceNow will happily save a malformed
// condition — `=3b8b43…` failed 17 times in production because nothing
// rejected it at authoring time. This engine rejects at publish.
var validOperators = map[Operator]bool{
	OpEq: true, OpNeq: true, OpLt: true, OpLte: true, OpGt: true, OpGte: true,
	OpIn: true, OpNotIn: true, OpExists: true, OpNotExists: true,
	OpChanged: true, OpChangedTo: true, OpChangedFrom: true,
	OpEndsWith: true, OpStartsWith: true, OpContains: true, OpNotContains: true,
	OpMatches: true,
}

// unaryOperators take no Value.
var unaryOperators = map[Operator]bool{
	OpExists: true, OpNotExists: true, OpChanged: true,
}
