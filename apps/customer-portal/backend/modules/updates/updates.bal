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

# Get recommended update levels for a user.
#
# + email - Email of the user
# + return - An array of RecommendedUpdateLevel records, or an error if the operation fails
public isolated function getRecommendedUpdateLevels(string email) returns RecommendedUpdateLevel[]|error {
    return updatesClient->/updates/recommended\-update\-levels.get(user = email);
}

# List updates based on the provided parameters.
#
# + idToken - ID token for authentication
# + payload - Payload for listing updates
# + return - List of updates, or an error if the operation fails
public isolated function listUpdates(string idToken, ListUpdatePayload payload) returns UpdateResponse|error {
    return updatesClient->/updates/list\-updates.post(payload, generateHeaders(idToken), readOnly = true);
}

# Get product update levels based on the provided parameters.
#
# + idToken - ID token for authentication
# + return - List of product update levels, or an error if the operation fails
public isolated function getProductUpdateLevels(string idToken) returns ProductUpdateLevel[]|error {
    return updatesClient->/updates/product\-update\-levels.get(generateHeaders(idToken));
}
