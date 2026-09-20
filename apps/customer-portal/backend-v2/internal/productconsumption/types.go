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

// These types mirror the upstream product-consumption service's wire format
// 1:1 so json.Unmarshal can decode its responses directly.

// consumptionStatus enumerates the product-consumption service's per-project
// license-provisioning state machine.
type consumptionStatus int

const (
	statusPending              consumptionStatus = 1
	statusCreated              consumptionStatus = 2
	statusSubscribed           consumptionStatus = 3
	statusGeneratedCredentials consumptionStatus = 4
	statusGeneratedSecretKeys  consumptionStatus = 5
)

// ConsumptionStatusRequest is the input for POST /projects/{projectId}/consumption/status.
type ConsumptionStatusRequest struct {
	Email        string `json:"email"`
	DeploymentID string `json:"deploymentId"`
}

// ConsumptionData carries the current provisioning state for a project.
type ConsumptionData struct {
	// jsonInt, not int: the upstream sends this as 5.0 — see jsonint.go.
	Status        jsonInt `json:"status"`
	ApplicationID *string `json:"applicationId,omitempty"`
	Name          *string `json:"name,omitempty"`
	Description   *string `json:"description,omitempty"`
}

// ConsumptionResult wraps ConsumptionData with top-level message/applicationId
// fields — matches the upstream service's own (slightly redundant) response shape.
type ConsumptionResult struct {
	Message       *string         `json:"message,omitempty"`
	ApplicationID *string         `json:"applicationId,omitempty"`
	Result        ConsumptionData `json:"result"`
}

// ApplicationCreateRequest is the input for POST /applications.
type ApplicationCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ApplicationCreateResponse is the response for POST /applications.
type ApplicationCreateResponse struct {
	Name          string `json:"name"`
	ApplicationID string `json:"applicationId"`
}

// UpdateProjectStatusRequest is the input for PATCH /projects/{projectId} —
// the product-consumption service's own per-project state record, distinct
// from entity-service's Project entity. Only the fields relevant to the
// current state-machine step are set on each call.
type UpdateProjectStatusRequest struct {
	Status             int     `json:"status"`
	ApplicationID      *string `json:"applicationId,omitempty"`
	ConsumerKey        *string `json:"consumerKey,omitempty"`
	ConsumerSecret     *string `json:"consumerSecret,omitempty"`
	PrimarySecretKey   *string `json:"primarySecretKey,omitempty"`
	SecondarySecretKey *string `json:"secondarySecretKey,omitempty"`
}

// ApplicationSubscriptionResponse is the response for
// POST /applications/{applicationId}/subscribe.
type ApplicationSubscriptionResponse struct {
	ApplicationID  string `json:"applicationId"`
	SubscriptionID string `json:"subscriptionId"`
	APIID          string `json:"apiId"`
}

// ApplicationKeyGenerationResponse is the response for
// POST /applications/{applicationId}/generate-credentials.
type ApplicationKeyGenerationResponse struct {
	ConsumerKey    string `json:"consumerKey"`
	ConsumerSecret string `json:"consumerSecret"`
}

// SecretKeysResponse is the response for POST /generate-secret-keys.
type SecretKeysResponse struct {
	PrimarySecretKey   string `json:"primarySecretKey"`
	SecondarySecretKey string `json:"secondarySecretKey"`
}

// DeploymentLicenseRequest is the input for
// POST /projects/{projectId}/deployments/{deploymentId}/license.
type DeploymentLicenseRequest struct {
	Email string `json:"email"`
}

// SubscriptionData carries the deployment's license/subscription details.
type SubscriptionData struct {
	DeploymentID    string `json:"deploymentId"`
	DeploymentName  string `json:"deploymentName"`
	SubscriptionKey string `json:"subscriptionKey"`
	ClientID        string `json:"clientId"`
	ClientSecret    string `json:"clientSecret"`
	Secrets         string `json:"secrets"`
}

// License is the final license payload returned to the caller.
type License struct {
	SubscriptionData SubscriptionData `json:"subscriptionData"`
	Signature        string           `json:"signature"`
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
