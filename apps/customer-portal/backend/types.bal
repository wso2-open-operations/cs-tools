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
import customer_portal.entity;

# Cache configuration record.
public type CacheConfig record {|
    # Maximum number of entries in cache
    int capacity = 500;
    # Default maximum age of cache entries in seconds
    decimal defaultMaxAge = 3600;
    # Eviction factor for cache cleanup
    float evictionFactor = 0.2;
    # Cleanup interval in seconds
    decimal cleanupInterval = 1800.0;
|};

# Case search filters.
public type CaseSearchFilters record {|
    # List of case types to filter
    string[] caseTypes?;
    # Status ID
    int statusId?;
    # Severity ID
    int severityId?;
    # Deployment ID
    string deploymentId?;
|};

# Payload for case search.
public type CaseSearchPayload record {|
    # Filter criteria
    CaseSearchFilters filters?;
    # Sort configuration
    entity:SortBy sortBy?;
    # Pagination details
    entity:Pagination pagination?;
|};

# Base case.
public type Case record {|
    # Case ID
    string id;
    # Internal case ID
    string internalId;
    # Case number
    string number;
    # Created date and time
    string createdOn;
    # Case title
    string? title;
    # Case description
    string? description;
    # Assigned engineer
    ReferenceItem? assignedEngineer;
    # Associated project
    ReferenceItem? project;
    # Type of the case
    ReferenceItem? 'type;
    # Deployment
    ReferenceItem? deployment;
    # Severity of the case
    ReferenceItem? severity;
    # State of the case
    ReferenceItem? status;
|};

# Reference item.
public type ReferenceItem record {|
    # ID
    string id;
    # Label
    string label;
|};

# Cases list response with pagination.
public type CaseSearchResponse record {|
    # List of cases
    Case[] cases;
    # Total records count
    int totalRecords;
    *entity:Pagination;
|};

# User.
public type User record {|
    # ID of the user
    string id;
    # Email address of the user
    string email;
    # First name of the user
    string firstName;
    # Last name of the user
    string lastName;
    # Phone number of the user
    string phoneNumber?;
|};

# Case filter options.
public type CaseFilterOptions record {|
    # List of case statuses
    ReferenceItem[] statuses;
    # List of case severities
    ReferenceItem[] severities;
    # List of case types
    ReferenceItem[] caseTypes;
    // TODO: Add other filters once implemented
|};
