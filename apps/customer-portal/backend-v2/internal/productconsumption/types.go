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

package productconsumption

import "encoding/json"

// These types mirror the upstream product-consumption service's wire format
// 1:1 so json.Unmarshal can decode its responses directly.

// DeploymentLicenseRequest is the input for
// POST /projects/{projectId}/deployments/{deploymentId}/license.
type DeploymentLicenseRequest struct {
	Email string `json:"email"`
}

// License is the final license payload returned to the caller.
//
// SubscriptionData is carried verbatim rather than modelled as a struct.
// ServiceNow signs an HMAC over the canonicalised subscription data, so any
// field lost in transit breaks verification in the customer's product — and a
// closed struct loses every field it does not name. The six-field struct this
// replaces dropped usageDataPublishingUrl, which is both signed and the address
// the product publishes usage to. The Ballerina implementation avoids this by
// declaring an open record (`json...`).
type License struct {
	SubscriptionData json.RawMessage `json:"subscriptionData"`
	Signature        string          `json:"signature"`
}

// LicenseResult wraps License with a success flag.
type LicenseResult struct {
	Success bool    `json:"success"`
	License License `json:"license"`
}

// LicenseResponse is the response for
// POST /projects/{projectId}/deployments/{deploymentId}/license.
type LicenseResponse struct {
	Result LicenseResult `json:"result"`
}

// ImportUsageRequest is the input for POST /deployment-usages.
type ImportUsageRequest struct {
	Email string `json:"email"`
	Zip   string `json:"zip"` // base64-encoded zip file content
}

// ImportUsageResponse is the response for POST /deployment-usages.
type ImportUsageResponse struct {
	Message *string `json:"message,omitempty"`
	Result  any     `json:"result,omitempty"`
}
