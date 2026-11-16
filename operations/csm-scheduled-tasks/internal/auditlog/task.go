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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.
package auditlog

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// Maintainer is the subset of *Client this package's handler depends on,
// declared here so a test can substitute a fake.
type Maintainer interface {
	RunMaintenance(ctx context.Context) (Result, error)
}

// Maintenance returns a registry.Task.Handler that runs the audit log upkeep and
// fails when it found anything a person should look at: a health problem, a
// trigger that was repaired (the history had a gap while it was off) or one
// that is still wrong. The failure names each one, and the engine mails it.
// The upkeep is safe to repeat, so the retry is just the next run; a repaired
// trigger is clean by then and the retry succeeds.
func Maintenance(m Maintainer) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		result, err := m.RunMaintenance(ctx)
		if err != nil {
			return fmt.Errorf("audit log maintenance: %w", err)
		}

		slog.InfoContext(ctx, "audit log maintenance: finished",
			"partitionsCreated", result.PartitionsCreated, "healthProblems", len(result.HealthProblems),
			"repairedTriggers", len(result.RepairedTriggers), "unresolvedTriggers", len(result.UnresolvedTriggers))

		problems := result.problems()
		if len(problems) == 0 {
			return nil
		}
		return fmt.Errorf("audit log maintenance found %d problem(s): %s", len(problems), strings.Join(problems, "; "))
	}
}

// problems lists what the run found, in words fit for the failure mail.
func (r Result) problems() []string {
	problems := append([]string(nil), r.HealthProblems...)
	for _, t := range r.UnresolvedTriggers {
		problems = append(problems, fmt.Sprintf("trigger on %s.%s is still wrong (%s)", t.Schema, t.Table, t.Problem))
	}
	for _, t := range r.RepairedTriggers {
		problems = append(problems, fmt.Sprintf("trigger on %s.%s was repaired (%s); its history had a gap", t.Schema, t.Table, t.Problem))
	}
	return problems
}
