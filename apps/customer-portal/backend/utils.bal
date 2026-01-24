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

# Search cases for a given project.
#
# + idToken - ID token for authorization
# + projectId - Project ID to filter cases
# + payload - Case search payload
# + return - Case search response or error
public isolated function searchCases(string idToken, string projectId, CaseSearchPayload payload)
    returns CaseSearchResponse|error {

    entity:CaseSearchPayload searchPayload = {
        filters: {
            projectIds: [projectId],
            caseTypes: payload.filters?.caseTypes,
            severityId: payload.filters?.severityId,
            stateId: payload.filters?.statusId,
            deploymentId: payload.filters?.deploymentId
        },
        pagination: payload.pagination,
        sortBy: payload.sortBy
    };
    entity:CaseSearchResponse casesResponse = check entity:searchCases(idToken, searchPayload);
    Case[] cases = from entity:Case {project, 'type, deployment, state, severity, assignedEngineer, ...rest}
        in casesResponse.cases
        select {
            ...rest,
            project: project != () ? {id: project.id, label: project.name} : (),
            'type: 'type != () ? {id: 'type.id, label: 'type.name} : (),
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
    ReferenceItem[] caseTypes = from entity:ReferenceTableItem item in caseMetadata.caseTypes
        select {id: item.id, label: item.name};

    // TODO: Other project specific filters will be added later
    return {
        statuses,
        severities,
        caseTypes
    };
}

# Validates the given ID.
#
# + id - ID to validate
# + return - True if valid, else false
public isolated function isValidId(string id) returns boolean => id.trim().length() != 0;

# Get HTTP status code from the given error.
#
# + projectId - Unique ID of the project
# + return - Project case statistics if found, else nil
public isolated function getCaseStatsFromCache(string projectId) returns ProjectCaseStats? {
    string cacheKey = string `${projectId}:caseStats`;
    if statsCache.hasKey(cacheKey) {
        ProjectCaseStats|error cachedStats = statsCache.get(cacheKey).ensureType();
        if cachedStats is ProjectCaseStats {
            return cachedStats;
        }
        log:printWarn(string `Unable to read cached stats for project: ${projectId}`);
    }
    return;
}

# Cache case statistics for a project.
#
# + projectId - Unique ID of the project
# + caseStats - Project case statistics
public isolated function updateCaseStatsCache(string projectId, entity:ProjectCaseStatsResponse caseStats) {
    ProjectCaseStats caseStatsToCache = {
        totalCases: caseStats.totalCount,
        openCases: caseStats.openCount,
        averageResponseTime: caseStats.averageResponseTime,
        activeCases: caseStats.activeCount,
        resolvedCases: caseStats.resolvedCount,
        outstandingIncidents: caseStats.outstandingIncidentsCount
    };
    error? cacheError = statsCache.put(string `${projectId}:caseStats`, caseStatsToCache);
    if cacheError is error {
        log:printWarn(string `Error writing case stats of project: ${projectId} to cache`, cacheError);
    }
    return;
}

# Get support statistics from cache for a given project.
#
# + projectId - Unique ID of the project
# + return - Project chat statistics if found, else nil
public isolated function getSupportStatsFromCache(string projectId) returns ProjectSupportStats? {
    string cacheKey = string `${projectId}:supportStats`;
    if statsCache.hasKey(cacheKey) {
        ProjectSupportStats|error cachedStats = statsCache.get(cacheKey).ensureType();
        if cachedStats is ProjectSupportStats {
            return cachedStats;
        }
        log:printWarn(string `Unable to read cached support stats for project: ${projectId}`);
    }
    return;
}

# Cache support statistics for a project.
#
# + projectId - Unique ID of the project
# + chatStats - Project chat statistics
# + totalCases - Total cases count
public isolated function updateSupportStatsCache(string projectId, entity:ProjectChatStatsResponse chatStats,
        int totalCases) {

    ProjectSupportStats supportStatsToCache = {
        totalCases,
        activeChats: chatStats.activeCount,
        sessionChats: chatStats.sessionCount,
        resolvedChats: chatStats.resolvedCount
    };
    error? cacheError = statsCache.put(string `${projectId}:supportStats`, supportStatsToCache);
    if cacheError is error {
        log:printWarn(string `Error writing support stats of project: ${projectId} to cache`, cacheError);
    }
    return;
}

# Extract error message from the given error.
#
# + err - Error to handle
# + return - Error message
public isolated function extractErrorMessage(error err) returns string {
    map<anydata|readonly> & readonly errorDetails = err.detail();
    anydata|readonly errorMessage = errorDetails[ERR_BODY] ?: ();
    return errorMessage is string ? errorMessage : UNEXPECTED_ERROR_MSG;
}
