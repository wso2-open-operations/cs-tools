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

package service

import (
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestPgProjectUpdateService_ClosureStateRequiresInternalCaller proves a
// registered contact of the project may change the AI assistant settings but
// not the closure-state fields.
func TestPgProjectUpdateService_ClosureStateRequiresInternalCaller(t *testing.T) {
	const projectID = "11111111-1111-1111-1111-111111111111"
	access := stubAccess{scope: AccessScope{ProjectIDs: []string{projectID}, ViewerEmail: "jane.doe@example.com"}}
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	state := "open"

	for name, req := range map[string]domain.ProjectUpdateRequest{
		"endDateClosureState":             {EndDateClosureState: &state},
		"invoiceDueDateClosureState":      {InvoiceDueDateClosureState: &state},
		"complianceViolationClosureState": {ComplianceViolationClosureState: &state},
	} {
		repo := &stubProjectUpdateRepo{}
		_, err := NewProjectUpdateService(repo, testProjectUserRepo(), access).UpdateProject(ctx, projectID, req)
		requireForbidden(t, name, err)
	}

	on := true
	repo := &stubProjectUpdateRepo{}
	if _, err := NewProjectUpdateService(repo, testProjectUserRepo(), access).UpdateProject(ctx, projectID, domain.ProjectUpdateRequest{HasAgent: &on}); err != nil {
		t.Fatalf("hasAgent by a project contact: unexpected error: %v", err)
	}
}
