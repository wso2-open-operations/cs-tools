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

	"github.com/adhocore/gronx"
)

func TestProductIssueReconcileTask(t *testing.T) {
	build := func(overrides map[string]string) (name, schedule string, attempts int, hasHandler bool) {
		task := productIssueReconcileTask("https://entity.example.test", "https://auth.example.test/token", "client", "secret",
			[]string{"scope"}, overrides, nil)
		return task.Name, task.Schedule, task.MaxAttempts, task.Handler != nil
	}
	name, schedule, attempts, ok := build(nil)
	if name != "product_issue_reconcile" || schedule != "0 1 * * *" || attempts != acpMaxAttempts || !ok {
		t.Errorf("task = %q %q %d %v", name, schedule, attempts, ok)
	}
	if !gronx.IsValid(schedule) {
		t.Errorf("default schedule %q is not a valid cron expression", schedule)
	}
	// SUB_CRON_SCHEDULES overrides it, as for every sub-cron.
	if _, schedule, _, _ = build(map[string]string{"product_issue_reconcile": "15 2 * * *"}); schedule != "15 2 * * *" {
		t.Errorf("overridden schedule = %q", schedule)
	}
}
