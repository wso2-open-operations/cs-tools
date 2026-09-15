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
	"strconv"
	"strings"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/spec"
)

// isEmpty mirrors ServiceNow's ISEMPTY, which treats an absent field, an
// empty string and an empty collection alike. Matching that is deliberate:
// ported conditions must behave the same or the port is not faithful.
func isEmpty(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(t) == ""
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	default:
		return false
	}
}

// toString renders a value for string comparison. Numbers use the shortest
// representation that round-trips, so 3 does not become "3.000000" —
// conditions are authored against ServiceNow values, which arrive as JSON
// numbers and are usually integers.
func toString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// toNumber coerces to float64, accepting numeric strings.
//
// ServiceNow choice values arrive as strings far more often than not —
// `priority=10`, `stateCHANGESTO1006`, `u_stageCHANGESTO50 - Closed Won`. A
// strictly-typed comparison would make almost every ported condition fail, so
// numeric coercion is a requirement, not a convenience.
func toNumber(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

// looseEqual compares by value with the same coercions ServiceNow applies:
// numbers equal their string spelling, booleans equal "true"/"false".
//
// Strict Go equality would break nearly every ported condition, because a
// spec authored as `{"op":"eq","value":10}` is compared against a snapshot
// field that arrived from JSON as the string "10".
func looseEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if isEmpty(a) && isEmpty(b) {
		return true
	}
	// Prefer a numeric comparison when both sides look numeric, so 10 == "10".
	if na, ok := toNumber(a); ok {
		if nb, ok := toNumber(b); ok {
			return na == nb
		}
	}
	return toString(a) == toString(b)
}

func compareNumeric(op spec.Operator, left, right any, field string) (bool, error) {
	l, ok := toNumber(left)
	if !ok {
		return false, fmt.Errorf("eval: %q on %q needs a number, got %T", op, field, left)
	}
	r, ok := toNumber(right)
	if !ok {
		return false, fmt.Errorf("eval: %q on %q needs a numeric value, got %T", op, field, right)
	}
	switch op {
	case spec.OpLt:
		return l < r, nil
	case spec.OpLte:
		return l <= r, nil
	case spec.OpGt:
		return l > r, nil
	case spec.OpGte:
		return l >= r, nil
	}
	return false, fmt.Errorf("eval: %q is not a numeric comparison", op)
}

// isMember implements IN / NOT IN. The right side may be a JSON array, or a
// comma-separated string — ServiceNow writes the latter
// (`receive_typeINforward,new`), so both must work.
func isMember(left, right any, field string) (bool, error) {
	var candidates []any

	switch t := right.(type) {
	case []any:
		candidates = t
	case string:
		for _, part := range strings.Split(t, ",") {
			candidates = append(candidates, strings.TrimSpace(part))
		}
	default:
		return false, fmt.Errorf("eval: in/nin on %q needs an array or comma-separated string, got %T", field, right)
	}

	// A collection on the LEFT matches if any element is in the candidate set
	// — a case's watch list against a set of addresses, for instance.
	if items, ok := left.([]any); ok {
		for _, item := range items {
			for _, cand := range candidates {
				if looseEqual(item, cand) {
					return true, nil
				}
			}
		}
		return false, nil
	}

	for _, cand := range candidates {
		if looseEqual(left, cand) {
			return true, nil
		}
	}
	return false, nil
}

// compareString implements ENDSWITH / STARTSWITH / LIKE / NOT LIKE.
//
// Case-INSENSITIVE, matching ServiceNow: `account.u_owner.emailENDSWITHwso2.com`
// must match "Someone@WSO2.com". A case-sensitive port would silently stop
// matching a subset of real records.
func compareString(op spec.Operator, left, right any, field string) (bool, error) {
	needle, ok := right.(string)
	if !ok {
		return false, fmt.Errorf("eval: %q on %q needs a string value, got %T", op, field, right)
	}
	h := strings.ToLower(toString(left))
	n := strings.ToLower(needle)

	switch op {
	case spec.OpEndsWith:
		return strings.HasSuffix(h, n), nil
	case spec.OpStartsWith:
		return strings.HasPrefix(h, n), nil
	case spec.OpContains:
		return strings.Contains(h, n), nil
	case spec.OpNotContains:
		return !strings.Contains(h, n), nil
	}
	return false, fmt.Errorf("eval: %q is not a string comparison", op)
}
