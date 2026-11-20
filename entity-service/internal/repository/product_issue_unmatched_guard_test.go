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
	"context"
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// An unmatched product issue (no case) exists only through backfill, which says
// so with Unmatched. The create path, the manual registration and the shared
// insert refuse an empty case id; none of them reaches the database.
func TestProductIssueRegistration_RefusesAnEmptyCaseID(t *testing.T) {
	ctx := context.Background()
	var ve *apierror.ValidationError

	if _, err := (&productWorkItemRepo{}).RegisterProductIssue(ctx, RegisterParams{CaseID: "  "}); !errors.As(err, &ve) {
		t.Errorf("RegisterProductIssue with no case: %v", err)
	}
	if _, err := (&productIssueRepo{}).RegisterManualProductIssue(ctx, ManualRegisterParams{CaseID: ""}); !errors.As(err, &ve) {
		t.Errorf("RegisterManualProductIssue with no case: %v", err)
	}
	// The shared insert: no case without Unmatched, and Unmatched with a case, are both refused.
	for name, in := range map[string]productIssueRows{
		"no case, not unmatched": {},
		"unmatched with a case":  {CaseID: "c1", Unmatched: true},
	} {
		if _, _, err := insertProductIssueRows(ctx, nil, in); !errors.Is(err, errProductIssueCaseRequired) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := (&productIssueReconcileRepo{}).BackfillAttachCase(ctx, "wi", "", "actor"); !errors.Is(err, errProductIssueCaseRequired) {
		t.Errorf("BackfillAttachCase with no case: %v", err)
	}
}
