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

# Entity user data structure from the entity service.
public type EntityUser record {|
    # System ID of the user
    string sys_id;
    # Email address of the user
    string email;
    # Full name of the user
    string name;
    # First name of the user
    string first_name;
    # Last name of the user
    string last_name;
    # Username of the user
    string user_name;
    # Active status of the user
    string active;
|};

# Response structure from Entity Service.
public type EntityUserResponse record {|
    # Result object containing the response data
    record {|
        # Success status of the operation
        boolean success;
        # User data from the entity service
        EntityUser data;
        # Response message
        string message;
    |} result;
|};

# User info response structure.
public type UserInfo record {|
    # Unique identifier of the user
    string userId;
    # Email address of the user
    string userEmail;
    # First name of the user
    string firstName;
    # Last name of the user
    string lastName;
    # Username of the user
    string username;
    # Active status of the user
    boolean active;
|};

# Projects data structure from the entity service.
public type EntityProject record {|
    # Name of the project
    string name;
    # System ID of the project
    string sysId;
|};

# Entity projects response structure.
public type EntityProjectsResponse record {|
    # Result object containing the response data
    record {|
        # Success status of the operation
        boolean success;
        # Array of project data from the entity service
        EntityProject[] data;
    |} result;
|};

# Base record containing common fields for case data structures.
public type EntityBaseCase record {|
    # Case number
    string number;
    # WSO2 case identifier
    string wso2_case_id;
    # Short description of the case
    string short_description;
    # Detailed description of the case
    string description;
    # Type of the case
    string case_type;
    # Project deployment information
    string project_deployment;
    # Product associated with the case
    string product;
    # Priority level of the case
    string priority;
    # Current state of the case
    string state;
    # Contact person for the case
    string contact;
    # Last updated timestamp
    string updated_on;
    # Case opened timestamp
    string opened_on;
    # Case resolved timestamp
    string resolved_on;
    # Project information
    record {|
        # System ID of the project
        string sys_id;
        # Name of the project
        string name;
    |} project;
|};

# Entity case data structure from the entity service.
public type EntityCase record {|
    *EntityBaseCase;
    # System ID of the case
    string case_sys_id;
|};

# Pagination information structure.
public type EntityPagination record {|
    # Total number of records
    decimal total;
    # Offset for pagination
    decimal offset;
    # Limit for pagination
    decimal 'limit;
|};

# Filter options structure.
public type EntityFilters record {|
    # State filter options
    string[]? state;
    # Contact filter options
    string[]? contact;
|};

# Entity cases structure from the service.
public type EntityCases record {|
    # List of cases
    EntityCase[] cases;
    # Pagination information
    EntityPagination pagination;
    # Filter information
    EntityFilters filters;
|};

# Entity cases response structure.
public type EntityCasesResponse record {|
    # Result object containing the response data
    record {|
        # Success status of the operation
        boolean success;
        # Data object containing cases and metadata
        EntityCases data;
    |} result;
|};

# Base record containing common case fields.
public type BaseCase record {|
    # Case number
    string number;
    # WSO2 case identifier
    string wso2CaseId;
    # Short description of the case
    string shortDescription;
    # Type of the case
    string caseType;
    # Product associated with the case
    string product;
    # Priority level of the case
    string priority;
    # Current state of the case
    string state;
    # Contact person for the case
    string contact;
    # Last updated timestamp
    string updatedOn;
    # Case opened timestamp
    string openedOn;
    # Case resolved timestamp
    string resolvedOn;
|};

# Case info response structure.
public type CaseInfo record {|
    *BaseCase;
    # System ID of the case
    string caseSysId;
|};

# Cases response structure.
public type Cases record {|
    # List of case information
    CaseInfo[] cases;
    # Pagination information
    record {|
        # Total number of records
        int total;
        # Offset for pagination
        int offset;
        # Limit for pagination
        int 'limit;
    |} pagination;
|};

# Entity case details data structure from the service.
public type EntityCaseDetails record {|
    *EntityBaseCase;
    # System ID of the case
    string sys_id;
|};

# Entity case details response structure.
public type EntityCaseDetailsResponse record {|
    # Result object containing the response data
    record {|
        # Success status of the operation
        boolean success;
        # Case details data from the entity service
        EntityCaseDetails data;
    |} result;
|};

# Case details response structure.
public type CaseDetails record {|
    *BaseCase;
    # Detailed description of the case
    string description;
    # Project deployment information
    string projectDeployment;
    # Project name
    string projectName;
|};
