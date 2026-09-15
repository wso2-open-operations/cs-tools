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

package eval

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/spec"
)

// Evaluate reports whether a condition holds against ctx.
//
// An error is a RUN FAILURE, not a false. Silently treating an unresolvable
// reference as "condition not met" is how automation stops firing without
// anyone noticing — the failure must be visible.
func Evaluate(c spec.Condition, ctx Context) (bool, error) {
	if !c.IsLeaf() {
		return evaluateGroup(c, ctx)
	}
	return evaluateLeaf(c, ctx)
}

func evaluateGroup(c spec.Condition, ctx Context) (bool, error) {
	switch {
	case len(c.All) > 0:
		for _, sub := range c.All {
			ok, err := Evaluate(sub, ctx)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
		return true, nil

	case len(c.Any) > 0:
		// ServiceNow's ^NQ ("new query") is an OR across whole condition
		// groups; it arrives here as Any containing several All groups.
		for _, sub := range c.Any {
			ok, err := Evaluate(sub, ctx)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil

	case c.Not != nil:
		ok, err := Evaluate(*c.Not, ctx)
		if err != nil {
			return false, err
		}
		return !ok, nil
	}
	return false, fmt.Errorf("eval: empty condition group")
}

func evaluateLeaf(c spec.Condition, ctx Context) (bool, error) {
	// The "changed" family reads the event's before/after pairs rather than
	// the field's current value, so it resolves differently.
	switch c.Op {
	case spec.OpChanged, spec.OpChangedTo, spec.OpChangedFrom:
		return evaluateChange(c, ctx)
	}

	// exists / not_exists are the only operators for which an unresolved
	// reference is a legitimate answer rather than a failure — asking "is
	// this present" must not fail when it is absent.
	switch c.Op {
	case spec.OpExists, spec.OpNotExists:
		v, err := ctx.Resolve(c.Field)
		present := err == nil && !isEmpty(v)
		if c.Op == spec.OpExists {
			return present, nil
		}
		return !present, nil
	}

	left, err := ctx.Resolve(c.Field)
	if err != nil {
		return false, err
	}

	switch c.Op {
	case spec.OpEq:
		return looseEqual(left, c.Value), nil
	case spec.OpNeq:
		return !looseEqual(left, c.Value), nil

	case spec.OpLt, spec.OpLte, spec.OpGt, spec.OpGte:
		return compareNumeric(c.Op, left, c.Value, c.Field)

	case spec.OpIn:
		in, err := isMember(left, c.Value, c.Field)
		return in, err
	case spec.OpNotIn:
		in, err := isMember(left, c.Value, c.Field)
		return !in, err

	case spec.OpEndsWith, spec.OpStartsWith, spec.OpContains, spec.OpNotContains:
		return compareString(c.Op, left, c.Value, c.Field)

	case spec.OpMatches:
		pattern, ok := c.Value.(string)
		if !ok {
			return false, fmt.Errorf("eval: matches on %q needs a string pattern, got %T", c.Field, c.Value)
		}
		// Anchored, so a pattern cannot accidentally match a substring.
		re, err := regexp.Compile("^(?:" + pattern + ")$")
		if err != nil {
			return false, fmt.Errorf("eval: matches on %q: bad pattern: %w", c.Field, err)
		}
		return re.MatchString(toString(left)), nil
	}

	return false, fmt.Errorf("eval: unhandled operator %q on %q", c.Op, c.Field)
}

// evaluateChange implements changed / changed_to / changed_from over
// $.event.changes.<field>.{from,to}. The field is named without the
// "$.event.changes." prefix in a condition — "state", not the full path — so
// authored specs stay readable.
func evaluateChange(c spec.Condition, ctx Context) (bool, error) {
	field := strings.TrimPrefix(c.Field, "$.event.changes.")
	field = strings.TrimPrefix(field, "$.event.snapshot.")
	field = strings.TrimPrefix(field, "$.event.")

	changes, err := ctx.Resolve("$.event.changes")
	if err != nil {
		// No changes block at all means nothing changed — a legitimate false,
		// not a failure. A create event has no before/after pairs.
		return false, nil
	}
	m, ok := changes.(map[string]any)
	if !ok {
		return false, fmt.Errorf("eval: $.event.changes is %T, want an object", changes)
	}
	raw, present := m[field]
	if !present {
		return false, nil
	}
	pair, ok := raw.(map[string]any)
	if !ok {
		return false, fmt.Errorf("eval: $.event.changes.%s is %T, want {from,to}", field, raw)
	}

	switch c.Op {
	case spec.OpChanged:
		return true, nil
	case spec.OpChangedTo:
		return looseEqual(pair["to"], c.Value), nil
	case spec.OpChangedFrom:
		return looseEqual(pair["from"], c.Value), nil
	}
	return false, fmt.Errorf("eval: %q is not a change operator", c.Op)
}
