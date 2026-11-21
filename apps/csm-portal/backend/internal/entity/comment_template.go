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

package entity

import (
	"context"
	"net/http"
	"net/url"
)

// Saved case comment templates (entity-service migration 0293). The caller's
// own templates live under /users/me/case-comment-templates, keyed to the
// forwarded x-user-id-token's user; the shared ones under
// /case-comment-templates. Bodies are passed through as JSON: entity-service
// validates them and owns the response shape.

func personalCommentTemplatePath(id string) string {
	return "/users/me/case-comment-templates/" + url.PathEscape(id)
}

func sharedCommentTemplatePath(id string) string {
	return "/case-comment-templates/" + url.PathEscape(id)
}

// ListCaseCommentTemplates calls GET /users/me/case-comment-templates: the
// caller's personal templates, then the shared ones.
func (c *CustomerEntityClient) ListCaseCommentTemplates(ctx context.Context) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/users/me/case-comment-templates", nil)
}

// CreatePersonalCaseCommentTemplate calls POST /users/me/case-comment-templates.
func (c *CustomerEntityClient) CreatePersonalCaseCommentTemplate(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/users/me/case-comment-templates", body)
}

// UpdatePersonalCaseCommentTemplate calls PATCH /users/me/case-comment-templates/{id}.
// Another user's template is a 404, the same as a missing one.
func (c *CustomerEntityClient) UpdatePersonalCaseCommentTemplate(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, personalCommentTemplatePath(id), body)
}

// DeletePersonalCaseCommentTemplate calls DELETE /users/me/case-comment-templates/{id} (204).
func (c *CustomerEntityClient) DeletePersonalCaseCommentTemplate(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, personalCommentTemplatePath(id), nil)
	return err
}

// CreateSharedCaseCommentTemplate calls POST /case-comment-templates.
func (c *CustomerEntityClient) CreateSharedCaseCommentTemplate(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/case-comment-templates", body)
}

// UpdateSharedCaseCommentTemplate calls PATCH /case-comment-templates/{id}.
func (c *CustomerEntityClient) UpdateSharedCaseCommentTemplate(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, sharedCommentTemplatePath(id), body)
}

// DeleteSharedCaseCommentTemplate calls DELETE /case-comment-templates/{id} (204).
func (c *CustomerEntityClient) DeleteSharedCaseCommentTemplate(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, sharedCommentTemplatePath(id), nil)
	return err
}
