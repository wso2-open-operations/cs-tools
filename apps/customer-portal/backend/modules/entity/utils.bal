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
import ballerina/time;

public configurable CaseStateIds & readonly caseStateIds = {
    open: 1,
    closed: 3,
    waitingOnWso2: 1003,
    reopened: 1006,
    awaitingInfo: 18,
    solutionProposed: 6,
    workInProgress: 10
};
public configurable ConversationStateIds & readonly conversationStateIds =
    {open: 1, active: 2, resolved: 3, converted: 4, abandonded: 5};

# Generate authorization headers.
#
# + token - ID token for authorization
# + return - Map of headers with authorization
isolated function generateHeaders(string token) returns map<string|string[]> => {"x-user-id-token": token};

# Get comments for a given entity ID with pagination.
#
# + idToken - ID token for authorization
# + referenceType - Reference type (e.g., CASE, DEPLOYMENT)
# + id - Entity ID to filter comments
# + limit - Number of comments to retrieve
# + offset - Offset for pagination
# + return - Comments response or error
public isolated function getComments(string idToken, ReferenceType referenceType, string id, int? 'limit, int? offset)
    returns CommentsResponse|error {

    ReferenceSearchPayload payload = {
        referenceId: id,
        referenceType,
        pagination: {
            'limit: 'limit ?: DEFAULT_LIMIT,
            offset: offset ?: DEFAULT_OFFSET
        }
    };
    return searchComments(idToken, payload);
}

# Get attachments for a given entity ID with pagination.
#
# + idToken - ID token for authorization
# + id - Entity ID to filter attachments
# + referenceType - Reference type (e.g., CASE, DEPLOYMENT)
# + limit - Number of attachments to retrieve
# + offset - Offset for pagination
# + return - Attachments response or error
public isolated function getAttachments(string idToken, string id, ReferenceType referenceType, int? 'limit, int? offset)
    returns AttachmentsResponse|error {

    ReferenceSearchPayload payload = {
        referenceId: id,
        referenceType: referenceType,
        pagination: {
            'limit: 'limit ?: DEFAULT_LIMIT,
            offset: offset ?: DEFAULT_OFFSET
        }
    };
    return searchAttachments(idToken, payload);
}

# Validate call request update payload.
#
# + payload - Call request update payload
# + return - Error message if validation fails, nil otherwise
public isolated function validateCallRequestUpdatePayload(CallRequestUpdatePayload payload) returns string? {
    // Validate stateKey is either 2 (Pending on WSO2) or 6 (Canceled)
    if payload.stateKey != PENDING_ON_WSO2 && payload.stateKey != CANCELED {
        return "Invalid status. Allowed values are Pending on WSO2 or Canceled.";
    }

    string[]? utcTimes = payload.utcTimes;

    // If stateKey is 2 (Pending on WSO2), utcTimes is mandatory
    if payload.stateKey == PENDING_ON_WSO2 && (utcTimes is () || utcTimes.length() == 0) {
        return "At least one UTC time is required when the status is Pending on WSO2.";
    }

    // If stateKey is 6 (Canceled), utcTimes should not be present
    if payload.stateKey == CANCELED && utcTimes !is () {
        return "UTC times must not be provided when the status is Canceled.";
    }

    return ();
}

# Validate UTC times in the call request payloads.
#
# + utcTimes - Array of UTC time strings to validate
# + return - Error message if any UTC time is invalid or in the past, nil otherwise
public isolated function validateUtcTimes(DateTime[]? utcTimes) returns string|error? {
    if utcTimes != () {
        foreach string utcTime in utcTimes {
            time:Utc input = check time:utcFromString(utcTime);
            time:Utc now = time:utcNow();
            // TODO: Handle the timezone validation
            if input < now {
                return "UTC time cannot be in the past.";
            }
        }
    }

    return;
}

# Validate deployment update payload.
#
# + payload - Deployment update payload
# + return - Error message if validation fails, nil otherwise
public isolated function validateDeploymentUpdatePayload(DeploymentUpdatePayload payload) returns string? {
    boolean hasDeploymentFields = payload.name !is () || payload.typeKey !is () || payload?.description !is ();

    // Check if payload has at least one field
    if !hasDeploymentFields && payload.active is () {
        return "At least one field (name, typeKey, or active) must be provided for update.";
    }

    // Validate that deployment fields and active field are mutually exclusive
    if hasDeploymentFields && payload.active !is () {
        return "Deployment fields (name, typeKey, description) and active field cannot be updated together.";
    }

    // Validate that active field can only be false
    if payload.active !is () {
        boolean? activeValue = payload.active;
        if activeValue is boolean && activeValue {
            return "Active field can only be set to false.";
        }
    }

    return;
}

# Validate case update payload.
#
# + payload - Case update payload
# + return - Error message if validation fails, nil otherwise
public isolated function validateCaseUpdatePayload(CaseUpdatePayload payload) returns string? {
    if payload.stateKey != caseStateIds.closed && payload.stateKey != caseStateIds.reopened &&
    payload.stateKey != caseStateIds.waitingOnWso2 {
        return "Invalid status. Allowed values are Waiting on WSO2, Closed, or Reopened.";
    }
    return;
}

# Validate case create payload based on case type.
#
# + payload - Case create payload
# + return - Validation error message or null if valid
public isolated function validateCaseCreatePayload(CaseCreatePayload payload) returns string? {
    CaseType caseType = payload.'type;
    string? title = payload.title;
    string? description = payload.description;

    if caseType is DEFAULT_CASE {
        if title is () {
            return "Title is required for default case.";
        }
        if title.trim().length() == 0 || title.length() > 500 {
            return "Title must be between 1 and 500 characters long for default case.";
        }
        if description is () || description.trim().length() == 0 {
            return "Description cannot be empty for default case.";
        }
        if payload.issueTypeKey is () {
            return "Issue type key is required for default case.";
        }
        if payload.severityKey is () {
            return "Severity key is required for default case.";
        }
    } else if caseType is SERVICE_REQUEST {
        if payload.catalogId is () {
            return "Catalog is required for service request case.";
        }
        if payload.catalogItemId is () {
            return "Catalog Item is required for service request case.";
        }
        Variable[]? variables = payload.variables;
        if variables is () || variables.length() == 0 {
            return "At least one variable is required for service request case.";
        }
    } else if caseType is SECURITY_REPORT_ANALYSIS {
        if title is () {
            return "Title is required for security report analysis case type.";
        }
        if title.trim().length() == 0 || title.length() > 500 {
            return "Title must be between 1 and 500 characters long for security report analysis case.";
        }
        if description is () || description.trim().length() == 0 {
            return "Description is required for security report analysis case.";
        }
        CaseCreateAttachment[]? attachments = payload.attachments;
        if attachments is CaseCreateAttachment[] {
            foreach CaseCreateAttachment attachment in attachments {
                if attachment.name.trim().length() == 0 {
                    return "Attachment name cannot be empty for security analysis case.";
                }
                if attachment.file.trim().length() == 0 {
                    return "Attachment content cannot be empty for security analysis case.";
                }
            }
        } else {
            return "At least one attachment is required for security report analysis case.";
        }
    } else {
        return string `Case type ${caseType} is not supported.`;
    }
    return;
}

# Validate deployed product update payload.
#
# + payload - Deployed product update payload
# + return - Error message if validation fails, () otherwise
public isolated function validateDeployedProductUpdatePayload(DeployedProductUpdatePayload payload) returns string? {
    boolean? active = payload.active;
    int? cores = payload?.cores;
    decimal? tps = payload?.tps;
    string? description = payload?.description;
    ProductUpdate[]? updates = payload?.updates;
    if active is boolean {
        if active {
            return "Invalid value for active field. When updating cores or tps, active field should be set to false.";
        }
        if cores !is () || tps !is () || description !is () || updates !is () {
            return "When deactivating, cores, tps, description and updates fields should not be provided.";
        }
    } else if cores is () && tps is () && description is () && updates is () {
        return "At least one of cores or tps or description or updates should be provided when updating " +
            "deployed product details.";
    }
    return;
}

# Validate attachment update payload.
#
# + payload - Attachment update payload
# + return - Validation error message or null if valid
public isolated function validateAttachmentUpdatePayload(AttachmentUpdatePayload payload) returns string? {
    ReferenceType referenceType = payload.referenceType;
    string? description = payload?.description;
    string? name = payload?.name;

    // Validate reference type
    if referenceType != CASE && referenceType != DEPLOYMENT {
        return string `Invalid type '${referenceType}'. Only 'case' and 'deployment' are allowed.`;
    }

    // If referenceType is CASE, name is required and description should not be present
    if referenceType == CASE {
        if description !is () {
            return "Description field is not allowed for case type.";
        }
        if name is () || name.trim().length() == 0 {
            return "Name field is required for case type.";
        }
    }

    // If referenceType is DEPLOYMENT, at least one of name or description should be present
    if referenceType == DEPLOYMENT {
        boolean hasName = name !is () && name.trim().length() > 0;
        boolean hasDescription = description !is () && description.trim().length() > 0;

        if !hasName && !hasDescription {
            return "At least one field (name or description) must be provided for deployment type.";
        }
    }

    return;
}
