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

import (
	"time"
)

// Product catalogue: the product and EOL data CSM publishes to the wso2.com
// Support Matrix and Release Matrix (migrations 0270-0272), served by the
// products API (Product and ProductVersion carry the catalogue fields; see
// entity.go). Write payloads take dates as "YYYY-MM-DD".

// Catalogue entity types: the tables a write refers to.
const (
	CatalogEntityProduct        = "product"
	CatalogEntityProductVersion = "product_version"
	CatalogEntityReleaseLine    = "release_line"
)

// Public feeds a version can be listed on.
const (
	CatalogFeedSupportMatrix = "support-matrix"
	CatalogFeedReleaseMatrix = "release-matrix"
)

// CatalogCarbon is the Carbon platform of a version (product_version_carbon).
type CatalogCarbon struct {
	PlatformRelease     *string `json:"platformRelease"`
	PlatformVersion     *string `json:"platformVersion"`
	CarbonKernelVersion *string `json:"carbonKernelVersion"`
	ReleaseChunk        *string `json:"releaseChunk"`
	KernelPatchNo       *string `json:"kernelPatchNo"`
}

// SearchAllProductVersionsRequest is the body of POST /product-versions/search:
// the per-product search plus an optional productId filter.
type SearchAllProductVersionsRequest struct {
	SearchProductVersionsRequest
	ProductID string `json:"productId"`
}

// ProductVersionList is the body of the version writes: the rows created,
// changed or retired.
type ProductVersionList struct {
	ProductVersions []ProductVersion `json:"productVersions"`
}

// ReleaseLine is one release_line row.
type ReleaseLine struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	StartDate    *time.Time `json:"startDate"`
	Status       *string    `json:"status"`
	LegacyID     *int64     `json:"legacyId"`
	VersionCount int        `json:"versionCount"`
	RetiredOn    *time.Time `json:"retiredOn"`
	CreatedOn    time.Time  `json:"createdOn"`
	UpdatedOn    time.Time  `json:"updatedOn"`
}

// ReleaseLineList is the body of GET /release-lines.
type ReleaseLineList struct {
	ReleaseLines []ReleaseLine `json:"releaseLines"`
}

// CarbonPlatformHistory is one carbon_platform_history row. JSON names follow
// the legacy Product API so the compatible feed can pass them through.
type CarbonPlatformHistory struct {
	ID                             int     `json:"id"`
	CarbonVersion                  *string `json:"carbonVersion"`
	ReleaseDate                    *string `json:"releaseDate"`
	ReleaseDateText                *string `json:"releaseDateText"`
	ESBVersion                     *string `json:"esbVersion"`
	GovernanceRegistryVersion      *string `json:"governanceRegistryVersion"`
	ApplicationServerVersion       *string `json:"applicationServerVersion"`
	DataServicesServerVersion      *string `json:"dataServicesServerVersion"`
	IdentityServerVersion          *string `json:"identityServerVersion"`
	BusinessProcessServerVersion   *string `json:"businessProcessServerVersion"`
	MashupServerVersion            *string `json:"mashupServerVersion"`
	GadgetServerVersion            *string `json:"gadgetServerVersion"`
	BusinessActivityMonitorVersion *string `json:"businessActivityMonitorVersion"`
	APIManagerVersion              *string `json:"apiManagerVersion"`
	BusinessRulesServerVersion     *string `json:"businessRulesServerVersion"`
	EventServerVersion             *string `json:"eventServerVersion"`
	MessageBrokerVersion           *string `json:"messageBrokerVersion"`
	ComplexEventProcessorVersion   *string `json:"complexEventProcessorVersion"`
	ElasticLoadBalancerVersion     *string `json:"elasticLoadBalancerVersion"`
	StorageServerVersion           *string `json:"storageServerVersion"`
	StratosVersion                 *string `json:"stratosVersion"`
	CloudGatewayVersion            *string `json:"cloudGatewayVersion"`
	WSASVersion                    *string `json:"wsasVersion"`
	AppFactoryVersion              *string `json:"appFactoryVersion"`
	UserEngagementServerVersion    *string `json:"userEngagementServerVersion"`
}

// CarbonPlatformHistoryList is the body of GET /carbon-platform-history.
type CarbonPlatformHistoryList struct {
	Items []CarbonPlatformHistory `json:"items"`
}

// CarbonKernelRelease is one carbon_kernel_release row.
type CarbonKernelRelease struct {
	ID             int     `json:"id"`
	KernelRelease  *string `json:"kernelRelease"`
	ReleaseDate    *string `json:"releaseDate"`
	CarbonPlatform *string `json:"carbonPlatform"`
}

// CarbonKernelReleaseList is the body of GET /carbon-kernel-releases.
type CarbonKernelReleaseList struct {
	Items []CarbonKernelRelease `json:"items"`
}
