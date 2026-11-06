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
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/registry"
)

// The refresh is registered with its once-a-day default (or the
// SUB_CRON_SCHEDULES override), and --run-now finds it with no flags of its
// own, which is how it is triggered by hand.
func TestStandardChangeSuggestionsTask(t *testing.T) {
	task := standardChangeSuggestionsTask("https://entity.example.com", "https://auth.example.com/token", "id", "secret", nil, nil, nil)
	if task.Name != "standard_change_suggestions_refresh" || task.Schedule != "0 2 * * *" || task.Handler == nil {
		t.Fatalf("task = %+v", task)
	}
	over := standardChangeSuggestionsTask("https://entity.example.com", "https://auth.example.com/token", "id", "secret", nil,
		map[string]string{"standard_change_suggestions_refresh": "0 6 * * *"}, nil)
	if over.Schedule != "0 6 * * *" {
		t.Errorf("schedule override = %q", over.Schedule)
	}
	handler, _, err := resolveRunNow([]registry.Task{task}, runNowArgs{Task: task.Name})
	if err != nil || handler == nil {
		t.Fatalf("--run-now: %v", err)
	}
	if _, _, err := resolveRunNow([]registry.Task{task}, runNowArgs{Task: task.Name, Given: []string{"--dry-run"}}); err == nil {
		t.Error("--dry-run was accepted; the refresh has no dry run")
	}
}
