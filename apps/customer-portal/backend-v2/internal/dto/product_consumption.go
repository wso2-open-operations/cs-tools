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

package dto

import (
	"encoding/json"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/productconsumption"
)

// LicenseResponse is the portal's response for
// POST /projects/{projectId}/deployments/{deploymentId}/license.
//
// SubscriptionData is passed through verbatim — the one endpoint in this
// package that deliberately does not trim, and the one that must not even
// reshape. ServiceNow signs an HMAC over the canonicalised subscription data,
// and the customer's product recomputes that canonical string from the licence
// file to verify it. Every field is therefore load-bearing: drop one and the
// signature stops verifying.
//
// It was previously a closed struct of six named fields, which silently
// discarded the rest — including usageDataPublishingUrl, the address the
// customer's product publishes its usage to. Modelling the payload at all
// means re-deciding its shape every time ServiceNow adds a field; carrying the
// bytes means never having to.
type LicenseResponse struct {
	SubscriptionData json.RawMessage `json:"subscriptionData"`
	Signature        string          `json:"signature"`
}

// MapLicense builds the portal response from the product-consumption
// service's License.
func MapLicense(l productconsumption.License) LicenseResponse {
	return LicenseResponse{
		SubscriptionData: l.SubscriptionData,
		Signature:        l.Signature,
	}
}

// ImportDeploymentUsageResponse is the portal's response for POST /deployment-usages.
type ImportDeploymentUsageResponse struct {
	Message string `json:"message,omitempty"`
	Result  any    `json:"result,omitempty"`
}

// MapImportDeploymentUsage builds the portal response from the
// product-consumption service's ImportUsageResponse.
func MapImportDeploymentUsage(r productconsumption.ImportUsageResponse) ImportDeploymentUsageResponse {
	out := ImportDeploymentUsageResponse{Result: r.Result}
	if r.Message != nil {
		out.Message = *r.Message
	}
	return out
}
