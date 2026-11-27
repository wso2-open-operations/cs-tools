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
	"fmt"
	"net/http"
	"net/url"
)

// Alert mute rules are one-time windows that mute SRE alerts matching a set of
// conditions. Every method below is a 1:1 passthrough to entity-service; the
// response is returned as raw JSON. GET /alert-mute-rules/active is deliberately
// not here: only alert-core reads it, through csm-integration-service.

// SearchAlertMuteRules calls POST /alert-mute-rules/search on the entity service.
func (c *CustomerEntityClient) SearchAlertMuteRules(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/alert-mute-rules/search", body)
}

// GetAlertMuteRule calls GET /alert-mute-rules/{id} on the entity service.
func (c *CustomerEntityClient) GetAlertMuteRule(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/alert-mute-rules/%s", url.PathEscape(id)), nil)
}

// CreateAlertMuteRule calls POST /alert-mute-rules on the entity service.
func (c *CustomerEntityClient) CreateAlertMuteRule(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/alert-mute-rules", body)
}

// PatchAlertMuteRule calls PATCH /alert-mute-rules/{id} on the entity service.
func (c *CustomerEntityClient) PatchAlertMuteRule(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/alert-mute-rules/%s", url.PathEscape(id)), body)
}

// ExtendAlertMuteRule calls POST /alert-mute-rules/{id}/extend on the entity service.
func (c *CustomerEntityClient) ExtendAlertMuteRule(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/alert-mute-rules/%s/extend", url.PathEscape(id)), body)
}

// EndAlertMuteRule calls POST /alert-mute-rules/{id}/end on the entity service.
func (c *CustomerEntityClient) EndAlertMuteRule(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/alert-mute-rules/%s/end", url.PathEscape(id)), nil)
}

// DeleteAlertMuteRule calls DELETE /alert-mute-rules/{id} on the entity service.
func (c *CustomerEntityClient) DeleteAlertMuteRule(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/alert-mute-rules/%s", url.PathEscape(id)), nil)
	return err
}
