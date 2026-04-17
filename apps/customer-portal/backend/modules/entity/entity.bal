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

# Get metadata information.
#
# + idToken - ID token for authorization
# + return - Metadata response containing metadata information or error
public isolated function getMetadata(string idToken) returns MetadataResponse|error {
    return csEntityClient->/metadata.get(generateHeaders(idToken));
}

# Get logged-in user information.
#
# + email - Email of the user
# + idToken - ID token for authorization
# + return - User response or error
public isolated function getUserBasicInfo(string email, string idToken) returns UserResponse|error {
    return csEntityClient->/users/me.get(generateHeaders(idToken));
}

# Update logged-in user information.
#
# + idToken - ID token for authorization
# + payload - User update payload containing details to be updated in the user profile
# + return - User update response containing details of the updated user profile or error
public isolated function updateUser(string idToken, UserUpdatePayload payload) returns UserUpdateResponse|error {
    return csEntityClient->/users/me.patch(payload, generateHeaders(idToken));
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

# Update a project.
#
# + idToken - ID token for authorization
# + projectId - Unique ID of the project to be updated
# + payload - Project update payload containing details to be updated in the project
# + return - Project update response containing details of the updated project or error
public isolated function updateProject(string idToken, string projectId, ProjectUpdatePayload payload)
    returns ProjectUpdateResponse|error {

    return csEntityClient->/projects/[projectId].patch(payload, generateHeaders(idToken));
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
# + caseTypes - Optional array of case types to filter statistics
# + createdBy - Optional filter for cases created by a specific user
# + return - Project cases statistics or error
public isolated function getCaseStatsForProject(string idToken, string id, CaseType[]? caseTypes,
        StatsFilter? createdBy) returns ProjectCaseStatsResponse|error {

    if caseTypes is CaseType[] {
        if createdBy is StatsFilter {
            return csEntityClient->/projects/[id]/cases/stats.get(generateHeaders(idToken), caseTypes = caseTypes,
                createdBy = createdBy
            );
        }
        return csEntityClient->/projects/[id]/cases/stats.get(generateHeaders(idToken), caseTypes = caseTypes);
    }
    if createdBy is StatsFilter {
        return csEntityClient->/projects/[id]/cases/stats.get(generateHeaders(idToken), createdBy = createdBy);
    }
    return csEntityClient->/projects/[id]/cases/stats.get(generateHeaders(idToken));
}

# Get conversation statistics of a project by ID.
#
# + idToken - ID token for authorization
# + id - Unique ID of the project
# + createdBy - Optional filter for conversations created by a specific user
# + return - Project conversations statistics or error
public isolated function getConversationStatsForProject(string idToken, string id, StatsFilter? createdBy)
    returns ProjectConversationStatsResponse|error {

    if createdBy is StatsFilter {
        return csEntityClient->/projects/[id]/conversations/stats.get(generateHeaders(idToken), createdBy = createdBy);
    }
    return csEntityClient->/projects/[id]/conversations/stats.get(generateHeaders(idToken));
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

# Update an existing case.
#
# + idToken - ID token for authorization
# + caseId - Unique ID of the case to be updated
# + payload - Case update payload
# + return - Case update response or error
public isolated function updateCase(string idToken, string caseId, CaseUpdatePayload payload)
    returns CaseUpdateResponse|error {

    return csEntityClient->/cases/[caseId].patch(payload, generateHeaders(idToken));
}

# Search cases of a project.
#
# + idToken - ID token for authorization
# + payload - Request body for searching cases
# + return - Cases object or error
public isolated function searchCases(string idToken, CaseSearchPayload payload) returns CaseSearchResponse|error {
    return csEntityClient->/cases/search.post(payload, generateHeaders(idToken));
}

# Get project metadata.
#
# + idToken - ID token for authorization
# + projectId - Unique ID of the project
# + return - Project metadata response or error
public isolated function getProjectMetadata(string idToken, string projectId) returns ProjectMetadataResponse|error {
    return csEntityClient->/projects/[projectId]/metadata.get(generateHeaders(idToken));
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
public isolated function createAttachment(string idToken, AttachmentCreatePayload payload)
    returns AttachmentCreateResponse|error {

    return csEntityClient->/attachments.post(payload, generateHeaders(idToken));
}

# Get attachment by ID.
#
# + idToken - ID token for authorization
# + attachmentId - ID of the attachment
# + return - Attachment response or error
public isolated function getAttachment(string idToken, IdString attachmentId) returns AttachmentResponse|error {
    return csEntityClient->/attachments/[attachmentId].get(generateHeaders(idToken));
}

# Update an attachment.
#
# + idToken - ID token for authorization
# + attachmentId - Unique ID of the attachment to be updated
# + payload - Attachment update payload containing details to be updated in the attachment
# + return - Attachment update response containing details of the updated attachment or error
public isolated function updateAttachment(string idToken, IdString attachmentId, AttachmentUpdatePayload payload)
    returns AttachmentUpdateResponse|error {

    return csEntityClient->/attachments/[attachmentId].patch(payload, generateHeaders(idToken));
}

# Delete an attachment.
#
# + idToken - ID token for authorization
# + attachmentId - Unique ID of the attachment to be deleted
# + return - Attachment delete response containing success message or error
public isolated function deleteAttachment(string idToken, IdString attachmentId)
    returns AttachmentDeleteResponse|error {

    return csEntityClient->/attachments/[attachmentId].delete(headers = generateHeaders(idToken));
}

# Search products of a deployment.
#
# + idToken - ID token for authorization
# + payload - Payload for searching products of a deployment
# + return - Products response or error
public isolated function searchDeployedProducts(string idToken, DeployedProductSearchPayload payload)
    returns DeployedProductsResponse|error {

    return csEntityClient->/deployed\-products/search.post(payload, generateHeaders(idToken));
}

# Create a deployed product.
#
# + idToken - ID token for authorization
# + payload - Deployed product creation payload
# + return - Deployed product creation response or error
public isolated function createDeployedProduct(string idToken, DeployedProductCreatePayload payload)
    returns DeployedProductCreateResponse|error {

    return csEntityClient->/deployed\-products.post(payload, generateHeaders(idToken));
}

# Update a deployed product.
#
# + idToken - ID token for authorization
# + deployedProductId - ID of the deployed product to update
# + payload - Deployed product update payload
# + return - Deployed product update response or error
public isolated function updateDeployedProduct(string idToken, string deployedProductId,
        DeployedProductUpdatePayload payload) returns DeployedProductUpdateResponse|error {

    return csEntityClient->/deployed\-products/[deployedProductId].patch(payload, generateHeaders(idToken));
}

# Search deployments of a project.
#
# + idToken - ID token for authorization
# + payload - Payload for searching deployments of the project
# + return - Deployments response or error
public isolated function searchDeployments(string idToken, DeploymentSearchPayload payload)
    returns DeploymentsResponse|error {

    return csEntityClient->/deployments/search.post(payload, generateHeaders(idToken));
}

# Create a deployment.
#
# + idToken - ID token for authorization
# + payload - Deployment creation payload
# + return - Deployment creation response or error
public isolated function createDeployment(string idToken, DeploymentCreatePayload payload)
    returns DeploymentCreateResponse|error {

    return csEntityClient->/deployments.post(payload, generateHeaders(idToken));
}

# Update a deployment.
#
# + idToken - ID token for authorization
# + deploymentId - ID of the deployment to update
# + payload - Deployment update payload
# + return - Deployment update response or error
public isolated function updateDeployment(string idToken, string deploymentId, DeploymentUpdatePayload payload)
    returns DeploymentUpdateResponse|error {

    return csEntityClient->/deployments/[deploymentId].patch(payload, generateHeaders(idToken));
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

# Search product vulnerabilities.
#
# + idToken - ID token for authorization
# + payload - Product vulnerability search payload
# + return - Product vulnerability search response or error
public isolated function searchProductVulnerabilities(string idToken, ProductVulnerabilitySearchPayload payload)
    returns ProductVulnerabilitySearchResponse|error {

    return csEntityClient->/products/vulnerabilities/search.post(payload, generateHeaders(idToken));
}

# Get product vulnerability by ID.
#
# + idToken - ID token for authorization
# + vulnerabilityId - Unique ID of the vulnerability
# + return - Product vulnerability details or error
public isolated function getProductVulnerability(string idToken, string vulnerabilityId)
    returns ProductVulnerabilityResponse|error {

    return csEntityClient->/products/vulnerabilities/[vulnerabilityId].get(generateHeaders(idToken));
}

# Get product vulnerability metadata.
#
# + idToken - ID token for authorization
# + return - Product vulnerability metadata or error
public isolated function getProductVulnerabilityMetaData(string idToken) returns VulnerabilityMetaResponse|error {
    return csEntityClient->/products/vulnerabilities/meta.get(generateHeaders(idToken));
}

# Search call requests of a project.
#
# + idToken - ID token for authorization
# + payload - Call request search payload
# + return - Call requests response or error
public isolated function searchCallRequests(string idToken, CallRequestSearchPayload payload)
    returns CallRequestsResponse|error {

    return csEntityClient->/call\-requests/search.post(payload, generateHeaders(idToken));
}

# Create a call request.
#
# + idToken - ID token for authorization
# + payload - Call request creation payload
# + return - Call request creation response or error
public isolated function createCallRequest(string idToken, CallRequestCreatePayload payload)
    returns CallRequestCreateResponse|error {

    return csEntityClient->/call\-requests.post(payload, generateHeaders(idToken));
}

# Update a call request.
#
# + idToken - ID token for authorization
# + callRequestId - Unique ID of the call request to be updated
# + payload - Call request update payload
# + return - Call request update response or error
public isolated function updateCallRequest(string idToken, string callRequestId, CallRequestUpdatePayload payload)
    returns CallRequestUpdateResponse|error {

    return csEntityClient->/call\-requests/[callRequestId].patch(payload, generateHeaders(idToken));
}

# Get products by search criteria.
#
# + idToken - ID token for authorization
# + payload - Product search payload containing search criteria
# + return - Products response containing matching products or error
public isolated function getProducts(string idToken, ProductSearchPayload payload) returns ProductsResponse|error {
    return csEntityClient->/products/search.post(payload, generateHeaders(idToken));
}

# Search product versions by criteria.
#
# + idToken - ID token for authorization
# + productId - Unique ID of the product for which versions are to be searched
# + payload - Product version search payload containing search criteria
# + return - Product versions response containing matching product versions or error
public isolated function searchProductVersions(string idToken, string productId, ProductVersionSearchPayload payload)
    returns ProductVersionsResponse|error {

    return csEntityClient->/products/[productId]/versions/search.post(payload, generateHeaders(idToken));
}

# Search time cards.
#
# + idToken - ID token for authorization
# + payload - Payload containing search criteria for time cards
# + return - Response containing matching time cards or error
public isolated function searchTimeCards(string idToken, TimeCardSearchPayload payload)
    returns TimeCardsResponse|error {

    return csEntityClient->/time\-cards/search.post(payload, generateHeaders(idToken));
}

# Search conversations of a project.
#
# + idToken - ID token for authorization
# + payload - Conversation search payload containing search criteria
# + return - Conversations response containing matching conversations or error
public isolated function searchConversations(string idToken, ConversationSearchPayload payload)
    returns ConversationSearchResponse|error {

    return csEntityClient->/conversations/search.post(payload, generateHeaders(idToken));
}

# Create a conversation.
#
# + idToken - ID token for authorization
# + payload - Conversation creation payload containing details of the conversation to be created
# + return - Conversation creation response containing details of the created conversation or error
public isolated function createConversation(string idToken, ConversationCreatePayload payload)
    returns ConversationCreateResponse|error {

    return csEntityClient->/conversations.post(payload, generateHeaders(idToken));
}

# Update a conversation.
#
# + idToken - ID token for authorization
# + conversationId - Unique ID of the conversation to be updated
# + payload - Conversation update payload containing details to be updated in the conversation
# + return - Conversation update response containing details of the updated conversation or error
public isolated function updateConversation(string idToken, string conversationId, ConversationUpdatePayload payload)
    returns ConversationUpdateResponse|error {

    return csEntityClient->/conversations/[conversationId].patch(payload, generateHeaders(idToken));
}

# Get a conversation by ID.
#
# + idToken - ID token for authorization
# + conversationId - Unique ID of the conversation to be retrieved
# + return - Conversation response containing details of the retrieved conversation or error
public isolated function getConversation(string idToken, string conversationId) returns ConversationResponse|error {
    return csEntityClient->/conversations/[conversationId].get(generateHeaders(idToken));
}

# Get project time card statistics.
#
# + idToken - ID token for authorization
# + projectId - Unique ID of the project for which time card statistics are to be retrieved
# + startDate - Optional start date to filter time cards for statistics (inclusive)
# + endDate - Optional end date to filter time cards for statistics (inclusive)
# + return - Project time card statistics response containing aggregated time card data for the project or error
public isolated function getProjectTimeCardStats(string idToken, string projectId, string? startDate, string? endDate)
    returns ProjectTimeCardStatsResponse|error {

    if startDate is string && endDate is string {
        return csEntityClient->/projects/[projectId]/time\-cards/stats.get(generateHeaders(idToken),
            startDate = startDate, endDate = endDate
        );
    }
    if startDate is string {
        return csEntityClient->/projects/[projectId]/time\-cards/stats.get(generateHeaders(idToken),
            startDate = startDate
        );
    }
    if endDate is string {
        return csEntityClient->/projects/[projectId]/time\-cards/stats.get(generateHeaders(idToken),
            endDate = endDate
        );
    }

    return csEntityClient->/projects/[projectId]/time\-cards/stats.get(generateHeaders(idToken));
}

# Get change request by ID.
#
# + idToken - ID token for authorization
# + changeRequestId - Unique ID of the change request to be retrieved
# + return - Change request response containing details of the retrieved change request or error
public isolated function getChangeRequestDetails(string idToken, string changeRequestId)
    returns ChangeRequestResponse|error {

    return csEntityClient->/change\-requests/[changeRequestId].get(generateHeaders(idToken));
}

# Search change requests of a project.
#
# + idToken - ID token for authorization
# + payload - Change request search payload containing search criteria for change requests
# + return - Change request search response containing matching change requests or error
public isolated function searchChangeRequests(string idToken, ChangeRequestSearchPayload payload)
    returns ChangeRequestSearchResponse|error {

    return csEntityClient->/change\-requests/search.post(payload, generateHeaders(idToken));
}

# Update a change request.
#
# + idToken - ID token for authorization
# + changeRequestId - Unique ID of the change request to be updated
# + payload - Change request update payload containing details to be updated in the change request
# + return - Change request update response containing details of the updated change request or error
public isolated function updateChangeRequest(string idToken, string changeRequestId, ChangeRequestUpdatePayload payload)
    returns ChangeRequestUpdateResponse|error {

    return csEntityClient->/change\-requests/[changeRequestId].patch(payload, generateHeaders(idToken));
}

# Search catalogs.
#
# + idToken - ID token for authorization
# + payload - Catalog search payload containing search criteria for catalogs
# + return - Catalog search response containing matching catalogs or error
public isolated function searchCatalogs(string idToken, CatalogSearchPayload payload)
    returns CatalogSearchResponse|error {

    return csEntityClient->/catalogs/search.post(payload, generateHeaders(idToken));
}

# Get catalog item variables.
#
# + idToken - ID token for authorization
# + catalogId - Unique ID of the catalog to which the catalog item belongs
# + catalogItemId - Unique ID of the catalog item for which variables are to be retrieved
# + return - Catalog item variables response containing variables of the specified catalog item or error
public isolated function getCatalogItemVariable(string idToken, string catalogId, string catalogItemId)
    returns CatalogItemVariablesResponse|error {

    return csEntityClient->/catalogs/[catalogId]/items/[catalogItemId]/variables.get(generateHeaders(idToken));
}

# Get project change request statistics.
#
# + idToken - ID token for authorization
# + projectId - Unique ID of the project for which change request statistics are to be retrieved
# + return - Project change request statistics response or error
public isolated function getProjectChangeRequestStats(string idToken, string projectId)
    returns ProjectChangeRequestStatsResponse|error {

    return csEntityClient->/projects/[projectId]/change\-requests/stats.get(generateHeaders(idToken));
}

# Search instances by criteria.
# 
# + idToken - ID token for authorization
# + payload - Instance search payload containing search criteria for instances
# + return - Instances response containing matching instances or error
public isolated function searchInstances(string idToken, InstanceSearchPayload payload) 
    returns InstancesResponse|error {

    return csEntityClient->/instances/search.post(payload, generateHeaders(idToken));
}

# Search instance metrics by criteria.
#
# + idToken - ID token for authorization
# + payload - Instance metrics search payload containing search criteria for instance metrics
# + return - Instance metrics response containing matching instance metrics or error
public isolated function searchInstanceMetrics(string idToken, InstanceMetricsPayload payload)
    returns InstanceMetricsResponse|error {

    return csEntityClient->/instances/metrics/search.post(payload, generateHeaders(idToken));
}

# Search instance usage by criteria.
#
# + idToken - ID token for authorization
# + payload - Instance usage search payload containing search criteria for instance usage summary
# + return - Instance usage response containing matching instance usage summary or error
public isolated function searchInstanceUsage(string idToken, InstanceUsagePayload payload)
    returns InstanceUsageResponse|error {

    return csEntityClient->/instances/usages/search.post(payload, generateHeaders(idToken));
}
