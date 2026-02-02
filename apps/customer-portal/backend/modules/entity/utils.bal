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

# Generate authorization headers.
#
# + token - ID token for authorization
# + return - Map of headers with authorization
isolated function generateHeaders(string token) returns map<string|string[]> => {"x-user-id-token": token};

# Get comments for a given entity ID with pagination.
#
# + idToken - ID token for authorization
# + id - Entity ID to filter comments
# + limit - Number of comments to retrieve
# + offset - Offset for pagination
# + return - Comments response or error
public isolated function getComments(string idToken, string id, int? 'limit, int? offset)
    returns CommentsResponse|error {

    ReferenceSearchPayload payload = {
        referenceId: id,
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
# + limit - Number of attachments to retrieve
# + offset - Offset for pagination
# + return - Attachments response or error
public isolated function getAttachments(string idToken, string id, int? 'limit, int? offset)
    returns AttachmentsResponse|error {

    ReferenceSearchPayload payload = {
        referenceId: id,
        pagination: {
            'limit: 'limit ?: DEFAULT_LIMIT,
            offset: offset ?: DEFAULT_OFFSET
        }
    };
    return searchAttachments(idToken, payload);
}
