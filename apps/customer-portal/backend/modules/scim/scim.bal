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

# Searches for users belonging to a specific group from the SCIM operations service.
# 
# + email - Email of the user to be searched
# + return - An array of User records, or an error if the operation fails
public isolated function searchUsers(string email) returns User[]|error {
    boolean moreUsersExist = true;
    User[] users = [];
    int startIndex = 1;
    while moreUsersExist {
        string organization = isWso2Email(email) ? ORGANIZATION_INTERNAL : ORGANIZATION_EXTERNAL;
        UserSearchResult usersResult = check scimOperationsClient->/organizations/[organization]/users/search.post({
            domain: DOMAIN_DEFAULT,
            attributes: [ATTRIBUTE_PHONE_NUMBERS, ATTRIBUTE_USERNAME],
            filter: string `userName eq ${email}`,
            startIndex
        });
        users.push(...usersResult.Resources);
        moreUsersExist = (startIndex + usersResult.itemsPerPage - 1) < usersResult.totalResults;
        startIndex += usersResult.itemsPerPage;
    }
    return users;
}
