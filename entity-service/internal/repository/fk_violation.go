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
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// fkViolationField names the request field behind each foreign key the case
// and time card writes can violate. The constraints carry Postgres's default
// "<table>_<column>_fkey" names (no migration names them explicitly).
var fkViolationField = map[string]string{
	"work_item_project_id_fkey":                     "projectId",
	"work_item_account_id_fkey":                     "accountId",
	"work_item_deployment_id_fkey":                  "deploymentId",
	"work_item_deployed_product_id_fkey":            "deployedProductId",
	"work_item_product_id_fkey":                     "productId",
	"work_item_product_version_id_fkey":             "productVersionId",
	"work_item_parent_id_fkey":                      "parentId",
	"work_item_assigned_to_id_fkey":                 "assignedEngineerId",
	"work_item_assignment_group_id_fkey":            "assignedTeamId",
	"work_item_contact_user_id_fkey":                "contactId",
	"work_item_opened_by_user_id_fkey":              "openedBy",
	"work_item_acknowledged_by_user_id_fkey":        "acknowledgedBy",
	"work_item_workaround_provided_by_user_id_fkey": "workaroundProvidedBy",
	"work_item_conversation_id_fkey":                "conversationId",
	"case_related_case_id_fkey":                     "relatedCaseId",
	"case_closed_by_user_id_fkey":                   "closedBy",
	"case_attachment_case_id_fkey":                  "caseId",
	"case_attachment_uploaded_by_fkey":              "uploadedBy",
	"case_attachment_updated_by_fkey":               "updatedBy",
	"work_item_watcher_user_id_fkey":                "watchList",
	"time_card_case_id_fkey":                        "caseId",
	"time_card_customer_project_id_fkey":            "projectId",
	"time_card_user_id_fkey":                        "userId",
	"time_card_approved_by_id_fkey":                 "approvedBy",
	"time_card_approver_approver_id_fkey":           "approverIds",
}

// fkViolationError is the client-facing error for a foreign key violation:
// msg, plus the request field when the constraint is a known one. It never
// echoes pgErr.Detail, which quotes the real table, column and key value.
func fkViolationError(pgErr *pgconn.PgError, msg string) *apierror.ValidationError {
	if field := fkViolationField[pgErr.ConstraintName]; field != "" {
		return &apierror.ValidationError{Msg: msg + ": " + field}
	}
	return &apierror.ValidationError{Msg: msg}
}
