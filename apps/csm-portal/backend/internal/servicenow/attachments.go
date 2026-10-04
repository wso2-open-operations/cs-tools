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

package servicenow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// ErrAttachmentNotFound is returned by RequireCaseAttachment when
// attachmentSysID doesn't exist, or exists but isn't attached to a case --
// deliberately the same error (and the same 404 the caller maps it to)
// either way, so a caller can't use the response to distinguish "wrong id"
// from "that id belongs to some other ServiceNow table" and enumerate
// sys_ids across the instance.
var ErrAttachmentNotFound = &notFoundError{"no case attachment found for that id"}

type snAttachmentTableRef struct {
	TableName string `json:"table_name"`
}

type snAttachmentTableRefList struct {
	Result []snAttachmentTableRef `json:"result"`
}

// RequireCaseAttachment confirms attachmentSysID is attached to a record in
// the case table (sn_customerservice_case) before the caller downloads it.
// Without this, DownloadAttachment's only input is the attachment's own
// sys_id with no scoping at all: any caller holding the blanket download
// permission could pull any attachment on the instance -- case, incident,
// HR record, anything -- just by guessing/enumerating sys_ids. This has no
// per-case ownership check beyond "is it a case attachment at all", matching
// this app's existing flat SPL access model where any authorized caller can
// already open any case via the case list/search.
func (c *Client) RequireCaseAttachment(ctx context.Context, attachmentSysID string) error {
	if err := ValidateSysID(attachmentSysID); err != nil {
		return err
	}
	raw, err := c.TableQuery(ctx, "sys_attachment", url.Values{
		"sysparm_query":  {"sys_id=" + attachmentSysID},
		"sysparm_fields": {"table_name"},
		"sysparm_limit":  {"1"},
	})
	if err != nil {
		return err
	}
	var data snAttachmentTableRefList
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("servicenow: decode attachment table_name response: %w", err)
	}
	if len(data.Result) == 0 || data.Result[0].TableName != "sn_customerservice_case" {
		return ErrAttachmentNotFound
	}
	return nil
}

// DownloadAttachment retrieves an attachment's raw content by its
// ServiceNow sys_id — ported from Ballerina downloadAttachment
// (modules/operations/operations.bal). Returns the raw bytes together with
// the upstream Content-Type and Content-Disposition headers; the caller
// (handler) is responsible for deciding which of those to trust before
// writing them to the HTTP response (see internal/handler's existing
// GetCaseAttachmentContent for this backend's established
// content-type-allowlist convention). Authorization (downloadAttachmentGroups,
// and confirming the attachment is actually a case attachment via
// RequireCaseAttachment) is enforced by the caller, not here — this method
// has no access to the caller's group membership and, on its own, no way to
// tell a case attachment from any other ServiceNow attachment.
func (c *Client) DownloadAttachment(ctx context.Context, attachmentSysID string) (body []byte, contentType string, contentDisposition string, err error) {
	return c.GetBinary(ctx, "/api/now/attachment/"+url.PathEscape(attachmentSysID)+"/file", nil)
}
