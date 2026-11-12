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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// SystemDashboard is a dashboard definition entity-service stores as a system
// dashboard (/system-dashboards): shared by every user, keyed by Key, and
// written only by this backend's own service credentials. Definition is the
// dashboard JSON as text.
type SystemDashboard struct {
	ID         string `json:"id"`
	Key        string `json:"systemKey"`
	IsSystem   bool   `json:"isSystem"`
	Name       string `json:"name"`
	Definition string `json:"definition"`
	CreatedOn  string `json:"createdOn"`
	UpdatedOn  string `json:"updatedOn"`
}

type systemDashboardListResponse struct {
	Dashboards []SystemDashboard `json:"dashboards"`
}

// serviceContext is ctx without the caller's x-user-id-token. System dashboards
// belong to no user and entity-service gates them on this backend's own
// credentials alone, so the token of whichever request happens to trigger the
// call (a list is shared across users) is neither needed nor forwarded: an
// expired one would only make the service reject a call that needs no user.
func serviceContext(ctx context.Context) context.Context {
	return WithUserIDToken(ctx, "")
}

func systemDashboardPath(key string) string {
	return "/system-dashboards/" + url.PathEscape(key)
}

// ListSystemDashboards calls GET /system-dashboards, ordered by key. Never nil
// on success. The call carries only this backend's service credentials; the
// caller's user token, if ctx has one, is not forwarded.
func (c *CustomerEntityClient) ListSystemDashboards(ctx context.Context) ([]SystemDashboard, error) {
	raw, err := c.do(serviceContext(ctx), http.MethodGet, "/system-dashboards", nil)
	if err != nil {
		return nil, err
	}
	var resp systemDashboardListResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("entity: decode system dashboard list: %w", err)
	}
	if resp.Dashboards == nil {
		resp.Dashboards = []SystemDashboard{}
	}
	return resp.Dashboards, nil
}

// SaveSystemDashboard calls PUT /system-dashboards/{key}: creates the system
// dashboard, or overwrites it when the key exists.
func (c *CustomerEntityClient) SaveSystemDashboard(ctx context.Context, key, name, definition string) (SystemDashboard, error) {
	body, err := json.Marshal(saveCustomDashboardRequest{Name: name, Definition: definition})
	if err != nil {
		return SystemDashboard{}, fmt.Errorf("entity: encode system dashboard: %w", err)
	}
	raw, err := c.do(serviceContext(ctx), http.MethodPut, systemDashboardPath(key), body)
	if err != nil {
		return SystemDashboard{}, err
	}
	var d SystemDashboard
	if err := json.Unmarshal(raw, &d); err != nil {
		return SystemDashboard{}, fmt.Errorf("entity: decode saved system dashboard: %w", err)
	}
	return d, nil
}
