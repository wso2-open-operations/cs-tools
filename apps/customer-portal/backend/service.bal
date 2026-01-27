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
    resource function get users/me(http:RequestContext ctx) returns User|http:Forbidden|http:InternalServerError {
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
            User|error cachedUser = userCache.get(cacheKey).ensureType();
            if cachedUser is User {
                return cachedUser;
            }
            log:printWarn(string `Unable to read cached user info for ${userInfo.email}`);
        }

        entity:UserResponse|error userDetails = entity:getUserBasicInfo(userInfo.email, userInfo.idToken);
        if userDetails is error {
            if getStatusCode(userDetails) == http:STATUS_FORBIDDEN {
                // TODO: Will log the UUID once the PR #42 is merged
                log:printWarn(string `User: does not have access to the customer portal!`);
                return <http:Forbidden>{
                    body: {
                        message: "User is not authorized to access the customer portal!"
                    }
                };
            }

            string customError = "Error retrieving user data from entity service";
            log:printError(customError, userDetails);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        string? phoneNumber = getPhoneNumber(userInfo.email, userDetails.id);

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

        authorization:UserDataPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
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
                    message: "At least one field must be provided for update"
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
                            message: getErrorMessage(updatedUser)
                        }
                    };
                }

                string customError = "Error updating user phone number";
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
            updatedUserResponse.phoneNumber = processPhoneNumber(updatedUser);
        }

        if payload.timeZone is string {
            // TODO: Update timezone
        }

        return updatedUserResponse;
    }

    # Search projects of the logged-in user.
    #
    # + payload - Project search request body
    # + return - Projects list or error response
    resource function post projects/search(http:RequestContext ctx, entity:ProjectRequest payload)
        returns entity:ProjectsResponse|http:BadRequest|http:Forbidden|http:InternalServerError {

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
            if getStatusCode(projectsList) == http:STATUS_FORBIDDEN {
                // TODO: Will log the UUID once the PR #42 is merged
                log:printWarn(string `Access to requested projects are forbidden for user:`);
                return <http:Forbidden>{
                    body: {
                        message: "Access to the requested project is forbidden!"
                    }
                };
            }

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

    # Get project details by ID.
    #
    # + id - ID of the project
    # + return - Project details or error response
    resource function get projects/[string id](http:RequestContext ctx)
        returns entity:ProjectDetailsResponse|http:BadRequest|http:Forbidden|http:InternalServerError {

        authorization:UserDataPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        if !isValidId(id) {
            return <http:BadRequest>{
                body: {
                    message: "Project ID cannot be empty or whitespace"
                }
            };
        }

        entity:ProjectDetailsResponse|error projectResponse = entity:getProject(userInfo.idToken, id);
        if projectResponse is error {
            if getStatusCode(projectResponse) == http:STATUS_FORBIDDEN {
                // TODO: Will log the UUID once the PR #42 is merged
                log:printWarn(string `Access to project ID: ${id} is forbidden for user:`);
                return <http:Forbidden>{
                    body: {
                        message: "Access to the requested project is forbidden!"
                    }
                };
            }

            string customError = "Error retrieving project details";
            log:printError(customError, projectResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return projectResponse;
    }

    # Get case details by ID.
    # 
    # + id - ID of the case
    # + return - Case details or error
    resource function get cases/[string id](http:RequestContext ctx) 
        returns entity:CaseResponse|http:BadRequest|http:Forbidden|http:InternalServerError {

        authorization:UserDataPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        if !isValidId(id) {
            return <http:BadRequest>{
                body: {
                    message: "Case ID cannot be empty or whitespace"
                }
            };
        }

        entity:CaseResponse|error caseResponse = entity:getCase(userInfo.idToken, id);
        if caseResponse is error {
            if getStatusCode(caseResponse) == http:STATUS_FORBIDDEN {
                // TODO: Will log the UUID once the PR #42 is merged
                log:printWarn(string `Access to case ID: ${id} is forbidden for user:`);
                return <http:Forbidden>{
                    body: {
                        message: "Access to the requested case is forbidden!"
                    }
                };
            }

            string customError = "Error retrieving case details";
            log:printError(customError, caseResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return caseResponse;
    }

    # Search cases for a specific project with filters and pagination.
    #
    # + id - ID of the project
    # + payload - Case search request body
    # + return - Paginated cases or error
    resource function post projects/[string id]/cases/search(http:RequestContext ctx, CaseSearchPayload payload)
        returns CaseSearchResponse|http:BadRequest|http:InternalServerError {

        authorization:UserDataPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        if !isValidId(id) {
            return <http:BadRequest>{
                body: {
                    message: "Project ID cannot be empty or whitespace"
                }
            };
        }

        CaseSearchResponse|error casesResponse = searchCases(userInfo.idToken, id, payload);
        if casesResponse is error {
            string customError = "Error retrieving cases";
            log:printError(customError, casesResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return casesResponse;
    }

    # Get case filters for a project.
    #
    # + id - ID of the project
    # + return - Case filters or error
    resource function get projects/[string id]/cases/filters(http:RequestContext ctx)
        returns CaseFilterOptions|http:BadRequest|http:InternalServerError {

        authorization:UserDataPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        if !isValidId(id) {
            return <http:BadRequest>{
                body: {
                    message: "Project ID cannot be empty or whitespace"
                }
            };
        }

        entity:CaseMetadataResponse|error caseMetadata = entity:getCaseMetadata(userInfo.idToken);
        if caseMetadata is error {
            string customError = "Error retrieving case filters";
            log:printError(customError, caseMetadata);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return getCaseFilters(caseMetadata);
    }
}
