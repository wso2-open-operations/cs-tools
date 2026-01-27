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
import customer_portal.scim;

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

# Get mobile phone number from SCIM users.
#
# + email - Email address of the user
# + id - ID of the user (for logging purposes)
# + return - Mobile phone number if found, else nil
public isolated function getPhoneNumber(string email, string id) returns string? {
    string? mobilePhoneNumber = ();
    scim:User[]|error userResults = scim:searchUsers(email);
    if userResults is error {
        // Log the error and return nil
        log:printError("Error retrieving user phone number from scim service", userResults);
    } else {
        if userResults.length() == 0 {
            log:printError(string `No user found while searching phone number for user: ${id}`);
        } else {
            scim:PhoneNumber[]? phoneNumbers = userResults[0].phoneNumbers;
            if phoneNumbers != () {
                // Filter for mobile type phone numbers
                scim:PhoneNumber[] mobilePhoneNumbers =
                    phoneNumbers.filter(phoneNumber => phoneNumber.'type == MOBILE_PHONE_NUMBER_TYPE);
                mobilePhoneNumber = mobilePhoneNumbers.length() > 0 ? mobilePhoneNumbers[0].value : ();
            }
        }
    }
    return mobilePhoneNumber;
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
