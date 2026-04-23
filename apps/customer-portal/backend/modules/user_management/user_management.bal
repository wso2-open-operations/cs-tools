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
import ballerina/http;
import ballerina/log;

public type CONFLICT_ERROR distinct error;

# Get project contacts for the given project ID.
#
# + projectId - Salesforce ID of the project to get contacts for
# + return - Array of contacts or error
public isolated function getProjectContacts(string projectId) returns Contact[]|error {
    return userManagementClient->/projects/[projectId]/contacts.get();
}

# Create a new contact for the given project ID and payload.
#
# + projectId - Salesforce ID of the project to create a contact for
# + payload - Payload containing contact information
# + return - Created contact or error
public isolated function createProjectContact(string projectId, OnBoardContactPayload payload)
    returns Membership|error {

    string customError = "An error occurred while creating the contact for the project.";

    http:Response userCreateResponse = check userManagementClient->/projects/[projectId]/contact.post(payload);
    if userCreateResponse.statusCode != http:STATUS_CREATED {
        json|error errBody = userCreateResponse.getJsonPayload();
        if errBody is error {
            log:printError(customError, errBody);
            return error(customError);
        }
        return error(check errBody.message);
    }
    return (check userCreateResponse.getJsonPayload()).cloneWithType();
}

# Remove a contact from the given project ID using the contact's email and admin email.
#
# + projectId - Salesforce ID of the project to remove the contact from
# + contactEmail - Email of the contact to be removed
# + adminEmail - Email of the admin performing the removal
# + return - Membership information of the removed contact or error
public isolated function removeProjectContact(string projectId, string contactEmail, string adminEmail)
    returns Membership|error {

    string customError = "An error occurred while removing the contact from the project.";

    http:Response userRemoveResponse =
        check userManagementClient->/projects/[projectId]/contacts/[contactEmail].delete(adminEmail = adminEmail);
    if userRemoveResponse.statusCode != http:STATUS_OK {
        json|error errBody = userRemoveResponse.getJsonPayload();
        if errBody is error {
            log:printError(customError, errBody);
            return error(customError);
        }
        return error(check errBody.message);
    }
    return (check userRemoveResponse.getJsonPayload()).cloneWithType();
}

# Update the membership flag of a contact in the given project ID using the contact's email and payload.
#
# + projectId - Salesforce ID of the project to update the contact's membership flag for
# + contactEmail - Email of the contact whose membership flag is to be updated
# + payload - Payload containing the new membership flag information
# + return - Updated membership information of the contact or error
public isolated function updateMembershipFlag(string projectId, string contactEmail, MembershipSecurityPayload payload)
    returns Membership|error {

    string customError = "An error occurred while updating the contact's membership flag.";

    http:Response userUpdateResponse = check userManagementClient->/projects/[projectId]/contacts/[contactEmail].patch(payload);
    if userUpdateResponse.statusCode != http:STATUS_OK {
        json|error errBody = userUpdateResponse.getJsonPayload();
        if errBody is error {
            log:printError(customError, errBody);
            return error(customError);
        }
        return error(check errBody.message);
    }
    return (check userUpdateResponse.getJsonPayload()).cloneWithType();
}

# Validate a project contact using the provided payload.
#
# + payload - Payload containing information to validate the project contact
# + return - Validated contact information or error
public isolated function validateProjectContact(ValidationPayload payload) returns Contact|error? {

    string customError = "An error occurred while validating the project contact.";

    http:Response userManagementResponse = check userManagementClient->/validate\-project\-contact.post(payload);

    // If there's an existing Deactivated contact, return the contact details.
    // If the contact is valid and can be onboarded, return nill.
    if userManagementResponse.statusCode == http:STATUS_CREATED {
        return (check userManagementResponse.getJsonPayload()).cloneWithType();
    } else if userManagementResponse.statusCode == http:STATUS_ACCEPTED {
        return;
    }

    json|error errBody = userManagementResponse.getJsonPayload();
    if errBody is error {
        log:printError(customError, errBody);
        return error(customError);
    }

    // If there's an existing Active contact, return a conflict error with the contact details.
    if userManagementResponse.statusCode == http:STATUS_CONFLICT {
        log:printError(customError, info = errBody.toString());
        return error CONFLICT_ERROR(check errBody.message);
    }

    // For any other error status code, return a generic error with the error message.
    log:printError(customError, info = errBody.toString());
    return error(check errBody.message);
}
