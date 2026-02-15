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
import customer_portal.types;

# Generate authorization headers.
#
# + token - ID token for authorization
# + return - Map of headers with authorization
isolated function generateHeaders(string token) returns map<string|string[]> => {"x-jwt-assertion": token};

# Process recommended update levels for a user.
#
# + email - Email of the user
# + return - An array of recommended update levels, or an error if the operation fails
public isolated function processRecommendedUpdateLevels(string email) returns types:RecommendedUpdateLevel[]|error {
    RecommendedUpdateLevel[] recommendedUpdateLevels = check getRecommendedUpdateLevels(email);
    return from RecommendedUpdateLevel level in recommendedUpdateLevels
        select {
            productName: level.product\-name,
            productBaseVersion: level.product\-base\-version,
            channel: level.channel,
            startingUpdateLevel: level.starting\-update\-level,
            endingUpdateLevel: level.ending\-update\-level,
            installedUpdatesCount: level.installed\-updates\-count,
            installedSecurityUpdatesCount: level.installed\-security\-updates\-count,
            timestamp: level.timestamp,
            recommendedUpdateLevel: level.recommended\-update\-level,
            availableSecurityUpdatesCount: level.available\-security\-updates\-count,
            availableUpdatesCount: level.available\-updates\-count
        };
}

# Process list updates based on the provided parameters.
#
# + idToken - ID token for authentication
# + payload - Payload for listing updates
# + return - List of updates, or an error if the operation fails
public isolated function processListUpdates(string idToken, types:ListUpdatePayload payload)
    returns types:UpdateResponse|error {

    ListUpdatePayload requestPayload = {
        product\-name: payload.productName,
        product\-base\-version: payload.productBaseVersion,
        channel: payload.channel,
        start\-update\-level: payload.startUpdateLevel,
        end\-update\-level: payload.endUpdateLevel,
        hot\-fixes: payload.hotFixes
    };

    UpdateResponse response = check listUpdates(idToken, requestPayload);
    types:BasicFileInfo[] addedFiles = from BasicFileInfo info in response.file\-changes.added\-files
        select {
            filePath: info.file\-path,
            md5sum: info.md5sum,
            sha256: info.sha256,
            jwt: info.jwt,
            downloadUrl: info.download\-url
        };

    types:ModifiedFileInfo[] modifiedFiles = from ModifiedFileInfo info in response.file\-changes.modified\-files
        select {
            'type: info.'type,
            originalFile: {
                filePath: info.original\-file.file\-path,
                md5sum: info.original\-file.md5sum,
                sha256: info.original\-file.sha256,
                jwt: info.original\-file.jwt,
                downloadUrl: info.original\-file.download\-url
            },
            newFile: {
                filePath: info.new\-file.file\-path,
                md5sum: info.new\-file.md5sum,
                sha256: info.new\-file.sha256,
                jwt: info.new\-file.jwt,
                downloadUrl: info.new\-file.download\-url
            }
        };

    types:BundlesInfoChange[] bundlesInfoChanges =
    from BundlesInfoChange info in response.file\-changes.bundles\-info\-changes
    select {
        bundlesInfoPath: info.bundles\-info\-path,
        contentChange: info.content\-change,
        changeType: {value: info.change\-type.value},
        'order: info.'order
    };

    return {
        fileChanges: {
            addedFiles,
            modifiedFiles,
            removedFiles: response.file\-changes.removed\-files,
            bundlesInfoChanges
        },
        jwt: response.jwt,
        platformName: response.platform\-name,
        platformVersion: response.platform\-version,
        productName: response.product\-name,
        productBaseVersion: response.product\-base\-version,
        startingUpdateLevel: response.starting\-update\-level,
        endingUpdateLevel: response.ending\-update\-level,
        environment: response.environment,
        updateSummaryMessage: response.update\-summary\-message,
        updateSecurityMessage: response.update\-security\-message,
        totalUpdates: response.total\-updates,
        totalSecurityUpdates: response.total\-security\-updates,
        appliedUpdateNumbers: response.applied\-update\-numbers
    };
}

# Process product update levels based on the provided parameters.
#
# + idToken - ID token for authentication
# + return - List of product update levels, or an error if the operation fails
public isolated function processProductUpdateLevels(string idToken) returns types:ProductUpdateLevel[]|error {
    ProductUpdateLevel[] productUpdateLevels = check getProductUpdateLevels(idToken);
    return from ProductUpdateLevel level in productUpdateLevels
        select {
            productName: level.product\-name,
            productUpdateLevels: from UpdateLevel updateLevel in level.product\-update\-levels
                select {
                    productBaseVersion: updateLevel.product\-base\-version,
                    channel: updateLevel.channel,
                    updateLevels: updateLevel.update\-levels
                }
        };
}
