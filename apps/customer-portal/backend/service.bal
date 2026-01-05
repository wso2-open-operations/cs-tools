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

import customer_portal.authorization;
import customer_portal.entity;

import ballerina/cache;
import ballerina/http;
import ballerina/log;

configurable CacheConfig cacheConfig = ?;

final cache:Cache userCache = new ({
    capacity: cacheConfig.capacity,
    defaultMaxAge: cacheConfig.defaultMaxAge,
    evictionFactor: cacheConfig.evictionFactor,
    cleanupInterval: cacheConfig.cleanupInterval
});

service class ErrorInterceptor {
    *http:ResponseErrorInterceptor;

    # Intercepts the response error.
    #
    # + err - The error occurred during request processing
    # + return - Bad request response or error
    remote function interceptResponseError(error err) returns http:BadRequest|error {

        // Handle data-binding errors.
        if err is http:PayloadBindingError {
            string customError = "Payload binding failed!";
            log:printError(customError, err);
            return {
                body: {
                    message: customError
                }
            };
        }
        return err;
    }
}

@display {
    label: "Customer Portal",
    id: "cs/customer-portal"
}
service http:InterceptableService / on new http:Listener(9090) {
    public function createInterceptors() returns http:Interceptor[] =>
        [new authorization:JwtInterceptor(), new ErrorInterceptor()];

    # Service init function.
    #
    # + return - Error if initialization fails
    function init() returns error? {
        log:printInfo("Customer Portal backend started.");
    }

    # Fetch user information of the logged in user.
    #
    # + return - User info object or error response
    resource function get users/me(http:RequestContext ctx) returns entity:UserResponse|http:InternalServerError {
        authorization:UserDataPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        string cacheKey = string `${userInfo.email}:userinfo`;
        if userCache.hasKey(cacheKey) {
            entity:UserResponse|error cachedUser = userCache.get(cacheKey).ensureType();
            if cachedUser is entity:UserResponse {
                return cachedUser;
            }
            log:printWarn(string `Unable to read cached user info for ${userInfo.email}`);
        }

        entity:UserResponse|error userDetails = entity:fetchUserBasicInfo(userInfo.email, userInfo.idToken);
        if userDetails is error {
            string customError = "Error retrieving user data from entity service";
            log:printError(customError, userDetails);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        error? cacheError = userCache.put(cacheKey, userDetails);
        if cacheError is error {
            log:printWarn("Error writing user information to cache", cacheError);
        }
        return userDetails;
    }

    # Search projects of the logged-in user.
    #
    # + payload - Project search request body
    # + return - Projects list or error response
    resource function post projects/search(http:RequestContext ctx, entity:ProjectRequest payload)
        returns entity:ProjectsResponse|http:BadRequest|http:InternalServerError {

        authorization:UserDataPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:ProjectsResponse|error projectsList = entity:searchProjects(userInfo.idToken, payload);
        if projectsList is error {
            string customError = "Error retrieving projects list";
            log:printError(customError, projectsList);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return projectsList;
    }

    # Fetch specific project details.
    #
    # + projectId - Unique identifier of the project
    # + return - Project details or error response
    resource function get projects/[string projectId](http:RequestContext ctx)
        returns entity:ProjectDetailsResponse|http:BadRequest|http:InternalServerError {

        authorization:UserDataPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        if projectId.trim().length() == 0 {
            string customError = "Project ID cannot be empty or whitespace";
            log:printError(customError);
            return <http:BadRequest>{
                body: {
                    message: customError
                }
            };
        }

        string cacheKey = string `${userInfo.email}:project:${projectId}`;
        if userCache.hasKey(cacheKey) {
            entity:ProjectDetailsResponse|error cached = userCache.get(cacheKey).ensureType();
            if cached is entity:ProjectDetailsResponse {
                return cached;
            }
        }

        entity:ProjectDetailsResponse|error projectDetails = entity:fetchProjectDetails(projectId, userInfo.idToken);
        if projectDetails is error {
            string customError = "Error retrieving project information";
            log:printError(customError, projectDetails);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        error? cacheError = userCache.put(cacheKey, projectDetails);
        if cacheError is error {
            log:printWarn("Error writing project details to cache", cacheError);
        }
        return projectDetails;
    }

    # Fetch project overview.
    #
    # + projectId - Unique identifier of the project
    # + return - Project overview or error response
    resource function get projects/[string projectId]/overview(http:RequestContext ctx)
        returns entity:ProjectOverviewResponse|http:BadRequest|http:InternalServerError {

        authorization:UserDataPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        if projectId.trim().length() == 0 {
            string customError = "Project ID cannot be empty or whitespace";
            log:printError(customError);
            return <http:BadRequest>{
                body: {
                    message: customError
                }
            };
        }

        string cacheKey = string `${userInfo.email}:overview:${projectId}`;
        if userCache.hasKey(cacheKey) {
            entity:ProjectOverviewResponse|error cached = userCache.get(cacheKey).ensureType();
            if cached is entity:ProjectOverviewResponse {
                return cached;
            }
        }

        entity:ProjectOverviewResponse|error projectOverview = entity:fetchProjectOverview(projectId, userInfo.idToken);
        if projectOverview is error {
            string customError = "Error retrieving project overview";
            log:printError(customError, projectOverview);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        error? cacheError = userCache.put(cacheKey, projectOverview);
        if cacheError is error {
            log:printWarn("Error writing project overview to cache", cacheError);
        }
        return projectOverview;
    }

    # Fetch case filters for a specific project.
    #
    # + projectId - Unique identifier of the project
    # + return - Case filters or error response
    resource function get projects/[string projectId]/cases/filters(http:RequestContext ctx)
        returns entity:CaseFiltersResponse|http:BadRequest|http:InternalServerError {

        authorization:UserDataPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        if projectId.trim().length() == 0 {
            string customError = "Project ID cannot be empty or whitespace";
            log:printError(customError);
            return <http:BadRequest>{
                body: {
                    message: customError
                }
            };
        }

        string cacheKey = string `${userInfo.email}:casefilters:${projectId}`;
        if userCache.hasKey(cacheKey) {
            entity:CaseFiltersResponse|error cached = userCache.get(cacheKey).ensureType();
            if cached is entity:CaseFiltersResponse {
                return cached;
            }
        }

        entity:CaseFiltersResponse|error caseFilters = entity:fetchCasesFilters(projectId, userInfo.idToken);
        if caseFilters is error {
            string customError = "Error retrieving case filters";
            log:printError(customError, caseFilters);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        error? cacheError = userCache.put(cacheKey, caseFilters);
        if cacheError is error {
            log:printWarn("Error writing case filters to cache", cacheError);
        }
        return caseFilters;
    }

    # Fetch cases for a project with optional filters.
    #
    # + projectId - Project ID filter
    # + payload - Filter and pagination parameters
    # + return - Paginated cases or error response
    resource function post projects/[string projectId]/cases/search(http:RequestContext ctx,
            entity:CaseFiltersRequest payload)
        returns entity:CasesResponse|http:BadRequest|http:InternalServerError {

        authorization:UserDataPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        if projectId.trim().length() == 0 {
            string customError = "Project ID cannot be empty or whitespace";
            log:printError(customError);
            return <http:BadRequest>{
                body: {
                    message: customError
                }
            };
        }

        // Validate Pagination
        if payload.offset < 0 || payload.'limit <= 0 {
            string customError = "Invalid pagination parameters";
            log:printError(customError);
            return <http:BadRequest>{
                body: {
                    message: customError
                }
            };
        }

        string cacheKey = string `${userInfo.email}:cases:${projectId}:${payload.offset}:${payload.'limit}:${
            payload.contact ?: ""}:${payload.status ?: ""}:${payload.severity ?: ""}:${payload.product ?: ""}:${
            payload.category ?: ""}`;

        if userCache.hasKey(cacheKey) {
            entity:CasesResponse|error cached = userCache.get(cacheKey).ensureType();
            if cached is entity:CasesResponse {
                return cached;
            }
        }

        entity:CasesResponse|error cases = entity:fetchCases(userInfo.idToken, projectId, payload);
        if cases is error {
            string customError = "Error retrieving cases";
            log:printError(customError, cases);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        error? cacheError = userCache.put(cacheKey, cases);
        if cacheError is error {
            log:printWarn("Error writing cases to cache", cacheError);
        }
        return cases;
    }

    # Fetch details of a specific case.
    #
    # + projectId - Unique identifier of the project
    # + caseId - Unique identifier of the case
    # + return - Case details or error response
    resource function get projects/[string projectId]/cases/[string caseId](http:RequestContext ctx)
        returns entity:CaseDetailsResponse|http:BadRequest|http:InternalServerError {

        authorization:UserDataPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        if projectId.trim().length() == 0 || caseId.trim().length() == 0 {
            string customError = "Project ID and Case ID cannot be empty or whitespace";
            log:printError(customError);
            return <http:BadRequest>{
                body: {
                    message: customError
                }
            };
        }

        string cacheKey = string `${userInfo.email}:case:${projectId}:${caseId}`;
        if userCache.hasKey(cacheKey) {
            entity:CaseDetailsResponse|error cached = userCache.get(cacheKey).ensureType();
            if cached is entity:CaseDetailsResponse {
                return cached;
            }
        }

        entity:CaseDetailsResponse|error caseDetails = entity:fetchCaseDetails(projectId, caseId, userInfo.idToken);
        if caseDetails is error {
            string customError = "Error retrieving case details";
            log:printError(customError, caseDetails);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        error? cacheError = userCache.put(cacheKey, caseDetails);
        if cacheError is error {
            log:printWarn("Error writing case details to cache", cacheError);
        }

        return caseDetails;
    }
}
