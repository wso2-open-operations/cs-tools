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
import ballerina/constraint;

# Valid sort order values.
public enum SortOrder {
    ASC = "asc",
    DESC = "desc"
}

# Pagination information.
public type Pagination record {|
    # Offset for pagination
    @constraint:Int {
        minValue: 0
    }
    int offset = DEFAULT_OFFSET;
    # Limit for pagination
    @constraint:Int {
        minValue: 1,
        maxValue: 50
    }
    int 'limit = DEFAULT_LIMIT;
|};

# User data.
public type UserResponse record {|
    # System ID of the user
    string sysId;
    # Email address of the user
    string email;
    # First name of the user
    string firstName;
    # Last name of the user
    string lastName;
|};

# Project data from ServiceNow.
public type Project record {|
    # System ID of the project
    string sysId;
    # Name of the project
    string name;
    # Description of the project
    string? description;
    # Project key
    string projectKey;
    # Created date and time
    string createdOn;
|};

# Request body for searching projects.
public type ProjectRequest record {|
    # Pagination details
    Pagination pagination = {};
|};

# Projects response from ServiceNow.
public type ProjectsResponse record {|
    # List of projects
    Project[] projects;
    # Total records count
    int totalRecords;
    *Pagination;
|};

# Account owner information.
public type AccountOwner record {|
    # System ID of the user
    string sysId;
    # Name of the user
    string name;
    # Email of the user
    string email;
|};

# Project information.
public type ProjectDetailsResponse record {|
    *Project;
    # Project type
    string projectType;
    # Subscription start date
    string? subscriptionStart;
    # Subscription end date
    string? subscriptionEnd;
    # Support tier
    string? supportTier;
    # SLA status
    string slaStatus;
    # Account owner information
    AccountOwner accountOwner;
    # Technical owner information
    AccountOwner? technicalOwner;
    json...;
|};

# Base case.
public type Case record {|
    # Case ID
    string caseId;
    # System ID of the project
    string projectId;
    # Case number
    string number;
    # State code
    int state;
    # State label
    string stateLabel;
    # Created date and time
    string createdOn;
    # Assigned engineer name
    string? assignedEngineer;
    # Case title
    string? title;
    # Case description
    string? description;
    # Type of the case
    string? caseType;
    # Severity code
    int? severity;
    # Severity label
    string? severityLabel;
    # Deployment name
    string? deployment;
    # Deployment ID
    string? deploymentId;
|};

# Case search filters.
public type CaseSearchFilters record {|
    # State ID
    int state?;
    # Severity ID
    int severity?;
    # Deployment ID
    string deployment?;
|};

# Cases list response with pagination.
public type CasesResponse record {|
    # List of cases
    Case[] cases;
    # Total records count
    int totalRecords;
    *Pagination;
|};

# Payload for case search.
public type CaseSearchPayload record {|
    # List of case types to filter
    string[] caseTypes?;
    # Filter criteria
    CaseSearchFilters filters?;
    # Sort configuration
    SortBy sortBy?;
    # Pagination details
    Pagination pagination?;
|};

# Request payload for searching cases for entity.
public type CaseRequestPayload record {|
    # List of project IDs to filter
    string[] projectIds?;
    *CaseSearchPayload;
|};

# Sort configuration.
public type SortBy record {|
    # Field to sort by
    string 'field;
    # Sort order
    SortOrder 'order;
|};
