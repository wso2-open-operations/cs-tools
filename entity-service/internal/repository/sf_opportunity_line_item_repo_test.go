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

package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The standalone upsert takes the parent opportunity's lock (the one the
// derived set replace holds) and reuses the derived path's statements, then
// re-derives the linked projects' onboarding before the ledger.
func TestWriteSfOpportunityLineItem_OpportunityLockAndSharedSQL(t *testing.T) {
	q := &updateTagQuerier{update: updateSfOpportunityProductQuery, rows: 0}
	created, err := writeSfOpportunityLineItem(context.Background(), q, "006OPP", "opp-row",
		domain.SalesforceOpportunityLineItemUpsert{LineItemSfID: "00kLI"}, partnerState())
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	want := []string{"pg_advisory_xact_lock", updateSfOpportunityProductQuery, insertSfOpportunityProductQuery, deriveOnboardingForOpportunityQuery, "salesforce_ingest_state"}
	if len(q.statements) != len(want) {
		t.Fatalf("statements = %d, want %d", len(q.statements), len(want))
	}
	for i, w := range want {
		if !strings.Contains(q.statements[i], w) {
			t.Errorf("statement %d = %.50q", i, q.statements[i])
		}
	}
	if q.args[0][0] != sfOpportunityLockKey("006OPP") || q.args[1][1] != "opp-row" || q.args[1][16] != "00kLI" {
		t.Errorf("lock %v, opportunity %v, line item %v", q.args[0][0], q.args[1][1], q.args[1][16])
	}
}
