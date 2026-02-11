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

import customer_portal.authorization;
import customer_portal.entity;
import customer_portal.scim;
import customer_portal.updates;

import ballerina/cache;
import ballerina/http;
import ballerina/log;

final cache:Cache userCache = new ({
    capacity: 500,
    defaultMaxAge: 3600,
    evictionFactor: 0.2,
    cleanupInterval: 1800
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
    resource function get users/me(http:RequestContext ctx) returns User|http:Unauthorized|http:InternalServerError {
        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        string cacheKey = string `${userInfo.email}:userinfo`;
        if userCache.hasKey(cacheKey) {
            User|error cachedUser = userCache.get(cacheKey).ensureType();
            if cachedUser is User {
                return cachedUser;
            }
            log:printWarn(string `Unable to read cached user info for ${userInfo.email}`);
        }

        entity:UserResponse|error userDetails = entity:getUserBasicInfo(userInfo.email, userInfo.idToken);
        if userDetails is error {
            if getStatusCode(userDetails) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `Access denied for user: ${userInfo.userId} to Customer Portal`);
                return <http:Unauthorized>{
                    body: {
                        message: "Unauthorized access to the customer portal."
                    }
                };
            }

            string customError = "Failed to retrieve user data.";
            log:printError(customError, userDetails);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        string? phoneNumber = ();
        scim:User[]|error userResults = scim:searchUsers(userInfo.email);
        if userResults is error {
            // Log the error and return nil
            log:printError("Error retrieving user phone number from scim service", userResults);
        } else {
            if userResults.length() == 0 {
                log:printError(string `No user found while searching phone number for user: ${userInfo.userId}`);
            } else {
                phoneNumber = scim:processPhoneNumber(userResults[0]);
            }
        }

        User user = {
            id: userDetails.id,
            email: userDetails.email,
            firstName: userDetails.firstName,
            lastName: userDetails.lastName,
            timeZone: userDetails.timeZone,
            phoneNumber
        };

        error? cacheError = userCache.put(cacheKey, user);
        if cacheError is error {
            log:printWarn("Error writing user information to cache", cacheError);
        }
        return user;
    }

    # Update user information of the logged in user.
    #
    # + payload - User update payload
    # + return - Updated user object or error response
    resource function patch users/me(http:RequestContext ctx, UserUpdatePayload payload)
        returns UpdatedUser|http:BadRequest|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        if payload.keys().length() == 0 {
            return <http:BadRequest>{
                body: {
                    message: "At least one field must be provided for update!"
                }
            };
        }

        UpdatedUser updatedUserResponse = {};

        if payload.phoneNumber is string {
            scim:Phone phoneNumber = {mobile: payload.phoneNumber};
            scim:User|error updatedUser = scim:updateUser({phoneNumber}, userInfo.email, userInfo.userId);
            if updatedUser is error {
                if getStatusCode(updatedUser) == http:STATUS_BAD_REQUEST {
                    return <http:BadRequest>{
                        body: {
                            message: extractErrorMessage(updatedUser)
                        }
                    };
                }

                string customError = "Failed to update phone number.";
                log:printError(customError, updatedUser);
                return <http:InternalServerError>{
                    body: {
                        message: customError
                    }
                };
            }

            error? cacheInvalidate = userCache.invalidate(string `${userInfo.email}:userinfo`);
            if cacheInvalidate is error {
                log:printWarn("Error invalidating user information from cache", cacheInvalidate);
            }
            updatedUserResponse.phoneNumber = scim:processPhoneNumber(updatedUser);
        }

        if payload.timeZone is string {
            // TODO: Update timezone
        }

        return updatedUserResponse;
    }

    # Search projects of the logged-in user.
    #
    # + payload - Project search payload
    # + return - Projects list or error response
    resource function post projects/search(http:RequestContext ctx, entity:ProjectSearchPayload payload)
        returns http:Ok|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:ProjectsResponse|error projectsList = entity:searchProjects(userInfo.idToken, payload);
        if projectsList is error {
            if getStatusCode(projectsList) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            if getStatusCode(projectsList) == http:STATUS_FORBIDDEN {
                log:printWarn(string `Access to requested projects are forbidden for user: ${userInfo.userId}`);
                return <http:Forbidden>{
                    body: {
                        message: ERR_MSG_PROJECT_ACCESS_FORBIDDEN
                    }
                };
            }

            string customError = "Failed to retrieve project list.";
            log:printError(customError, projectsList);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return <http:Ok>{
            body: projectsList
        };
    }

    # Get project details by ID.
    #
    # + id - ID of the project
    # + return - Project details or error response
    resource function get projects/[string id](http:RequestContext ctx)
        returns entity:ProjectResponse|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        if isEmptyId(id) {
            return <http:BadRequest>{
                body: {
                    message: ERR_MSG_PROJECT_ID_EMPTY
                }
            };
        }

        entity:ProjectResponse|error projectResponse = entity:getProject(userInfo.idToken, id);
        if projectResponse is error {
            if getStatusCode(projectResponse) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            if getStatusCode(projectResponse) == http:STATUS_FORBIDDEN {
                logForbiddenProjectAccess(id, userInfo.userId);
                return <http:Forbidden>{
                    body: {
                        message: ERR_MSG_PROJECT_ACCESS_FORBIDDEN
                    }
                };
            }

            string customError = "Failed to retrieve project details.";
            log:printError(customError, projectResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return projectResponse;
    }

    # Get deployments of a project by ID.
    #
    # + id - ID of the project
    # + return - Deployments response or error response
    resource function get projects/[string id]/deployments(http:RequestContext ctx)
        returns Deployment[]|http:BadRequest|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        if isEmptyId(id) {
            return <http:BadRequest>{
                body: {
                    message: ERR_MSG_PROJECT_ID_EMPTY
                }
            };
        }

        entity:DeploymentsResponse|error deploymentsResponse = entity:getDeployments(userInfo.idToken, id);
        if deploymentsResponse is error {
            if getStatusCode(deploymentsResponse) == http:STATUS_FORBIDDEN {
                logForbiddenProjectAccess(id, userInfo.userId);
                return <http:Forbidden>{
                    body: {
                        message: ERR_MSG_PROJECT_ACCESS_FORBIDDEN
                    }
                };
            }

            string customError = "Failed to retrieve deployments for the project.";
            log:printError(customError, deploymentsResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return mapDeployments(deploymentsResponse);
    }

    # Get overall project statistics by ID.
    #
    # + id - ID of the project
    # + return - Project statistics response or error
    resource function get projects/[string id]/stats(http:RequestContext ctx)
        returns ProjectStatsResponse|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        if isEmptyId(id) {
            return <http:BadRequest>{
                body: {
                    message: ERR_MSG_PROJECT_ID_EMPTY
                }
            };
        }

        // Verify project access
        entity:ProjectResponse|error projectResponse = entity:getProject(userInfo.idToken, id);
        if projectResponse is error {
            if getStatusCode(projectResponse) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            if getStatusCode(projectResponse) == http:STATUS_FORBIDDEN {
                logForbiddenProjectAccess(id, userInfo.userId);
                return <http:Forbidden>{
                    body: {
                        message: ERR_MSG_PROJECT_ACCESS_FORBIDDEN
                    }
                };
            }

            log:printError(ERR_MSG_FETCHING_PROJECT_DETAILS, projectResponse);
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_FETCHING_PROJECT_DETAILS
                }
            };
        }

        // Fetch case stats
        entity:ProjectCaseStatsResponse|error caseStats = entity:getCaseStatsForProject(userInfo.idToken, id);
        if caseStats is error {
            string customError = "Failed to retrieve project case statistics.";
            log:printError(customError, caseStats);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        // Fetch chat stats
        entity:ProjectChatStatsResponse|error chatStats = entity:getChatStatsForProject(userInfo.idToken, id);
        if chatStats is error {
            string customError = "Failed to retrieve project chat statistics.";
            log:printError(customError, chatStats);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        // Fetch deployment stats
        entity:ProjectDeploymentStatsResponse|error deploymentStats =
            entity:getDeploymentStatsForProject(userInfo.idToken, id);
        if deploymentStats is error {
            string customError = "Failed to retrieve project deployment statistics.";
            log:printError(customError, deploymentStats);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        // Fetch project activity stats
        entity:ProjectStatsResponse|error projectActivityStats = entity:getProjectActivityStats(userInfo.idToken, id);
        if projectActivityStats is error {
            string customError = "Failed to retrieve project activity statistics.";
            log:printError(customError, projectActivityStats);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return {
            projectStats: {
                openCases: caseStats.openCount,
                activeChats: chatStats.activeCount,
                deployments: deploymentStats.totalCount,
                slaStatus: projectActivityStats.slaStatus
            },
            recentActivity: {
                totalTimeLogged: projectActivityStats.totalTimeLogged,
                billableHours: projectActivityStats.billableHours,
                lastDeploymentOn: deploymentStats.lastDeploymentOn,
                systemHealth: projectActivityStats.systemHealth
            }
        };
    }

    # Get cases statistics for a project by ID.
    #
    # + id - ID of the project
    # + return - Project statistics overview or error response
    resource function get projects/[string id]/stats/cases(http:RequestContext ctx)
        returns ProjectCaseStats|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        if isEmptyId(id) {
            return <http:BadRequest>{
                body: {
                    message: ERR_MSG_PROJECT_ID_EMPTY
                }
            };
        }

        // Verify project access
        entity:ProjectResponse|error projectResponse = entity:getProject(userInfo.idToken, id);
        if projectResponse is error {
            if getStatusCode(projectResponse) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            if getStatusCode(projectResponse) == http:STATUS_FORBIDDEN {
                logForbiddenProjectAccess(id, userInfo.userId);
                return <http:Forbidden>{
                    body: {
                        message: ERR_MSG_PROJECT_ACCESS_FORBIDDEN
                    }
                };
            }

            log:printError(ERR_MSG_FETCHING_PROJECT_DETAILS, projectResponse);
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_FETCHING_PROJECT_DETAILS
                }
            };
        }

        entity:ProjectCaseStatsResponse|error caseStats = entity:getCaseStatsForProject(userInfo.idToken, id);
        if caseStats is error {
            string customError = "Failed to retrieve project case statistics.";
            log:printError(customError, caseStats);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return {
            totalCases: caseStats.totalCount,
            openCases: caseStats.openCount,
            averageResponseTime: caseStats.averageResponseTime,
            activeCases: caseStats.activeCount,
            resolvedCases: caseStats.resolvedCount,
            outstandingCases: caseStats.outstandingCasesCount
        };
    }

    # Get project support statistics by ID.
    #
    # + id - ID of the project
    # + return - Project support statistics or error response
    resource function get projects/[string id]/stats/support(http:RequestContext ctx)
        returns ProjectSupportStats|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        if isEmptyId(id) {
            return <http:BadRequest>{
                body: {
                    message: ERR_MSG_PROJECT_ID_EMPTY
                }
            };
        }

        // Verify project access
        entity:ProjectResponse|error projectResponse = entity:getProject(userInfo.idToken, id);
        if projectResponse is error {
            if getStatusCode(projectResponse) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            if getStatusCode(projectResponse) == http:STATUS_FORBIDDEN {
                logForbiddenProjectAccess(id, userInfo.userId);
                return <http:Forbidden>{
                    body: {
                        message: ERR_MSG_PROJECT_ACCESS_FORBIDDEN
                    }
                };
            }

            log:printError(ERR_MSG_FETCHING_PROJECT_DETAILS, projectResponse);
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_FETCHING_PROJECT_DETAILS
                }
            };
        }

        // Fetch case stats 
        entity:ProjectCaseStatsResponse|error caseStats = entity:getCaseStatsForProject(userInfo.idToken, id);
        if caseStats is error {
            string customError = "Failed to retrieve project case statistics.";
            log:printError(customError, caseStats);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        // Fetch chat stats
        entity:ProjectChatStatsResponse|error chatStats = entity:getChatStatsForProject(userInfo.idToken, id);
        if chatStats is error {
            string customError = "Failed to retrieve project chat statistics.";
            log:printError(customError, chatStats);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return {
            totalCases: caseStats.totalCount,
            activeChats: chatStats.activeCount,
            sessionChats: chatStats.sessionCount,
            resolvedChats: chatStats.resolvedCount
        };
    }

    # Get case details by ID.
    #
    # + id - ID of the case
    # + return - Case details or error
    resource function get cases/[string id](http:RequestContext ctx)
        returns entity:CaseResponse|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        if isEmptyId(id) {
            return <http:BadRequest>{
                body: {
                    message: ERR_MSG_CASE_ID_EMPTY
                }
            };
        }

        entity:CaseResponse|error caseResponse = entity:getCase(userInfo.idToken, id);
        if caseResponse is error {
            if getStatusCode(caseResponse) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            if getStatusCode(caseResponse) == http:STATUS_FORBIDDEN {
                logForbiddenCaseAccess(id, userInfo.userId);
                return <http:Forbidden>{
                    body: {
                        message: ERR_MSG_CASE_ACCESS_FORBIDDEN
                    }
                };
            }

            string customError = "Failed to retrieve case details.";
            log:printError(customError, caseResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return caseResponse;
    }

    # Create a new case.
    #
    # + payload - Case creation payload
    # + return - Success message or error response
    resource function post cases(http:RequestContext ctx, entity:CaseCreatePayload payload)
        returns http:Created|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:CaseCreateResponse|error createdCaseResponse = entity:createCase(userInfo.idToken, payload);
        if createdCaseResponse is error {
            if getStatusCode(createdCaseResponse) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            if getStatusCode(createdCaseResponse) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${userInfo.userId} is forbidden to create a case for project: ${
                        payload.projectId}!`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to create a case for the selected project. " +
                        "Please check your access permissions or contact support."
                    }
                };
            }

            string customError = "Failed to create a new case.";
            log:printError(customError, createdCaseResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return <http:Created>{
            body: createdCaseResponse.case
        };
    }

    # Search cases for a specific project with filters and pagination.
    #
    # + id - ID of the project
    # + payload - Case search request body
    # + return - Paginated cases or error
    resource function post projects/[string id]/cases/search(http:RequestContext ctx, CaseSearchPayload payload)
        returns http:Ok|http:BadRequest|http:Unauthorized|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        if isEmptyId(id) {
            return <http:BadRequest>{
                body: {
                    message: ERR_MSG_PROJECT_ID_EMPTY
                }
            };
        }

        CaseSearchResponse|error casesResponse = searchCases(userInfo.idToken, id, payload);
        if casesResponse is error {
            if getStatusCode(casesResponse) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            string customError = "Failed to retrieve cases.";
            log:printError(customError, casesResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return <http:Ok>{
            body: casesResponse
        };
    }

    # Get case filters for a project.
    #
    # + id - ID of the project
    # + return - Case filters or error
    resource function get projects/[string id]/cases/filters(http:RequestContext ctx)
        returns CaseFilterOptions|http:BadRequest|http:Unauthorized|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        if isEmptyId(id) {
            return <http:BadRequest>{
                body: {
                    message: ERR_MSG_PROJECT_ID_EMPTY
                }
            };
        }

        entity:CaseMetadataResponse|error caseMetadata = entity:getCaseMetadata(userInfo.idToken);
        if caseMetadata is error {
            if getStatusCode(caseMetadata) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            string customError = "Failed to retrieve case filters.";
            log:printError(customError, caseMetadata);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return getCaseFilters(caseMetadata);
    }

    # Get comments for a specific case.
    #
    # + id - ID of the case
    # + limit - Number of comments to retrieve
    # + offset - Offset for pagination
    # + return - Comments response or error
    resource function get cases/[string id]/comments(http:RequestContext ctx, int? 'limit, int? offset)
        returns CommentsResponse|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        if isEmptyId(id) {
            return <http:BadRequest>{
                body: {
                    message: ERR_MSG_CASE_ID_EMPTY
                }
            };
        }

        if isInvalidLimitOffset('limit, offset) {
            return <http:BadRequest>{
                body: {
                    message: ERR_LIMIT_OFFSET_INVALID
                }
            };
        }

        // Verify case validation for the user
        entity:CaseResponse|error caseDetails = entity:getCase(userInfo.idToken, id);
        if caseDetails is error {
            if getStatusCode(caseDetails) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            if getStatusCode(caseDetails) == http:STATUS_FORBIDDEN {
                logForbiddenCaseAccess(id, userInfo.userId);
                return <http:Forbidden>{
                    body: {
                        message: ERR_MSG_CASE_ACCESS_FORBIDDEN
                    }
                };
            }

            string customError = "Failed to retrieve case details.";
            log:printError(customError, caseDetails);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        entity:CommentsResponse|error commentsResponse = entity:getComments(userInfo.idToken, id, 'limit, offset);
        if commentsResponse is error {
            string customError = "Failed to retrieve comments.";
            log:printError(customError, commentsResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return mapCommentsResponse(commentsResponse);
    }

    # Get attachments for a specific case.
    #
    # + id - ID of the case
    # + limit - Number of attachments to retrieve
    # + offset - Offset for pagination
    # + return - Attachments response or error
    resource function get cases/[string id]/attachments(http:RequestContext ctx, int? 'limit, int? offset)
        returns AttachmentsResponse|http:BadRequest|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        if isEmptyId(id) {
            return <http:BadRequest>{
                body: {
                    message: ERR_MSG_CASE_ID_EMPTY
                }
            };
        }

        if isInvalidLimitOffset('limit, offset) {
            return <http:BadRequest>{
                body: {
                    message: ERR_LIMIT_OFFSET_INVALID
                }
            };
        }

        // Verify case validation for the user
        entity:CaseResponse|error caseDetails = entity:getCase(userInfo.idToken, id);
        if caseDetails is error {
            if getStatusCode(caseDetails) == http:STATUS_FORBIDDEN {
                logForbiddenCaseAccess(id, userInfo.userId);
                return <http:Forbidden>{
                    body: {
                        message: ERR_MSG_CASE_ACCESS_FORBIDDEN
                    }
                };
            }

            string customError = "Failed to retrieve case details.";
            log:printError(customError, caseDetails);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        entity:AttachmentsResponse|error attachmentResponse =
            entity:getAttachments(userInfo.idToken, id, 'limit, offset);
        if attachmentResponse is error {
            string customError = "Failed to retrieve attachments.";
            log:printError(customError, attachmentResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return mapAttachmentsResponse(attachmentResponse);
    }

    # Get products of a deployment by deployment ID.
    #
    # + id - ID of the deployment
    # + return - Deployed products response or error response
    resource function get deployments/[string id]/products(http:RequestContext ctx)
        returns DeployedProduct[]|http:BadRequest|http:Forbidden|http:InternalServerError {
    
        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        if isEmptyId(id) {
            return <http:BadRequest>{
                body: {
                    message: "Deployment ID cannot be empty!"
                }
            };
        }

        entity:DeployedProductsResponse|error productsResponse = entity:getDeployedProducts(userInfo.idToken, id);
        if productsResponse is error {
            if getStatusCode(productsResponse) == http:STATUS_FORBIDDEN {
                log:printWarn(string `Access to deployment ID: ${id} is forbidden for user: ${userInfo.userId}`);
                return <http:Forbidden>{
                    body: {
                        message: "Access to the requested deployment is forbidden!"
                    }
                };
            }

            string customError = "Failed to retrieve products for the deployment.";
            log:printError(customError, productsResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return mapDeployedProducts(productsResponse);
    }

    # Get recommended update levels.
    #
    # + return - List of recommended update levels or an error
    resource function get updates/recommended\-update\-levels(http:RequestContext ctx)
        returns updates:RecommendedUpdateLevel[]|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        updates:RecommendedUpdateLevel[]|error recommendedUpdateLevels =
            updates:getRecommendedUpdateLevels(userInfo.email);
        if recommendedUpdateLevels is error {
            string customError = "Failed to retrieve recommended update levels.";
            log:printError(customError, recommendedUpdateLevels);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return recommendedUpdateLevels;
    }

    # Search updates based on provided filters.
    #
    # + payload - Update search payload containing filters
    # + return - List of updates matching or an error
    resource function post updates/search(http:RequestContext ctx, updates:ListUpdatePayload payload)
        returns updates:UpdateResponse|http:BadRequest|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        updates:UpdateResponse|error updateResponse = updates:listUpdates(userInfo.idToken, payload);
        if updateResponse is error {
            string customError = "Failed to search updates.";
            log:printError(customError, updateResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return updateResponse;
    }

    # Get product update levels.
    # 
    # + return - List of product update levels or an error
    resource function get updates/product\-update\-levels(http:RequestContext ctx)
        returns updates:ProductUpdateLevel[]|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        updates:ProductUpdateLevel[]|error productUpdateLevels = updates:getProductUpdateLevels(userInfo.idToken);
        if productUpdateLevels is error {
            string customError = "Failed to get product update levels.";
            log:printError(customError, productUpdateLevels);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return productUpdateLevels;
    }
}
