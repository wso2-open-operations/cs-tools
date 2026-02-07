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

# Add update entry.
#
# + idToken - ID token for authentication
# + payload - UpdatesPayload to be added
# + return - Error if the operation fails
public isolated function addUpdate(string idToken, UpdatesPayload payload) returns error? {
    return updatesClient->/updates.post(payload, generateHeaders(idToken));
}

# Get recommended update levels for a user.
#
# + email - Email of the user
# + return - An array of RecommendedUpdateLevel records, or an error if the operation fails
public isolated function getRecommendedUpdateLevels(string email) returns RecommendedUpdateLevel[]|error {
    return updatesClient->/updates/recommended\-update\-levels.get(userEmail = email);
}

# List updates based on the provided parameters.
#
# + idToken - ID token for authentication
# + payload - Payload for listing updates
# + readOnly - Optional boolean parameter to indicate if the updates should be read-only
# + updateLevelState - Optional string parameter to filter updates based on their level state
# + return - List of updates, or an error if the operation fails
public isolated function listUpdates(string idToken, ListUpdatePayload payload, boolean? readOnly,
        string? updateLevelState) returns UpdateResponse|error {

    http:QueryParams queryParams = {};
    if readOnly is boolean {
        queryParams["readOnly"] = readOnly.toString();
    }
    if updateLevelState is string {
        queryParams["updateLevelState"] = updateLevelState;
    }

    return updatesClient->/updates/list\-updates.post(payload, generateHeaders(idToken), params = queryParams);
}

# Get product update levels based on the provided parameters.
#
# + idToken - ID token for authentication
# + updateLevelState - Optional string parameter to filter updates based on their level state
# + return - List of product update levels, or an error if the operation fails
public isolated function getProductUpdateLevels(string idToken, string? updateLevelState)
    returns ProductUpdateLevel[]|error {

    string? updateLevelStateToUse = updateLevelState;
    if updateLevelStateToUse is string {
        return updatesClient->/product\-update\-levels.get(generateHeaders(idToken),
            updateLevelState = updateLevelStateToUse
        );
    }
    return updatesClient->/updates/product\-update\-levels.get(generateHeaders(idToken));
}
