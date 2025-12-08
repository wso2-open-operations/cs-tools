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

import ballerina/cache;
import ballerina/log;
import ballerina/mime;

final cache:Cache userCache = new ({
    capacity: 200,
    defaultMaxAge: 86400.0,
    evictionFactor: 0.25
});

# Fetches user basic information from entity service.
#
# + email - Email of the user
# + idToken - ID token for authorization
# + return - User Information or error
public isolated function fetchUserBasicInfo(string email, string idToken) returns UserInfo|error {
    map<string|string[]> headers = {
        "Authorization": "Bearer " + idToken
    };

    // Check cache for logged in user.
    if userCache.hasKey(email) {
        UserInfo|error cachedUserInfo = userCache.get(email).ensureType();
        if cachedUserInfo is error {
            log:printWarn(`Unable to read the cached user info for user: ${email}`, cachedUserInfo);
        } else {
            return cachedUserInfo;
        }
    }

    EntityUserResponse {'result} = check csEntityClient->/users.post(
        {email},
        headers = headers,
        mediaType = mime:APPLICATION_JSON
    );
    if !'result.success {
        return error(string `Entity service returned an unsuccessful response for user info: ${
            'result.message}`);
    }

    EntityUser entityUserData = 'result.data;
    UserInfo userInfo = {
        userId: entityUserData.sys_id,
        userEmail: entityUserData.email,
        firstName: entityUserData.first_name,
        lastName: entityUserData.last_name,
        username: entityUserData.user_name,
        active: string:toLowerAscii(entityUserData.active.trim()) == "true"
    };

    // Cache the response.
    error? cacheError = userCache.put(email, userInfo);
    if cacheError is error {
        log:printWarn("An error occurred while writing user info to the cache", cacheError);
    }

    return userInfo;
}

# Fetch projects from entity service.
#
# + email - Email of the user
# + idToken - ID token for authorization
# + return - List of projects or error
public isolated function fetchProjects(string email, string idToken) returns EntityProject[]|error {
    map<string|string[]> headers = {
        "Authorization": "Bearer " + idToken
    };

    EntityProjectsResponse {'result} = check csEntityClient->/projectcontact/getprojectsbycontact/projects.post(
        {email},
        headers = headers,
        mediaType = mime:APPLICATION_JSON
    );
    if !'result.success {
        return error(string `Entity service returned unsuccessful response for projects with email: ${email}`);
    }
    return 'result.data;
}

# Fetch cases from entity service.
#
# + email - Email of the user
# + idToken - ID token for authorization
# + offset - Offset for pagination
# + 'limit - Limit for pagination
# + return - List of cases or error
public isolated function fetchCases(string email, string idToken, int offset = 0, int 'limit = 10)
    returns Cases|error {

    map<string|string[]> headers = {
        "Authorization": "Bearer " + idToken
    };

    // Retrieve all project IDs for the logged-in user's email.
    EntityProject[] projectsResponse = check fetchProjects(email, idToken);
    string[] projectIdArray = from EntityProject project in projectsResponse
        select project.sysId;

    // User has no projects, return an empty CasesResponse.
    if projectIdArray.length() == 0 {
        return {
            cases: [],
            pagination: {
                total: 0,
                offset: offset,
                'limit: 'limit
            }
        };
    }

    string projectIds = string:'join(",", ...projectIdArray);

    EntityCasesResponse {'result} = check csEntityClient->/cases.get(
        project = projectIds,
        offset = offset,
        'limit = 'limit,
        headers = headers
    );
    if !'result.success {
        return error(string `Entity service returned unsuccessful response for cases`);
    }

    EntityCases entityCasesData = 'result.data;
    EntityCase[] entityCaseArray = entityCasesData.cases;
    CaseInfo[] transformedCases = from EntityCase entityCase in entityCaseArray
        select {
            caseSysId: entityCase.case_sys_id,
            number: entityCase.number,
            wso2CaseId: entityCase.wso2_case_id,
            shortDescription: entityCase.short_description,
            caseType: entityCase.case_type,
            product: entityCase.product,
            priority: entityCase.priority,
            state: entityCase.state,
            contact: entityCase.contact,
            updatedOn: entityCase.updated_on,
            openedOn: entityCase.opened_on,
            resolvedOn: entityCase.resolved_on
        };

    EntityPagination entityPaginationData = entityCasesData.pagination;
    return {
        cases: transformedCases,
        pagination: {
            total: <int>entityPaginationData.total,
            offset: <int>entityPaginationData.offset,
            'limit: <int>entityPaginationData.'limit
        }
    };
}

# Fetch case details from entity service.
#
# + caseId - Case ID to fetch details for
# + idToken - ID token for authorization
# + return - Case details or error
public isolated function fetchCaseDetails(string caseId, string idToken) returns CaseDetails|error {
    map<string|string[]> headers = {
        "Authorization": "Bearer " + idToken
    };

    EntityCaseDetailsResponse {'result} = check csEntityClient->/cases/caseDetails.get(
        case = caseId,
        headers = headers
    );
    if !'result.success {
        return error(string `Entity service returned unsuccessful response for case details with caseId: ${caseId}`);
    }

    EntityCaseDetails entityCaseDetailsData = 'result.data;
    CaseDetails caseDetails = {
        number: entityCaseDetailsData.number,
        wso2CaseId: entityCaseDetailsData.wso2_case_id,
        shortDescription: entityCaseDetailsData.short_description,
        description: entityCaseDetailsData.description,
        caseType: entityCaseDetailsData.case_type,
        projectDeployment: entityCaseDetailsData.project_deployment,
        product: entityCaseDetailsData.product,
        priority: entityCaseDetailsData.priority,
        state: entityCaseDetailsData.state,
        contact: entityCaseDetailsData.contact,
        updatedOn: entityCaseDetailsData.updated_on,
        openedOn: entityCaseDetailsData.opened_on,
        resolvedOn: entityCaseDetailsData.resolved_on,
        projectName: entityCaseDetailsData.project.name
    };
    return caseDetails;
}
