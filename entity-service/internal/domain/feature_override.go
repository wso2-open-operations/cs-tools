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

package domain

import "time"

// Stored feature-visibility states. A row is never deleted: available is
// enabled, and preview is what the portal used to call work in progress.
const (
	FeatureEnabled = "enabled"
	FeaturePreview = "preview"
)

// FeatureOverride is one row of feature_override.
type FeatureOverride struct {
	NavNodeID string    `json:"navNodeId"`
	State     string    `json:"state"`
	UpdatedOn time.Time `json:"updatedOn"`
	UpdatedBy string    `json:"updatedBy"`
}

// FeatureOverrideList is the body of GET /feature-overrides.
type FeatureOverrideList struct {
	Overrides []FeatureOverride `json:"overrides"`
	SavedOnce bool              `json:"savedOnce"`
}

// SaveFeatureOverrideRequest is the body of PUT /feature-overrides.
// MarkSaved records the first successful save. It is set on the last write
// of that save so a stopped save does not hide config.js.
type SaveFeatureOverrideRequest struct {
	NavNodeID string `json:"navNodeId"`
	State     string `json:"state"`
	MarkSaved bool   `json:"markSaved"`
}

// IsFeatureState reports whether state is one this table stores.
func IsFeatureState(state string) bool {
	return state == FeatureEnabled || state == FeaturePreview
}
