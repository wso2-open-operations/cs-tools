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
	"encoding/json"
	"time"
)

// ConsumptionStatus is a step in a project's product-consumption provisioning
// state machine. The flow is resumable: a caller reads the current status and
// runs only the steps above it, so the numeric ordering is load-bearing, not
// cosmetic.
//
// These values are the ServiceNow choice-list numbers this data has always
// used (u_product_consumption_choreo_application_status), kept identical so
// the Postgres and ServiceNow paths speak the same wire vocabulary.
type ConsumptionStatus int16

const (
	// ConsumptionStatusPending means nothing has been provisioned yet.
	ConsumptionStatusPending ConsumptionStatus = 1
	// ConsumptionStatusCreated means the Choreo application exists and its ID
	// has been recorded.
	ConsumptionStatusCreated ConsumptionStatus = 2
	// ConsumptionStatusSubscribed means that application is subscribed to the
	// product-consumption tracking API.
	ConsumptionStatusSubscribed ConsumptionStatus = 3
	// ConsumptionStatusGeneratedCredentials means the application's OAuth2
	// consumer key and secret have been minted and stored.
	ConsumptionStatusGeneratedCredentials ConsumptionStatus = 4
	// ConsumptionStatusGeneratedSecretKeys means the deployment's primary and
	// secondary subscription secret keys have been generated and stored. Only
	// at this point can a license be issued.
	ConsumptionStatusGeneratedSecretKeys ConsumptionStatus = 5
)

// Valid reports whether s is a known step.
func (s ConsumptionStatus) Valid() bool {
	return s >= ConsumptionStatusPending && s <= ConsumptionStatusGeneratedSecretKeys
}

// ProjectConsumption is the stored provisioning state for one project.
//
// The credential fields are decrypted on read and encrypted on write by the
// repository; this struct always holds them in the clear, which is why it is
// never serialised to a response as-is. See ProjectConsumptionView for what
// callers actually receive.
type ProjectConsumption struct {
	ProjectID           string
	Status              ConsumptionStatus
	ChoreoApplicationID *string
	ConsumerKey         *string
	ConsumerSecret      *string
	PrimarySecretKey    *string
	SecondarySecretKey  *string
	CreatedOn           time.Time
	UpdatedOn           time.Time
}

// ProjectConsumptionView is the read response for a project's provisioning
// state.
//
// It deliberately carries no secret material. The consumer secret and the two
// subscription secret keys are write-only as far as this API is concerned:
// they are stored so that license generation can use them later, and the only
// caller that ever needs their values is the license-issuing step itself. A
// status read is made on every license download, so exposing them here would
// put three long-lived credentials on the wire routinely for no reason.
//
// Name and Description are the project's own, included because the caller
// needs them to name the Choreo application when Status is
// ConsumptionStatusPending — that is the one piece of data the provisioning
// flow would otherwise have to make a second call for.
type ProjectConsumptionView struct {
	ProjectID         string    `json:"projectId"`
	Status            int16     `json:"status"`
	ApplicationID     *string   `json:"applicationId"`
	ConsumerKey       *string   `json:"consumerKey"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	HasConsumerSecret bool      `json:"hasConsumerSecret"`
	HasSecretKeys     bool      `json:"hasSecretKeys"`
	UpdatedOn         time.Time `json:"updatedOn"`
}

// UpdateProjectConsumptionRequest advances a project's provisioning state.
//
// Status is required and may only move forward — see
// ProjectConsumptionService.UpdateProjectConsumption for why. The remaining
// fields are the artefacts produced by the step being recorded; each is
// optional here and validated against Status by the service, since which ones
// are mandatory depends entirely on which step this call represents.
type UpdateProjectConsumptionRequest struct {
	Status             int16   `json:"status"`
	ApplicationID      *string `json:"applicationId,omitempty"`
	ConsumerKey        *string `json:"consumerKey,omitempty"`
	ConsumerSecret     *string `json:"consumerSecret,omitempty"`
	PrimarySecretKey   *string `json:"primarySecretKey,omitempty"`
	SecondarySecretKey *string `json:"secondarySecretKey,omitempty"`
}

// UpdateProjectConsumptionResponse is returned after a successful update.
type UpdateProjectConsumptionResponse struct {
	Message string                 `json:"message"`
	Result  ProjectConsumptionView `json:"result"`
}

// License is the deployment license payload issued by ServiceNow.
//
// SubscriptionData is carried verbatim and deliberately NOT modelled as a
// struct. ServiceNow signs an HMAC over the canonicalised subscription data —
// every key sorted, key and value concatenated — so the customer's product
// recomputes that string from the licence it receives. Any field this service
// fails to carry across changes the string the product computes, and the
// signature no longer verifies.
//
// A closed struct silently drops the fields it does not name, and the payload
// has more than the obvious ones: usageDataPublishingUrl, which is also where
// the product publishes its usage, is signed but was absent from the struct
// this replaces. The Ballerina implementation this was ported from gets it
// right by declaring an open record (`json...`); json.RawMessage is the Go
// equivalent, and it guarantees byte-fidelity rather than best-effort field
// coverage.
//
// Do not "improve" this into a struct.
type License struct {
	SubscriptionData json.RawMessage `json:"subscriptionData"`
	Signature        string          `json:"signature"`
}

// DeploymentLicenseRequest is the request body for
// POST /projects/{id}/deployments/{deploymentId}/license.
type DeploymentLicenseRequest struct {
	Email string `json:"email"`
}
