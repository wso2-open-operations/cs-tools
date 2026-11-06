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

package repository

import (
	"errors"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// TestTemplateUsableRefusal: which refusal a templated change request gets, in
// which order, and that only one already raised is told to cancel itself.
func TestTemplateUsableRefusal(t *testing.T) {
	const v1, v2 = "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"
	cases := []struct {
		name, state, current string
		existing             bool
		want                 string // "" = usable
		notWant              string
	}{
		{"active, current", "ACTIVE", v1, true, "", ""},
		{"suspended", "SUSPENDED", v1, true, `"T" is suspended`, ""},
		{"retired, existing", "RETIRED", v1, true, "has been retired, and a retired template is never used again; cancel this change request, and raise a Normal change", ""},
		{"retired, on create", "RETIRED", v1, false, `"T" has been retired; raise a Normal change instead`, "cancel this change request"},
		{"revised, existing", "ACTIVE", v2, true, "has been revised since; cancel this change request and raise a new one from the current version", ""},
		{"revised, on create", "ACTIVE", v2, false, "has been revised since it was loaded; reload it", "cancel this change request"},
		// Resuming would not unblock it: the revision is what is left, so it is named.
		{"suspended and revised", "SUSPENDED", v2, true, "has been revised since", "suspended"},
		{"retired and revised", "RETIRED", v2, true, "has been retired", "revised"},
		{"unreadable", "", "", true, "could not be found", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := templateUsableRefusal(c.state, c.current, v1, "T", "approval cannot be requested", c.existing)
			if c.want == "" {
				if err != nil {
					t.Fatalf("want usable, got %v", err)
				}
				return
			}
			var ce *apierror.ConflictError
			if !errors.As(err, &ce) || ce.Code != apierror.CodeStandardChangeTemplateUnavailable {
				t.Fatalf("want a 409 standard_change_template_unavailable, got %#v", err)
			}
			if !strings.HasPrefix(ce.Msg, "approval cannot be requested: ") || !strings.Contains(ce.Msg, c.want) {
				t.Fatalf("message = %q, want it to contain %q", ce.Msg, c.want)
			}
			if c.notWant != "" && strings.Contains(ce.Msg, c.notWant) {
				t.Fatalf("message = %q, must not contain %q", ce.Msg, c.notWant)
			}
		})
	}
}
