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
import ballerina/constraint;

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

# Contact search payload.
public type ContactSearchPayload record {|
    # Email
    @constraint:String {
        pattern: EMAIL_CONSTRAINT
    }
    string? email = ();
    # Limit
    int? 'limit = SALES_DEFAULT_RECORD_LIMIT;
    # Offset
    int? offset = SALES_DEFAULT_RECORD_OFFSET;
|};

# [Entity] Account
public type Account record {|
    # Account ID (Salesforce)
    string? id;
    json...;
|};

# [Entity] Contact
public type Contact record {|
    # CRM id
    string? id;
    # Email of the customer
    string? email;
    # Account (Salesforce) related to the contact
    Account? account = ();
    json...;
|};

# Common ID string type with length constraint.
@constraint:String {
    pattern: re `^[0-9A-Fa-f]{32}$`
}
public type IdString string;

# Date string type with YYYY-MM-DD format constraint.
@constraint:String {
    pattern: re `^\d{4}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])$`
}
public type Date string;

# DateTime string type with YYYY-MM-DD HH:MM:SS format constraint.
@constraint:String {
    pattern: re `^\d{4}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01]) ([01]\d|2[0-3]):[0-5]\d:[0-5]\d$`
}
public type DateTime string;

# Pagination information.
public type Pagination record {|
    # Offset for pagination
    @constraint:Int {
        minValue: {
            value: 0,
            message: "Offset must be a non-negative integer (>= 0)."
        }
    }
    int offset = CS_DEFAULT_OFFSET;
    # Limit for pagination
    @constraint:Int {
        minValue: {
            value: 1,
            message: "Limit must be at least 1."
        },
        maxValue: {
            value: 50,
            message: "Limit cannot exceed 50."
        }
    }
    int 'limit = CS_DEFAULT_LIMIT;
|};

# Choice list item information.
public type ChoiceListItem record {|
    # Choice list item value
    int|string id;
    # Choice list item label
    string label;
    # Count value
    int count?;
    json...;
|};

# Basic table information.
public type ReferenceTableItem record {|
    # System ID
    IdString id;
    # Display name
    string name;
    # number
    string? number?;
    # Count value
    int count?;
    # Abbreviation
    string? abbreviation?;
    json...;
|};

# Request payload for searching deployments.
public type DeploymentSearchPayload record {|
    # Filter criteria
    record {|
        # List of project IDs to filter
        IdString[] projectIds?;
        # List of Salesforce IDs to filter
        string[] sfIds?;
    |} filters?;
    # Pagination details
    Pagination pagination?;
|};

# Deployment data from CS Entity Service.
public type Deployment record {|
    # System ID of the deployment
    IdString id;
    # Name of the deployment
    string name;
    # Created date and time
    string createdOn;
    # Updated date and time
    string updatedOn;
    # Description of the deployment
    string? description;
    # URL of the deployment
    string? url;
    # Associated project
    ReferenceTableItem project;
    # Type of the deployment
    ChoiceListItem 'type;
    json...;
|};

# Deployments response from CS Entity Service.
public type DeploymentsResponse record {|
    # List of deployments
    Deployment[] deployments;
    # Total records count
    int totalRecords;
    *Pagination;
|};

# Request payload for searching deployed products.
public type DeployedProductSearchPayload record {|
    # Filter criteria
    record {|
        # List of project IDs to filter
        IdString[] projectIds?;
        # List of Salesforce IDs to filter
        string[] sfIds?;
        # List of deployment IDs to filter
        IdString[] deploymentIds?;
    |} filters?;
    # Pagination details
    Pagination pagination?;
|};

# Product update information.
public type ProductUpdate record {|
    # Update level
    int updateLevel;
    # Update date
    Date date;
    # Update details
    string? details?;
    json...;
|};

# Product data from CS Entity Service.
public type DeployedProduct record {|
    # System ID of the product
    IdString id;
    # Description of the product
    string? description;
    # Cores allocated for the product
    int? cores;
    # TPS allocated for the product
    decimal? tps;
    # Release date of the product
    string? releasedOn;
    # End of life date of the product
    string? endOfLifeOn;
    # Product updates
    ProductUpdate[]? updates;
    # Created date and time
    string createdOn;
    # Updated date and time
    string updatedOn;
    # Category of the product
    ReferenceTableItem? category;
    # Associated deployment
    ReferenceTableItem? deployment;
    # Product information
    ReferenceTableItem? product;
    # Product version
    ReferenceTableItem? version;
    json...;
|};

# Deployed products response from CS Entity Service.
public type DeployedProductsResponse record {|
    # List of deployed products
    DeployedProduct[] deployedProducts;
    # Total records count
    int totalRecords;
    *Pagination;
|};
