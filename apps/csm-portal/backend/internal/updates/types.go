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

package updates

// channel is the fixed update channel value forwarded to the upstream service,
// mirroring the CHANNEL constant in the Ballerina updates module.
const channel = "full"

const updateTypeSecurity = "security"
const updateTypeRegular = "regular"

// ---- upstream (snake-case) types ----
// These mirror the record types defined in modules/updates/types.bal.

type upstreamRecommendedUpdateLevel struct {
	ProductName                   string `json:"product-name"`
	ProductBaseVersion            string `json:"product-base-version"`
	Channel                       string `json:"channel"`
	StartingUpdateLevel           int    `json:"starting-update-level"`
	EndingUpdateLevel             int    `json:"ending-update-level"`
	InstalledUpdatesCount         int    `json:"installed-updates-count"`
	InstalledSecurityUpdatesCount int    `json:"installed-security-updates-count"`
	Timestamp                     int64  `json:"timestamp"`
	RecommendedUpdateLevel        int    `json:"recommended-update-level"`
	AvailableUpdatesCount         int    `json:"available-updates-count"`
	AvailableSecurityUpdatesCount int    `json:"available-security-updates-count"`
}

type upstreamProductUpdateLevel struct {
	ProductName         string               `json:"product-name"`
	ProductUpdateLevels []upstreamUpdateLevel `json:"product-update-levels"`
}

type upstreamUpdateLevel struct {
	ProductBaseVersion string `json:"product-base-version"`
	Channel            string `json:"channel"`
	UpdateLevels       []int  `json:"update-levels"`
}

type upstreamUpdateDescriptionRequest struct {
	ProductName         string `json:"product-name"`
	ProductVersion      string `json:"product-version"`
	Channel             string `json:"channel"`
	StartingUpdateLevel int    `json:"starting-update-level"`
	EndingUpdateLevel   int    `json:"ending-update-level"`
	UserEmail           string `json:"user-email"`
}

type upstreamUpdateDescription struct {
	ProductName        string                    `json:"product-name"`
	ProductVersion     string                    `json:"product-version"`
	Channel            string                    `json:"channel"`
	UpdateLevel        int                       `json:"update-level"`
	UpdateNumber       int                       `json:"update-number"`
	Description        *string                   `json:"description,omitempty"`
	Instructions       *string                   `json:"instructions,omitempty"`
	BugFixes           *string                   `json:"bug-fixes,omitempty"`
	FilesAdded         *string                   `json:"files-added,omitempty"`
	FilesModified      *string                   `json:"files-modified,omitempty"`
	FilesRemoved       *string                   `json:"files-removed,omitempty"`
	BundlesInfoChanges *string                   `json:"bundles-info-changes,omitempty"`
	DependantReleases  []upstreamDependantRelease `json:"dependant-releases,omitempty"`
	UpdateType         string                    `json:"update-type"`
	Timestamp          int64                     `json:"timestamp"`
	SecurityAdvisories []upstreamSecurityAdvisory `json:"security-advisories"`
}

type upstreamDependantRelease struct {
	Repository     string `json:"repository"`
	ReleaseVersion string `json:"release-version"`
}

type upstreamSecurityAdvisory struct {
	ID          string `json:"id"`
	Overview    string `json:"overview"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
	Solution    string `json:"solution"`
	Notes       string `json:"notes"`
	Credits     string `json:"credits"`
}

// ---- portal (camelCase) types ----
// These mirror the types defined in modules/types/types.bal and are what the
// CSM portal returns to its callers.

// RecommendedUpdateLevel is the portal response shape for a recommended update level.
type RecommendedUpdateLevel struct {
	ProductName                   string `json:"productName"`
	ProductBaseVersion            string `json:"productBaseVersion"`
	Channel                       string `json:"channel"`
	StartingUpdateLevel           int    `json:"startingUpdateLevel"`
	EndingUpdateLevel             int    `json:"endingUpdateLevel"`
	InstalledUpdatesCount         int    `json:"installedUpdatesCount"`
	InstalledSecurityUpdatesCount int    `json:"installedSecurityUpdatesCount"`
	Timestamp                     int64  `json:"timestamp"`
	RecommendedUpdateLevel        int    `json:"recommendedUpdateLevel"`
	AvailableUpdatesCount         int    `json:"availableUpdatesCount"`
	AvailableSecurityUpdatesCount int    `json:"availableSecurityUpdatesCount"`
}

// ProductUpdateLevel is the portal response shape for a product's update levels.
type ProductUpdateLevel struct {
	ProductName         string        `json:"productName"`
	ProductUpdateLevels []UpdateLevel `json:"productUpdateLevels"`
}

// UpdateLevel is the portal response shape for a single update level entry.
type UpdateLevel struct {
	ProductBaseVersion string `json:"productBaseVersion"`
	Channel            string `json:"channel"`
	UpdateLevels       []int  `json:"updateLevels"`
}

// SearchPayload is the portal request shape for searching updates between levels.
// Received as camelCase JSON from callers; mapped to the upstream snake-case format before forwarding.
type SearchPayload struct {
	ProductName         string `json:"productName"`
	ProductVersion      string `json:"productVersion"`
	StartingUpdateLevel int    `json:"startingUpdateLevel"`
	EndingUpdateLevel   int    `json:"endingUpdateLevel"`
}

// UpdateLevelGroup is the portal response shape grouping update descriptions by update level,
// mirroring the types:UpdateLevelGroup record in the Ballerina updates module.
type UpdateLevelGroup struct {
	UpdateType              string              `json:"updateType"`
	UpdateDescriptionLevels []UpdateDescription `json:"updateDescriptionLevels"`
}

// UpdateDescription is the portal response shape for a single update description.
type UpdateDescription struct {
	UpdateLevel        int                `json:"updateLevel"`
	ProductName        string             `json:"productName"`
	ProductVersion     string             `json:"productVersion"`
	Channel            string             `json:"channel"`
	UpdateType         string             `json:"updateType"`
	UpdateNumber       int                `json:"updateNumber"`
	Description        *string            `json:"description,omitempty"`
	Instructions       *string            `json:"instructions,omitempty"`
	BugFixes           *string            `json:"bugFixes,omitempty"`
	FilesAdded         *string            `json:"filesAdded,omitempty"`
	FilesModified      *string            `json:"filesModified,omitempty"`
	FilesRemoved       *string            `json:"filesRemoved,omitempty"`
	BundlesInfoChanges *string            `json:"bundlesInfoChanges,omitempty"`
	DependantReleases  []DependantRelease `json:"dependantReleases,omitempty"`
	Timestamp          int64              `json:"timestamp"`
	SecurityAdvisories []SecurityAdvisory `json:"securityAdvisories"`
}

// SecurityAdvisory is the portal response shape for a security advisory.
type SecurityAdvisory struct {
	ID          string `json:"id"`
	Overview    string `json:"overview"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
	Solution    string `json:"solution"`
	Notes       string `json:"notes"`
	Credits     string `json:"credits"`
}

// DependantRelease is the portal response shape for a dependant release.
type DependantRelease struct {
	Repository     string `json:"repository"`
	ReleaseVersion string `json:"releaseVersion"`
}
