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
import customer_portal.scim;

import ballerina/http;
import ballerina/log;

service class ErrorInterceptor {
    *http:ResponseErrorInterceptor;

    remote function interceptResponseError(error err, http:RequestContext ctx) returns http:BadRequest|error {

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

    # Fetch user information of the logged in users.
    #
    # + ctx - Request context object
    # + return - User info object|Error
    resource function get user\-info(http:RequestContext ctx)
        returns entity:UserInfo|http:InternalServerError {

        // Get user info from JWT header.
        authorization:UserDataPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: "User information header not found!"
                }
            };
        }

        log:printDebug(string `Processing user-info request for user: ${userInfo.email}`);

        // Fetch user info from entity service with ID token.
        entity:UserInfo|error userInfoResponse = entity:fetchUserBasicInfo(userInfo.email, userInfo.idToken);

        if userInfoResponse is error {
            string customError = string `Error occurred while retrieving user data: ${userInfo.email}!`;
            log:printError(customError, userInfoResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return userInfoResponse;
    }

    # Fetch cases for a specific project.
    #
    # + ctx - Request context object
    # + offset - Offset for pagination
    # + 'limit - Limit for pagination
    # + return - Cases response object|Error
    isolated resource function get cases(http:RequestContext ctx, int offset = 0, int 'limit = 10)
        returns entity:Cases|http:BadRequest|http:InternalServerError {

        authorization:UserDataPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            log:printError("User information header not found", userInfo);
            return <http:InternalServerError>{
                body: {
                    message: "User information header not found!"
                }
            };
        }

        log:printDebug(string `Processing cases request for user: ${userInfo.email}`);

        // Fetch cases from entity service.
        entity:Cases|error casesResponse = entity:fetchCases(
                email = userInfo.email, idToken = userInfo.idToken, offset = offset, 'limit = 'limit);

        if casesResponse is error {
            string customError = "Error occurred while retrieving cases data!";
            log:printError(customError, casesResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return casesResponse;
    }

    # Retrieve case details for a specific case.
    #
    # + ctx - Request context object
    # + caseId - Unique identifier of the case
    # + return - Case details object|Error
    isolated resource function get cases/[string caseId](http:RequestContext ctx)
        returns entity:CaseDetails|http:BadRequest|http:InternalServerError {

        authorization:UserDataPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            log:printError("User information header not found", userInfo);
            return <http:InternalServerError>{
                body: {
                    message: "User information header not found!"
                }
            };
        }

        log:printDebug(string `Processing case details request for user: ${userInfo.email}, caseId: ${caseId}`);

        // Validate caseId
        if caseId.trim().length() == 0 {
            string customError = "Case ID cannot be empty";
            return <http:BadRequest>{
                body: {
                    message: customError
                }
            };

        }

        entity:CaseDetails|error caseDetailsResponse = entity:fetchCaseDetails(
                caseId = caseId, idToken = userInfo.idToken);
        if caseDetailsResponse is error {
            string customError = string `Error occurred while retrieving case details for caseId: ${caseId}!`;
            log:printError(customError, caseDetailsResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return caseDetailsResponse;
    }

    # Add users to a group via SCIM operations.
    #
    # + ctx - Request context object
    # + group - Group name to add users to
    # + payload - Request payload containing array of user emails
    # + return - Created response on success, error responses on failure
    isolated resource function post groups/[string group]/users(
            http:RequestContext ctx, @http:Payload scim:AddUsersRequest payload)
            returns http:Created|http:BadRequest|http:InternalServerError {

        authorization:UserDataPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            string customError = "User information header not found!";
            log:printError(customError, userInfo);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        // Adding debug logs as this involves ServiceNow integration.
        log:printDebug(string `Processing add users to group request for user: ${userInfo.email}, group: ${group}`);

        // Validate input
        error? validation = validateGroupName(group) ?: validateEmails(payload.emails);
        if validation is error {
            string customError = string `Validation failed: ${validation.message()}`;
            log:printError(customError, validation);
            return <http:BadRequest>{
                body: {
                    message: customError
                }
            };
        }

        // Invoke SCIM operations service
        scim:AddUsersResponse|error result = scim:addUsersToGroup(group, payload);
        if result is error {
            string customError = string `Error occurred while adding users to group: ${group}`;
            log:printError(customError, result);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return <http:Created>{
            body: {
                message: MSG_USERS_ADDED_SUCCESS,
                addedUsers: result.addedUsers,
                failedUsers: result.failedUsers
            }
        };
    }

    # Fetch projects for a user using their email.
    #
    # + ctx - Request context object
    # + return - Array of projects, or an error
    isolated resource function get projects(http:RequestContext ctx)
        returns entity:EntityProject[]|http:InternalServerError {

        authorization:UserDataPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            string customError = "User information header not found!";
            log:printError("User information header not found", userInfo);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        log:printDebug(string `Processing projects request for user: ${userInfo.email}`);

        // Fetch projects from entity service.
        entity:EntityProject[]|error projectsResponse = entity:fetchProjects(
                email = userInfo.email, idToken = userInfo.idToken);

        if projectsResponse is error {
            string customError = "Error occurred while retrieving projects data!";
            log:printError(customError, projectsResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return projectsResponse;
    }
}
