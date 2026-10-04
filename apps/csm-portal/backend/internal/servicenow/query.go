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

package servicenow

import (
	"regexp"
	"strings"
)

// ErrUnsafeQueryValue is returned by SanitizeQueryValue when value cannot be
// safely interpolated into a ServiceNow encoded query (sysparm_query)
// string.
type ErrUnsafeQueryValue struct {
	Value string
}

func (e *ErrUnsafeQueryValue) Error() string {
	return "servicenow: value is not safe to use in a query filter"
}

// SanitizeQueryValue must be called on every caller-supplied string (email,
// search phrase, name, ID, etc.) before it is concatenated into a
// sysparm_query encoded-query string — every table/custom-API query in this
// package is built by plain string concatenation, exactly mirroring the
// Ballerina source it was ported from (e.g. "u_owner.email=" + email +
// "^OR..."), which has no parameterized-query facility to fall back to.
//
// ServiceNow's encoded query language uses "^" as the clause separator
// (AND) and "^OR"/"^NQ" for its other combinators — a caller-supplied value
// containing "^" can inject additional clauses the query author never
// intended (e.g. a crafted email value widening or replacing the intended
// filter). This is the fix for that: reject (rather than attempt to escape,
// since the encoded query language has no escape mechanism) any value
// containing "^", plus control characters, which have no legitimate reason
// to appear in an email/name/search phrase and could otherwise be used to
// smuggle further query syntax.
//
// This only defends values THIS package concatenates into a query string;
// it has no effect on, and must not be applied to, values already
// interpolated safely elsewhere (URL query parameters set via url.Values,
// which net/url encodes correctly on its own).
func SanitizeQueryValue(value string) error {
	if strings.ContainsRune(value, '^') {
		return &ErrUnsafeQueryValue{Value: value}
	}
	if strings.Contains(strings.ToLower(value), "javascript:") {
		return &ErrUnsafeQueryValue{Value: value}
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return &ErrUnsafeQueryValue{Value: value}
		}
	}
	return nil
}

// BuildEncodedQuery joins non-empty clauses with "^" (ServiceNow's AND
// combinator), the same shape every hand-built sysparm_query string in the
// Ballerina source uses. Callers must run every user-supplied fragment
// through SanitizeQueryValue before it reaches a clause passed here.
func BuildEncodedQuery(clauses ...string) string {
	nonEmpty := make([]string, 0, len(clauses))
	for _, c := range clauses {
		if c != "" {
			nonEmpty = append(nonEmpty, c)
		}
	}
	return strings.Join(nonEmpty, "^")
}

var (
	recordNumberRe = regexp.MustCompile(`^[A-Za-z]{2,5}[0-9]{1,20}$`)
	sysIDRe        = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

// ValidateRecordNumber accepts only a record number (2-5 letters then
// digits, e.g. a case number). Used instead of SanitizeQueryValue for values
// that are always record numbers.
func ValidateRecordNumber(value string) error {
	if !recordNumberRe.MatchString(value) {
		return &ErrUnsafeQueryValue{Value: value}
	}
	return nil
}

// ValidateSysID accepts only a 32-character lower-case hexadecimal record id.
func ValidateSysID(value string) error {
	if !sysIDRe.MatchString(value) {
		return &ErrUnsafeQueryValue{Value: value}
	}
	return nil
}

// ValidateRecordNumberOrSysID accepts either shape, for parameters that are
// documented as one but reached with the other on some paths.
func ValidateRecordNumberOrSysID(value string) error {
	if recordNumberRe.MatchString(value) || sysIDRe.MatchString(value) {
		return nil
	}
	return &ErrUnsafeQueryValue{Value: value}
}
