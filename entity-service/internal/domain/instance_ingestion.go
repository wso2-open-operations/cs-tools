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

// InstanceIngestionActor is the created_by/updated_by stamped on rows written by
// product-consumption-tracking telemetry, the value that service wrote when it
// still talked to Postgres directly.
const InstanceIngestionActor = "product-consumption-tracking"

// InstanceNodeMeta is the deployment meta information a telemetry payload
// carries about the node that sent it. Every ingestion request carries it so a
// node seen for the first time through info or usage telemetry is registered
// with its project, deployment and product, not as a bare node id. A blank
// field never clears what is already stored; Product and Version only move the
// stored product version when both are sent.
type InstanceNodeMeta struct {
	ProjectKey       string `json:"projectKey,omitempty"`
	DeploymentNumber string `json:"deploymentNumber,omitempty"`
	Product          string `json:"product,omitempty"`
	Version          string `json:"version,omitempty"`
}

// InstanceMetaRequest carries metadata to register or update a deployment node.
type InstanceMetaRequest struct {
	NodeID string `json:"nodeId"`
	InstanceNodeMeta
}

// InstanceMetaResponse carries the UUID of the registered deployment node.
type InstanceMetaResponse struct {
	ID string `json:"id"`
}

// InstanceInfoRequest carries JDK and host environment details for a deployment node.
// CreatedTime is RFC 3339, or YYYY-MM-DD HH:MM:SS read as UTC; empty means now.
type InstanceInfoRequest struct {
	NodeID         string `json:"nodeId"`
	JDKVersion     string `json:"jdkVersion"`
	CoreCount      int    `json:"coreCount"`
	DeploymentInfo any    `json:"deploymentInfo"`
	Hash           string `json:"hash"`
	CreatedTime    string `json:"createdTime"`
	InstanceNodeMeta
}

// InstanceInfoResponse carries the UUID of the deployment_information record,
// the existing one when the same snapshot (node and hash) was already stored.
type InstanceInfoResponse struct {
	ID string `json:"id"`
}

// InstanceUsageIngestRequest carries live hourly and daily telemetry for a deployment node.
// CreatedTime is RFC 3339, or YYYY-MM-DD HH:MM:SS read as UTC; empty means now.
type InstanceUsageIngestRequest struct {
	NodeID      string `json:"nodeId"`
	CountType   string `json:"countType"`
	Count       int    `json:"count"`
	CreatedTime string `json:"createdTime"`
	InstanceNodeMeta
}

// InstanceUsageIngestResponse carries status of usage stats insertion.
type InstanceUsageIngestResponse struct {
	Status string `json:"status"`
}

// DailyUsageBatchItem represents a single daily metric value in an offline import.
// NodeID and DeploymentNumber override the batch's own for this entry, as the
// offline ZIP format allows.
type DailyUsageBatchItem struct {
	Date             string `json:"date"`
	Type             string `json:"type"`
	Value            int    `json:"value"`
	NodeID           string `json:"nodeId,omitempty"`
	DeploymentNumber string `json:"deploymentNumber,omitempty"`
}

// DailyUsageBatchNode is a node an offline report describes, with the product
// it runs. Its product and version override the batch's for that node.
type DailyUsageBatchNode struct {
	NodeID  string `json:"nodeId"`
	Product string `json:"product,omitempty"`
	Version string `json:"version,omitempty"`
}

// DailyUsageBatchRequest carries parsed daily usage entries from an offline ZIP file.
type DailyUsageBatchRequest struct {
	NodeID string `json:"nodeId"`
	InstanceNodeMeta
	UserEmail string                `json:"userEmail"`
	Nodes     []DailyUsageBatchNode `json:"nodes,omitempty"`
	Usages    []DailyUsageBatchItem `json:"usages"`
}

// DailyUsageBatchResponse carries the number of imported daily usage entries.
type DailyUsageBatchResponse struct {
	ImportedCount int    `json:"importedCount"`
	Status        string `json:"status"`
}

// ProjectSecretKeyResponse carries the HMAC primary secret key for a project.
type ProjectSecretKeyResponse struct {
	SecretKey string `json:"secretKey"`
}

// DeploymentConsumptionSecretResponse carries a deployment's project and the
// project's HMAC primary secret key, for verifying an offline usage report,
// which names its deployment but not its project.
type DeploymentConsumptionSecretResponse struct {
	DeploymentNumber string `json:"deploymentNumber"`
	ProjectID        string `json:"projectId"`
	ProjectKey       string `json:"projectKey"`
	SecretKey        string `json:"secretKey"`
}
