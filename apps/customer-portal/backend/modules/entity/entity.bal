// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

# Get logged-in user information.
#
# + email - Email of the user
# + idToken - ID token for authorization
# + return - User response or error
public isolated function getUserBasicInfo(string email, string idToken) returns UserResponse|error {
    return csEntityClient->/users/me.get(generateHeaders(idToken));
}

# Get project by ID.
#
# + idToken - ID token for authorization
# + projectId - Unique ID of the project
# + return - Project response or error
public isolated function getProject(string idToken, string projectId) returns ProjectResponse|error {
    return csEntityClient->/projects/[projectId].get(generateHeaders(idToken));
}

# Search projects of the logged-in user.
#
# + idToken - ID token for authorization
# + payload - Payload for searching projects
# + return - Projects response or error
public isolated function searchProjects(string idToken, ProjectSearchPayload payload) returns ProjectsResponse|error {
    return csEntityClient->/projects/search.post(payload, generateHeaders(idToken));
}

# Get project activity statistics by ID.
#
# + idToken - ID token for authorization
# + id - Unique ID of the project
# + return - Project statistics or error
public isolated function getProjectActivityStats(string idToken, string id) returns ProjectStatsResponse|error {
    return csEntityClient->/projects/[id]/stats.get(generateHeaders(idToken));
}

# Get cases statistics of a project by ID.
#
# + idToken - ID token for authorization
# + id - Unique ID of the project
# + return - Project cases statistics or error
public isolated function getCaseStatsForProject(string idToken, string id) returns ProjectCaseStatsResponse|error {
    return csEntityClient->/projects/[id]/cases/stats.get(generateHeaders(idToken));
}

# Get chats statistics of a project by ID.
#
# + idToken - ID token for authorization
# + id - Unique ID of the project
# + return - Project chats statistics or error
public isolated function getChatStatsForProject(string idToken, string id) returns ProjectChatStatsResponse|error {
    return csEntityClient->/projects/[id]/chats/stats.get(generateHeaders(idToken));
}

# Get deployments statistics of a project by ID.
#
# + idToken - ID token for authorization
# + id - Unique ID of the project
# + return - Project deployments statistics or error
public isolated function getDeploymentStatsForProject(string idToken, string id)
    returns ProjectDeploymentStatsResponse|error {

    return csEntityClient->/projects/[id]/deployments/stats.get(generateHeaders(idToken));
}

# Get case by ID.
#
# + idToken - ID token for authorization
# + caseId - Unique ID of the case
# + return - Case details or error
public isolated function getCase(string idToken, string caseId) returns CaseResponse|error {
    return csEntityClient->/cases/[caseId].get(generateHeaders(idToken));
}

# Create a new case.
#
# + idToken - ID token for authorization
# + payload - Case creation payload
# + return - Case creation response or error
public isolated function createCase(string idToken, CaseCreatePayload payload) returns CaseCreateResponse|error {
    return csEntityClient->/cases.post(payload, generateHeaders(idToken));
}

# Search cases of a project.
#
# + idToken - ID token for authorization
# + payload - Request body for searching cases
# + return - Cases object or error
public isolated function searchCases(string idToken, CaseSearchPayload payload) returns CaseSearchResponse|error {
    return csEntityClient->/cases/search.post(payload, generateHeaders(idToken));
}

# Get case metadata.
#
# + idToken - ID token for authorization
# + return - Case metadata response or error
public isolated function getCaseMetadata(string idToken) returns CaseMetadataResponse|error {
    return csEntityClient->/cases/meta.get(generateHeaders(idToken));
}

# Search comments.
#
# + idToken - ID token for authorization
# + payload - Comment request payload
# + return - Comments response or error
public isolated function searchComments(string idToken, ReferenceSearchPayload payload) returns CommentsResponse|error {
    return csEntityClient->/comments/search.post(payload, generateHeaders(idToken));
}

# Search attachments.
#
# + idToken - ID token for authorization
# + payload - Attachment request payload
# + return - Attachments response or error
public isolated function searchAttachments(string idToken, ReferenceSearchPayload payload)
    returns AttachmentsResponse|error {

    return csEntityClient->/attachments/search.post(payload, generateHeaders(idToken));
}

# Create an attachment.
#
# + idToken - ID token for authorization
# + payload - Attachment creation payload
# + return - Attachment creation response or error
public isolated function createAttachment(string idToken, AttachmentPayload payload)
    returns AttachmentCreateResponse|error {

    return csEntityClient->/attachments.post(payload, generateHeaders(idToken));
}

# Get products of a deployment.
#
# + idToken - ID token for authorization
# + deploymentId - ID of the deployment
# + return - Products response or error
public isolated function getDeployedProducts(string idToken, string deploymentId)
    returns DeployedProductsResponse|error {

    return csEntityClient->/deployed\-products/search.post({deploymentId}, generateHeaders(idToken));
}

# Get deployments of a project.
#
# + idToken - ID token for authorization
# + projectId - ID of the project
# + return - Deployments response or error
public isolated function getDeployments(string idToken, string projectId) returns DeploymentsResponse|error {
    return csEntityClient->/deployments/search.post({filters: {projectIds: [projectId]}}, generateHeaders(idToken));
}

# Create a comment for a case.
#
# + idToken - ID token for authorization
# + payload - Comment creation payload
# + return - Comment creation response or error
public isolated function createComment(string idToken, CommentCreatePayload payload)
    returns CommentCreateResponse|error {

    return csEntityClient->/comments.post(payload, generateHeaders(idToken));
}
