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
public type ClientCredentialsOauth2Config record {|
    # Token URL
    string tokenUrl;
    # Client ID
    string clientId;
    # Client Secret
    string clientSecret;
    # Scopes
    string[] scopes = [];
|};

# Deployment usage import response.
public type DeploymentUsageImportResponse record {|
    # Response message
    string message?;
    # Response result
    json result?;
    json...;
|};

# Import Request payload.
#
# + email - email address of the user performing the import operation  
# + zip - zip file content as base64 string
public type ImportRequest record {|
    string email;
    string zip;
|};
