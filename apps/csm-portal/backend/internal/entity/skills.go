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

// GetSkillLevels calls GET /skills/levels on the entity service: every level
// scale with its levels. Response is returned as raw JSON.
func (c *CustomerEntityClient) GetSkillLevels(ctx context.Context) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/skills/levels", nil)
}

// GetSkillCategories calls GET /skills/categories: the skill category tree, flat.
func (c *CustomerEntityClient) GetSkillCategories(ctx context.Context) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/skills/categories", nil)
}

// SearchSkills calls POST /skills/search: the skill catalogue, paged and filtered.
func (c *CustomerEntityClient) SearchSkills(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/skills/search", body)
}

// GetSkill calls GET /skills/{id}.
func (c *CustomerEntityClient) GetSkill(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/skills/"+url.PathEscape(id), nil)
}

// CreateSkill calls POST /skills. Admin only; the entity service decides.
func (c *CustomerEntityClient) CreateSkill(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/skills", body)
}

// UpdateSkill calls PATCH /skills/{id}. Admin only; the entity service decides.
func (c *CustomerEntityClient) UpdateSkill(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, "/skills/"+url.PathEscape(id), body)
}

// GetMySkills calls GET /skills/me. "My" is the user the forwarded token names.
func (c *CustomerEntityClient) GetMySkills(ctx context.Context) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/skills/me", nil)
}

// AddMySkill calls POST /skills/me.
func (c *CustomerEntityClient) AddMySkill(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/skills/me", body)
}

// SetMySkillLevel calls PUT /skills/me/{skillId}.
func (c *CustomerEntityClient) SetMySkillLevel(ctx context.Context, skillID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPut, "/skills/me/"+url.PathEscape(skillID), body)
}

// RemoveMySkill calls DELETE /skills/me/{skillId}.
func (c *CustomerEntityClient) RemoveMySkill(ctx context.Context, skillID string) ([]byte, error) {
	return c.do(ctx, http.MethodDelete, "/skills/me/"+url.PathEscape(skillID), nil)
}

// GetMySkillHistory calls GET /skills/me/history. An empty skillID, limit or
// offset is left off the query, so the entity service applies its defaults.
func (c *CustomerEntityClient) GetMySkillHistory(ctx context.Context, skillID, limit, offset string) ([]byte, error) {
	q := url.Values{}
	for name, v := range map[string]string{"skillId": skillID, "limit": limit, "offset": offset} {
		if v != "" {
			q.Set(name, v)
		}
	}
	path := "/skills/me/history"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	return c.do(ctx, http.MethodGet, path, nil)
}

// SearchUserSkills calls POST /skills/users/search: everybody's skills, for the
// skill manager. The entity service decides who may.
func (c *CustomerEntityClient) SearchUserSkills(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/skills/users/search", body)
}
