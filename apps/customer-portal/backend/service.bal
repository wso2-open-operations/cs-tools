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

import customer_portal.ai_chat_agent;
import customer_portal.authorization;
import customer_portal.entity;
import customer_portal.product_consumption_subscription;
import customer_portal.registry;
import customer_portal.scim;
import customer_portal.types;
import customer_portal.updates;
import customer_portal.user_management;

import ballerina/http;
import ballerina/log;
import ballerina/time;
import ballerina/websocket;

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

configurable int wsPort = 9091;

http:ListenerConfiguration listenerConf = {
    requestLimits: {
        maxHeaderSize: 32768
    }
};

@display {
    label: "Customer Portal",
    id: "cs/customer-portal"
}
service http:InterceptableService / on new http:Listener(9090, listenerConf) {
    public function createInterceptors() returns http:Interceptor[] =>
        [new authorization:JwtInterceptor(), new authorization:ResponseInterceptor(), new ErrorInterceptor()];

    # Service init function.
    #
    # + return - Error if initialization fails
    function init() returns error? {
        log:printInfo("Customer Portal backend started.");
    }

    # Fetch metadata information for the customer portal.
    #
    # + return - Metadata information or error response
    resource function get metadata(http:RequestContext ctx)
        returns types:MetadataResponse|http:Unauthorized|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }
        entity:MetadataResponse|error metadataResponse = entity:getMetadata(userInfo.idToken);
        if metadataResponse is error {
            if getStatusCode(metadataResponse) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            string customError = "Failed to retrieve metadata information.";
            log:printError(customError, metadataResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return mapMetadataResponse(metadataResponse);
    }

    # Fetch user information of the logged in user.
    #
    # + return - User info object or error response
    resource function get users/me(http:RequestContext ctx)
        returns types:User|http:Unauthorized|http:NotFound|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
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
            if getStatusCode(userDetails) == http:STATUS_NOT_FOUND {
                log:printWarn(string `User details not found for user: ${userInfo.userId}`);
                return <http:NotFound>{
                    body: {
                        message: "User information not found."
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
        string? lastPasswordUpdateTime = ();
        scim:User[]|error userResults = scim:searchUsers(userInfo.email);
        if userResults is error {
            // Log the error and return nil
            log:printError("Error retrieving user phone number and last password update time from scim service",
                    userResults);
        } else {
            if userResults.length() == 0 {
                log:printError(string
                        `No user found while searching phone number and last password update time for user: ${
                        userInfo.userId}`);
            } else {
                phoneNumber = scim:processPhoneNumber(userResults[0]);
                lastPasswordUpdateTime = scim:processLastPasswordUpdateTime(userResults[0]);
            }
        }

        return {
            id: userDetails.id,
            email: userDetails.email,
            firstName: userDetails.firstName,
            lastName: userDetails.lastName,
            timeZone: userDetails.timeZone,
            roles: userDetails.roles,
            phoneNumber,
            lastPasswordUpdateTime
        };
    }

    # Update user information of the logged in user.
    #
    # + payload - User update payload
    # + return - Updated user object or error response
    resource function patch users/me(http:RequestContext ctx, types:UserUpdatePayload payload)
        returns types:UpdatedUser|http:BadRequest|http:InternalServerError {

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

        types:UpdatedUser updatedUserResponse = {};

        if payload.phoneNumber is string {
            scim:Phone phoneNumber = {mobile: payload.phoneNumber};
            scim:User|error updatedUser = scim:updateUser({phoneNumber}, userInfo.email, userInfo.userId);
            if updatedUser is error {
                if getStatusCode(updatedUser) == http:STATUS_BAD_REQUEST {
                    log:printWarn(extractErrorMessage(updatedUser));
                } else {
                    log:printError("Failed to update phone number.", updatedUser);
                }
            } else {
                updatedUserResponse.phoneNumber = scim:processPhoneNumber(updatedUser);
            }
        }

        string? timeZone = payload.timeZone;
        if timeZone is string {
            entity:UserUpdateResponse|error response = entity:updateUser(userInfo.idToken, {timeZone});
            if response is error {
                if getStatusCode(response) == http:STATUS_BAD_REQUEST {
                    log:printWarn("Invalid timezone key provided.");
                } else {
                    log:printError("Failed to update user timezone.", response);
                }
            } else {
                updatedUserResponse.timeZone = timeZone;
            }
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

            if getStatusCode(projectsList) == http:STATUS_BAD_REQUEST {
                string customError = "Invalid request parameters for searching projects.";
                log:printWarn(customError, projectsList);
                return <http:BadRequest>{
                    body: {
                        message: customError
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
            body: mapProjectsResponse(projectsList)
        };
    }

    # Get project details by ID.
    #
    # + id - ID of the project
    # + return - Project details or error response
    resource function get projects/[entity:IdString id](http:RequestContext ctx)
        returns types:ProjectResponse|http:BadRequest|http:Unauthorized|http:Forbidden|http:NotFound|
        http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
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
            if getStatusCode(projectResponse) == http:STATUS_NOT_FOUND {
                log:printWarn(string `Project with ID: ${id} not found for user: ${userInfo.userId}`);
                return <http:NotFound>{
                    body: {
                        message: "The requested project does not exist or you don't have access to it."
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
        return mapProjectResponse(projectResponse);
    }

    # Update project details by ID.
    #
    # + id - ID of the project
    # + payload - Project update payload
    # + return - Updated project details or error response
    resource function patch projects/[entity:IdString id](http:RequestContext ctx, entity:ProjectUpdatePayload payload)
        returns entity:UpdatedProject|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        string? validateProjectUpdatePayload = entity:validateProjectUpdatePayload(payload);
        if validateProjectUpdatePayload is string {
            log:printWarn(validateProjectUpdatePayload);
            return <http:BadRequest>{
                body: {
                    message: validateProjectUpdatePayload
                }
            };
        }

        entity:ProjectUpdateResponse|error response = entity:updateProject(userInfo.idToken, id, payload);
        if response is error {
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                logForbiddenProjectAccess(id, userInfo.userId);
                return <http:Forbidden>{
                    body: {
                        message: ERR_MSG_PROJECT_ACCESS_FORBIDDEN
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_BAD_REQUEST {
                return <http:BadRequest>{
                    body: {
                        message: "Invalid input provided for project update. Please check the payload and try again."
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            string customError = "Failed to update the project.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return response.project;
    }

    # Search deployments of a project.
    #
    # + id - ID of the project
    # + payload - Payload for searching deployments of the project
    # + return - Deployments response or error response
    resource function post projects/[entity:IdString id]/deployments/search(http:RequestContext ctx,
            types:DeploymentSearchPayload payload)
        returns http:Ok|http:BadRequest|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:DeploymentsResponse|error deploymentsResponse = entity:searchDeployments(userInfo.idToken,
                {
                    filters: {
                        projectIds: [id]
                    },
                    pagination: payload.pagination
                });
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
        return <http:Ok>{body: mapDeployments(deploymentsResponse)};
    }

    # Create a new deployment for a project.
    #
    # + id - ID of the project
    # + payload - Deployment creation payload
    # + return - Created deployment or error response
    resource function post projects/[entity:IdString id]/deployments(http:RequestContext ctx,
            types:DeploymentCreatePayload payload)
        returns entity:CreatedDeployment|http:BadRequest|http:Unauthorized|http:Forbidden|http:Conflict|
        http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:DeploymentCreateResponse|error deploymentResponse = entity:createDeployment(userInfo.idToken,
                {
                    projectId: id,
                    name: payload.name,
                    description: payload.description,
                    typeKey: payload.deploymentTypeKey
                });
        if deploymentResponse is error {
            if getStatusCode(deploymentResponse) == http:STATUS_FORBIDDEN {
                logForbiddenProjectAccess(id, userInfo.userId);
                return <http:Forbidden>{
                    body: {
                        message: ERR_MSG_PROJECT_ACCESS_FORBIDDEN
                    }
                };
            }

            if getStatusCode(deploymentResponse) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            if getStatusCode(deploymentResponse) == http:STATUS_BAD_REQUEST {
                string customError = "Invalid request parameters for creating deployment for the project.";
                log:printWarn(customError, deploymentResponse);
                return <http:BadRequest>{
                    body: {
                        message: customError
                    }
                };
            }

            if getStatusCode(deploymentResponse) == http:STATUS_CONFLICT {
                string customError = "A deployment with the same name already exists for the project.";
                log:printWarn(customError, deploymentResponse);
                return <http:Conflict>{
                    body: {
                        message: customError
                    }
                };
            }

            string customError = "Failed to create deployment for the project.";
            log:printError(customError, deploymentResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return deploymentResponse.deployment;
    }

    # Update an existing deployment for a project.
    #
    # + projectId - ID of the project
    # + deploymentId - ID of the deployment to be updated
    # + payload - Deployment update payload
    # + return - Updated deployment or error response
    resource function patch projects/[entity:IdString projectId]/deployments/[entity:IdString deploymentId](
            http:RequestContext ctx, entity:DeploymentUpdatePayload payload)
        returns entity:UpdatedDeployment|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        string? validateDeploymentUpdatePayload = entity:validateDeploymentUpdatePayload(payload);
        if validateDeploymentUpdatePayload is string {
            log:printWarn(validateDeploymentUpdatePayload);
            return <http:BadRequest>{
                body: {
                    message: validateDeploymentUpdatePayload
                }
            };
        }

        entity:DeploymentUpdateResponse|error response = entity:updateDeployment(userInfo.idToken,
                deploymentId, payload);
        if response is error {
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${userInfo.userId} is forbidden to update deployment: ${
                        deploymentId} for project: ${projectId}`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to update the deployment for the selected project."
                    }
                };
            }

            if getStatusCode(response) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            string customError = "Failed to update the deployment.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return response.deployment;
    }

    # Search instances of a project.
    #
    # + id - ID of the project
    # + payload - Payload for searching instances of the project
    # + return - Instances response or error response
    resource function post projects/[entity:IdString id]/instances/search(http:RequestContext ctx,
            types:InstanceSearchPayload payload)
        returns http:Ok|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }
        entity:InstancesResponse|error instances = entity:searchInstances(userInfo.idToken,
                {
                    filters: {
                        projectIds: [id],
                        startDate: payload.filters?.startDate,
                        endDate: payload.filters?.endDate
                    },
                    pagination: payload.pagination
                });
        if instances is error {
            if getStatusCode(instances) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${
                        userInfo.userId} is forbidden to search instances for project with ID: ${id}!`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to search instances for the project. " +
                        "Please check your access permissions or contact support."
                    }
                };
            }
            if getStatusCode(instances) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(instances) == http:STATUS_BAD_REQUEST {
                string customError = "Invalid request parameters for searching instances for the project.";
                log:printWarn(customError, instances);
                return <http:BadRequest>{
                    body: {
                        message: customError
                    }
                };
            }

            string customError = "Failed to search instances for the project.";
            log:printError(customError, instances);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return <http:Ok>{body: mapInstancesResponse(instances)};
    }

    # Get attachments for a specific deployment.
    #
    # + id - ID of the deployment
    # + limit - Number of attachments to retrieve
    # + offset - Offset for pagination
    # + return - Attachments response or error
    resource function get deployments/[entity:IdString id]/attachments(http:RequestContext ctx, int? 'limit,
            int? offset) returns types:AttachmentsResponse|http:BadRequest|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
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

        entity:AttachmentsResponse|error attachmentResponse =
            entity:getAttachments(userInfo.idToken, id, entity:DEPLOYMENT, 'limit, offset);
        if attachmentResponse is error {
            if getStatusCode(attachmentResponse) == http:STATUS_FORBIDDEN {
                log:printWarn(string `Access to deployment attachments is forbidden for user: ${userInfo.userId}`);
                return <http:Forbidden>{
                    body: {
                        message: "Access to deployment attachments is forbidden!"
                    }
                };
            }

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

    # Add an attachment to a deployment.
    #
    # + id - ID of the deployment
    # + payload - Attachment creation payload
    # + return - Created attachment or error response
    resource function post deployments/[entity:IdString id]/attachments(http:RequestContext ctx,
            types:AttachmentCreatePayload payload)
        returns types:CreatedAttachment|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:AttachmentCreateResponse|error createdAttachmentResponse = entity:createAttachment(userInfo.idToken,
                {
                    referenceId: id,
                    referenceType: entity:DEPLOYMENT,
                    name: payload.name,
                    'type: payload.'type,
                    file: payload.content,
                    description: payload?.description
                });
        if createdAttachmentResponse is error {
            if getStatusCode(createdAttachmentResponse) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            if getStatusCode(createdAttachmentResponse) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${userInfo.userId} is forbidden to add attachment to deployment with ID: ${
                        id}!`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to add attachments to the requested deployment. " +
                        "Please check your access permissions or contact support."
                    }
                };
            }

            string customError = "Failed to create a new attachment.";
            log:printError(customError, createdAttachmentResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return {
            id: createdAttachmentResponse.attachment.id,
            size: createdAttachmentResponse.attachment.sizeBytes,
            createdOn: createdAttachmentResponse.attachment.createdOn,
            createdBy: createdAttachmentResponse.attachment.createdBy,
            downloadUrl: createdAttachmentResponse.attachment.downloadUrl
        };
    }

    # Update an attachment of a deployment.
    #
    # + deploymentId - ID of the deployment
    # + attachmentId - ID of the attachment to be updated
    # + payload - Attachment update payload
    # + return - Updated attachment or error response
    resource function patch deployments/[entity:IdString deploymentId]/attachments/[entity:IdString attachmentId](
            http:RequestContext ctx, types:AttachmentUpdatePayload payload)
        returns entity:UpdatedAttachment|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:AttachmentUpdatePayload updatePayload = {
            referenceId: deploymentId,
            referenceType: entity:DEPLOYMENT,
            name: payload?.name,
            description: payload?.description
        };
        string? validateAttachmentUpdatePayload = entity:validateAttachmentUpdatePayload(updatePayload);
        if validateAttachmentUpdatePayload is string {
            return <http:BadRequest>{
                body: {
                    message: validateAttachmentUpdatePayload
                }
            };
        }

        entity:AttachmentUpdateResponse|error response = entity:updateAttachment(userInfo.idToken, attachmentId,
                updatePayload);
        if response is error {
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${userInfo.userId} is forbidden to update attachment with ID: ${
                        attachmentId}!`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to update attachments for the requested deployment. " +
                        "Please check your access permissions or contact support."
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_BAD_REQUEST {
                string customError = "Invalid request parameters for updating the attachment.";
                log:printWarn(customError);
                return <http:BadRequest>{
                    body: {
                        message: customError
                    }
                };
            }

            string customError = "Failed to update the attachment.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return response.attachment;
    }

    # Search instances of a deployment.
    #
    # + id - ID of the deployment
    # + payload - Payload for searching instances of the deployment
    # + return - Instances response or error response
    resource function post deployments/[entity:IdString id]/instances/search(http:RequestContext ctx,
            types:InstanceSearchPayload payload)
        returns http:Ok|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }
        entity:InstancesResponse|error instances = entity:searchInstances(userInfo.idToken,
                {
                    filters: {
                        deploymentIds: [id],
                        startDate: payload.filters?.startDate,
                        endDate: payload.filters?.endDate
                    },
                    pagination: payload.pagination
                });
        if instances is error {
            if getStatusCode(instances) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${
                        userInfo.userId} is forbidden to search instances for deployment with ID: ${id}!`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to search instances for the deployment. " +
                        "Please check your access permissions or contact support."
                    }
                };
            }
            if getStatusCode(instances) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(instances) == http:STATUS_BAD_REQUEST {
                string customError = "Invalid request parameters for searching instances for the deployment.";
                log:printWarn(customError, instances);
                return <http:BadRequest>{
                    body: {
                        message: customError
                    }
                };
            }

            string customError = "Failed to search instances for the deployment.";
            log:printError(customError, instances);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return <http:Ok>{body: mapInstancesResponse(instances)};
    }

    # Get overall project statistics by ID.
    #
    # + id - ID of the project
    # + return - Project statistics response or error
    resource function get projects/[entity:IdString id]/stats(http:RequestContext ctx, entity:CaseType[]? caseTypes,
            entity:StatsFilter? createdBy)
        returns types:ProjectStatsResponse|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        // Fetch case stats
        entity:ProjectCaseStatsResponse|error caseStats =
            entity:getCaseStatsForProject(userInfo.idToken, id, caseTypes, createdBy);
        if caseStats is error {
            log:printError(ERR_MSG_CASES_STATISTICS, caseStats);
            // To return other stats even if case stats retrieval fails, error will not be returned.
        }

        // Fetch conversation stats
        entity:ProjectConversationStatsResponse|error conversationStats =
            entity:getConversationStatsForProject(userInfo.idToken, id, createdBy);
        if conversationStats is error {
            log:printError(ERR_MSG_CONVERSATION_STATISTICS, conversationStats);
            // To return other stats even if conversation stats retrieval fails, error will not be returned.
        }

        // Fetch deployment stats
        entity:ProjectDeploymentStatsResponse|error deploymentStats =
            entity:getDeploymentStatsForProject(userInfo.idToken, id);
        if deploymentStats is error {
            log:printError("Failed to retrieve project deployment statistics.", deploymentStats);
            // To return other stats even if deployment stats retrieval fails, error will not be returned.
        }

        // Fetch project activity stats
        entity:ProjectStatsResponse|error projectActivityStats = entity:getProjectActivityStats(userInfo.idToken, id);
        if projectActivityStats is error {
            log:printError("Failed to retrieve project activity statistics.", projectActivityStats);
            // To return other stats even if project activity stats retrieval fails, error will not be returned.
        }

        return {
            projectStats: {
                openCases: caseStats is entity:ProjectCaseStatsResponse ?
                    getOpenCasesCountFromProjectCasesStats(caseStats) : (),
                activeChats: conversationStats is entity:ProjectConversationStatsResponse ?
                    conversationStats.activeCount : (),
                deployments: deploymentStats is entity:ProjectDeploymentStatsResponse ? deploymentStats.totalCount : (),
                slaStatus: projectActivityStats is entity:ProjectStatsResponse ? projectActivityStats.slaStatus : (),
                outstandingCaseCount: projectActivityStats is entity:ProjectStatsResponse ?
                    projectActivityStats.outstandingCount.caseCount : (),
                outstandingServiceRequestCount: projectActivityStats is entity:ProjectStatsResponse ?
                    projectActivityStats.outstandingCount.serviceRequestCount : (),
                outstandingEngagementCount: projectActivityStats is entity:ProjectStatsResponse ?
                    projectActivityStats.outstandingCount.engagementCount : (),
                outstandingSraCount: projectActivityStats is entity:ProjectStatsResponse ?
                    projectActivityStats.outstandingCount.sraCount : (),
                outstandingChangeRequestCount: projectActivityStats is entity:ProjectStatsResponse ?
                    projectActivityStats.outstandingCount.changeRequestCount : (),
                outstandingAnnouncementCount: projectActivityStats is entity:ProjectStatsResponse ?
                    projectActivityStats.outstandingCount.announcementCount : ()
            },
            recentActivity: {
                totalHours:
                    projectActivityStats is entity:ProjectStatsResponse ? projectActivityStats.totalHours : (),
                billableHours:
                    projectActivityStats is entity:ProjectStatsResponse ? projectActivityStats.billableHours : (),
                lastDeploymentOn:
                    deploymentStats is entity:ProjectDeploymentStatsResponse ? deploymentStats.lastDeploymentOn : ()
            }
        };
    }

    # Get cases statistics for a project by ID.
    #
    # + id - ID of the project
    # + return - Project statistics overview or error response
    resource function get projects/[entity:IdString id]/stats/cases(http:RequestContext ctx,
            entity:CaseType[]? caseTypes, entity:StatsFilter? createdBy)
        returns types:ProjectCaseStats|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:ProjectCaseStatsResponse|error caseStats =
            entity:getCaseStatsForProject(userInfo.idToken, id, caseTypes, createdBy);
        if caseStats is error {
            log:printError(ERR_MSG_CASES_STATISTICS, caseStats);
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_CASES_STATISTICS
                }
            };
        }

        entity:ProjectChangeRequestStatsResponse|error changeReqStats =
            entity:getProjectChangeRequestStats(userInfo.idToken, id);
        if changeReqStats is error {
            log:printError("Failed to retrieve change request statistics.", changeReqStats);
            // To return other stats even if change request stats retrieval fails, error will not be returned.
        }

        return mapCaseStats(caseStats);
    }

    # Get conversation statistics for a project by ID.
    #
    # + id - ID of the project
    # + return - Conversation statistics overview or error response
    resource function get projects/[entity:IdString id]/stats/conversations(http:RequestContext ctx,
            entity:StatsFilter? createdBy)
        returns types:ConversationStats|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:ProjectConversationStatsResponse|error conversationStats =
            entity:getConversationStatsForProject(userInfo.idToken, id, createdBy);
        if conversationStats is error {
            if getStatusCode(conversationStats) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            if getStatusCode(conversationStats) == http:STATUS_FORBIDDEN {
                log:printWarn(string `Access to conversation statistics is forbidden for user: ${userInfo.userId}`);
                return <http:Forbidden>{
                    body: {
                        message: "Access to conversation statistics is forbidden!"
                    }
                };
            }
            string customError = "Failed to retrieve conversation statistics for the project.";
            log:printError(customError, conversationStats);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        types:OverallConversationStats {openCount, resolvedCount, activeCount, abandonedCount} =
            getConversationStats(conversationStats);

        return {openCount, resolvedCount, activeCount, abandonedCount};
    }

    # Get project support statistics by ID.
    #
    # + id - ID of the project
    # + return - Project support statistics or error response
    resource function get projects/[entity:IdString id]/stats/support(http:RequestContext ctx,
            entity:CaseType[]? caseTypes, entity:StatsFilter? createdBy)
        returns types:ProjectSupportStats|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        // Fetch case stats 
        entity:ProjectCaseStatsResponse|error caseStats =
            entity:getCaseStatsForProject(userInfo.idToken, id, caseTypes, createdBy);
        if caseStats is error {
            log:printError(ERR_MSG_CASES_STATISTICS, caseStats);
            // To return other stats even if case stats retrieval fails, error will not be returned.
        }

        // Fetch conversation stats
        entity:ProjectConversationStatsResponse|error conversationStats =
            entity:getConversationStatsForProject(userInfo.idToken, id, createdBy);
        if conversationStats is error {
            log:printError(ERR_MSG_CONVERSATION_STATISTICS, conversationStats);
            // To return other stats even if conversation stats retrieval fails, error will not be returned.
        }
        types:OverallConversationStats mappedConversationStats = getConversationStats(conversationStats);

        return {
            ongoingCases: caseStats is entity:ProjectCaseStatsResponse ? caseStats.activeCount : (),
            activeChats:
                conversationStats is entity:ProjectConversationStatsResponse ? conversationStats.activeCount : (),
            resolvedPast30DaysCasesCount:
                caseStats is entity:ProjectCaseStatsResponse ? caseStats.resolvedCount.pastThirtyDays : (),
            resolvedChats: mappedConversationStats.resolvedCount
        };
    }

    # Get case details by ID.
    #
    # + id - ID of the case
    # + return - Case details or error
    resource function get cases/[entity:IdString id](http:RequestContext ctx)
        returns types:CaseResponse|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
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
        return mapCaseResponse(caseResponse);
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

        string? validateCaseCreatePayload = entity:validateCaseCreatePayload(payload);
        if validateCaseCreatePayload is string {
            log:printWarn(validateCaseCreatePayload);
            return <http:BadRequest>{
                body: {
                    message: validateCaseCreatePayload
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
            if getStatusCode(createdCaseResponse) == http:STATUS_BAD_REQUEST {
                string customError = "Invalid request parameters for creating case for the project.";
                log:printWarn(customError, createdCaseResponse);
                return <http:BadRequest>{
                    body: {
                        message: customError
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
            body: mapCreatedCase(createdCaseResponse.case)
        };
    }

    # Update an existing case.
    #
    # + id - ID of the case to be updated
    # + payload - Case update payload
    # + return - Updated case details or error response
    resource function patch cases/[entity:IdString id](http:RequestContext ctx, entity:CaseUpdatePayload payload)
        returns types:UpdatedCase|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        string? validateCaseUpdatePayload = entity:validateCaseUpdatePayload(payload);
        if validateCaseUpdatePayload is string {
            log:printWarn(validateCaseUpdatePayload);
            return <http:BadRequest>{
                body: {
                    message: validateCaseUpdatePayload
                }
            };
        }

        entity:CaseUpdateResponse|error response = entity:updateCase(userInfo.idToken, id, payload);
        if response is error {
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${userInfo.userId} is forbidden to update case: ${
                        id}`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to update the selected case. " +
                        "Please check your access permissions or contact support."
                    }
                };
            }

            if getStatusCode(response) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            if getStatusCode(response) == http:STATUS_BAD_REQUEST {
                return <http:BadRequest>{
                    body: {
                        message: "Invalid request parameters for updating the case."
                    }
                };
            }

            if getStatusCode(response) == http:STATUS_NOT_FOUND {
                return <http:BadRequest>{
                    body: {
                        message: "The case you're trying to update does not exist. Please check and try again."
                    }
                };
            }

            string customError = "Failed to update the case.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return mapUpdatedCaseResponse(response.case);
    }

    # Search cases for a specific project with filters and pagination.
    #
    # + id - ID of the project
    # + payload - Case search request body
    # + return - Paginated cases or error
    resource function post projects/[entity:IdString id]/cases/search(http:RequestContext ctx,
            types:CaseSearchPayload payload)
        returns http:Ok|http:BadRequest|http:Unauthorized|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        types:CaseSearchResponse|error casesResponse = searchCases(userInfo.idToken, id, payload);
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
    resource function get projects/[entity:IdString id]/filters(http:RequestContext ctx)
        returns types:ProjectFilterOptions|http:Unauthorized|http:Forbidden|http:NotFound|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:ProjectMetadataResponse|error projectMetadata = entity:getProjectMetadata(userInfo.idToken, id);
        if projectMetadata is error {
            if getStatusCode(projectMetadata) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            if getStatusCode(projectMetadata) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${userInfo.userId} is forbidden to access project filters for project: ${
                        id}`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to access the filters for the selected project."
                    }
                };
            }

            if getStatusCode(projectMetadata) == http:STATUS_NOT_FOUND {
                log:printWarn(string `Project with ID: ${id} not found for user: ${userInfo.userId}`);
                return <http:NotFound>{
                    body: {
                        message: "The requested project does not exist or you don't have access to it."
                    }
                };
            }

            string customError = "Failed to retrieve project filters.";
            log:printError(customError, projectMetadata);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return getProjectFilters(projectMetadata);
    }

    # Get features for a project.
    #
    # + id - ID of the project
    # + return - Project features or error
    resource function get projects/[entity:IdString id]/features(http:RequestContext ctx)
        returns types:ProjectFeatures|http:Unauthorized|http:Forbidden|http:NotFound|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:ProjectMetadataResponse|error projectMetadata = entity:getProjectMetadata(userInfo.idToken, id);
        if projectMetadata is error {
            if getStatusCode(projectMetadata) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            if getStatusCode(projectMetadata) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${userInfo.userId} is forbidden to access project features for project: ${
                        id}`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to access the features for the selected project."
                    }
                };
            }

            if getStatusCode(projectMetadata) == http:STATUS_NOT_FOUND {
                log:printWarn(string `Project with ID: ${id} not found for user: ${userInfo.userId}`);
                return <http:NotFound>{
                    body: {
                        message: "The requested project does not exist or you don't have access to it."
                    }
                };
            }

            string customError = "Failed to retrieve project features.";
            log:printError(customError, projectMetadata);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return mapProjectFeatures(projectMetadata);
    }

    # Classify the case using AI chat agent.
    #
    # + payload - Case classification payload
    # + return - Case classification response or an error
    resource function post cases/classify(http:RequestContext ctx, ai_chat_agent:CaseClassificationPayload payload)
        returns ai_chat_agent:CaseClassificationResponse|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        ai_chat_agent:CaseClassificationResponse|error classificationResponse =
            ai_chat_agent:createCaseClassification(payload);
        if classificationResponse is error {
            string customError = "Failed to classify chat message.";
            log:printError(customError, classificationResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return classificationResponse;
    }

    # Get recommendations for a given chat history and conversation context.
    #
    # + payload - Recommendation request payload
    # + return - Recommendation response or an error
    resource function post conversations/recommendations/search(http:RequestContext ctx,
            ai_chat_agent:RecommendationRequest payload)
        returns ai_chat_agent:RecommendationResponse|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        ai_chat_agent:RecommendationResponse|error recommendationResponse =
            ai_chat_agent:getRecommendation(payload);
        if recommendationResponse is error {
            string customError = "Failed to retrieve recommendations.";
            log:printError(customError, recommendationResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return recommendationResponse;
    }

    # Search conversations for a specific project with filters and pagination.
    #
    # + id - ID of the project
    # + payload - Conversation search request body
    # + return - Paginated conversations or error
    resource function post projects/[entity:IdString id]/conversations/search(http:RequestContext ctx,
            types:ConversationSearchPayload payload)
        returns http:Ok|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:ConversationSearchResponse|error conversationResponse = entity:searchConversations(userInfo.idToken,
                {
                    filters: {
                        projectIds: [id],
                        stateKeys: payload.filters?.stateKeys,
                        searchQuery: payload.filters?.searchQuery,
                        createdByMe: payload.filters?.createdByMe
                    },
                    sortBy: payload.sortBy,
                    pagination: payload.pagination
                });
        if conversationResponse is error {
            if getStatusCode(conversationResponse) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(conversationResponse) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${userInfo.userId} is forbidden to search conversations for project: ${
                        id}`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to search conversations for the selected project."
                    }
                };
            }

            string customError = "Failed to retrieve conversations.";
            log:printError(customError, conversationResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return <http:Ok>{
            body: mapConversationSearchResponse(conversationResponse)
        };
    }

    # Create a new conversation for a project and get response using AI chat agent.
    #
    # + id - ID of the project
    # + payload - Conversation payload
    # + return - Chat response or an error
    resource function post projects/[entity:IdString id]/conversations(
            http:RequestContext ctx, ai_chat_agent:ConversationPayload payload)
        returns http:Ok|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:ConversationCreateResponse|error conversationResponse = entity:createConversation(userInfo.idToken,
                {
                    projectId: id,
                    initialMessage: payload.message
                });
        if conversationResponse is error {
            if getStatusCode(conversationResponse) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(conversationResponse) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${userInfo.userId} is forbidden to create conversations for project: ${
                        id}`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to create conversations for the selected project."
                    }
                };
            }
            if getStatusCode(conversationResponse) == http:STATUS_BAD_REQUEST {
                return <http:BadRequest>{
                    body: {
                        message: "Invalid request parameters for creating a conversation."
                    }
                };
            }

            string customError = "Failed to create a new conversation.";
            log:printError(customError, conversationResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        log:printDebug(string `Created conversation with ID: ${conversationResponse.conversation.id} for project: ${
                id}`);

        // Save the conversation message in agent and get the agent response for the conversation
        ai_chat_agent:ChatResponse|error chatResponse = ai_chat_agent:createChat(id,
                conversationResponse.conversation.id, payload);
        if chatResponse is error {
            string customError = "Failed to process chat message.";
            log:printError(customError, chatResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        log:printDebug(string `Received chat response from AI agent for conversation ID: ${
                conversationResponse.conversation.id}`);

        if payload.region.length() == 0 || payload.tier.length() == 0 {
            log:printWarn("Skipping recommendations due to missing region/tier in chat payload");
        } else {
            ai_chat_agent:RecommendationResponse|error recommendationResponse =
                    ai_chat_agent:getRecommendations(payload.message, chatResponse.message, payload.envProducts ?: {},
                    payload.region, payload.tier);
            if recommendationResponse is ai_chat_agent:RecommendationResponse {
                chatResponse.recommendations = recommendationResponse;
            } else {
                log:printWarn("Failed to retrieve recommendations for the first chat invocation",
                        recommendationResponse);
            }
        }

        // Save the agent response under comments
        entity:CommentCreateResponse|error createdCaseResponse = entity:createComment(userInfo.idToken,
                {
                    referenceId: conversationResponse.conversation.id,
                    referenceType: entity:CONVERSATION,
                    content: chatResponse.message,
                    'type: entity:COMMENTS,
                    createdBy: entity:CHAT_SENT_AGENT // Identify the comment is created from chat agent.
                });
        if createdCaseResponse is error {
            string customError = "Failed to save chat response as comment.";
            log:printError(customError, createdCaseResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        log:printDebug(string `Saved AI agent response as comment for conversation ID: ${
                conversationResponse.conversation.id}`);

        boolean? isIssueResolved = chatResponse.resolved;
        if isIssueResolved is boolean && isIssueResolved {
            // If the issue is resolved in the initial conversation, update the conversation state to resolved.
            entity:ConversationUpdateResponse|error conversationUpdateResponse =
                    entity:updateConversation(userInfo.idToken, conversationResponse.conversation.id,
                    {stateKey: entity:RESOLVED});
            if conversationUpdateResponse is error {
                string customError = "Failed to update conversation state to resolved.";
                log:printError(customError, conversationUpdateResponse);
                // Not returning error response since the main flow of creating conversation and getting chat response is successful.
            }
        }

        return <http:Ok>{
            body: chatResponse
        };
    }

    # Get Conversation summary for a project.
    #
    # + Id - ID of the project
    # + conversationId - ID of the conversation
    # + return - Conversation summary or error
    resource function get projects/[entity:IdString Id]/conversations/[entity:IdString conversationId]/summary(
            http:RequestContext ctx) returns ai_chat_agent:ConversationSummaryResponse|http:BadRequest|
        http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        // Verify user has access to the project before returning summary
        entity:ProjectResponse|error projectResponse = entity:getProject(userInfo.idToken, Id);
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
                log:printWarn(string `User: ${userInfo.userId} is forbidden to access project with ID: ${Id}!`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to access the requested project."
                    }
                };
            }

            string customError = "Failed to verify project access.";
            log:printError(customError, projectResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        ai_chat_agent:ConversationSummaryResponse|error summaryResponse = ai_chat_agent:getSummary(Id, conversationId);
        if summaryResponse is error {
            if getStatusCode(summaryResponse) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(summaryResponse) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${userInfo.userId} is forbidden to access conversation summary for conversation ID: ${
                        conversationId}`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to access the summary for the requested conversation."
                    }
                };
            }

            string customError = "Failed to retrieve conversation summary.";
            log:printError(customError, summaryResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return summaryResponse;
    }

    # Add a message to an existing conversation and get response using AI chat agent.
    #
    # + projectId - ID of the project
    # + conversationId - ID of the conversation
    # + payload - Conversation message payload
    # + return - Chat response or an error
    resource function post projects/[entity:IdString projectId]/conversations/[entity:IdString conversationId]/messages(
            http:RequestContext ctx, ai_chat_agent:ConversationPayload payload)
        returns http:Ok|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        // Save the conversation message in agent and get the agent response for the conversation
        ai_chat_agent:ChatResponse|error chatResponse = ai_chat_agent:createChat(projectId, conversationId, payload);
        if chatResponse is error {
            string customError = "Failed to process conversation message.";
            log:printError(customError, chatResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        log:printDebug(string `Received chat response from AI agent for conversation ID: ${
                conversationId} for follow-up message`);

        // Save the user query under comments
        entity:CommentCreateResponse|error createdCaseResponse = entity:createComment(userInfo.idToken,
                {
                    referenceId: conversationId,
                    referenceType: entity:CONVERSATION, // Indicate that the comment is related to a conversation.
                    content: payload.message,
                    'type: entity:COMMENTS,
                    createdBy: userInfo.email
                });
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
                log:printWarn(string `User: ${userInfo.userId} is forbidden to add comments to conversation with ID: ${
                        conversationId}!`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to add comments to the requested conversation. " +
                        "Please check your access permissions or contact support."
                    }
                };
            }
            if getStatusCode(createdCaseResponse) == http:STATUS_BAD_REQUEST {
                return <http:BadRequest>{
                    body: {
                        message: "Invalid request parameters for adding a comment to the conversation."
                    }
                };
            }

            string customError = "Failed to save user message as comment.";
            log:printError(customError, createdCaseResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        log:printDebug(string `Saved user message as comment for conversation ID: ${
                conversationId} for follow-up message`);

        // Save the agent response under comments
        entity:CommentCreateResponse|error createdAgentResponse = entity:createComment(userInfo.idToken,
                {
                    referenceId: conversationId,
                    referenceType: entity:CONVERSATION, // Indicate that the comment is related to a conversation.
                    content: chatResponse.message,
                    'type: entity:COMMENTS,
                    createdBy: entity:CHAT_SENT_AGENT // Identify the comment is created from chat agent.
                });
        if createdAgentResponse is error {
            string customError = "Failed to save chat response as comment.";
            log:printError(customError, createdAgentResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        log:printDebug(string `Saved AI agent response as comment for conversation ID: ${
                conversationId} for follow-up message`);

        boolean? isIssueResolved = chatResponse.resolved;
        if isIssueResolved is boolean && isIssueResolved {
            // If the issue is resolved in the follow-up message, update the conversation state to resolved.
            entity:ConversationUpdateResponse|error conversationUpdateResponse =
                    entity:updateConversation(userInfo.idToken, conversationId, {stateKey: entity:RESOLVED});
            if conversationUpdateResponse is error {
                string customError = "Failed to update conversation state to resolved.";
                log:printError(customError, conversationUpdateResponse);
                // Not returning error response since the main flow of creating conversation and getting chat response is successful.
            }
        }

        return <http:Ok>{
            body: chatResponse
        };
    }

    # Get conversation details by ID.
    #
    # + id - ID of the conversation
    # + return - Conversation details or error
    resource function get conversations/[entity:IdString id](http:RequestContext ctx)
        returns types:ConversationResponse|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:ConversationResponse|error conversationResponse = entity:getConversation(userInfo.idToken, id);
        if conversationResponse is error {
            if getStatusCode(conversationResponse) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            if getStatusCode(conversationResponse) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${userInfo.userId} is forbidden to access conversation with ID: ${id}!`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to access the requested conversation."
                    }
                };
            }

            string customError = "Failed to retrieve conversation details.";
            log:printError(customError, conversationResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return mapConversationResponse(conversationResponse);
    }

    # Get comments for a specific case.
    #
    # + id - ID of the case
    # + limit - Number of comments to retrieve
    # + offset - Offset for pagination
    # + return - Comments response or error
    resource function get cases/[entity:IdString id]/comments(http:RequestContext ctx, int? 'limit, int? offset)
        returns types:CommentsResponse|http:BadRequest|http:Unauthorized|http:Forbidden|http:NotFound|
        http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
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

        entity:CommentsResponse|error commentsResponse = entity:getComments(userInfo.idToken, entity:CASE, id,
                'limit, offset);
        if commentsResponse is error {
            if getStatusCode(commentsResponse) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(commentsResponse) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${userInfo.userId} is forbidden to access comments for case with ID: ${id}!`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to access the comments for the requested case. " +
                        "Please check your access permissions or contact support."
                    }
                };
            }
            if getStatusCode(commentsResponse) == http:STATUS_NOT_FOUND {
                return <http:NotFound>{
                    body: {
                        message: "The case for which you're trying to retrieve comments does not exist. " +
                        "Please check and try again."
                    }
                };
            }

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

    # Get messages for a specific conversation.
    #
    # + id - ID of the conversation
    # + limit - Number of messages to retrieve
    # + offset - Offset for pagination
    # + return - Comments response or error
    resource function get conversations/[entity:IdString id]/messages(http:RequestContext ctx, int? 'limit, int? offset)
        returns types:CommentsResponse|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
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

        entity:CommentsResponse|error commentsResponse = entity:getComments(userInfo.idToken, entity:CONVERSATION, id,
                'limit, offset);
        if commentsResponse is error {
            if getStatusCode(commentsResponse) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(commentsResponse) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${
                        userInfo.userId} is forbidden to access messages for conversation with ID: ${id}!`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to access the messages for the requested conversation."
                    }
                };
            }
            string customError = "Failed to retrieve messages.";
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
    resource function get cases/[entity:IdString id]/attachments(http:RequestContext ctx, int? 'limit, int? offset)
        returns types:AttachmentsResponse|http:BadRequest|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
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

        entity:AttachmentsResponse|error attachmentResponse =
            entity:getAttachments(userInfo.idToken, id, entity:CASE, 'limit, offset);
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

    # Create a new comment for a specific case.
    #
    # + id - ID of the case
    # + payload - Comment creation payload
    # + return - Created comment or error response
    resource function post cases/[entity:IdString id]/comments(http:RequestContext ctx,
            types:CommentCreatePayload payload)
        returns entity:CreatedComment|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:CommentCreateResponse|error createdCommentResponse = entity:createComment(userInfo.idToken,
                {
                    referenceId: id,
                    referenceType: entity:CASE,
                    content: payload.content,
                    'type: entity:COMMENTS,
                    createdBy: userInfo.email
                });
        if createdCommentResponse is error {
            if getStatusCode(createdCommentResponse) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            if getStatusCode(createdCommentResponse) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${userInfo.userId} is forbidden to comment on case with ID: ${id}!`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to comment on the requested case. " +
                        "Please check your access permissions or contact support."
                    }
                };
            }

            if getStatusCode(createdCommentResponse) == http:STATUS_BAD_REQUEST {
                string customError = "Invalid request parameters for creating comment for the case.";
                log:printWarn(customError, createdCommentResponse);
                return <http:BadRequest>{
                    body: {
                        message: customError
                    }
                };
            }

            string customError = "Failed to create a new comment.";
            log:printError(customError, createdCommentResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return createdCommentResponse.comment;
    }

    # Create a new attachment for a specific case.
    #
    # + id - ID of the case
    # + return - Created attachment or error response
    resource function post cases/[entity:IdString id]/attachments(http:RequestContext ctx,
            types:AttachmentCreatePayload payload)
        returns types:CreatedAttachment|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:AttachmentCreateResponse|error createdAttachmentResponse = entity:createAttachment(userInfo.idToken,
                {
                    referenceId: id,
                    referenceType: entity:CASE,
                    name: payload.name,
                    'type: payload.'type,
                    file: payload.content
                });
        if createdAttachmentResponse is error {
            if getStatusCode(createdAttachmentResponse) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            if getStatusCode(createdAttachmentResponse) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${userInfo.userId} is forbidden to add attachment to case with ID: ${id}!`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to add attachments to the requested case. " +
                        "Please check your access permissions or contact support."
                    }
                };
            }

            string customError = "Failed to create a new attachment.";
            log:printError(customError, createdAttachmentResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return {
            id: createdAttachmentResponse.attachment.id,
            size: createdAttachmentResponse.attachment.sizeBytes,
            createdOn: createdAttachmentResponse.attachment.createdOn,
            createdBy: createdAttachmentResponse.attachment.createdBy,
            downloadUrl: createdAttachmentResponse.attachment.downloadUrl
        };
    }

    # Update an attachment for a specific case.
    #
    # + caseId - ID of the case
    # + attachmentId - ID of the attachment
    # + payload - Attachment update payload
    # + return - Updated attachment or error response
    resource function patch cases/[entity:IdString caseId]/attachments/[entity:IdString attachmentId](
            http:RequestContext ctx, types:AttachmentUpdatePayload payload)
        returns entity:UpdatedAttachment|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:AttachmentUpdatePayload updatePayload = {
            referenceId: caseId,
            referenceType: entity:CASE,
            name: payload?.name
        };
        string? validateAttachmentUpdatePayload = entity:validateAttachmentUpdatePayload(updatePayload);
        if validateAttachmentUpdatePayload is string {
            return <http:BadRequest>{
                body: {
                    message: validateAttachmentUpdatePayload
                }
            };
        }

        entity:AttachmentUpdateResponse|error response = entity:updateAttachment(userInfo.idToken, attachmentId,
                updatePayload);
        if response is error {
            if getStatusCode(response) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${userInfo.userId} is forbidden to update attachment with ID: ${
                        attachmentId} for case with ID: ${caseId}!`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to update attachments for the requested case. " +
                        "Please check your access permissions or contact support."
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_BAD_REQUEST {
                string customError = "Invalid request parameters for updating the attachment.";
                log:printWarn(customError);
                return <http:BadRequest>{
                    body: {
                        message: customError
                    }
                };
            }

            string customError = "Failed to update the attachment.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return response.attachment;
    }

    # Get an attachment.
    #
    # + id - ID of the attachment
    # + return - Attachment response or error
    resource function get attachments/[entity:IdString id](http:RequestContext ctx)
        returns entity:AttachmentResponse|http:Unauthorized|http:Forbidden|http:NotFound|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:AttachmentResponse|error response = entity:getAttachment(userInfo.idToken, id);
        if response is error {
            if getStatusCode(response) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${userInfo.userId} is forbidden to get the attachment with ID: ${id}!`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to get the requested attachment. " +
                        "Please check your access permissions or contact support."
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_NOT_FOUND {
                log:printWarn(string `Attachment with ID: ${id} not found for user: ${userInfo.userId}`);
                return <http:NotFound>{
                    body: {
                        message: "The attachment you're trying to get does not exist. " +
                        "Please check and try again."
                    }
                };
            }

            string customError = "Failed to get the attachment.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return response;
    }

    # Delete an attachment.
    #
    # + id - ID of the attachment
    # + return - Success message or error response
    resource function delete attachments/[entity:IdString id](http:RequestContext ctx)
        returns http:Ok|http:Unauthorized|http:Forbidden|http:NotFound|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:AttachmentDeleteResponse|error response = entity:deleteAttachment(userInfo.idToken, id);
        if response is error {
            if getStatusCode(response) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${userInfo.userId} is forbidden to delete attachment with ID: ${id}!`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to delete the requested attachment. " +
                        "Please check your access permissions or contact support."
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_NOT_FOUND {
                log:printWarn(string `Attachment with ID: ${id} not found for user: ${userInfo.userId}`);
                return <http:NotFound>{
                    body: {
                        message: "The attachment you're trying to delete does not exist. " +
                        "Please check and try again."
                    }
                };
            }

            string customError = "Failed to delete the attachment.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return <http:Ok>{
            body: {
                message: "Attachment deleted successfully."
            }
        };
    }

    # Search products of a deployment.
    #
    # + payload - Deployed product search payload
    # + return - Deployed products response or error response
    resource function post deployments/[entity:IdString id]/products/search(http:RequestContext ctx,
            types:DeployedProductSearchPayload payload)
        returns http:Ok|http:BadRequest|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:DeployedProductsResponse|error productsResponse = entity:searchDeployedProducts(userInfo.idToken,
                {
                    filters: {
                        deploymentIds: [id],
                        productCategories: payload?.filters?.productCategories
                    },
                    pagination: payload.pagination
                });
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

        return <http:Ok>{body: mapDeployedProducts(productsResponse)};
    }

    # Add a product to a deployment by deployment ID.
    #
    # + id - ID of the deployment
    # + payload - Deployed product creation payload
    # + return - Created deployed product or error response
    resource function post deployments/[entity:IdString id]/products(http:RequestContext ctx,
            types:DeployedProductCreatePayload payload) returns
        entity:CreatedDeployedProduct|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:DeployedProductCreateResponse|error response = entity:createDeployedProduct(userInfo.idToken,
                {
                    deploymentId: id,
                    productId: payload.productId,
                    projectId: payload.projectId,
                    versionId: payload.versionId,
                    cores: payload?.cores,
                    tps: payload?.tps,
                    description: payload?.description
                });
        if response is error {
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${userInfo.userId} is forbidden to add product to deployment with ID: ${
                        id}!`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to add products to the deployment. " +
                        "Please check your access permissions or contact support."
                    }
                };
            }

            if getStatusCode(response) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            if getStatusCode(response) == http:STATUS_BAD_REQUEST {
                string customError = "Invalid request parameters for adding product to the deployment.";
                log:printWarn(customError, response);
                return <http:BadRequest>{
                    body: {
                        message: customError
                    }
                };
            }
            string customError = "Failed to add product to the deployment.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return response.deployedProduct;
    }

    # Update a product in a deployment by deployment ID and product ID.
    #
    # + deploymentId - ID of the deployment
    # + productId - ID of the product to be updated
    # + payload - Deployed product update payload
    # + return - Updated deployed product or error response
    resource function patch deployments/[entity:IdString deploymentId]/products/[entity:IdString productId](
            http:RequestContext ctx, entity:DeployedProductUpdatePayload payload) returns
        entity:UpdatedDeployedProduct|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        string? validateDeployedProductUpdatePayload = entity:validateDeployedProductUpdatePayload(payload);
        if validateDeployedProductUpdatePayload is string {
            return <http:BadRequest>{
                body: {
                    message: validateDeployedProductUpdatePayload
                }
            };
        }

        entity:DeployedProductUpdateResponse|error response =
            entity:updateDeployedProduct(userInfo.idToken, productId, payload);
        if response is error {
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${userInfo.userId} is forbidden to update product with ID: ${
                        productId} in deployment with ID: ${deploymentId}!`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to update products in the deployment. " +
                        "Please check your access permissions or contact support."
                    }
                };
            }

            if getStatusCode(response) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            if getStatusCode(response) == http:STATUS_BAD_REQUEST {
                string customError = "Invalid request parameters for updating product in the deployment.";
                log:printWarn(customError, response);
                return <http:BadRequest>{
                    body: {
                        message: customError
                    }
                };
            }

            string customError = "Failed to update product in the deployment.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return response.deployedProduct;
    }

    # Search catalogs for a specific deployed product with filters and pagination.
    #
    # + id - ID of the deployed product
    # + payload - Catalog search request body
    # + return - Paginated catalogs or error
    resource function post deployments/products/[entity:IdString id]/catalogs/search(http:RequestContext ctx,
            types:CatalogSearchPayload payload)
        returns http:Ok|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:CatalogSearchResponse|error searchResponse = entity:searchCatalogs(userInfo.idToken,
                {
                    deployedProductId: id,
                    pagination: payload.pagination
                });
        if searchResponse is error {
            if getStatusCode(searchResponse) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${
                        userInfo.userId} is forbidden to search catalogs for deployed product with ID: ${id}!`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to search catalogs for the deployed product. " +
                        "Please check your access permissions or contact support."
                    }
                };
            }
            if getStatusCode(searchResponse) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            string customError = "Failed to search catalogs for the deployed product.";
            log:printError(customError, searchResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return <http:Ok>{body: mapCatalogSearchResponse(searchResponse)};
    }

    # Search instances for a specific deployed product with filters and pagination.
    #
    # + id - ID of the deployed product
    # + payload - Instance search request body
    # + return - Paginated instances or error
    resource function post deployments/products/[entity:IdString id]/instances/search(http:RequestContext ctx,
            types:InstanceSearchPayload payload)
        returns http:Ok|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:InstancesResponse|error instances = entity:searchInstances(userInfo.idToken,
                {
                    filters: {
                        deployedProductIds: [id],
                        startDate: payload.filters?.startDate,
                        endDate: payload.filters?.endDate
                    },
                    pagination: payload.pagination
                });
        if instances is error {
            if getStatusCode(instances) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${
                        userInfo.userId} is forbidden to search instances for deployed product with ID: ${id}!`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to search instances for the deployed product. " +
                        "Please check your access permissions or contact support."
                    }
                };
            }
            if getStatusCode(instances) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(instances) == http:STATUS_BAD_REQUEST {
                string customError = "Invalid request parameters for searching instances for the deployed product.";
                log:printWarn(customError, instances);
                return <http:BadRequest>{
                    body: {
                        message: customError
                    }
                };
            }

            string customError = "Failed to search instances for the deployed product.";
            log:printError(customError, instances);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return <http:Ok>{body: mapInstancesResponse(instances)};
    }

    # Get catalog item variables by catalog ID and item ID.
    #
    # + catalogId - ID of the catalog
    # + itemId - ID of the catalog item
    # + return - Catalog item details or error
    resource function get catalogs/[entity:IdString catalogId]/items/[entity:IdString itemId](http:RequestContext ctx)
        returns http:Ok|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:CatalogItemVariablesResponse|error itemResponse =
            entity:getCatalogItemVariable(userInfo.idToken, catalogId, itemId);
        if itemResponse is error {
            if getStatusCode(itemResponse) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${userInfo.userId} is forbidden to access catalog item with ID: ${
                        itemId} in catalog with ID: ${catalogId}!`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to access the catalog item. " +
                        "Please check your access permissions or contact support."
                    }
                };
            }
            if getStatusCode(itemResponse) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            string customError = "Failed to retrieve catalog item details.";
            log:printError(customError, itemResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return <http:Ok>{body: itemResponse};
    }

    # Get recommended update levels.
    #
    # + return - List of recommended update levels or an error
    resource function get updates/recommended\-update\-levels(http:RequestContext ctx)
        returns types:RecommendedUpdateLevel[]|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        types:RecommendedUpdateLevel[]|error recommendedUpdateLevels =
            updates:processRecommendedUpdateLevels(userInfo.email);
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
    resource function post updates/levels/search(http:RequestContext ctx, types:UpdateDescriptionPayload payload)
        returns http:Ok|http:BadRequest|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        map<types:UpdateLevelGroup>|error updateResponse =
            updates:processSearchUpdatesBetweenUpdateLevels(userInfo.email, payload);
        if updateResponse is error {
            string customError = "Failed to search updates based on the provided filters.";
            log:printError(customError, updateResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return <http:Ok>{
            body: updateResponse
        };
    }

    # Get product update levels.
    #
    # + return - List of product update levels or an error
    resource function get updates/product\-update\-levels(http:RequestContext ctx)
        returns types:ProductUpdateLevel[]|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        types:ProductUpdateLevel[]|error productUpdateLevels = updates:processProductUpdateLevels();
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

    # Get products.
    #
    # + return - List of products or an error
    resource function get products(http:RequestContext ctx, entity:ProductClass? 'class, int? offset, int? 'limit)
        returns entity:ProductsResponse|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:ProductsResponse|error response = entity:getProducts(userInfo.idToken,
                {
                    filters: {'class},
                    pagination: {
                        offset: offset ?: DEFAULT_OFFSET,
                        'limit: 'limit ?: DEFAULT_LIMIT
                    }
                });
        if response is error {
            string customError = "Failed to retrieve products.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return response;
    }

    # Search product vulnerabilities based on provided filters.
    #
    # + payload - Product vulnerability search payload containing filters
    # + return - List of product vulnerabilities or an error
    resource function post products/vulnerabilities/search(http:RequestContext ctx,
            entity:ProductVulnerabilitySearchPayload payload) returns http:Ok|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:ProductVulnerabilitySearchResponse|error response =
            entity:searchProductVulnerabilities(userInfo.idToken, payload);
        if response is error {
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `Access to product vulnerabilities information is forbidden for user: ${
                        userInfo.userId}`);
                return <http:Forbidden>{
                    body: {
                        message: "Access to product vulnerabilities information is forbidden for the user!"
                    }
                };
            }

            string customError = "Failed to search product vulnerabilities.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return <http:Ok>{
            body: mapProductVulnerabilitySearchResponse(response)
        };
    }

    # Get product vulnerability by ID.
    #
    # + id - ID of the product vulnerability
    # + return - Product vulnerability details or error
    resource function get products/vulnerabilities/[entity:IdString id](http:RequestContext ctx)
            returns types:ProductVulnerabilityResponse|http:Forbidden|http:NotFound|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:ProductVulnerabilityResponse|error response = entity:getProductVulnerability(userInfo.idToken, id);
        if response is error {
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `Access to product vulnerability information is forbidden for user: ${
                        userInfo.userId}`);
                return <http:Forbidden>{
                    body: {
                        message: "Access to product vulnerability information is forbidden for the user!"
                    }
                };
            }

            if getStatusCode(response) == http:STATUS_NOT_FOUND {
                log:printWarn(string `Requested product vulnerability is not found for the user: ${userInfo.userId}`);
                return <http:NotFound>{
                    body: {
                        message: "Requested product vulnerability is not found!"
                    }
                };
            }

            string customError = "Failed to retrieve product vulnerability details.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return mapProductVulnerabilityResponse(response);
    }

    # Get product vulnerability metadata.
    #
    # + return - Product vulnerability metadata or error
    resource function get products/vulnerabilities/meta(http:RequestContext ctx)
        returns types:ProductVulnerabilityMetaResponse|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:VulnerabilityMetaResponse|error response = entity:getProductVulnerabilityMetaData(userInfo.idToken);
        if response is error {
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `Access to product vulnerability information is forbidden for user: ${
                        userInfo.userId}`);
                return <http:Forbidden>{
                    body: {
                        message: "Access to product vulnerability information is forbidden for the user!"
                    }
                };
            }

            string customError = "Failed to retrieve product vulnerability metadata.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return mapProductVulnerabilityMetadataResponse(response);
    }

    # Get contacts of a project by project ID.
    #
    # + id - ID of the project
    # + return - List of project contacts or error
    resource function get projects/[entity:IdString id]/contacts(http:RequestContext ctx)
        returns user_management:Contact[]|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
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

        user_management:Contact[]|error response = user_management:getProjectContacts(projectResponse.sfId);
        if response is error {
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `Access to project contacts are forbidden for user: ${
                        userInfo.userId}`);
                return <http:Forbidden>{
                    body: {
                        message: "Access to project contacts is forbidden for the user!"
                    }
                };
            }

            string customError = "Failed to retrieve project contacts.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return response;
    }

    # Add a contact to a project by project ID.
    #
    # + id - ID of the project
    # + payload - Contact information to be added
    # + return - Membership information or error response
    resource function post projects/[entity:IdString id]/contacts(http:RequestContext ctx,
            types:ContactOnboardPayload payload)
        returns user_management:Membership|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
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

        user_management:Membership|error response = user_management:createProjectContact(projectResponse.sfId,
                {
                    contactEmail: payload.contactEmail,
                    adminEmail: userInfo.email,
                    contactFirstName: payload.contactFirstName,
                    contactLastName: payload.contactLastName,
                    isCsIntegrationUser: payload.isCsIntegrationUser,
                    isSecurityContact: payload.isSecurityContact

                });
        if response is error {
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `Access to add project contact is forbidden for user: ${
                        userInfo.userId}`);
                return <http:Forbidden>{
                    body: {
                        message: response.message()
                    }
                };
            }

            string customError = "Failed to add project contact.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: response.message()
                }
            };
        }
        return response;
    }

    # Remove a contact from a project by project ID and contact email.
    #
    # + id - ID of the project
    # + email - Email of the contact to be removed
    # + return - Membership information or error response
    resource function delete projects/[entity:IdString id]/contacts/[string email](http:RequestContext ctx)
        returns http:Ok|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
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

        user_management:Membership|error response = user_management:removeProjectContact(projectResponse.sfId, email, userInfo.email);
        if response is error {
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `Access to remove project contact is forbidden for user: ${
                        userInfo.userId}`);
                return <http:Forbidden>{
                    body: {
                        message: response.message()
                    }
                };
            }

            string customError = "Failed to remove project contact.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: response.message()
                }
            };
        }
        return <http:Ok>{
            body: {
                message: "Project contact removed successfully!"
            }
        };
    }

    # Update a contact's role in a project by project ID and contact email.
    #
    # + id - ID of the project
    # + email - Email of the contact whose role is to be updated
    # + payload - Updated role information
    # + return - Membership information or error response
    resource function patch projects/[entity:IdString id]/contacts/[string email](http:RequestContext ctx,
            types:MembershipSecurityPayload payload)
        returns user_management:Membership|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
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

        user_management:Membership|error response = user_management:updateMembershipFlag(projectResponse.sfId, email,
                {
                    adminEmail: userInfo.email,
                    isSecurityContact: payload.isSecurityContact
                });
        if response is error {
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `Access to update project contact is forbidden for user: ${
                        userInfo.userId}`);
                return <http:Forbidden>{
                    body: {
                        message: response.message()
                    }
                };
            }

            string customError = "Failed to update project contact.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: response.message()
                }
            };
        }
        return response;
    }

    # Validate if a contact can be added to a project by project ID.
    #
    # + id - ID of the project
    # + payload - Contact information to be validated
    # + return - Contact information if valid or error response
    resource function post projects/[entity:IdString id]/contacts/validate(http:RequestContext ctx,
            types:ValidationPayload payload)
        returns http:Ok|http:Conflict|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
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

        user_management:Contact|error? validationResponse = user_management:validateProjectContact(
                {
                    contactEmail: payload.contactEmail,
                    adminEmail: userInfo.email,
                    projectId: projectResponse.sfId
                });
        if validationResponse is user_management:CONFLICT_ERROR {
            log:printWarn(string `Contact with email: ${payload.contactEmail} already exists in project with ID: ${
                    id}`);
            return <http:Conflict>{
                body: {
                    message: "Contact with the provided email already exists in the project!"
                }
            };
        } else if validationResponse is error {
            if getStatusCode(validationResponse) == http:STATUS_FORBIDDEN {
                log:printWarn(string `Access to validate project contact is forbidden for user: ${
                        userInfo.userId}`);
                return <http:Forbidden>{
                    body: {
                        message: validationResponse.message()
                    }
                };
            }

            string customError = "Failed to validate project contact.";
            log:printError(customError, validationResponse);
            return <http:InternalServerError>{
                body: {
                    message: validationResponse.message()
                }
            };
        } else if validationResponse is user_management:Contact {
            return <http:Ok>{
                body: {
                    isContactValid: true,
                    message: "Contact is valid but already exists in the project!",
                    contactDetails: validationResponse
                }
            };
        }
        return <http:Ok>{
            body: {
                isContactValid: true,
                message: "Project contact is valid and can be added to the project!"
            }
        };
    }

    # Search call requests for a specific case with filters and pagination.
    #
    # + id - ID of the case
    # + payload - Call request search payload containing filters and pagination info
    # + return - List of call requests matching the criteria or an error
    resource function post cases/[entity:IdString id]/call\-requests/search(http:RequestContext ctx,
            types:CallRequestSearchPayload payload)
        returns http:Ok|http:BadRequest|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:CallRequestsResponse|error response = entity:searchCallRequests(userInfo.idToken,
                {
                    caseId: id,
                    filters: payload.filters,
                    pagination: payload.pagination
                });
        if response is error {
            if getStatusCode(response) == http:STATUS_BAD_REQUEST {
                return <http:BadRequest>{
                    body: {
                        message: "Invalid request parameters for searching call requests."
                    }
                };
            }

            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `Access to call request information is forbidden for user: ${
                        userInfo.userId}`);
                return <http:Forbidden>{
                    body: {
                        message: "Access to call request information is forbidden for the user!"
                    }
                };
            }

            string customError = "Failed to search call requests.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return <http:Ok>{
            body: mapSearchCallRequestResponse(response)
        };
    }

    # Create a call request for a specific case.
    #
    # + id - ID of the case
    # + payload - Call request creation payload
    # + return - Created call request details or an error
    resource function post cases/[entity:IdString id]/call\-requests(http:RequestContext ctx,
            types:CallRequestCreatePayload payload)
        returns entity:CreatedCallRequest|http:BadRequest|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        string|error? validateUtcTimesError = entity:validateUtcTimes(payload.utcTimes);
        if validateUtcTimesError is string {
            log:printWarn(validateUtcTimesError);
            return <http:BadRequest>{
                body: {
                    message: validateUtcTimesError
                }
            };
        }

        if validateUtcTimesError is error {
            string customError = "Failed to validate UTC times for call request creation.";
            log:printError(customError, validateUtcTimesError);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        entity:CallRequestCreateResponse|error response = entity:createCallRequest(userInfo.idToken,
                {
                    caseId: id,
                    reason: payload.reason,
                    utcTimes: payload.utcTimes,
                    durationInMinutes: payload.durationInMinutes
                });
        if response is error {
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `Access to create call request is forbidden for the user: ${userInfo.userId}`);
                return <http:Forbidden>{
                    body: {
                        message: "Access to create call request is forbidden for the user!"
                    }
                };
            }

            if getStatusCode(response) == http:STATUS_BAD_REQUEST {
                return <http:BadRequest>{
                    body: {
                        message: "Invalid request parameters for creating a call request."
                    }
                };
            }

            string customError = "Failed to create call request.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return response.callRequest;
    }

    # Update a call request for a specific case.
    #
    # + caseId - ID of the case
    # + callRequestId - ID of the call request
    # + payload - Call request update payload
    # + return - Updated call request details or an error
    resource function patch cases/[entity:IdString caseId]/call\-requests/[entity:IdString callRequestId](
            http:RequestContext ctx, entity:CallRequestUpdatePayload payload)
        returns entity:UpdatedCallRequest|http:BadRequest|http:Forbidden|http:NotFound|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        string? validationError = entity:validateCallRequestUpdatePayload(payload);
        if validationError is string {
            log:printWarn(validationError);
            return <http:BadRequest>{
                body: {
                    message: validationError
                }
            };
        }

        string|error? validateUtcTimesError = entity:validateUtcTimes(payload.utcTimes);
        if validateUtcTimesError is string {
            log:printWarn(validateUtcTimesError);
            return <http:BadRequest>{
                body: {
                    message: validateUtcTimesError
                }
            };
        }

        if validateUtcTimesError is error {
            string customError = "Failed to validate UTC times for call request update.";
            log:printError(customError, validateUtcTimesError);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        string? validateCallRequestUpdatePayload = entity:validateCallRequestUpdatePayload(payload);
        if validateCallRequestUpdatePayload is string {
            log:printWarn(validateCallRequestUpdatePayload);
            return <http:BadRequest>{
                body: {
                    message: validateCallRequestUpdatePayload
                }
            };
        }

        entity:CallRequestUpdateResponse|error response = entity:updateCallRequest(userInfo.idToken, callRequestId,
                payload);
        if response is error {
            if getStatusCode(response) == http:STATUS_BAD_REQUEST {
                return <http:BadRequest>{
                    body: {
                        message: "Invalid request parameters for updating call request."
                    }
                };
            }

            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `Access to update call request is forbidden for user: ${userInfo.userId}`);
                return <http:Forbidden>{
                    body: {
                        message: "Access to update call request is forbidden for the user!"
                    }
                };
            }

            if getStatusCode(response) == http:STATUS_NOT_FOUND {
                return <http:NotFound>{
                    body: {
                        message: "The call request to be updated is not found!"
                    }
                };
            }

            string customError = "Failed to update call request.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return response.callRequest;
    }

    # Search product versions based on provided filters.
    #
    # + id - ID of the product
    # + payload - Product version search payload containing filters and pagination info
    # + return - List of product versions matching the criteria or an error
    resource function post products/[entity:IdString id]/versions/search(http:RequestContext ctx,
            entity:ProductVersionSearchPayload payload)
        returns http:Ok|http:BadRequest|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:ProductVersionsResponse|error response = entity:searchProductVersions(userInfo.idToken, id, payload);
        if response is error {
            if getStatusCode(response) == http:STATUS_BAD_REQUEST {
                return <http:BadRequest>{
                    body: {
                        message: "Invalid request parameters for searching product versions."
                    }
                };
            }

            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `Access to product version information is forbidden for user: ${userInfo.userId}`);
                return <http:Forbidden>{
                    body: {
                        message: "Access to product version information is forbidden for the user!"
                    }
                };
            }

            string customError = "Failed to search product versions.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return <http:Ok>{
            body: mapProductVersionsResponse(response)
        };
    }

    # Search time cards based on provided filters.
    #
    # + id - ID of the project
    # + payload - Time card search payload containing filters and pagination info
    # + return - List of time cards matching the criteria or an error
    resource function post projects/[entity:IdString id]/time\-cards/search(http:RequestContext ctx,
            types:TimeCardSearchPayload payload)
        returns http:Ok|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:TimeCardsResponse|error response = entity:searchTimeCards(userInfo.idToken,
                {
                    filters: {
                        projectIds: [id],
                        startDate: payload.filters?.startDate,
                        endDate: payload.filters?.endDate,
                        states: payload.filters?.states
                    },
                    pagination: payload.pagination
                });
        if response is error {
            if getStatusCode(response) == http:STATUS_BAD_REQUEST {
                return <http:BadRequest>{
                    body: {
                        message: "Invalid request parameters for searching time cards."
                    }
                };
            }

            if getStatusCode(response) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access time card information!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `Access to time card information is forbidden for user: ${userInfo.userId}`);
                return <http:Forbidden>{
                    body: {
                        message: "Access to time card information is forbidden for the user!"
                    }
                };
            }

            string customError = "Failed to search time cards.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return <http:Ok>{
            body: mapTimeCardSearchResponse(response)
        };
    }

    # Search time cards grouped by cases based on provided filters.
    #
    # + id - ID of the project
    # + payload - Time card search payload containing filters and pagination info
    # + return - List of time cards grouped by cases matching the criteria or an error
    resource function post projects/[entity:IdString id]/cases/time\-cards/search(http:RequestContext ctx,
            types:TimeCardSearchPayload payload)
        returns http:Ok|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {
        
        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:CaseTimeCardsSearchResponse|error response = entity:searchTimeCardsGroupedByCases(userInfo.idToken,
                {
                    filters: {
                        projectIds: [id],
                        startDate: payload.filters?.startDate,
                        endDate: payload.filters?.endDate,
                        states: payload.filters?.states
                    },
                    pagination: payload.pagination
                });
        if response is error {
            if getStatusCode(response) == http:STATUS_BAD_REQUEST {
                return <http:BadRequest>{
                    body: {
                        message: "Invalid request parameters for searching time cards grouped by cases."
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access time card information!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `Access to time card information is forbidden for user: ${userInfo.userId}`);
                return <http:Forbidden>{
                    body: {
                        message: "Access to time card information is forbidden for the user!"
                    }
                };
            }

            string customError = "Failed to search time cards grouped by cases.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return <http:Ok>{
            body: mapTimeCardSearchResponseGroupedByCases(response)
        };
    }

    # Get time card statistics for a project based on provided date range.
    #
    # + id - ID of the project
    # + startDate - Start date for the statistics (optional)
    # + endDate - End date for the statistics (optional)
    # + return - Time card statistics for the project or an error
    resource function get projects/[entity:IdString id]/stats/time\-cards(http:RequestContext ctx,
            entity:Date? startDate, entity:Date? endDate)
        returns entity:ProjectTimeCardStatsResponse|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:ProjectTimeCardStatsResponse|error response =
            entity:getProjectTimeCardStats(userInfo.idToken, id, startDate, endDate);
        if response is error {
            if getStatusCode(response) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access project time card stats!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `Access to project time card stats is forbidden for user: ${userInfo.userId}`);
                return <http:Forbidden>{
                    body: {
                        message: "Access to project time card stats is forbidden for the user!"
                    }
                };
            }

            string customError = "Failed to retrieve project time card stats.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return response;
    }

    # Search change requests for a project based on provided filters and pagination.
    #
    # + id - ID of the project
    # + payload - Change request search payload containing filters and pagination info
    # + return - List of change requests matching the criteria or an error
    resource function post projects/[entity:IdString id]/change\-requests/search(http:RequestContext ctx,
            types:ChangeRequestSearchPayload payload)
        returns http:Ok|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:ChangeRequestSearchResponse|error response = entity:searchChangeRequests(userInfo.idToken,
                {
                    filters: {
                        projectIds: [id],
                        searchQuery: payload.filters?.searchQuery,
                        stateKeys: payload.filters?.stateKeys,
                        impactKey: payload.filters?.impactKey
                    },
                    pagination: payload.pagination
                });
        if response is error {
            if getStatusCode(response) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${
                        userInfo.userId} is not authorized to access change request information!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }

            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `Access to change request information is forbidden for user: ${userInfo.userId}`);
                return <http:Forbidden>{
                    body: {
                        message: "Access to change request information is forbidden for the user!"
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_BAD_REQUEST {
                return <http:BadRequest>{
                    body: {
                        message: "Invalid request parameters for searching change requests."
                    }
                };
            }

            string customError = "Failed to search change requests.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return <http:Ok>{
            body: mapChangeRequestSearchResponse(response)
        };
    }

    # Get change request details by change request ID.
    #
    # + id - ID of the change request
    # + return - Change request details or an error
    resource function get change\-requests/[entity:IdString id](http:RequestContext ctx)
        returns types:ChangeRequestResponse|http:Unauthorized|http:Forbidden|http:NotFound|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:ChangeRequestResponse|error response = entity:getChangeRequestDetails(userInfo.idToken, id);
        if response is error {
            if getStatusCode(response) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${
                        userInfo.userId} is not authorized to access change request information!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `Access to change request information is forbidden for user: ${userInfo.userId}`);
                return <http:Forbidden>{
                    body: {
                        message: "Access to change request information is forbidden for the user!"
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_NOT_FOUND {
                return <http:NotFound>{
                    body: {
                        message: "The requested change request is not found!"
                    }
                };
            }

            string customError = "Failed to retrieve change request details.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return mapChangeRequestResponse(response);
    }

    # Update a change request by change request ID.
    #
    # + id - ID of the change request
    # + payload - Change request update payload containing fields to be updated
    # + return - Updated change request details or an error
    resource function patch change\-requests/[entity:IdString id](http:RequestContext ctx,
            entity:ChangeRequestUpdatePayload payload) returns entity:UpdatedChangeRequest|http:BadRequest|
        http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        string? validateChangeRequestPayload = entity:validateChangeRequestUpdatePayload(payload);
        if validateChangeRequestPayload is string {
            log:printWarn(validateChangeRequestPayload);
            return <http:BadRequest>{
                body: {
                    message: validateChangeRequestPayload
                }
            };
        }

        entity:ChangeRequestUpdateResponse|error response = entity:updateChangeRequest(userInfo.idToken, id, payload);
        if response is error {
            if getStatusCode(response) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${
                        userInfo.userId} is not authorized to update change request information!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `Access to update change request information is forbidden for user: ${
                        userInfo.userId}`);
                return <http:Forbidden>{
                    body: {
                        message: "Access to update change request information is forbidden for the user!"
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_BAD_REQUEST {
                return <http:BadRequest>{
                    body: {
                        message: "Invalid request parameters for updating change request."
                    }
                };
            }

            string customError = "Failed to update change request.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return response.changeRequest;
    }

    # Get change request statistics for a project.
    #
    # + id - ID of the project
    # + return - Change request statistics for the project or an error
    resource function get projects/[entity:IdString id]/stats/change\-requests(http:RequestContext ctx)
        returns types:ProjectChangeRequestStatsResponse|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:ProjectChangeRequestStatsResponse|error response = entity:getProjectChangeRequestStats(userInfo.idToken,
                id);
        if response is error {
            if getStatusCode(response) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${
                        userInfo.userId} is not authorized to access change request information!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `Access to change request information is forbidden for user: ${userInfo.userId}`);
                return <http:Forbidden>{
                    body: {
                        message: "Access to change request information is forbidden for the user!"
                    }
                };
            }

            string customError = "Failed to retrieve change request stats.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return mapProjectChangeRequestStatsResponse(response);
    }

    # Get comments for a specific change request.
    #
    # + id - ID of the change request
    # + limit - Number of comments to retrieve
    # + offset - Offset for pagination
    # + return - Comments response or error
    resource function get change\-requests/[entity:IdString id]/comments(http:RequestContext ctx, int? 'limit,
            int? offset) returns types:CommentsResponse|http:BadRequest|http:Unauthorized|
        http:Forbidden|http:NotFound|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
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

        entity:CommentsResponse|error commentsResponse = entity:getComments(userInfo.idToken, entity:CHANGE_REQUEST, id,
                'limit, offset);
        if commentsResponse is error {
            if getStatusCode(commentsResponse) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User : ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(commentsResponse) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${
                        userInfo.userId} is forbidden to access comments for change request with ID: ${id}!`);
                return <http:Forbidden>{
                    body: {
                        message: "You're not authorized to access the comments for the requested change request."
                    }
                };
            }
            if getStatusCode(commentsResponse) == http:STATUS_NOT_FOUND {
                return <http:NotFound>{
                    body: {
                        message: "The requested change request or its comments are not found!"
                    }
                };
            }

            string customError = "Failed to retrieve comments for change request.";
            log:printError(customError, commentsResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return mapCommentsResponse(commentsResponse);
    }

    # Create a registry token.
    #
    # + id - ID of the project for which the registry token is to be created
    # + payload - Registry token creation payload containing project ID and token details
    # + return - Created registry token details or an error response
    resource function post projects/[entity:IdString id]/registry\-tokens(http:RequestContext ctx,
            types:RegistryTokenCreatePayload payload) returns registry:TokenCreationResponse|http:BadRequest|
                http:Unauthorized|http:Forbidden|http:NotFound|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
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

        boolean isAdmin = authorization:checkRoles([authorization:authorizedRoles.adminRole], userDetails.roles);

        // Only allow admins to create service tokens
        if !isAdmin && payload.tokenType == registry:SERVICE_TOKEN {
            log:printWarn(string `User: ${
                    userInfo.userId} attempted to create a service token without admin privileges.`);
            return <http:Forbidden>{
                body: {
                    message: "Only admins can create service tokens."
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
            if getStatusCode(projectResponse) == http:STATUS_NOT_FOUND {
                log:printWarn(string `Project with ID: ${id} not found for user: ${userInfo.userId}`);
                return <http:NotFound>{
                    body: {
                        message: "The requested project does not exist or you don't have access to it."
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

        string? accountName = projectResponse.account.name;
        if accountName is () {
            log:printError("Failed to retrieve account name for the project. Account name is missing.");
            return <http:InternalServerError>{
                body: {
                    message: "Failed to retrieve account name for the project."
                }
            };
        }

        // Default 'createdFor' to the requesting user's email for user tokens before any ownership checks.
        string? createdFor = payload.tokenType == registry:USER_TOKEN ? userInfo.email : payload.createdFor;

        // Enforce 'createdFor' email for service tokens.
        if payload.tokenType == registry:SERVICE_TOKEN && createdFor is () {
            log:printWarn(string `User: ${
                    userInfo.userId} attempted to create a service token without specifying 'createdFor' email.`);
            return <http:BadRequest>{
                body: {
                    message: "Service tokens are created on behalf of an integration user. Please specify the" +
                        " 'createdFor' email of the integration user this token is intended for."
                }
            };
        }

        // Enforce non-admins only to create tokens for themselves.
        if !isAdmin && createdFor != userInfo.email {
            log:printWarn(string `User: ${
                    userInfo.userId} attempted to create a token for another user without admin privileges.`);
            return <http:Forbidden>{
                body: {
                    message: "Only admins can create tokens for other users."
                }
            };
        }

        registry:TokenCreationResponse|error response = registry:createToken(
                {
                    snProjectId: id,
                    accountName,
                    projectKey: projectResponse.key,
                    robotName: payload.robotName,
                    snAccountId: projectResponse.account.id,
                    tokenType: payload.tokenType,
                    createdBy: userInfo.email,
                    createdFor: createdFor.toString()
                }
        );
        if response is error {
            if getStatusCode(response) == http:STATUS_BAD_REQUEST {
                return <http:BadRequest>{
                    body: {
                        message: "Invalid request parameters for creating registry token."
                    }
                };
            }

            string customError = "Failed to create registry token.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return response;
    }

    # Search registry tokens for a project based on provided filters.
    #
    # + id - ID of the project for which registry tokens are to be searched
    # + return - List of registry tokens matching the criteria or an error response
    resource function post projects/[entity:IdString id]/registry\-tokens/search(http:RequestContext ctx)
        returns http:Ok|http:BadRequest|http:Forbidden|http:NotFound|http:Unauthorized|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
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
            if getStatusCode(projectResponse) == http:STATUS_NOT_FOUND {
                log:printWarn(string `Project with ID: ${id} not found for user: ${userInfo.userId}`);
                return <http:NotFound>{
                    body: {
                        message: "The requested project does not exist or you don't have access to it."
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

        boolean isAdmin = authorization:checkRoles([authorization:authorizedRoles.adminRole], userDetails.roles);

        registry:Token[]|error response = registry:searchTokens(
                {
                    snProjectId: id,
                    snAccountId: projectResponse.account.id,
                    isAdmin,
                    userEmail: isAdmin ? () : userInfo.email
                }
        );
        if response is error {
            if getStatusCode(response) == http:STATUS_BAD_REQUEST {
                return <http:BadRequest>{
                    body: {
                        message: "Invalid request parameters for searching registry tokens."
                    }
                };
            }

            string customError = "Failed to search registry tokens.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return <http:Ok>{
            body: response
        };
    }

    # Delete a registry token by token ID.
    #
    # + id - ID of the registry token to be deleted
    # + return - Success message or an error response
    resource function delete registry\-tokens/[string id](http:RequestContext ctx)
        returns http:Ok|http:NotFound|http:Unauthorized|http:Forbidden|http:InternalServerError {

        string deletionCustomErr = "Failed to delete token";

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
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

            log:printError("Failed to retrieve user data.", userDetails);
            return <http:InternalServerError>{
                body: {
                    message: deletionCustomErr
                }
            };
        }

        registry:Token|error token = registry:getTokenById(id);
        if token is error {
            if getStatusCode(token) == http:STATUS_NOT_FOUND {
                log:printWarn(string `Registry token with ID: ${id} not found for user: ${userInfo.userId}`);
                return <http:NotFound>{
                    body: {
                        message: "The registry token is not found!"
                    }
                };
            }
            log:printError("Failed to retrieve registry token details for authorization.", token);
            return <http:InternalServerError>{
                body: {
                    message: deletionCustomErr
                }
            };
        }

        registry:TokenDescriptionInfo|error tokenInformation
            = registry:deriveTokenInfoFromDescription(token.description);
        if tokenInformation is error {
            log:printError("Failed to derive token information.", tokenInformation);
            return <http:InternalServerError>{
                body: {
                    message: deletionCustomErr
                }
            };
        }

        entity:ProjectResponse|error projectResponse
            = entity:getProject(userInfo.idToken, tokenInformation.snProjectId);
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
                logForbiddenProjectAccess(tokenInformation.snProjectId, userInfo.userId);
                return <http:Forbidden>{
                    body: {
                        message: ERR_MSG_PROJECT_ACCESS_FORBIDDEN
                    }
                };
            }
            if getStatusCode(projectResponse) == http:STATUS_NOT_FOUND {
                log:printWarn(string `Project with ID: ${tokenInformation.snProjectId} not found for user: ${
                        userInfo.userId}`);
                return <http:NotFound>{
                    body: {
                        message: "The requested token does not exist or you don't have access to it."
                    }
                };
            }

            log:printError("Failed to retrieve project details.", projectResponse);
            return <http:InternalServerError>{
                body: {
                    message: deletionCustomErr
                }
            };
        }

        boolean isAdmin = authorization:checkRoles([authorization:authorizedRoles.adminRole], userDetails.roles);

        // Enforce that only admins can delete service tokens, and users can only delete their own tokens.
        if !isAdmin &&
            (tokenInformation.tokenType == registry:SERVICE_TOKEN || tokenInformation.createdFor != userInfo.email) {

            log:printWarn(string `User: ${
                    userInfo.userId} attempted to delete a service token without proper privileges`);
            return <http:Forbidden>{
                body: {
                    message: "You don't have the necessary permissions to delete this registry token."
                }
            };
        }

        error? response = registry:deleteToken(id);
        if response is error {
            if getStatusCode(response) == http:STATUS_NOT_FOUND {
                return <http:NotFound>{
                    body: {
                        message: "The registry token is not found!"
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to delete registry token with ID: ${
                        id}.`);
                return <http:Forbidden>{
                    body: {
                        message: "You are not authorized to delete this registry token."
                    }
                };
            }

            log:printError(deletionCustomErr, response);
            return <http:InternalServerError>{
                body: {
                    message: deletionCustomErr
                }
            };
        }
        return <http:Ok>{
            body: {
                message: "Registry token deleted successfully!"
            }
        };
    }

    # Regenerate the secret of a registry token by token ID.
    #
    # + id - ID of the registry token to be regenerated
    # + return - Regenerated registry token details or an error response
    resource function post registry\-tokens/[string id]/regenerate(http:RequestContext ctx)
        returns registry:TokenCreationResponse|http:NotFound|http:Unauthorized|http:Forbidden
            |http:InternalServerError {

        string reGenerateCustomErr = "Failed to re-generate token";

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
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
                    message: reGenerateCustomErr
                }
            };
        }

        registry:Token|error token = registry:getTokenById(id);
        if token is error {
            if getStatusCode(token) == http:STATUS_NOT_FOUND {
                log:printWarn(string `Registry token with ID: ${id} not found for user: ${userInfo.userId}`);
                return <http:NotFound>{
                    body: {
                        message: "The registry token is not found!"
                    }
                };
            }
            log:printError("Failed to retrieve registry token details for authorization.", token);
            return <http:InternalServerError>{
                body: {
                    message: reGenerateCustomErr
                }
            };
        }

        registry:TokenDescriptionInfo|error tokenInformation
            = registry:deriveTokenInfoFromDescription(token.description);
        if tokenInformation is error {
            log:printError("Failed to derive token information.", tokenInformation);
            return <http:InternalServerError>{
                body: {
                    message: reGenerateCustomErr
                }
            };
        }

        entity:ProjectResponse|error projectResponse = entity:getProject(userInfo.idToken,
                tokenInformation.snProjectId);
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
                logForbiddenProjectAccess(tokenInformation.snProjectId, userInfo.userId);
                return <http:Forbidden>{
                    body: {
                        message: ERR_MSG_PROJECT_ACCESS_FORBIDDEN
                    }
                };
            }
            if getStatusCode(projectResponse) == http:STATUS_NOT_FOUND {
                log:printWarn(string `Project with ID: ${tokenInformation.snProjectId} not found for user: ${
                        userInfo.userId}`);
                return <http:NotFound>{
                    body: {
                        message: "The requested token does not exist or you don't have access to it."
                    }
                };
            }

            log:printError("Failed to retrieve project details.", projectResponse);
            return <http:InternalServerError>{
                body: {
                    message: reGenerateCustomErr
                }
            };
        }

        boolean isAdmin = authorization:checkRoles([authorization:authorizedRoles.adminRole], userDetails.roles);

        // Enforce that only admins can delete service tokens, and users can only delete their own tokens.
        if !isAdmin &&
            (tokenInformation.tokenType == registry:SERVICE_TOKEN || tokenInformation.createdFor != userInfo.email) {

            log:printWarn(string `User: ${
                    userInfo.userId} attempted to delete a service token without proper privileges`);
            return <http:Forbidden>{
                body: {
                    message: "You don't have the necessary permissions to delete this registry token."
                }
            };
        }

        registry:TokenCreationResponse|error response = registry:regenerateToken(id);
        if response is error {
            if getStatusCode(response) == http:STATUS_NOT_FOUND {
                return <http:NotFound>{
                    body: {
                        message: "The registry token to be regenerated is not found!"
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to regenerate registry token with ID: ${id}.`);
                return <http:Forbidden>{
                    body: {
                        message: "You are not authorized to regenerate this registry token."
                    }
                };
            }

            log:printError(reGenerateCustomErr, response);
            return <http:InternalServerError>{
                body: {
                    message: reGenerateCustomErr
                }
            };
        }
        return response;
    }

    # Get integration users of a project by project ID.
    #
    # + id - ID of the project
    # + return - List of integration users or error
    isolated resource function get projects/[entity:IdString id]/integration\-users(http:RequestContext ctx)
        returns registry:IntegrationUser[]|http:Unauthorized|http:Forbidden|http:NotFound|
            http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
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
            if getStatusCode(projectResponse) == http:STATUS_NOT_FOUND {
                log:printWarn(string `Project with ID: ${id} not found for user: ${userInfo.userId}`);
                return <http:NotFound>{
                    body: {
                        message: "The requested project does not exist or you don't have access to it."
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

        registry:IntegrationUser[]|error response = registry:getIntegrationUsersByProjectId(projectResponse.sfId);
        if response is error {
            string customError = "Failed to retrieve integration users.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return response;
    }

    # Download license for a project deployment.
    #
    # + projectId - ID of the project
    # + deploymentId - ID of the deployment
    # + return - Change request details object or Error
    isolated resource function post projects/[string projectId]/deployments/[string deploymentId]/license
            (http:RequestContext ctx) returns product_consumption_subscription:License|http:InternalServerError
            |http:Unauthorized|http:Forbidden|http:NotFound {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:ProjectResponse|error projectResponse = entity:getProject(userInfo.idToken, projectId);
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
                logForbiddenProjectAccess(projectId, userInfo.userId);
                return <http:Forbidden>{
                    body: {
                        message: ERR_MSG_PROJECT_ACCESS_FORBIDDEN
                    }
                };
            }
            if getStatusCode(projectResponse) == http:STATUS_NOT_FOUND {
                log:printWarn(string `Project with ID: ${projectId} not found for user: ${userInfo.userId}`);
                return <http:NotFound>{
                    body: {
                        message: "The requested project does not exist or you don't have access to it."
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

        product_consumption_subscription:License|error licenseResponse =
            product_consumption_subscription:processLicenseDownload({email: userInfo.email, deploymentId, projectId});
        if licenseResponse is error {
            string customError = "Failed to retrieve license.";
            log:printError(customError, licenseResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return licenseResponse;
    }

    # Get usage stats for a specific project.
    #
    # + id - ID of the project
    # + return - Project stats response or error
    resource function get projects/[entity:IdString id]/stats/usage(http:RequestContext ctx)
        returns types:UsageStats|http:InternalServerError|http:Unauthorized|http:Forbidden|http:NotFound {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:ProjectStatsResponse|error statsResponse = entity:getProjectActivityStats(userInfo.idToken, id);
        if statsResponse is error {
            if getStatusCode(statsResponse) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(statsResponse) == http:STATUS_FORBIDDEN {
                logForbiddenProjectAccess(id, userInfo.userId);
                return <http:Forbidden>{
                    body: {
                        message: ERR_MSG_PROJECT_ACCESS_FORBIDDEN
                    }
                };
            }
            if getStatusCode(statsResponse) == http:STATUS_NOT_FOUND {
                log:printWarn(string `Project with ID: ${id} not found for user: ${userInfo.userId}`);
                return <http:NotFound>{
                    body: {
                        message: "The requested project does not exist or you don't have access to it."
                    }
                };
            }

            string customError = "Failed to retrieve project stats.";
            log:printError(customError, statsResponse);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return mapUsageStats(statsResponse);
    }

    # Search instance metrics for a specific project based on provided filters.
    #
    # + id - ID of the project
    # + payload - Instance metrics search payload containing filter criteria
    # + return - List of instance metrics matching the criteria or an error response
    resource function post projects/[entity:IdString id]/instances/metrics/search(http:RequestContext ctx,
            types:InstanceMetricsPayload payload)
        returns http:Ok|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:InstanceMetricsResponse|error response = entity:searchInstanceMetrics(userInfo.idToken,
                {
                    filters: {
                        projectIds: [id],
                        startDate: payload.filters.startDate,
                        endDate: payload.filters.endDate
                    }
                });
        if response is error {
            if getStatusCode(response) == http:STATUS_BAD_REQUEST {
                return <http:BadRequest>{
                    body: {
                        message: "Invalid request parameters for searching instance metrics for the project."
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                logForbiddenProjectAccess(id, userInfo.userId);
                return <http:Forbidden>{
                    body: {
                        message: ERR_MSG_PROJECT_ACCESS_FORBIDDEN
                    }
                };
            }

            string customError = "Failed to search instance metrics for the project.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return <http:Ok>{body: mapInstanceMetrics(response)};
    }

    # Search instance metrics for a specific deployment based on provided filters.
    #
    # + id - ID of the deployment
    # + payload - Instance metrics search payload containing filter criteria
    # + return - List of instance metrics matching the criteria or an error response
    resource function post deployments/[entity:IdString id]/instances/metrics/search(http:RequestContext ctx,
            types:InstanceMetricsPayload payload)
        returns http:Ok|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:InstanceMetricsResponse|error response = entity:searchInstanceMetrics(userInfo.idToken,
                {
                    filters: {
                        deploymentIds: [id],
                        startDate: payload.filters.startDate,
                        endDate: payload.filters.endDate
                    }
                });
        if response is error {
            if getStatusCode(response) == http:STATUS_BAD_REQUEST {
                return <http:BadRequest>{
                    body: {
                        message: "Invalid request parameters for searching instance metrics for a deployment."
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                logForbiddenProjectAccess(id, userInfo.userId);
                return <http:Forbidden>{
                    body: {
                        message: ERR_MSG_PROJECT_ACCESS_FORBIDDEN
                    }
                };
            }

            string customError = "Failed to search instance metrics for the deployment.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return <http:Ok>{body: mapInstanceMetrics(response)};
    }

    # Search instance metrics for a specific deployed product based on provided filters.
    #
    # + id - ID of the deployed product
    # + payload - Instance metrics search payload containing filter criteria
    # + return - List of instance metrics matching the criteria or an error response
    resource function post deployments/products/[entity:IdString id]/instances/metrics/search(http:RequestContext ctx,
            types:InstanceMetricsPayload payload)
        returns http:Ok|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:InstanceMetricsResponse|error response = entity:searchInstanceMetrics(userInfo.idToken,
                {
                    filters: {
                        deployedProductIds: [id],
                        startDate: payload.filters.startDate,
                        endDate: payload.filters.endDate
                    }
                });
        if response is error {
            if getStatusCode(response) == http:STATUS_BAD_REQUEST {
                return <http:BadRequest>{
                    body: {
                        message: "Invalid request parameters for searching instance metrics for a deployed product."
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                logForbiddenProjectAccess(id, userInfo.userId);
                return <http:Forbidden>{
                    body: {
                        message: ERR_MSG_PROJECT_ACCESS_FORBIDDEN
                    }
                };
            }

            string customError = "Failed to search instance metrics for the deployed product.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return <http:Ok>{body: mapInstanceMetrics(response)};
    }

    # Search instance usages for a specific project based on provided filters.
    #
    # + id - ID of the project
    # + payload - Instance metrics search payload containing filter criteria
    # + return - List of instance metrics matching the criteria or an error response
    resource function post projects/[entity:IdString id]/instances/usages/search(http:RequestContext ctx,
            types:InstanceMetricsPayload payload)
        returns http:Ok|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:InstanceUsageResponse|error response = entity:searchInstanceUsage(userInfo.idToken,
                {
                    filters: {
                        projectIds: [id],
                        startDate: payload.filters.startDate,
                        endDate: payload.filters.endDate
                    }
                });
        if response is error {
            if getStatusCode(response) == http:STATUS_BAD_REQUEST {
                return <http:BadRequest>{
                    body: {
                        message: "Invalid request parameters for searching instance usage for the project."
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                logForbiddenProjectAccess(id, userInfo.userId);
                return <http:Forbidden>{
                    body: {
                        message: ERR_MSG_PROJECT_ACCESS_FORBIDDEN
                    }
                };
            }

            string customError = "Failed to search instance usage for the project.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return <http:Ok>{body: mapInstanceUsages(response)};
    }

    # Search instance usages for a specific deployment based on provided filters.
    #
    # + id - ID of the deployment
    # + payload - Instance metrics search payload containing filter criteria
    # + return - List of instance metrics matching the criteria or an error response
    resource function post deployments/[entity:IdString id]/instances/usages/search(http:RequestContext ctx,
            types:InstanceMetricsPayload payload)
        returns http:Ok|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:InstanceUsageResponse|error response = entity:searchInstanceUsage(userInfo.idToken,
                {
                    filters: {
                        deploymentIds: [id],
                        startDate: payload.filters.startDate,
                        endDate: payload.filters.endDate
                    }
                });
        if response is error {
            if getStatusCode(response) == http:STATUS_BAD_REQUEST {
                return <http:BadRequest>{
                    body: {
                        message: "Invalid request parameters for searching instance usage for the deployment."
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                logForbiddenProjectAccess(id, userInfo.userId);
                return <http:Forbidden>{
                    body: {
                        message: ERR_MSG_PROJECT_ACCESS_FORBIDDEN
                    }
                };
            }

            string customError = "Failed to search instance usage for the deployment.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return <http:Ok>{body: mapInstanceUsages(response)};
    }

    # Search instance usages for a specific deployed product based on provided filters.
    #
    # + id - ID of the deployed product
    # + payload - Instance metrics search payload containing filter criteria
    # + return - List of instance metrics matching the criteria or an error response
    resource function post deployments/products/[entity:IdString id]/instances/usages/search(http:RequestContext ctx,
            types:InstanceMetricsPayload payload)
        returns http:Ok|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:InstanceUsageResponse|error response = entity:searchInstanceUsage(userInfo.idToken,
                {
                    filters: {
                        deployedProductIds: [id],
                        startDate: payload.filters.startDate,
                        endDate: payload.filters.endDate
                    }
                });
        if response is error {
            if getStatusCode(response) == http:STATUS_BAD_REQUEST {
                return <http:BadRequest>{
                    body: {
                        message: "Invalid request parameters for searching instance usage for the deployed product."
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                logForbiddenProjectAccess(id, userInfo.userId);
                return <http:Forbidden>{
                    body: {
                        message: ERR_MSG_PROJECT_ACCESS_FORBIDDEN
                    }
                };
            }

            string customError = "Failed to search instance usage for the deployed product.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        return <http:Ok>{body: mapInstanceUsages(response)};
    }

    # Search case activities for a specific case based on provided filters.
    #
    # + id - ID of the case
    # + payload - Case activity search payload containing filter criteria
    # + return - List of case activities matching the criteria or an error response
    resource function post cases/[entity:IdString id]/activities/search(http:RequestContext ctx,
            entity:CaseActivitySearchPayload payload)
        returns http:Ok|http:BadRequest|http:Unauthorized|http:Forbidden|http:InternalServerError {

        authorization:UserInfoPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        entity:CaseActivitySearchResponse|error response = entity:searchCaseActivities(userInfo.idToken, id, payload);
        if response is error {
            if getStatusCode(response) == http:STATUS_UNAUTHORIZED {
                log:printWarn(string `User: ${userInfo.userId} is not authorized to access the customer portal!`);
                return <http:Unauthorized>{
                    body: {
                        message: ERR_MSG_UNAUTHORIZED_ACCESS
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_FORBIDDEN {
                logForbiddenCaseAccess(id, userInfo.userId);
                return <http:Forbidden>{
                    body: {
                        message: ERR_MSG_CASE_ACCESS_FORBIDDEN
                    }
                };
            }
            if getStatusCode(response) == http:STATUS_BAD_REQUEST {
                return <http:BadRequest>{
                    body: {
                        message: "Invalid request parameters for searching case activities."
                    }
                };
            }

            string customError = "Failed to search case activities.";
            log:printError(customError, response);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }
        return <http:Ok>{body: mapCaseActivitySummaryResponse(response)};
    }
}

# WebSocket service to proxy messages between the browser and the upstream Python AI chat agent for real-time communication in chat sessions.
@websocket:ServiceConfig {
    subProtocols: ["cs-customer-portal"]
}
isolated service / on new websocket:Listener(wsPort) {

    # Upgrade an HTTP request to WebSocket for a given chat session.
    #
    # + req - The HTTP request for the WebSocket upgrade
    # + sessionId - Account/project ID passed as a query parameter
    # + return - WebSocket service or upgrade error
    isolated resource function get ws(http:Request req, string sessionId) returns websocket:Service|websocket:UpgradeError {
        // Try standard header first (e.g., when Choreo gateway injects it).
        string userIdToken;
        string|error headerToken = req.getHeader(authorization:USER_ID_TOKEN_HEADER);
        if headerToken is string {
            userIdToken = headerToken;
        } else {
            // Fallback: extract x-user-id-token from Sec-WebSocket-Protocol header.
            // When routed through Choreo, the gateway consumes the auth token and forwards
            // only the remaining subprotocol values as the header.
            // Format from client: "choreo-oauth2-token, <accessToken>, cs-customer-portal, <x-user-id-token>"
            // After Choreo strips first two: "cs-customer-portal, <x-user-id-token>"
            string|error protocolHeader = req.getHeader("Sec-WebSocket-Protocol");
            if protocolHeader is error {
                log:printError(string `No auth headers found for project: ${sessionId}`);
                return error websocket:UpgradeError(ERR_MSG_USER_INFO_HEADER_NOT_FOUND);
            }
            string[] parts = re `,`.split(protocolHeader);
            userIdToken = parts[parts.length() - 1].trim();
        }
        // Decode the user ID token to extract user info (email, userId)
        authorization:UserInfoPayload|error userInfo = authorization:getUserInfoFromTokens(userIdToken);
        if userInfo is error {
            log:printError(string `WebSocket auth failed for project: ${sessionId}`, userInfo);
            return error websocket:UpgradeError(ERR_MSG_UNAUTHORIZED_ACCESS);
        }
        log:printInfo(string `WebSocket upgrade successful for project: ${sessionId}`);
        return new WsProxyService(sessionId, userInfo);
    }
}

# AI chat agent related service functions that interact with the upstream AI chat agent through the client module.
isolated service class WsProxyService {
    *websocket:Service;
    private final string projectId;
    private final string idToken;
    private final string userEmail;
    private string? conversationId = ();
    private boolean streaming = false;

    isolated function init(string projectId, authorization:UserInfoPayload userInfo) {
        self.projectId = projectId;
        self.idToken = userInfo.idToken;
        self.userEmail = userInfo.email;
    }

    # Handles incoming WebSocket text messages from the browser client.
    # Responds to ping messages, enforces single-stream concurrency, and proxies
    # user messages to the upstream AI chat agent via the ai_chat_agent module.
    #
    # + caller - The WebSocket caller representing the connected browser client
    # + data - Raw text message received from the client
    # + return - Error if message handling or forwarding fails
    remote function onMessage(websocket:Caller caller, string data) returns error? {
        json|error parsed = data.fromJsonString();
        boolean isPing = data.trim().toLowerAscii() == "ping"
            || (parsed is map<json> && (parsed["type"] ?: "").toString() == "ping");
        if isPing {
            var [epochSecs, fracSecs] = time:utcNow();
            decimal ts = <decimal>epochSecs + fracSecs;
            check caller->writeTextMessage(string `{"type":"pong","ts":${ts}}`);
            return;
        }
        boolean alreadyStreaming;
        lock {
            alreadyStreaming = self.streaming;
            if !alreadyStreaming {
                self.streaming = true;
            }
        }
        if alreadyStreaming {
            json busyPayload = {"type": "error", "message": "A response is already being streamed. Please wait."};
            check caller->writeTextMessage(busyPayload.toJsonString());
            return;
        }

        log:printDebug(string `Received message for project: ${self.projectId}, payload: ${data}`);
        string? existingConversationId;
        lock {
            existingConversationId = self.conversationId;
        }
        if existingConversationId is () {
            string clientConvId = (parsed is map<json> ? (parsed["conversationId"] ?: "").toString() : "");
            log:printDebug(string `Parsed conversationId from payload: '${clientConvId}'`);
            if clientConvId.length() > 0 {
                lock {
                    self.conversationId = clientConvId;
                }
                log:printInfo(string `Resuming existing conversation: ${clientConvId} for project: ${self.projectId}`);
            } else {
                string userMessage = (parsed is map<json> ? (parsed["message"] ?: "").toString() : data);
                log:printInfo(string `Creating new conversation for project: ${self.projectId}`);
                entity:ConversationCreateResponse|error conversationResponse = entity:createConversation(
                        self.idToken,
                        {
                            projectId: self.projectId,
                            initialMessage: userMessage
                        });
                if conversationResponse is error {
                    lock {
                        self.streaming = false;
                    }
                    log:printError("Failed to create a new conversation.", conversationResponse);
                    json errorPayload = {"type": "error", "message": "Failed to create a new conversation."};
                    check caller->writeTextMessage(errorPayload.toJsonString());
                    return;
                }
                string convId = conversationResponse.conversation.id;
                lock {
                    self.conversationId = convId;
                }
                log:printDebug(string `Created conversation with ID: ${convId} for project: ${self.projectId}`);
                json createdEvent = {"type": "conversation_created", "conversationId": convId};
                check caller->writeTextMessage(createdEvent.toJsonString());
            }
        }

        string conversationId;
        lock {
            conversationId = self.conversationId ?: "";
        }
        string sessionId = string `${self.projectId}:${conversationId}`;
        string userMessage = (parsed is map<json> ? (parsed["message"] ?: "").toString() : data);
        string enrichedPayload;

        if parsed is map<json> {
            parsed["conversationId"] = conversationId;
            enrichedPayload = parsed.toJsonString();
        } else {
            enrichedPayload = data;
        }

        // Stream the conversation message to the upstream AI chat agent and get the final response
        map<json>|error result = ai_chat_agent:streamChat(sessionId, enrichedPayload, caller);
        lock {
            self.streaming = false;
        }
        if result is error {
            log:printError("WebSocket proxy stream error", result);
            json errorPayload = {"type": "error", "message": result.message()};
            error? writeErr = caller->writeTextMessage(errorPayload.toJsonString());
            if writeErr is error {
                log:printError("Failed to send error to caller (client disconnected)", writeErr);
            }
            return;
        }
        // Post-stream processing: persist conversation comments and update state based on the final AI response

        // Save the user query under comments
        entity:CommentCreateResponse|error userCommentResponse = entity:createComment(self.idToken,
                {
                    referenceId: conversationId,
                    referenceType: entity:CONVERSATION,
                    content: userMessage,
                    'type: entity:COMMENTS,
                    createdBy: self.userEmail
                });
        if userCommentResponse is error {
            log:printError("Failed to save user message as comment.", userCommentResponse);
        } else {
            log:printDebug(string `Saved user message as comment for conversation ID: ${conversationId}`);
        }

        // Save the AI agent response under comments
        string responseMessage = (result["message"] ?: "").toString();
        if responseMessage.length() > 0 {
            entity:CommentCreateResponse|error agentCommentResponse = entity:createComment(self.idToken,
                    {
                        referenceId: conversationId,
                        referenceType: entity:CONVERSATION,
                        content: responseMessage,
                        'type: entity:COMMENTS,
                        createdBy: entity:CHAT_SENT_AGENT
                    });
            if agentCommentResponse is error {
                log:printError("Failed to save chat response as comment.", agentCommentResponse);
            } else {
                log:printDebug(string `Saved AI agent response as comment for conversation ID: ${
                        conversationId}`);
            }
        }

        // Update conversation state if issue is resolved
        json resolvedVal = result["resolved"] ?: ();
        if resolvedVal is boolean && resolvedVal {
            log:printInfo(string `Issue resolved for conversation ID: ${conversationId}, updating state`);
            entity:ConversationUpdateResponse|error conversationUpdateResponse =
                    entity:updateConversation(self.idToken, conversationId,
                    {stateKey: entity:RESOLVED});
            if conversationUpdateResponse is error {
                string customError = "Failed to update conversation state to resolved.";
                log:printError(customError, conversationUpdateResponse);
            } else {
                log:printDebug(string `Updated conversation state to resolved for conversation ID: ${
                        conversationId}`);
            }
        }
    }

    # Handles WebSocket connection errors on the proxy link.
    #
    # + caller - The WebSocket caller representing the connected browser client
    # + err - The error that occurred on the connection
    # + return - Error if error handling itself fails
    remote function onError(websocket:Caller caller, error err) returns error? {
        log:printError("WebSocket proxy connection error", err);
    }

    # Handles WebSocket connection closure events.
    #
    # + caller - The WebSocket caller representing the connected browser client
    # + statusCode - WebSocket close status code
    # + reason - Reason string for the closure
    remote function onClose(websocket:Caller caller, int statusCode, string reason) {
        log:printInfo(string `WebSocket proxy closed [${statusCode}]: ${reason}`);
    }
}
