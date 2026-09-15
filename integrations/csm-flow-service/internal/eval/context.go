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

// Package eval evaluates flow conditions against a run context.
//
// It is PURE: no I/O, no clock, no database. Everything a condition needs is
// already in the Context by the time this package sees it — which is what
// makes it exhaustively testable, and why it is the most heavily tested code
// in the service. A subtly wrong operator silently misfires customer-facing
// automation.
//
// I/O lives in the action layer instead: a `lookup` node fetches and deposits
// into Vars, and the evaluator only reads what is already there.
package eval

import (
	"fmt"
	"strings"
)

// Context is the document a condition is evaluated against. Three roots, and
// nothing else — there is no expression language beyond path resolution (P4:
// no embedded scripting).
type Context struct {
	// Event is the triggering event: entityType, entityId, changes, snapshot,
	// actor. Read as "$.event.…".
	Event map[string]any
	// Vars is run scope — whatever `lookup` and `set_vars` deposited. Read as
	// "$.vars.…".
	//
	// Unbounded by construction: a lookup with Max Results=5000 (a real value
	// on the WSO2 instance) puts 5000 rows here, and a run waiting 48 hours
	// holds them the whole time. Cap lookup results at publish and again at
	// execution.
	Vars map[string]any
	// Loop is the current for_each bindings, innermost first. Read as
	// "$.loop.<binding>.…".
	Loop map[string]any
}

// ErrUnresolved is returned when a reference cannot be resolved.
//
// Resolution is deliberately STRICT. An unresolvable path fails the run rather
// than yielding an empty string, because a notification silently addressed to
// "" is worse than a failed run — and the failure table exists precisely so
// this is visible. ServiceNow's own estate shows the alternative: 4,416
// failures reading "Email validation failed: Email has no recipients."
type ErrUnresolved struct {
	Ref    string
	Reason string
}

func (e *ErrUnresolved) Error() string {
	return fmt.Sprintf("eval: cannot resolve %q: %s", e.Ref, e.Reason)
}

// lenSuffix is the one suffix operator. "did the lookup return anything" is
// the most common branch there is, and the alternative was a `count` action
// nobody wants.
const lenSuffix = "|len"

// Resolve looks up a "$."-rooted reference. A literal (anything not starting
// with "$.") is returned unchanged, so conditions can compare against
// constants without a separate syntax.
func (c Context) Resolve(ref string) (any, error) {
	if !strings.HasPrefix(ref, "$.") {
		return ref, nil
	}

	wantLen := strings.HasSuffix(ref, lenSuffix)
	path := strings.TrimSuffix(ref, lenSuffix)

	parts := strings.Split(strings.TrimPrefix(path, "$."), ".")
	if len(parts) == 0 || parts[0] == "" {
		return nil, &ErrUnresolved{Ref: ref, Reason: "empty path after $."}
	}

	var cursor any
	switch parts[0] {
	case "event":
		cursor = c.Event
	case "vars":
		cursor = c.Vars
	case "loop":
		cursor = c.Loop
	default:
		return nil, &ErrUnresolved{
			Ref:    ref,
			Reason: fmt.Sprintf("unknown root %q — must be event, vars or loop", parts[0]),
		}
	}

	for i, key := range parts[1:] {
		m, ok := cursor.(map[string]any)
		if !ok {
			return nil, &ErrUnresolved{
				Ref:    ref,
				Reason: fmt.Sprintf("%q is not an object, cannot read %q from it", strings.Join(parts[:i+1], "."), key),
			}
		}
		v, present := m[key]
		if !present {
			return nil, &ErrUnresolved{
				Ref:    ref,
				Reason: fmt.Sprintf("%q has no key %q", strings.Join(parts[:i+1], "."), key),
			}
		}
		cursor = v
	}

	if wantLen {
		return lengthOf(cursor, ref)
	}
	return cursor, nil
}

// lengthOf implements the |len suffix over the shapes a lookup can produce.
func lengthOf(v any, ref string) (any, error) {
	switch t := v.(type) {
	case nil:
		return 0, nil
	case []any:
		return len(t), nil
	case map[string]any:
		return len(t), nil
	case string:
		return len(t), nil
	default:
		return nil, &ErrUnresolved{
			Ref:    ref,
			Reason: fmt.Sprintf("|len is not defined for %T", v),
		}
	}
}
