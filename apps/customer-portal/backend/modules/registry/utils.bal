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

const int DESCRIPTION_PART_COUNT = 5;

# Derive token information from it's description.
# Description format: <snAccountId>##<snProjectId>##<TokenType>##<createdFor>##<createdBy>
#
# + description - The token description string
# + return - Derived token description info or an error if the format is invalid
public isolated function deriveTokenInfoFromDescription(string description) returns TokenDescriptionInfo|error {
    string[] parts = re `##`.split(description);
    if parts.length() != DESCRIPTION_PART_COUNT {
        return error("Invalid token description format");
    }
    return {
        snAccountId: parts[0],
        snProjectId: parts[1],
        tokenType: check parts[2].ensureType(TokenType),
        createdFor: parts[3],
        createdBy: parts[4]
    };
}
