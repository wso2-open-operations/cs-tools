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

# Process product update levels based on the provided parameters.
#
# + return - List of product update levels, or an error if the operation fails
public isolated function processProductUpdateLevels() returns types:ProductUpdateLevel[]|error {
    ProductUpdateLevel[] productUpdateLevels = check getProductUpdateLevels();
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

# Check if the given update description level is a security update.
#
# + description - The update description level to check
# + return - true if the update description level is a security update, false otherwise
isolated function isSecurityUpdate(types:UpdateDescription description) returns boolean =>
    description.updateType == UPDATE_TYPE_SECURITY;

# Group update description levels by their update level and determine the overall update type for each group.
#
# + updateDescriptions - An array of update descriptions to be grouped
# + return - A map where the key is the update level and the value is an object containing the overall update 
# type and the list of update description levels for that update level
isolated function groupByUpdateLevel(types:UpdateDescription[] updateDescriptions)
    returns map<types:UpdateLevelGroup> {

    map<types:UpdateLevelGroup> groupedUpdateLevels = {};
    foreach types:UpdateDescription description in updateDescriptions {
        string levelKey = description.updateLevel.toString();

        if !groupedUpdateLevels.hasKey(levelKey) {
            groupedUpdateLevels[levelKey] = {
                updateType: isSecurityUpdate(description) ? UPDATE_TYPE_SECURITY : UPDATE_TYPE_REGULAR,
                updateDescriptionLevels: []
            };
        }
        types:UpdateDescription[]? existingLevels = groupedUpdateLevels[levelKey]?.updateDescriptionLevels;
        if existingLevels is types:UpdateDescription[] {
            existingLevels.push(description);
        }

        if isSecurityUpdate(description) {
            types:UpdateLevelGroup? group = groupedUpdateLevels[levelKey];
            if group is types:UpdateLevelGroup {
                group.updateType = UPDATE_TYPE_SECURITY;
            }
        }

    }

    return groupedUpdateLevels;
}

# Process search for updates between specified update levels.
#
# + email - Email of the user
# + payload - Payload containing the update levels to search between
# + return - Update description for the specified update levels, or an error if the operation fails
public isolated function processSearchUpdatesBetweenUpdateLevels(string email, types:UpdateDescriptionPayload payload)
    returns map<types:UpdateLevelGroup>|error {

    UpdateDescriptionRequest requestPayload = {
        product\-name: payload.productName,
        product\-version: payload.productVersion,
        channel: CHANNEL,
        starting\-update\-level: payload.startingUpdateLevel,
        ending\-update\-level: payload.endingUpdateLevel,
        user\-email: email
    };

    UpdateDescription[] response = check searchUpdatesBetweenUpdateLevels(requestPayload);
    types:UpdateDescription[] updateDescriptions = from UpdateDescription description in response
        let DependantRelease[]? dependantReleases = description?.dependant\-releases
        select {
            productName: description.product\-name,
            productVersion: description.product\-version,
            channel: description.channel,
            updateLevel: description.update\-level,
            updateNumber: description.update\-number,
            description: description?.description,
            instructions: description?.instructions,
            bugFixes: description?.bug\-fixes,
            filesAdded: description?.files\-added,
            filesModified: description?.files\-modified,
            filesRemoved: description?.files\-removed,
            bundlesInfoChanges: description?.bundles\-info\-changes,
            updateType: description?.update\-type,
            timestamp: description.timestamp,
            securityAdvisories: from SecurityAdvisoryDescription advisory in description.security\-advisories
                select {
                    id: advisory.id,
                    overview: advisory.overview,
                    severity: advisory.severity,
                    description: advisory.description,
                    impact: advisory.impact,
                    solution: advisory.solution,
                    notes: advisory.notes,
                    credits: advisory.credits
                },
            dependantReleases: dependantReleases is () ? () : from DependantRelease release in dependantReleases
                    select {
                        repository: release.repository,
                        releaseVersion: release.release\-version
                    }
        };
    return groupByUpdateLevel(updateDescriptions);
}
