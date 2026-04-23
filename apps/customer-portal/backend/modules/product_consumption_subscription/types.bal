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
|};

# Result.
public type Result record {|
    # Response message
    string message?;
    # Choreo application ID
    string applicationId?;
    # Status data
    Data result;
|};

# Data.
public type Data record {|
    # Response message
    int status;
    # Choreo application ID
    string applicationId?;
    # Application name
    string name?;
    # Application description
    string description?;
    json...;
|};

# Project status request payload structure (inbound to this service).
public type LicenseDownloadPayload record {
    # Email of the user
    string email;
    # Unique deployment identifier
    string deploymentId;
    # Unique project identifier
    string projectId;
};

# Choreo application creation response structure.
public type ApplicationCreateResponse record {|
    # Application name
    string name;
    # Application ID
    string applicationId;
    json...;
|};

# Choreo credentials generation response structure.
public type CredentialsResponse record {|
    # OAuth2 consumer key
    string consumerKey;
    # OAuth2 consumer secret
    string consumerSecret;
    json...;
|};

# Choreo secret keys generation response structure.
public type SecretKeysResponse record {|
    # Primary subscription secret key
    string primarySecretKey;
    # Secondary subscription secret key
    string secondarySecretKey;
    json...;
|};

# Subscription data within the license response.
public type SubscriptionData record {|
    # Deployment identifier
    string deploymentId;
    # Deployment name
    string deploymentName;
    # Subscription key
    string subscriptionKey;
    # OAuth2 client ID
    string clientId;
    # OAuth2 client secret
    string clientSecret;
    # Subscription secrets
    string secrets;
    json...;
|};

# License details within the license response.
public type License record {|
    # Subscription data
    SubscriptionData subscriptionData;
    # Response signature
    string signature;
|};

# License download response structure.
public type LicenseResult record {|
    # Success flag
    boolean success;
    # License details
    License license;
|};

# License download response structure.
public type LicenseResponse record {|
    # Result object
    LicenseResult result;
    json...;
|};

# Project status response structure.
public type ProjectStatusResponse record {|
    # Application status code
    int status;
    # Choreo application ID
    string applicationId?;
    json...;
|};

# Application Create payload structure.
public type ApplicationCreatePayload record {|
    # Name of the application
    string name;
    # Description of the application
    string description;
|};

# Application Subscription payload structure.
public type ApplicationSubscriptionPayload record {|
    # Application ID
    string applicationId?;
|};

# Application subscription response structure.
public type ApplicationSubscriptionResponse record {|
    # Application ID
    string applicationId;
    # Subscription ID
    string subscriptionId;
    # API ID
    string apiId;
    json...;
|};

# Application key generation response structure.
public type ApplicationKeyGenerationResponse record {|
    # Consumer key
    string consumerKey;
    # Consumer secret
    string consumerSecret;
    json...;
|};
