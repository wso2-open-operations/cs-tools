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
import customer_portal.types;

import ballerina/http;
import ballerina/log;

configurable int stateIdOpen = 1;
configurable types:FeatureFlags featureFlags = {
    usageMetricsEnabled: true
};
configurable string[] restrictedChangeRequestStateIds = ["-3", "-4", "-5"];

# Search cases for a given project.
#
# + idToken - ID token for authorization
# + projectId - Project ID to filter cases
# + payload - Case search payload
# + return - Case search response or error
public isolated function searchCases(string idToken, string projectId, types:CaseSearchPayload payload)
    returns types:CaseSearchResponse|error {

    int? issueId = payload.filters?.issueId;
    entity:CaseSearchPayload searchPayload = {
        filters: {
            projectIds: [projectId],
            searchQuery: payload.filters?.searchQuery,
            issueTypeKeys: issueId != () ? [issueId] : (),
            severityKey: payload.filters?.severityId,
            caseTypes: payload.filters?.caseTypes,
            stateKeys: payload.filters?.statusIds,
            deploymentId: payload.filters?.deploymentId,
            createdByMe: payload.filters?.createdByMe
        },
        pagination: payload.pagination,
        sortBy: payload.sortBy
    };
    entity:CaseSearchResponse casesResponse = check entity:searchCases(idToken, searchPayload);
    types:CaseMetaData[] cases = from entity:Case case in casesResponse.cases
        let entity:ReferenceTableItem? project = case.project
        let entity:ReferenceTableItem? 'type = case.caseType
        let entity:ReferenceTableItem? deployedProduct = case.deployedProduct
        let entity:ChoiceListItem? issueType = case.issueType
        let entity:ReferenceTableItem? deployment = case.deployment
        let entity:ReferenceTableItem? assignedEngineer = case.assignedEngineer
        let entity:ReferenceTableItem? parentCase = case.parentCase
        let entity:ReferenceTableItem? relatedCase = case.relatedCase
        let entity:ReferenceTableItem? conversation = case.conversation
        let entity:ChoiceListItem? severity = case.severity
        let entity:ChoiceListItem? state = case.state
        let entity:ChoiceListItem? engagementType = case.engagementType
        let entity:ReferenceTableItem? catalog = case?.catalog
        let entity:ReferenceTableItem? catalogItem = case?.catalogItem
        let entity:ReferenceTableItem? assignedTeam = case.assignedTeam
        let entity:ReferenceTableItem? product = case.product
        select {
            id: case.id,
            internalId: case.internalId,
            number: case.number,
            title: case.title,
            createdOn: case.createdOn,
            createdBy: case.createdBy,
            description: case.description,
            duration: case.duration,
            project: project != () ? {id: project.id, label: project.name} : (),
            'type: 'type != () ? {id: 'type.id, label: 'type.name} : (),
            deployedProduct: deployedProduct != () ? {id: deployedProduct.id, label: deployedProduct.name} : (),
            issueType: issueType != () ? {id: issueType.id.toString(), label: issueType.label} : (),
            deployment: deployment != () ? {id: deployment.id, label: deployment.name} : (),
            assignedEngineer: assignedEngineer != () ? {id: assignedEngineer.id, label: assignedEngineer.name} : (),
            parentCase: parentCase != () ? {id: parentCase.id, label: parentCase.name} : (),
            relatedCase: relatedCase != () ? {id: relatedCase.id, label: relatedCase.name} : (),
            conversation: conversation != () ? {id: conversation.id, label: conversation.name} : (),
            severity: severity != () ? {id: severity.id.toString(), label: severity.label} : (),
            status: state != () ? {id: state.id.toString(), label: state.label} : (),
            engagementType: engagementType != () ? {id: engagementType.id.toString(), label: engagementType.label} : (),
            catalog: catalog != () ? {id: catalog.id, label: catalog.name} : (),
            catalogItem: catalogItem != () ? {id: catalogItem.id, label: catalogItem.name} : (),
            assignedTeam: assignedTeam != () ? {id: assignedTeam.id, label: assignedTeam.name} : (),
            product: product != () ? {id: product.id, label: product.name} : ()
        };

    return {
        cases,
        totalRecords: casesResponse.totalRecords,
        'limit: casesResponse.'limit,
        offset: casesResponse.offset
    };
}

# Map project features for a given project.
#
# + projectMetadata - Project metadata response
# + return - Project features or error
public isolated function mapProjectFeatures(entity:ProjectMetadataResponse projectMetadata)
    returns types:ProjectFeatures {

    types:ReferenceItem[] acceptedSeverityValues =
        from entity:ChoiceListItem item in projectMetadata.features.acceptedSeverityValues
    select {id: item.id.toString(), label: item.label};
    return {
        acceptedSeverityValues,
        hasServiceRequestWriteAccess: projectMetadata.features.hasServiceRequestWriteAccess,
        hasServiceRequestReadAccess: projectMetadata.features.hasServiceRequestReadAccess,
        hasSraWriteAccess: projectMetadata.features.hasSraWriteAccess,
        hasSraReadAccess: projectMetadata.features.hasSraReadAccess,
        hasChangeRequestReadAccess: projectMetadata.features.hasChangeRequestReadAccess,
        hasEngagementsReadAccess: projectMetadata.features.hasEngagementsReadAccess,
        hasUpdatesReadAccess: projectMetadata.features.hasUpdatesReadAccess,
        hasTimeLogsReadAccess: projectMetadata.features.hasTimeLogsReadAccess,
        hasDeploymentWriteAccess: projectMetadata.features.hasDeploymentWriteAccess,
        hasDeploymentReadAccess: projectMetadata.features.hasDeploymentReadAccess,
        defaultCaseProductCategories: projectMetadata.features.defaultCaseProductCategories,
        srProductCategories: projectMetadata.features.srProductCategories
    };
}

# Get project filters for a given project.
#
# + projectMetadata - Project metadata response
# + return - Project filters or error
public isolated function getProjectFilters(entity:ProjectMetadataResponse projectMetadata)
    returns types:ProjectFilterOptions {

    types:ReferenceItem[] caseStates = from entity:ChoiceListItem item in projectMetadata.caseStates
        select {id: item.id.toString(), label: item.label};
    types:ReferenceItem[] severities = from entity:ChoiceListItem item in projectMetadata.severities
        select {id: item.id.toString(), label: item.label};
    types:ReferenceItem[] issueTypes = from entity:ChoiceListItem item in projectMetadata.issueTypes
        select {id: item.id.toString(), label: item.label};
    types:ReferenceItem[] deploymentTypes = from entity:ChoiceListItem item in projectMetadata.deploymentTypes
        select {id: item.id.toString(), label: item.label};
    types:ReferenceItem[] callRequestStates = from entity:ChoiceListItem item in projectMetadata.callRequestStates
        select {id: item.id.toString(), label: item.label};
    types:ReferenceItem[] changeRequestStates = from entity:ChoiceListItem item in projectMetadata.changeRequestStates
        select {id: item.id.toString(), label: item.label};
    types:ReferenceItem[] timeCardStates = from entity:ChoiceListItem item in projectMetadata.timeCardStates
        select {id: item.id.toString(), label: item.label};
    types:ReferenceItem[] changeRequestImpacts = from entity:ChoiceListItem item in projectMetadata.changeRequestImpacts
        select {id: item.id.toString(), label: item.label};
    types:ReferenceItem[] caseTypes = from entity:ReferenceTableItem item in projectMetadata.caseTypes
        select {id: item.id, label: item.name};
    types:ReferenceItem[] conversationStates = from entity:ChoiceListItem item in projectMetadata.conversationStates
        select {id: item.id.toString(), label: item.label};
    types:ReferenceItem[] engagementTypes = from entity:ChoiceListItem item in projectMetadata.engagementTypes
        select {id: item.id.toString(), label: item.label};
    types:ReferenceItem[] engagementPaymentTypes = from entity:ChoiceListItem item in projectMetadata.engagementPaymentTypes
        select {id: item.id.toString(), label: item.label};

    types:ReferenceItem[] nonRestrictedChangeRequestStates =
        from types:ReferenceItem changeRequestState in changeRequestStates
    where restrictedChangeRequestStateIds.indexOf(changeRequestState.id) is ()
    select changeRequestState;

    return {
        caseStates,
        severities,
        issueTypes,
        deploymentTypes,
        callRequestStates,
        changeRequestStates: nonRestrictedChangeRequestStates,
        changeRequestImpacts,
        caseTypes,
        conversationStates,
        timeCardStates,
        engagementTypes,
        engagementPaymentTypes,
        severityBasedAllocationTime: projectMetadata.severityBasedAllocationTime
    };
}

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
public isolated function mapCommentsResponse(entity:CommentsResponse response) returns types:CommentsResponse {
    types:Comment[] comments = from entity:Comment comment in response.comments
        select {
            id: comment.id,
            content: comment.content,
            'type: comment.'type,
            createdOn: comment.createdOn,
            createdBy: comment.createdBy,
            isEscalated: comment.isEscalated,
            hasInlineAttachments: comment.hasInlineAttachments,
            inlineAttachments: comment.inlineAttachments,
            createdByFirstName: comment.createdByFirstName,
            createdByLastName: comment.createdByLastName,
            createdByFullName: comment.createdByFullName
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
public isolated function mapAttachmentsResponse(entity:AttachmentsResponse response) returns types:AttachmentsResponse {
    types:Attachment[] attachments = from entity:Attachment attachment in response.attachments
        select {
            id: attachment.id,
            name: attachment.name,
            'type: attachment.'type,
            size: attachment.sizeBytes,
            createdBy: attachment.createdBy,
            createdOn: attachment.createdOn,
            downloadUrl: attachment.downloadUrl,
            previewUrl: attachment.previewUrl,
            description: attachment.description
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
public isolated function mapDeployments(entity:DeploymentsResponse response) returns types:DeploymentsResponse {
    types:Deployment[] deployments = from entity:Deployment deployment in response.deployments
        let entity:ReferenceTableItem? project = deployment.project
        let entity:ChoiceListItem? 'type = deployment.'type
        select {
            id: deployment.id,
            name: deployment.name,
            number: deployment.number,
            createdOn: deployment.createdOn,
            updatedOn: deployment.updatedOn,
            description: deployment.description,
            url: deployment.url,
            project: project != () ? {id: project.id, label: project.name} : (),
            'type: 'type != () ? {id: 'type.id.toString(), label: 'type.label} : ()
        };
    return {deployments, totalRecords: response.totalRecords, offset: response.offset, 'limit: response.'limit};
}

# Map deployed products response to the desired structure.
#
# + response - Deployed products response from the entity service
# + return - Mapped deployed products response
public isolated function mapDeployedProducts(entity:DeployedProductsResponse response)
    returns types:DeployedProductsResponse {

    types:DeployedProduct[] deployedProducts = from entity:DeployedProduct product in response.deployedProducts
        let entity:ReferenceTableItem? associatedProduct = product.product
        let entity:ReferenceTableItem? deployment = product.deployment
        let entity:ReferenceTableItem? version = product.version
        let entity:ReferenceTableItem? category = product.category
        select {
            id: product.id,
            createdOn: product.createdOn,
            updatedOn: product.updatedOn,
            description: product.description,
            cores: product.cores,
            tps: product.tps,
            updates: product.updates,
            product: associatedProduct != () ? {
                    id: associatedProduct.id,
                    label: associatedProduct.name,
                    abbreviation: associatedProduct?.abbreviation
                } : (),
            deployment: deployment != () ? {id: deployment.id, label: deployment.name} : (),
            version: version != () ? {
                    id: version.id,
                    label: version.name,
                    releasedOn: associatedProduct?.releasedOn,
                    endOfLifeOn: associatedProduct?.endOfLifeOn
                } : (),
            category: category != () ? {id: category.id, label: category.name} : ()
        };

    return {
        deployedProducts,
        totalRecords: response.totalRecords,
        'limit: response.'limit,
        offset: response.offset
    };
}

# Map created case response to the desired structure.
#
# + createdCase - Created case response from the entity service
# + return - Mapped created case response
public isolated function mapCreatedCase(entity:CreatedCase createdCase) returns types:CreatedCase {
    return {
        id: createdCase.id,
        internalId: createdCase.internalId,
        number: createdCase.number,
        createdBy: createdCase.createdBy,
        createdOn: createdCase.createdOn,
        state: {id: createdCase.state.id.toString(), label: createdCase.state.label},
        'type: {id: createdCase.'type.id.toString(), label: createdCase.'type.name}
    };
}

# Map product vulnerability search response to the desired structure.
#
# + response - Product vulnerability search response from the entity service
# + return - Mapped product vulnerability search response
public isolated function mapProductVulnerabilitySearchResponse(entity:ProductVulnerabilitySearchResponse response)
    returns types:ProductVulnerabilitySearchResponse {

    types:ProductVulnerability[] productVulnerabilities =
    from entity:ProductVulnerability vulnerability in response.productVulnerabilities
    select {
        id: vulnerability.id,
        cveId: vulnerability.cveId,
        vulnerabilityId: vulnerability.vulnerabilityId,
        severity: {id: vulnerability.severity.id.toString(), label: vulnerability.severity.label},
        productName: vulnerability.productName,
        productVersion: vulnerability.productVersion,
        componentName: vulnerability.componentName,
        version: vulnerability.version,
        'type: vulnerability.'type,
        componentType: vulnerability.componentType,
        updateLevel: vulnerability.updateLevel,
        useCase: vulnerability.useCase,
        justification: vulnerability.justification,
        resolution: vulnerability.resolution
    };
    return {
        productVulnerabilities,
        totalRecords: response.totalRecords,
        'limit: response.'limit,
        offset: response.offset
    };
}

# Map product vulnerability response to the desired structure.
#
# + response - Product vulnerability response from the entity service
# + return - Mapped product vulnerability response
public isolated function mapProductVulnerabilityResponse(entity:ProductVulnerabilityResponse response)
    returns types:ProductVulnerabilityResponse {

    return {
        id: response.id,
        cveId: response.cveId,
        vulnerabilityId: response.vulnerabilityId,
        severity: {id: response.severity.id.toString(), label: response.severity.label},
        productName: response.productName,
        productVersion: response.productVersion,
        componentName: response.componentName,
        version: response.version,
        'type: response.'type,
        useCase: response.useCase,
        justification: response.justification,
        resolution: response.resolution,
        componentType: response.componentType,
        updateLevel: response.updateLevel
    };
}

# Map product vulnerability metadata response to the desired structure.
#
# + response - Product vulnerability metadata response from the entity service
# + return - Mapped product vulnerability metadata response
public isolated function mapProductVulnerabilityMetadataResponse(entity:VulnerabilityMetaResponse response)
    returns types:ProductVulnerabilityMetaResponse {

    types:ReferenceItem[] severities = from entity:ChoiceListItem item in response.severities
        select {id: item.id.toString(), label: item.label};
    return {severities};
}

# Map project case stats response to the desired structure.
#
# + response - Project case stats response from the entity service
# + return - Mapped project case stats response
public isolated function mapCaseStats(entity:ProjectCaseStatsResponse response) returns types:ProjectCaseStats {
    types:ReferenceItem[] stateCount = from entity:ChoiceListItem item in response.stateCount
        select {id: item.id.toString(), label: item.label, count: item.count};
    types:ReferenceItem[] severityCount = from entity:ChoiceListItem item in response.severityCount
        select {id: item.id.toString(), label: item.label, count: item.count};
    types:ReferenceItem[] outstandingSeverityCount =
        from entity:ChoiceListItem item in response.outstandingSeverityCount
    select {id: item.id.toString(), label: item.label, count: item.count};
    types:ReferenceItem[] caseTypeCount = from entity:ReferenceTableItem item in response.caseTypeCount
        select {id: item.id, label: item.name, count: item.count};
    types:ReferenceItem[] engagementTypeCount = from entity:ChoiceListItem item in response.engagementTypeCount
        select {id: item.id.toString(), label: item.label, count: item.count};
    types:ReferenceItem[] outstandingEngagementTypeCount =
    from entity:ChoiceListItem item in response.outstandingEngagementTypeCount
    select {id: item.id.toString(), label: item.label, count: item.count};

    return {
        totalCount: response.totalCount,
        averageResponseTime: response.averageResponseTime,
        resolvedCases: response.resolvedCount,
        changeRate: response.changeRate,
        activeCount: response.activeCount,
        outstandingCount: response.outstandingCount,
        actionRequiredCount: response.actionRequiredCount,
        stateCount,
        severityCount,
        outstandingSeverityCount,
        caseTypeCount,
        engagementTypeCount,
        outstandingEngagementTypeCount,
        casesTrend: response.casesTrend
    };
}

# Get open cases count from project case stats response.
#
# + response - Project case stats response from the entity service
# + return - Count of open cases, or null if not available
public isolated function getOpenCasesCountFromProjectCasesStats(entity:ProjectCaseStatsResponse response) returns int? {
    types:ProjectCaseStats stats = mapCaseStats(response);
    types:ReferenceItem[] openCases = stats.stateCount.filter(stat => stat.id == stateIdOpen.toString());
    return openCases.length() > 0 ? openCases[0].count : ();
}

# Map call requests response to the desired structure.
#
# + response - Call requests response from the entity service
# + return - Mapped call requests response
public isolated function mapSearchCallRequestResponse(entity:CallRequestsResponse response)
    returns types:CallRequestsResponse {

    types:CallRequest[] callRequests = from entity:CallRequest callRequest in response.callRequests
        let entity:ReferenceTableItem case = callRequest.case
        let entity:ChoiceListItem state = callRequest.state
        select {
            id: callRequest.id,
            reason: callRequest.reason,
            preferredTimes: callRequest.preferredTimes,
            durationMin: callRequest.durationMin,
            scheduleTime: callRequest.scheduleTime,
            createdOn: callRequest.createdOn,
            updatedOn: callRequest.updatedOn,
            case: {id: case.id, label: case.name},
            state: {id: state.id.toString(), label: state.label},
            number: callRequest.number,
            meetingLink: callRequest.meetingLink
        };

    return {callRequests, totalRecords: response.totalRecords, 'limit: response.'limit, offset: response.offset};
}

# Map product versions response to the desired structure.
#
# + response - Product versions response from the entity service
# + return - Mapped product versions response
public isolated function mapProductVersionsResponse(entity:ProductVersionsResponse response)
    returns types:ProductVersionsResponse {

    types:ProductVersion[] versions = from entity:ProductVersion version in response.versions
        let entity:ReferenceTableItem? product = version.product
        select {
            id: version.id,
            version: version.version,
            currentSupportStatus: version.currentSupportStatus,
            releaseDate: version.releaseDate,
            supportEolDate: version.supportEolDate,
            earliestPossibleSupportEolDate: version.earliestPossibleSupportEolDate,
            product: product != () ? {id: product.id, label: product.name} : ()
        };
    return {versions, totalRecords: response.totalRecords, 'limit: response.'limit, offset: response.offset};
}

# Map time cards search response to the desired structure.
#
# + response - Time cards search response from the entity service
# + return - Mapped time cards search response
public isolated function mapTimeCardSearchResponse(entity:TimeCardsResponse response) returns types:TimeCardsResponse {
    types:TimeCard[] timeCards = from entity:TimeCard timeCard in response.timeCards
        let entity:ReferenceTableItem? approvedBy = timeCard.approvedBy
        let entity:ReferenceTableItem? reportedBy = timeCard.user
        let entity:ReferenceTableItem? project = timeCard.project
        let entity:TimeCardCase? case = timeCard.case
        let entity:ChoiceListItem? state = timeCard.state
        select {
            id: timeCard.id,
            totalTime: timeCard.totalTime,
            createdOn: timeCard.createdOn,
            hasBillable: timeCard.hasBillable,
            state: state != () ? {id: state.id.toString(), label: state.label} : (),
            reportedBy: reportedBy != () ? {id: reportedBy.id, label: reportedBy.name} : (),
            approvedBy: approvedBy != () ? {id: approvedBy.id, label: approvedBy.name} : (),
            project: project != () ? {id: project.id, label: project.name} : (),
            case: case != () ? {id: case.id, label: case.name, number: case.number} : ()
        };

    return {timeCards, totalRecords: response.totalRecords, 'limit: response.'limit, offset: response.offset};
}

# Map conversation search response to the desired structure.
#
# + response - Conversation search response from the entity service
# + return - Mapped conversation search response
public isolated function mapConversationSearchResponse(entity:ConversationSearchResponse response)
    returns types:ConversationSearchResponse {

    types:Conversation[] conversations = from entity:Conversation conversation in response.conversations
        let entity:ReferenceTableItem? project = conversation.project
        let entity:ReferenceTableItem? case = conversation.case
        let entity:ChoiceListItem? state = conversation.state
        select {
            id: conversation.id,
            number: conversation.number,
            initialMessage: conversation.initialMessage,
            messageCount: conversation.messageCount,
            createdOn: conversation.createdOn,
            createdBy: conversation.createdBy,
            project: project != () ? {id: project.id, label: project.name} : (),
            case: case != () ? {id: case.id, label: case.name} : (),
            state: state != () ? {id: state.id.toString(), label: state.label} : ()
        };

    return {
        conversations,
        totalRecords: response.totalRecords,
        'limit: response.'limit,
        offset: response.offset
    };
}

# Map case response to the desired structure.
#
# + response - Case response from the entity service
# + return - Mapped case response
public isolated function mapCaseResponse(entity:CaseResponse response) returns types:CaseResponse {
    entity:ReferenceTableItem? project = response.project;
    entity:ReferenceTableItem? caseType = response.caseType;
    entity:ChoiceListItem? issueType = response.issueType;
    entity:ReferenceTableItem? assignedEngineer = response.assignedEngineer;
    entity:ReferenceTableItem? parentCase = response.parentCase;
    entity:ReferenceTableItem? relatedCase = response.relatedCase;
    entity:ReferenceTableItem? conversation = response.conversation;
    entity:ChoiceListItem? severity = response.severity;
    entity:ChoiceListItem? state = response.state;
    entity:ReferenceTableItem? catalog = response?.catalog;
    entity:ReferenceTableItem? catalogItem = response?.catalogItem;
    entity:ReferenceTableItem? assignedTeam = response.assignedTeam;
    entity:ReferenceTableItem[]? changeRequests = response?.changeRequests;
    entity:ChoiceListItem? engagementPaymentType = response.engagementPaymentType;
    entity:ServiceRequestVariable[]? variables = response?.variables;
    entity:ReferenceTableItem? product = response.product;
    entity:ChoiceListItem? engagementType = response.engagementType;

    return {
        id: response.id,
        internalId: response.internalId,
        number: response.number,
        title: response.title,
        description: response.description,
        duration: response.duration,
        createdOn: response.createdOn,
        createdBy: response.createdBy,
        slaResponseTime: response.slaResponseTime,
        project: project != () ? {id: project.id, label: project.name} : (),
        'type: caseType != () ? {id: caseType.id, label: caseType.name} : (),
        issueType: issueType != () ? {id: issueType.id.toString(), label: issueType.label} : (),
        assignedEngineer: assignedEngineer != () ? {id: assignedEngineer.id, label: assignedEngineer.name} : (),
        parentCase: parentCase != () ? {id: parentCase.id, label: parentCase.name} : (),
        relatedCase: relatedCase != () ? {id: relatedCase.id, label: relatedCase.name} : (),
        conversation: conversation != () ? {id: conversation.id, label: conversation.name} : (),
        severity: severity != () ? {id: severity.id.toString(), label: severity.label} : (),
        status: state != () ? {id: state.id.toString(), label: state.label} : (),
        engagementType: engagementType != () ? {id: engagementType.id.toString(), label: engagementType.label} : (),
        catalog: catalog != () ? {id: catalog.id, label: catalog.name} : (),
        catalogItem: catalogItem != () ? {id: catalogItem.id, label: catalogItem.name} : (),
        assignedTeam: assignedTeam != () ? {id: assignedTeam.id, label: assignedTeam.name} : (),
        product: product != () ? {id: product.id, label: product.name} : (),
        updatedOn: response.updatedOn,
        deployedProduct: response.deployedProduct != () ? {
                id: response.deployedProduct?.id ?: "",
                label: response.deployedProduct?.name ?: "",
                version: response.deployedProduct?.version
            } : (),
        deployment: response.deployment != () ? {
                id: response.deployment?.id ?: "",
                label: response.deployment?.name ?: "",
                'type: response.deployment?.'type
            } : (),
        account: response.account != () ? {
                id: response?.account?.id ?: "",
                label: response?.account?.name ?: "",
                count: response?.account?.count,
                'type: response?.account?.'type
            } : (),
        csManager: response?.csManager != () ? {
                id: response?.csManager?.id ?: "",
                label: response?.csManager?.name ?: "",
                count: response?.csManager?.count,
                email: response?.csManager?.email ?: ""
            } : (),
        closeNotes: response?.closeNotes,
        closedOn: response?.closedOn,
        closedBy: response?.closedBy != () ? {
                id: response?.closedBy?.id ?: "",
                label: response?.closedBy?.name ?: "",
                count: response?.closedBy?.count
            } : (),
        changeRequests: changeRequests != () ? from entity:ReferenceTableItem item in changeRequests
                select {id: item.id, label: item.name} : (),
        engagementPaymentType: engagementPaymentType != () ? {
                id: engagementPaymentType.id.toString(),
                label: engagementPaymentType.label
            } : (),
        hasAutoClosed: response?.hasAutoClosed,
        engagementStartDate: response?.engagementStartDate,
        engagementEndDate: response?.engagementEndDate,
        variables
    };
}

# Map conversation response to the desired structure.
#
# + response - Conversation response from the entity service
# + return - Mapped conversation response
public isolated function mapConversationResponse(entity:ConversationResponse response)
    returns types:ConversationResponse {

    return {
        id: response.id,
        number: response.number,
        initialMessage: response.initialMessage,
        messageCount: response.messageCount,
        createdOn: response.createdOn,
        createdBy: response.createdBy,
        project: response.project != () ? {id: response.project?.id ?: "", label: response.project?.name ?: ""} : (),
        case: response.case != () ? {id: response.case?.id ?: "", label: response.case?.name ?: ""} : (),
        state: response.state != () ?
            {id: response.state?.id.toString(), label: response.state?.label ?: ""} : (),
        updatedBy: response.updatedBy,
        updatedOn: response.updatedOn
    };
}

# Get conversation stats from project conversation stats response.
#
# + response - Project conversation stats response from the entity service
# + return - Conversation stats with counts for each conversation state
public isolated function getConversationStats(entity:ProjectConversationStatsResponse|error response)
    returns types:OverallConversationStats {

    if response is entity:ProjectConversationStatsResponse {
        types:ReferenceItem[] mappedConversationStats = from entity:ChoiceListItem item in response.stateCount
            select {id: item.id.toString(), label: item.label, count: item.count};

        types:ReferenceItem[] openCases = mappedConversationStats.filter(stat =>
        stat.id == entity:conversationStateIds.open.toString());
        types:ReferenceItem[] activeCases = mappedConversationStats.filter(stat =>
        stat.id == entity:conversationStateIds.active.toString());
        types:ReferenceItem[] resolvedCases = mappedConversationStats.filter(stat =>
        stat.id == entity:conversationStateIds.resolved.toString());
        types:ReferenceItem[] convertedCases = mappedConversationStats.filter(stat =>
        stat.id == entity:conversationStateIds.converted.toString());
        types:ReferenceItem[] abandondedCases = mappedConversationStats.filter(stat =>
        stat.id == entity:conversationStateIds.abandonded.toString());
        // TODO: Add session chats after entity service supports it

        return {
            openCount: openCases.length() > 0 ? openCases[0].count : (),
            activeCount: activeCases.length() > 0 ? activeCases[0].count : (),
            resolvedCount: resolvedCases.length() > 0 ? resolvedCases[0].count : (),
            convertedCount: convertedCases.length() > 0 ? convertedCases[0].count : (),
            abandonedCount: abandondedCases.length() > 0 ? abandondedCases[0].count : ()
        };
    }
    return {
        openCount: (),
        activeCount: (),
        resolvedCount: (),
        convertedCount: (),
        abandonedCount: (),
        sessionCount: ()
    };
}

# Map change request search response to the desired structure.
#
# + response - Change request search response from the entity service
# + return - Mapped change request search response
public isolated function mapChangeRequestSearchResponse(entity:ChangeRequestSearchResponse response)
    returns types:ChangeRequestSearchResponse {

    types:ChangeRequest[] changeRequests = from entity:ChangeRequest changeRequest in response.changeRequests
        let entity:ReferenceTableItem? project = changeRequest.project
        let entity:ReferenceTableItem? case = changeRequest.case
        let entity:ReferenceTableItem? deployment = changeRequest.deployment
        let entity:ReferenceTableItem? deployedProduct = changeRequest.deployedProduct
        let entity:ReferenceTableItem? product = changeRequest.product
        let entity:ReferenceTableItem? assignedEngineer = changeRequest.assignedEngineer
        let entity:ReferenceTableItem? assignedTeam = changeRequest.assignedTeam
        let entity:ChoiceListItem? state = changeRequest.state
        let entity:ChoiceListItem? impact = changeRequest.impact
        let entity:ChoiceListItem? 'type = changeRequest.'type
        select {
            id: changeRequest.id,
            number: changeRequest.number,
            title: changeRequest.title,
            startDate: changeRequest.plannedStartOn,
            endDate: changeRequest.plannedEndOn,
            duration: changeRequest.duration,
            description: changeRequest.description,
            hasServiceOutage: changeRequest.hasServiceOutage,
            createdOn: changeRequest.createdOn,
            updatedOn: changeRequest.updatedOn,
            project: project != () ? {id: project.id, label: project.name, number: project?.number} : (),
            case: case != () ? {id: case.id, label: case.name, number: case?.number} : (),
            deployment: deployment != () ? {id: deployment.id, label: deployment.name, number: deployment?.number} : (),
            deployedProduct: deployedProduct != () ?
                {id: deployedProduct.id, label: deployedProduct.name, number: deployedProduct?.number} : (),
            product: product != () ?
                {id: product.id, label: product.name, number: product?.number} : (),
            state: state != () ? {id: state.id.toString(), label: state.label} : (),
            impact: impact != () ? {id: impact.id.toString(), label: impact.label} : (),
            'type: 'type != () ? {id: 'type.id.toString(), label: 'type.label} : (),
            assignedEngineer: assignedEngineer != () ? {id: assignedEngineer.id, label: assignedEngineer.name} : (),
            assignedTeam: assignedTeam != () ? {id: assignedTeam.id, label: assignedTeam.name} : ()
        };

    return {
        changeRequests,
        totalRecords: response.totalRecords,
        'limit: response.'limit,
        offset: response.offset
    };
}

# Map catalog search response to the desired structure.
#
# + response - Catalog search response from the entity service
# + return - Mapped catalog search response
public isolated function mapCatalogSearchResponse(entity:CatalogSearchResponse response)
    returns types:CatalogSearchResponse {

    types:Catalog[] catalogs = from entity:Catalog item in response.catalogs
        let entity:ReferenceTableItem[] catalogItems = item.catalogItems
        select {
            id: item.id,
            name: item.name,
            catalogItems: from entity:ReferenceTableItem catalogItem in catalogItems
                select {id: catalogItem.id, label: catalogItem.name}
        };

    return {catalogs, totalRecords: response.totalRecords, 'limit: response.'limit, offset: response.offset};
}

# Map change request response to the desired structure.
#
# + response - Change request response from the entity service
# + return - Mapped change request response
public isolated function mapChangeRequestResponse(entity:ChangeRequestResponse response)
    returns types:ChangeRequestResponse {

    entity:ReferenceTableItem? project = response.project;
    entity:ReferenceTableItem? case = response.case;
    entity:ReferenceTableItem? deployment = response.deployment;
    entity:ReferenceTableItem? deployedProduct = response.deployedProduct;
    entity:ReferenceTableItem? product = response.product;
    entity:ReferenceTableItem? approvedBy = response.approvedBy;
    entity:ReferenceTableItem? assignedEngineer = response.assignedEngineer;
    entity:ReferenceTableItem? assignedTeam = response.assignedTeam;
    entity:ChoiceListItem? state = response.state;
    entity:ChoiceListItem? impact = response.impact;
    entity:ChoiceListItem? 'type = response.'type;
    return {
        id: response.id,
        number: response.number,
        title: response.title,
        startDate: response.plannedStartOn,
        endDate: response.plannedEndOn,
        duration: response.duration,
        hasServiceOutage: response.hasServiceOutage,
        createdOn: response.createdOn,
        updatedOn: response.updatedOn,
        project: project != () ? {id: project.id, label: project.name, number: project?.number} : (),
        case: case != () ? {id: case.id, label: case.name, number: case?.number} : (),
        deployment: deployment != () ? {id: deployment.id, label: deployment.name, number: deployment?.number} : (),
        deployedProduct: deployedProduct != () ?
            {id: deployedProduct.id, label: deployedProduct.name, number: deployedProduct?.number} : (),
        product: product != () ?
            {id: product.id, label: product.name, number: product?.number} : (),
        state: state != () ? {id: state.id.toString(), label: state.label} : (),
        impact: impact != () ? {id: impact.id.toString(), label: impact.label} : (),
        'type: 'type != () ? {id: 'type.id.toString(), label: 'type.label} : (),
        approvedBy: approvedBy != () ? {id: approvedBy.id, label: approvedBy.name} : (),
        assignedEngineer: assignedEngineer != () ? {id: assignedEngineer.id, label: assignedEngineer.name} : (),
        assignedTeam: assignedTeam != () ? {id: assignedTeam.id, label: assignedTeam.name} : (),
        description: response.description,
        createdBy: response.createdBy,
        justification: response.justification,
        impactDescription: response.impactDescription,
        serviceOutage: response.serviceOutage,
        communicationPlan: response.communicationPlan,
        rollbackPlan: response.rollbackPlan,
        testPlan: response.testPlan,
        hasCustomerApproved: response.hasCustomerApproved,
        hasCustomerReviewed: response.hasCustomerReviewed,
        approvedOn: response.approvedOn
    };
}

# Map project change request stats response to the desired structure.
#
# + response - Project change request stats response from the entity service
# + return - Mapped project change request stats response
public isolated function mapProjectChangeRequestStatsResponse(entity:ProjectChangeRequestStatsResponse response)
    returns types:ProjectChangeRequestStatsResponse {

    types:ReferenceItem[] stateCount = from entity:ChoiceListItem item in response.stateCount
        select {id: item.id.toString(), label: item.label, count: item.count};
    return {
        totalCount: response.totalCount,
        activeCount: response.activeCount,
        outstandingCount: response.outstandingCount,
        actionRequiredCount: response.actionRequiredCount,
        stateCount
    };
}

# Map projects response to the desired structure.
#
# + response - Projects response from the entity service
# + return - Mapped projects response
public isolated function mapProjectsResponse(entity:ProjectsResponse response) returns types:ProjectsResponse {
    types:Project[] projects = from entity:Project project in response.projects
        select {
            id: project.id,
            'key: project.key,
            name: project.name,
            description: project.description,
            createdOn: project.createdOn,
            closureState: project.closureState,
            'type: {id: project.'type.id, label: project.'type.name},
            hasPdpSubscription: project.hasPdpSubscription,
            hasAgent: project.hasAgent,
            hasKbReferences: project.hasKbReferences,
            activeCasesCount: project.activeCasesCount,
            activeChatsCount: project.activeChatsCount,
            slaStatus: project.slaStatus
        };

    return {projects, totalRecords: response.totalRecords, 'limit: response.'limit, offset: response.offset};
}

# Map project response to the desired structure.
#
# + response - Project response from the entity service
# + return - Mapped project response
public isolated function mapProjectResponse(entity:ProjectResponse response) returns types:ProjectResponse => {
    id: response.id,
    'key: response.key,
    name: response.name,
    description: response.description,
    createdOn: response.createdOn,
    'type: {id: response.'type.id, label: response.'type.name},
    sfId: response.sfId,
    closureState: response.closureState,
    hasPdpSubscription: response.hasPdpSubscription,
    startDate: response.startDate,
    endDate: response.endDate,
    account: {
        id: response.account.id,
        hasAgent: response.account.hasAgent,
        hasKbReferences: response.account.hasKbReferences,
        name: response.account.name,
        activationDate: response.account.activationDate,
        deactivationDate: response.account.deactivationDate,
        supportTier: response.account.supportTier,
        region: response.account.region,
        ownerEmail: response.account.ownerEmail,
        technicalOwnerEmail: response.account.technicalOwnerEmail
    },
    totalQueryHours: response.totalQueryHours,
    consumedQueryHours: response.consumedQueryHours,
    consumedOnboardingHours: response.consumedOnboardingHours,
    remainingQueryHours: response.remainingQueryHours,
    goLiveDate: response.goLiveDate,
    goLivePlanDate: response.goLivePlanDate,
    totalOnboardingHours: response.totalOnboardingHours,
    remainingOnboardingHours: response.remainingOnboardingHours,
    onboardingExpiryDate: response.onboardingExpiryDate,
    onboardingStatus: response.onboardingStatus
};

# Map metadata response to the desired structure.
#
# + response - Metadata response from the entity service
# + return - Mapped metadata response
public isolated function mapMetadataResponse(entity:MetadataResponse response) returns types:MetadataResponse {
    types:ReferenceItem[] timeZones = from entity:ChoiceListItem item in response.timeZones
        select {id: item.id.toString(), label: item.label};
    types:ReferenceItem[] projectTypes = from entity:ReferenceTableItem item in response.projectTypes
        select {id: item.id, label: item.name};
    return {timeZones, projectTypes, featureFlags};
}

# Map usage stats response to the desired structure.
#
# + response - Usage stats response from the entity service
# + return - Mapped usage stats response
public isolated function mapUsageStats(entity:ProjectStatsResponse response) returns types:UsageStats {
    // TODO: Add more stats
    return {
        deploymentCount: response.deploymentCount,
        deployedProductCount: response.deployedProductCount,
        instanceCount: response.instanceCount
    };
}

# Map instances response to the desired structure.
#
# + response - Instances response from the entity service
# + return - Mapped instances response
public isolated function mapInstancesResponse(entity:InstancesResponse response) returns types:InstancesResponse {
    types:Instance[] instances = from entity:Instance instance in response.instances
        let entity:ReferenceTableItem? project = instance.project
        let entity:ReferenceTableItem? deployedProduct = instance.deployedProduct
        let entity:ReferenceTableItem? deployment = instance.deployment
        let entity:ReferenceTableItem? product = instance.product
        select {
            id: instance.id,
            key: instance.key,
            createdOn: instance.createdOn,
            updatedOn: instance.updatedOn,
            project: project != () ? {id: project.id, label: project.name} : (),
            deployedProduct: deployedProduct != () ? {id: deployedProduct.id, label: deployedProduct.name} : (),
            deployment: deployment != () ? {id: deployment.id, label: deployment.name} : (),
            product: product != () ? {id: product.id, label: product.name} : (),
            metadata: instance.metadata
        };

    return {instances, totalRecords: response.totalRecords, 'limit: response.'limit, offset: response.offset};
}

# Map instance metrics response to the desired structure.
#
# + response - Instance metrics response from the entity service
# + return - Mapped instance metrics response
public isolated function mapInstanceMetrics(entity:InstanceMetricsResponse response)
    returns types:InstanceMetricsResponse {

    types:InstanceMetric[] metrics = from entity:InstanceMetric metric in response.metrics
        let entity:ReferenceTableItem? project = metric.project
        let entity:ReferenceTableItem? deployment = metric.deployment
        let entity:ReferenceTableItem? product = metric.product
        let entity:ReferenceTableItem? deployedProduct = metric.deployedProduct
        select {
            instanceId: metric.instanceId,
            instanceKey: metric.instanceKey,
            project: project != () ? {id: project.id, label: project.name} : (),
            deployment: deployment != () ? {id: deployment.id, label: deployment.name} : (),
            product: product != () ? {id: product.id, label: product.name} : (),
            deployedProduct: deployedProduct != () ? {id: deployedProduct.id, label: deployedProduct.name} : (),
            dataPoints: metric.dataPoints
        };
    return {
        metrics,
        totalInstances: response.totalInstances,
        startDate: response.startDate,
        endDate: response.endDate
    };
}

# Map instance usage response to the desired structure.
#
# + response - Instance usage response from the entity service
# + return - Mapped instance usage response
public isolated function mapInstanceUsages(entity:InstanceUsageResponse response)
    returns types:InstanceUsageResponse {

    types:InstanceUsageEntry[] usages = from entity:InstanceUsageEntry usage in response.usages
        let entity:ReferenceTableItem? project = usage.project
        let entity:ReferenceTableItem? deployment = usage.deployment
        let entity:ReferenceTableItem? product = usage.product
        let entity:ReferenceTableItem? deployedProduct = usage.deployedProduct
        select {
            instanceId: usage.instanceId,
            instanceKey: usage.instanceKey,
            project: project != () ? {id: project.id, label: project.name} : (),
            deployment: deployment != () ? {id: deployment.id, label: deployment.name} : (),
            product: product != () ? {id: product.id, label: product.name} : (),
            deployedProduct: deployedProduct != () ? {id: deployedProduct.id, label: deployedProduct.name} : (),
            periodSummaries: usage.periodSummaries
        };
    return {
        usages,
        totalInstances: response.totalInstances,
        startDate: response.startDate,
        endDate: response.endDate
    };
}

# Map time cards search response grouped by cases to the desired structure.
#
# + response - Time cards search response grouped by cases from the entity service
# + return - Mapped time cards search response grouped by cases
public isolated function mapTimeCardSearchResponseGroupedByCases(entity:CaseTimeCardsSearchResponse response)
    returns types:CaseTimeCardsSearchResponse {

    types:CaseTimeCard[] caseTimeCards = from entity:CaseTimeCardSummary caseTimeCard in response.cases
        let entity:ReferenceTableItem? project = caseTimeCard.case.project
        select {
            case: {
                id: caseTimeCard.case.id,
                number: caseTimeCard.case.number,
                name: caseTimeCard.case.name,
                updatedOn: caseTimeCard.case.updatedOn,
                project: project != () ? {id: project.id, label: project.name} : ()
            },
            totalTime: caseTimeCard.totalTime,
            totalCount: caseTimeCard.totalCount,
            billable: caseTimeCard.billable,
            nonBillable: caseTimeCard.nonBillable
        };
    return {caseTimeCards, totalRecords: response.totalRecords, 'limit: response.'limit, offset: response.offset};
}

# Map updated case response to the desired structure.
#
# + updatedCase - Updated case response from the entity service
# + return - Mapped updated case response
public isolated function mapUpdatedCaseResponse(entity:UpdatedCase updatedCase) returns types:UpdatedCase {
    entity:ChoiceListItem state = updatedCase.state;
    entity:ReferenceTableItem 'type = updatedCase.'type;
    return {
        id: updatedCase.id,
        updatedOn: updatedCase.updatedOn,
        state: {id: state.id.toString(), label: state.label},
        'type: {id: 'type.id, label: 'type.name},
        updatedBy: updatedCase.updatedBy
    };
}

# Map case activity search response to the desired structure.
# 
# + response - Case activity search response from the entity service
# + return - Mapped case activity search response
public isolated function mapCaseActivitySummaryResponse(entity:CaseActivitySearchResponse response)
    returns types:CaseActivitySearchResponse {

    types:Activity[] activities = from entity:Activity activity in response.activity
        select {
            id: activity.id,
            'type: activity.'type,
            createdOn: activity.createdOn,
            createdBy: activity.createdBy,
            content: activity.content,
            createdByFirstName: activity.createdByFirstName,
            createdByLastName: activity.createdByLastName,
            createdByFullName: activity.createdByFullName,
            fileName: activity.fileName,
            contentType: activity.contentType,
            sizeBytes: activity.sizeBytes,
            downloadUrl: activity.downloadUrl,
            commentType: activity.commentType
        };
    return {activities, totalRecords: response.totalRecords, 'limit: response.'limit, offset: response.offset};
}
