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
import ballerina/data.jsondata;

# [Configurable] Client credentials grant type oauth2 configuration.
type ClientCredentialsOauth2Config record {|
    # OAuth2 token endpoint
    string tokenUrl;
    # OAuth2 client ID
    string clientId;
    # OAuth2 client secret
    string clientSecret;
    # OAuth2 scopes
    string[] scopes = [];
|};

# Case classification payload.
public type CaseClassificationPayload record {|
    # Chat history
    string chat_history;
    # Product details
    string[] productDetails;
    # Environments
    string[] environments;
    # Region
    string region;
    # Tier
    string tier;
|};

# Case classification response.
public type CaseClassificationResponse record {|
    # Issue type
    string issueType;
    # Case information
    @jsondata:Name {value: "case_info"}
    ChatCaseInfo caseInfo;
    # Severity level
    string severityLevel;
    json...;
|};

# Chat case information.
public type ChatCaseInfo record {|
    # Description
    string description;
    # Short description
    string shortDescription;
    # Product name
    string productName;
    # Product version
    string productVersion;
    # Environment
    string environment;
    # Tier
    string tier;
    # Region
    string region;
    json...;
|};
