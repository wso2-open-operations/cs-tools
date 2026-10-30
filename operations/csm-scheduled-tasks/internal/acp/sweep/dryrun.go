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

package sweep

import "context"

// DryRunProjectUpdater satisfies projectUpdater without ever calling
// entity-service or sales-entity-service — it silently no-ops instead of
// writing. This is the
// entire mechanism behind ACP_DRY_RUN: cmd/server/main.go injects this instead of the real
// *Writer when dry-run is active, and processProject/Run are
// completely unaware of the distinction — they just call whichever
// projectUpdater they were given.
//
// Deliberately does not log the write it would have made: per user
// direction, the only log output that matters for a dry run is the
// notify.LoggingNotifier "notice" line (the actual email content) — a
// separate "would update project" line describing the raw PATCH body is
// noise once every window produces a real notice log, and stays noise once
// real email sending replaces LoggingNotifier.
type DryRunProjectUpdater struct{}

// UpdateProject always succeeds without doing anything.
func (u *DryRunProjectUpdater) UpdateProject(ctx context.Context, id string, body []byte) ([]byte, error) {
	return []byte(`{}`), nil
}

// SuspendInSalesforce always succeeds without doing anything.
func (u *DryRunProjectUpdater) SuspendInSalesforce(ctx context.Context, sfID string) error {
	return nil
}
