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

# Create a registry token.
#
# + payload - Token creation payload
# + return - Created registry token or an error
public isolated function createToken(TokenCreatePayload payload) returns TokenCreationResponse|error {
    return registryClient->/robot\-accounts.post(payload);
}

# Search registry tokens.
#
# + payload - Registry token search payload
# + return - List of registry tokens matching the search criteria or an error
public isolated function searchTokens(TokenSearchPayload payload) returns Token[]|error {
    return registryClient->/robot\-accounts/search.post(payload);
}

# Get registry token by ID.
# 
# + tokenId - ID of the registry token to be retrieved
# + return - Registry token details or an error
public isolated function getTokenById(string tokenId) returns Token|error {
    return registryClient->/robot\-accounts/[tokenId].get();
}

# Delete a registry token.
#
# + tokenId - ID of the registry token to be deleted
# + return - Success message or an error
public isolated function deleteToken(string tokenId) returns error? {
    return registryClient->/robot\-accounts/[tokenId].delete();
}

# Regenerate the secret of a registry token.
#
# + tokenId - ID of the registry token to be regenerated
# + return - Regenerated registry token or an error
public isolated function regenerateToken(string tokenId) returns TokenCreationResponse|error {
    return registryClient->/robot\-accounts/[tokenId]/regenerate\-token.post({});
}

# Get integration users of a project by project ID.
#
# + projectId - Unique ID of the project
# + return - List of integration users or error
public isolated function getIntegrationUsersByProjectId(string projectId) returns IntegrationUser[]|error {
    return registryClient->/projects/[projectId]/integration\-users.get();
}
