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

package apierror

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestSummary_StripsUpstreamBodyButKeepsContext(t *testing.T) {
	inner := &Error{StatusCode: 422, Body: `{"error":"invalid recipient jane.doe@example.com"}`}
	err := fmt.Errorf("dispatch: send invitation for membership m-1: %w", fmt.Errorf("notifications: %w", inner))

	got := Summary(err)
	if strings.Contains(got, "example.com") {
		t.Fatalf("Summary() = %q, still carries the upstream body", got)
	}
	if want := "dispatch: send invitation for membership m-1: notifications: upstream returned 422"; got != want {
		t.Errorf("Summary() = %q, want %q", got, want)
	}
}

func TestSummary_PlainErrorIsUnchanged(t *testing.T) {
	err := errors.New("decode envelope: unexpected end of JSON input")
	if got := Summary(err); got != err.Error() {
		t.Errorf("Summary() = %q, want %q", got, err.Error())
	}
	if got := Summary(nil); got != "" {
		t.Errorf("Summary(nil) = %q, want empty", got)
	}
}

func TestSummary_BodylessUpstreamErrorIsUnchanged(t *testing.T) {
	err := fmt.Errorf("scim: %w", &Error{StatusCode: 503})
	if got := Summary(err); got != "scim: upstream returned 503" {
		t.Errorf("Summary() = %q", got)
	}
}

// rewordingError wraps without embedding the inner message verbatim, so a
// textual replacement cannot find it.
type rewordingError struct{ inner error }

func (e rewordingError) Error() string {
	return "problem (" + strings.ReplaceAll(e.inner.Error(), "upstream returned", "status") + ")"
}
func (e rewordingError) Unwrap() error { return e.inner }

func TestSummary_FallsBackToStatusWhenBodyCannotBeCutOut(t *testing.T) {
	err := rewordingError{&Error{StatusCode: 500, Body: "secret"}}
	if got := Summary(err); got != "upstream returned 500" {
		t.Errorf("Summary() = %q, want the status-only form", got)
	}
}
