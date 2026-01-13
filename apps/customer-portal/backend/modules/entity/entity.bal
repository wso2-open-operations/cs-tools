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

# Fetch logged-in user information.
# 
# + email - Email of the user
# + idToken - ID token for authorization
# + return - User response or error
public isolated function fetchUserBasicInfo(string email, string idToken) returns UserResponse|error {
    return csEntityClient->/users/me.get(generateHeaders(idToken));
}

# Search projects of the logged-in user.
#
# + idToken - ID token for authorization
# + payload - Request body for searching projects
# + return - Projects response or error
public isolated function searchProjects(string idToken, ProjectRequest payload) returns ProjectsResponse|error {
    return csEntityClient->/projects.post(payload, generateHeaders(idToken));
}

# Search cases of a project.
#
# + idToken - ID token for authorization
# + projectId - Unique ID of the project
# + payload - Request body for searching cases
# + return - Cases object or error
public isolated function searchCases(string idToken, string projectId, CaseSearchPayload payload)
    returns CasesResponse|error {

    CaseRequestPayload requestPayload = {
        projectIds: [projectId],
        caseTypes: payload.caseTypes,
        pagination: payload.pagination,
        filters: payload.filters,
        sortBy: payload.sortBy
    };
    return csEntityClient->/cases/search.post(requestPayload, generateHeaders(idToken));
}
