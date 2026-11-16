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

package service

import (
	"context"
	"log/slog"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
)

// reportRequiredMessage is what a security report creator sees without a file.
const reportRequiredMessage = "A security report needs at least one report file. Upload it first, then create the report."

// validateReportDrafts is what a create path calls BEFORE it creates anything. It
// checks the files named in req.AttachmentIDs and, for a security report created by
// a user, that there is at least one. It returns the creator's email, to hand to
// linkReportDrafts afterwards. A nil linker with no ids costs nothing.
//
// A machine caller (no user token, so no email) is not held to the minimum: it has no
// "uploaded by you" to prove, and the integrations that create records this way
// predate the rule.
func validateReportDrafts(ctx context.Context, linker any, req domain.CreateCaseRequest) (string, error) {
	email := callerEmailBestEffort(ctx)
	needed := req.Type == "security_report_analysis" && email != ""
	if len(req.AttachmentIDs) == 0 {
		if needed {
			return email, &apierror.ValidationError{Msg: reportRequiredMessage}
		}
		return email, nil
	}
	l, ok := linker.(ReportDraftLinker)
	if !ok {
		return email, &apierror.ValidationError{Msg: "attachments cannot be added when creating a record on this deployment"}
	}
	return email, l.ValidateReportDrafts(ctx, req.ProjectID, email, req.AttachmentIDs)
}

// linkReportDrafts links the validated files to the record just created. The record
// exists by now, so a failure cannot undo it: it is logged at ERROR with the ids, for
// someone to link by hand before the draft cleanup removes the files.
func linkReportDrafts(ctx context.Context, linker any, workItemID string, req domain.CreateCaseRequest, email string) {
	if len(req.AttachmentIDs) == 0 || workItemID == "" {
		return
	}
	l, ok := linker.(ReportDraftLinker)
	if !ok {
		return
	}
	if err := l.LinkReportDrafts(ctx, workItemID, req.ProjectID, email, req.AttachmentIDs); err != nil {
		slog.ErrorContext(ctx, "could not link the report files to the new record",
			"workItemId", workItemID, "attachmentIds", req.AttachmentIDs,
			"correlationId", middleware.CorrelationIDFromContext(ctx), "error", err)
	}
}
