// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

# [Configurable] Token validator configuration.
public type TokenValidatorConfig record {|
    # Issuer
    string issuer;
    # Audience  
    string audience;
    # JWKS EndPoint 
    string jwksEndPoint;
    # Clock skew
    decimal clockSkew;
|};

# JWT payload data structure from decoded token.
type CustomJwtPayload record {|
    # User email
    string email;
    # User ID
    string userid;
    # User groups
    string[] groups?;
    json...;
|};

# User info custom payload.
public type UserInfoPayload record {|
    # User email
    string email;
    # User ID
    string userId;
    # ID token
    string idToken;
    # User groups
    string[] groups?;
    json...;
|};

# Application specific role mapping.
public type AppRoles record {|
    # Role for an Admin
    string adminRole;
|};
