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
import customer_portal.entity;

import ballerina/http;
import ballerina/log;

# Search cases for a given project.
#
# + idToken - ID token for authorization
# + projectId - Project ID to filter cases
# + payload - Case search payload
# + return - Case search response or error
public isolated function searchCases(string idToken, string projectId, CaseSearchPayload payload)
    returns CaseSearchResponse|error {

    int? issueId = payload.filters?.issueId;
    entity:CaseSearchPayload searchPayload = {
        filters: {
            projectIds: [projectId],
            searchQuery: payload.filters?.searchQuery,
            issueTypeKeys: issueId != () ? [issueId] : (),
            severityKey: payload.filters?.severityId,
            stateKey: payload.filters?.statusId,
            deploymentId: payload.filters?.deploymentId
        },
        pagination: payload.pagination,
        sortBy: payload.sortBy
    };
    entity:CaseSearchResponse casesResponse = check entity:searchCases(idToken, searchPayload);
    Case[] cases = from entity:Case case in casesResponse.cases
        let entity:ReferenceTableItem? project = case.project
        let entity:ReferenceTableItem? deployedProduct = case.deployedProduct
        let entity:ChoiceListItem? issueType = case.issueType
        let entity:ReferenceTableItem? deployment = case.deployment
        let entity:ReferenceTableItem? assignedEngineer = case.assignedEngineer
        let entity:ChoiceListItem? severity = case.severity
        let entity:ChoiceListItem? state = case.state
        select {
            id: case.id,
            internalId: case.internalId,
            number: case.number,
            title: case.title,
            createdOn: case.createdOn,
            description: case.description,
            project: project != () ? {id: project.id, label: project.name} : (),
            deployedProduct: deployedProduct != () ? {id: deployedProduct.id, label: deployedProduct.name} : (),
            issueType: issueType != () ? {id: issueType.id.toString(), label: issueType.label} : (),
            deployment: deployment != () ? {id: deployment.id, label: deployment.name} : (),
            assignedEngineer: assignedEngineer != () ? {id: assignedEngineer.id, label: assignedEngineer.name} : (),
            severity: severity != () ? {id: severity.id.toString(), label: severity.label} : (),
            status: state != () ? {id: state.id.toString(), label: state.label} : ()
        };

    return {
        cases,
        totalRecords: casesResponse.totalRecords,
        'limit: casesResponse.'limit,
        offset: casesResponse.offset
    };
}

# Get case filters for a given project.
#
# + caseMetadata - Case metadata response
# + return - Case filters or error
public isolated function getCaseFilters(entity:CaseMetadataResponse caseMetadata) returns CaseFilterOptions {
    ReferenceItem[] statuses = from entity:ChoiceListItem item in caseMetadata.states
        select {id: item.id.toString(), label: item.label};
    ReferenceItem[] severities = from entity:ChoiceListItem item in caseMetadata.severities
        select {id: item.id.toString(), label: item.label};
    ReferenceItem[] issueTypes = from entity:ChoiceListItem item in caseMetadata.issueTypes
        select {id: item.id.toString(), label: item.label};

    // TODO: Other project specific filters will be added later
    return {
        statuses,
        severities,
        issueTypes
    };
}

# Check if the given ID is empty or whitespace.
#
# + id - ID to validate
# + return - True if empty/whitespace, else false
public isolated function isEmptyId(string id) returns boolean => id.trim().length() == 0;

# Get HTTP status code from the given error.
#
# + err - Error to handle
# + return - HTTP status code
public isolated function getStatusCode(error err) returns int {
    map<anydata|readonly> & readonly errorDetails = err.detail();
    anydata|readonly statusCodeValue = errorDetails[ERR_STATUS_CODE] ?: ();
    return statusCodeValue is int ? statusCodeValue : http:STATUS_INTERNAL_SERVER_ERROR;
}

# Get HTTP status code from the given error.
#
# + err - Error to handle
# + return - Error message
public isolated function extractErrorMessage(error err) returns string {
    map<anydata|readonly> & readonly errorDetails = err.detail();
    anydata|readonly errorMessage = errorDetails[ERR_BODY] ?: ();
    return errorMessage is string ? errorMessage : UNEXPECTED_ERROR_MSG;
}

# Log forbidden project access attempt.
#
# + id - Project ID
# + uuid - User UUID
public isolated function logForbiddenProjectAccess(string id, string uuid) =>
    log:printWarn(string `Access to project ID: ${id} is forbidden for user: ${uuid}`);

# Log forbidden case access attempt.
#
# + id - Case ID
# + uuid - User UUID
public isolated function logForbiddenCaseAccess(string id, string uuid) =>
    log:printWarn(string `Access to case ID: ${id} is forbidden for user: ${uuid}`);

# Map comments response to map to desired structure.
#
# + response - Comments response from the entity service
# + return - Map comments response
public isolated function mapCommentsResponse(entity:CommentsResponse response) returns CommentsResponse {
    Comment[] comments = from entity:Comment comment in response.comments
        select {
            id: comment.id,
            content: comment.content,
            'type: comment.'type,
            createdOn: comment.createdOn,
            createdBy: comment.createdBy,
            isEscalated: comment.isEscalated,
            hasInlineAttachments: comment.hasInlineAttachments,
            inlineAttachments: comment.inlineAttachments
        };

    return {
        comments,
        totalRecords: response.totalRecords,
        'limit: response.'limit,
        offset: response.offset
    };
}

# Validate limit and offset values.
#
# + limit - Limit value
# + offset - Offset value
# + return - True if invalid, else false
public isolated function isInvalidLimitOffset(int? 'limit, int? offset) returns boolean =>
    ('limit != () && ('limit < 1 || 'limit > 50)) || (offset != () && offset < 0);

# Map attachments response to map to desired structure.
#
# + response - Attachments response from the entity service
# + return - Mapped attachments response
public isolated function mapAttachmentsResponse(entity:AttachmentsResponse response) returns AttachmentsResponse {
    Attachment[] attachments = from entity:Attachment attachment in response.attachments
        select {
            id: attachment.id,
            name: attachment.name,
            'type: attachment.'type,
            size: attachment.sizeBytes,
            createdBy: attachment.createdBy,
            createdOn: attachment.createdOn,
            downloadUrl: attachment.downloadUrl
        };

    return {
        attachments,
        totalRecords: response.totalRecords,
        'limit: response.'limit,
        offset: response.offset
    };
}

# Map deployments response to the desired structure.
#
# + response - Deployments response from the entity service
# + return - Mapped deployments response
public isolated function mapDeployments(entity:DeploymentsResponse response) returns Deployment[] {
    return from entity:Deployment deployment in response.deployments
        let entity:ReferenceTableItem? project = deployment.project
        let entity:ChoiceListItem? 'type = deployment.'type
        select {
            id: deployment.id,
            name: deployment.name,
            createdOn: deployment.createdOn,
            updatedOn: deployment.updatedOn,
            description: deployment.description,
            url: deployment.url,
            project: project != () ? {id: project.id, label: project.name} : (),
            'type: 'type != () ? {id: 'type.id.toString(), label: 'type.label} : ()
        };
}

# Map deployed products response to the desired structure.
#
# + response - Deployed products response from the entity service
# + return - Mapped deployed products response
public isolated function mapDeployedProducts(entity:DeployedProductsResponse response)
    returns DeployedProduct[] {

    return from entity:DeployedProduct product in response.deployedProducts
        let entity:ReferenceTableItem? associatedProduct = product.product
        let entity:ReferenceTableItem? deployment = product.deployment
        select {
            id: product.id,
            createdOn: product.createdOn,
            updatedOn: product.updatedOn,
            description: product.description,
            product: associatedProduct != () ? {id: associatedProduct.id, label: associatedProduct.name} : (),
            deployment: deployment != () ? {id: deployment.id, label: deployment.name} : ()
        };
}
