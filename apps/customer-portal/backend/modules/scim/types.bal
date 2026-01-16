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

# [Configurable] Client credentials grant type oauth2 configuration.
type ClientCredentialsOauth2Config record {|
    # OAuth2 token endpoint
    string tokenUrl;
    # OAuth2 client ID
    string clientId;
    # OAuth2 client secret
    string clientSecret;
    # OAuth2 scopes
    string[] scopes;
|};

# User search result.
public type UserSearchResult record {|
    # Total number of users
    int totalResults;
    # Starting index of the response
    int startIndex;
    # Number of users returned in the response
    int itemsPerPage;
    # List of group details
    User[] Resources = [];
    json...;
|};

# User.
public type User record {|
    # User UUID
    string id;
    # Phone numbers
    PhoneNumber[]? phoneNumbers;
    json...;
|};

# Phone number.
public type PhoneNumber record {|
    # Type
    string 'type;
    # Value
    string value;
    json...;
|};
