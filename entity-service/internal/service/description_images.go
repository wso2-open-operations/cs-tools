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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
)

// callerEmailBestEffort is the email in the caller's x-user-id-token, or "" when
// there is none (a machine caller) or it cannot be read.
func callerEmailBestEffort(ctx context.Context) string {
	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return ""
	}
	email, err := emailFromJWT(token)
	if err != nil {
		return ""
	}
	return email
}

// validateDescriptionImages is what every create path that takes a rich-text
// description calls BEFORE it creates anything. It returns the author's email,
// to hand to linkDescriptionImages afterwards. A description with no image
// reference and no blob: source costs one regexp and nothing else; a nil linker
// is a no-op.
//
// A "data:image" base64 image is not refused, but for a user's content it counts
// against the same limits (ValidateEmbeddedImages): other callers still send their
// images inline, and this service stores no file for them.
func validateDescriptionImages(ctx context.Context, l InlineImageLinker, projectID, html string) (string, error) {
	if l == nil {
		return "", nil
	}
	email := callerEmailBestEffort(ctx)
	// Images embedded as base64 are held to the same limits for a user's content.
	if err := l.ValidateEmbeddedImages(ctx, email, html); err != nil {
		return email, err
	}
	if !mayHaveInlineImages(html) {
		return "", nil
	}
	return email, l.ValidateDescriptionImages(ctx, projectID, email, html)
}

// linkDescriptionImages links the drafts a just-created record's description
// shows to it. Best effort and free for a description with no image.
func linkDescriptionImages(ctx context.Context, l InlineImageLinker, workItemID, projectID, email, html string) {
	if l == nil || workItemID == "" || !mayHaveInlineImages(html) {
		return
	}
	l.LinkDescriptionImages(ctx, workItemID, projectID, email, html)
}
