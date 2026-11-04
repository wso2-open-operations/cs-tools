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

package main

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/registry"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/stalecases"
)

// The stale cases report built the way main builds it, run with an override:
// the mail goes to the override list (and --cc) only, whether or not the task
// has a "to" of its own, and a task with none still sends.
func TestStaleCasesRunNowOverride(t *testing.T) {
	override := []string{"jane.doe@example.com"}
	overrideCc := []string{"john.roe@example.com"}
	for name, realTo := range map[string][]string{
		"task has no to":   nil,
		"task has its own": {"team@example.com"},
	} {
		t.Run(name, func(t *testing.T) {
			raw := &recordingSender{}
			hook := reportRunNow(notify.ForReport(raw, nil), raw,
				func(email notify.Sender, rec []string) func(context.Context) error {
					return stalecases.SendReport(fakeStale{}, email, 30, orRecipients(realTo, rec), []string{"standing-cc@example.com"}, true)
				})
			h, _, err := hook(registry.RunNowOptions{Recipients: override, Cc: overrideCc})
			if err != nil {
				t.Fatal(err)
			}
			if err := h(context.Background()); err != nil {
				t.Fatal(err)
			}
			if raw.calls != 1 || !reflect.DeepEqual(raw.to, override) || !reflect.DeepEqual(raw.cc, overrideCc) {
				t.Fatalf("calls=%d to=%v cc=%v", raw.calls, raw.to, raw.cc)
			}
			if !strings.HasPrefix(raw.subject, "[Test copy] [Action-Required][Report] Cases open for more than 30 days") {
				t.Errorf("subject = %q", raw.subject)
			}
		})
	}
}
