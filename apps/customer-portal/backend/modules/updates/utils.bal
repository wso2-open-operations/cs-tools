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
isolated function generateHeaders(string token) returns map<string|string[]> => {"x-jwt-assertion": token};

# Search for updates based on the provided criteria.
#
# + idToken - ID token for authorization
# + payload - Search criteria for updates
# + return - List of updates, or an error if the operation fails
public isolated function searchUpdates(string idToken, UpdateSearchPayload payload) returns UpdateResponse|error {
    ListUpdatePayload listUpdatePayload = {
        productName: payload.productName,
        productBaseVersion: payload.productBaseVersion,
        channel: payload.channel,
        startingUpdateLevel: payload.startingUpdateLevel,
        endingUpdateLevel: payload.endingUpdateLevel,
        hotfixes: payload.hotfixes
    };
    return listUpdates(idToken, listUpdatePayload, payload.readOnly, payload.updateLevelState);
}
