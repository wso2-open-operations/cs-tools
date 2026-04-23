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

# Get product update levels.
#
# + return - List of product update levels, or an error if the operation fails
public isolated function getProductUpdateLevels() returns ProductUpdateLevel[]|error {
    return updatesClient->/updates/product\-update\-levels.get();
}

# Search for updates between specified update levels.
#
# + payload - Payload containing the update levels to search between
# + return - Update description for the specified update levels, or an error if the operation fails
public isolated function searchUpdatesBetweenUpdateLevels(UpdateDescriptionRequest payload)
    returns UpdateDescription[]|error {

    return updatesClient->/updates/descriptions.post(payload);
}
