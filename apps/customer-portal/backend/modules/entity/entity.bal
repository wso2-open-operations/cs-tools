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

# Fetch case filters for a specific project.
#
# + projectId - Unique ID of the project
# + idToken - ID token for authorization
# + return - Case filters object or error
public isolated function fetchCasesFilters(string projectId, string idToken) returns CaseFiltersResponse|error {
    return csEntityClient->/projects/[projectId]/cases/filters.get(generateHeaders(idToken));
}

# Fetch project overview from entity service.
#
# + projectId - Project ID to fetch overview for
# + idToken - ID token for authorization
# + return - Project overview or error
public isolated function fetchProjectOverview(string projectId, string idToken) returns ProjectOverviewResponse|error {
    return csEntityClient->/projects/[projectId]/overview.get(generateHeaders(idToken));
}

# Fetch project details by project ID.
#
# + projectId - Unique ID of the project
# + idToken - ID token for authorization
# + return - Project details object or error
public isolated function fetchProjectDetails(string projectId, string idToken) returns ProjectDetailsResponse|error {
    return csEntityClient->/projects/[projectId].get(generateHeaders(idToken));
}

# Fetch case details for a specific case in a project.
#
# + projectId - Unique ID of the project
# + caseId - Unique ID of the case
# + idToken - ID token for authorization
# + return - Case details object or error
public isolated function fetchCaseDetails(string projectId, string caseId, string idToken)
    returns CaseDetailsResponse|error {

    return csEntityClient->/projects/[projectId]/cases/[caseId].get(generateHeaders(idToken));
}

# Search projects of the logged-in user.
#
# + idToken - ID token for authorization
# + payload - Request body for searching projects
# + return - Projects response or error
public isolated function searchProjects(string idToken, ProjectRequest payload) returns ProjectsResponse|error {
    return csEntityClient->/projects.post(payload, generateHeaders(idToken));
}

# Fetch cases for a specific project with pagination and filters.
#
# + idToken - ID token for authorization
# + projectId - Unique ID of the project
# + filters - Optional filters and pagination parameters (offset, limit, contact, status, severity, product, category)
# + return - Cases object or error
public isolated function fetchCases(string idToken, string projectId, CaseFiltersRequest? filters = ())
    returns CasesResponse|error {

    CaseFiltersRequest requestBody = {
        offset: filters?.offset ?: DEFAULT_OFFSET,
        'limit: filters?.'limit ?: DEFAULT_LIMIT,
        contact: (),
        status: (),
        severity: (),
        product: (),
        category: ()
    };

    return csEntityClient->/projects/[projectId]/cases/search.post(requestBody, generateHeaders(idToken));
}
