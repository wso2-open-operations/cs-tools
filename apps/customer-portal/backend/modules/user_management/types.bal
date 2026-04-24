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
    string[] scopes = [];
|};

# Contact details record.
public type Contact record {|
    # ID
    string id;
    # Email
    string email;
    # First name
    string firstName;
    # Last name
    string lastName;
    # Whether the contact is a customer admin or not
    boolean isCsAdmin;
    # Whether the contact is an Integration user or not
    boolean isCsIntegrationUser;
    # Whether the contact is a portal user or not
    boolean? isPortalUser?;
    # Whether the contact is a Security contact or not
    boolean? isSecurityContact?;
    # Status of the Membership (Invited, Registered)
    string? membershipStatus?;
    # Account information
    Account? account?;
    json...;
|};

# Contact details returned by the user management service.
type UserManagementContact record {|
    *Contact;
    # Role of the Membership
    string? role?;
|};

# Account details record
public type Account record {|
    # ID
    string? id;
    # Allowed domain list of the account
    string? domainList = "";
    # Account classification
    string? classification = "";
    # Whether the account is a partner or not
    boolean? isPartner?;
    json...;
|};

# The request payload to be validated.
public type OnBoardContactPayload record {|
    # Email address of the Contact
    string contactEmail;
    # Email address of the Admin
    string adminEmail;
    # First name of the Contact
    string contactFirstName;
    # Last name of the Contact
    string contactLastName;
    # Whether the contact is System User or not
    boolean isCsIntegrationUser;
    # Whether the contact is a customer admin or not
    boolean isCsAdmin = false;
    # Whether the contact is a portal user or not
    boolean isPortalUser = false;
    # Whether the contact is Security Contact or not
    boolean isSecurityContact = false;
|};

# Contact onboard payload sent to the user management service.
type UserManagementOnBoardContactPayload record {|
    # Email address of the Contact
    string contactEmail;
    # Email address of the Admin
    string adminEmail;
    # First name of the Contact
    string contactFirstName;
    # Last name of the Contact
    string contactLastName;
    # Whether the contact is System User or not
    boolean isCsIntegrationUser;
    # Role list of the contact in the project
    string[]? role;
|};

# Membership details record.
public type Membership record {|
    # ID
    string id;
    # State of the Membership
    string state;
    # Whether the contact is a portal user or not
    boolean? isPortalUser;
    # Whether the contact is a Security contact or not
    boolean? isSecurityContact;
    # Contact details
    ContactMinimal contact?;
    json...;
|};

# Membership details returned by the user management service.
type UserManagementMembership record {|
    *Membership;
    # Role of the Membership
    string? role;
|};

# ContactMinimal Details.
public type ContactMinimal record {|
    # ID
    string? id;
    # Email of the contact
    string? email;
    json...;
|};

# Payload for updating membership roles.
public type MembershipRolePayload record {|
    # Admin email
    string adminEmail;
    # Whether the contact is a customer admin or not
    boolean isCsAdmin = false;
    # Whether the contact is a portal user or not
    boolean isPortalUser = false;
    # Whether the contact is a security contact or not
    boolean isSecurityContact = false;
|};

# Payload for updating membership roles in the user management service.
type UserManagementMembershipRolePayload record {|
    # Admin email
    string adminEmail;
    # Role list to replace the current membership roles
    string[] role;
|};

# The request payload to be validated.
public type ValidationPayload record {|
    # Salesforce ID of the Project
    string projectId;
    # Contact email
    string contactEmail;
    # Admin email
    string adminEmail;
|};
