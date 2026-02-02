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
    json...;
|};

# User data.
public type UserResponse record {|
    # ID
    string id;
    # Email address
    string email;
    # Last name
    string lastName;
    # First name
    string? firstName;
    # Time zone
    string? timeZone;
    json...;
|};

# Project data.
public type Project record {|
    # ID
    string id;
    # Name
    string name;
    # Project key
    string key;
    # Created date and time
    string createdOn;
    # Description
    string? description;
    json...;
|};

# Payload for searching projects.
public type ProjectSearchPayload record {|
    # Pagination details
    Pagination pagination = {};
|};

# Projects response.
public type ProjectsResponse record {|
    # List of projects
    Project[] projects;
    # Total records count
    int totalRecords;
    *Pagination;
    json...;
|};

# Project information.
public type ProjectResponse record {|
    *Project;
    # Project type
    string 'type;
    # Subscription information
    Subscription? subscription;
    json...;
|};

# Project subscription information.
public type Subscription record {|
    # Subscription start date
    string? startDate;
    # Subscription end date
    string? endDate;
    # Support tier
    string? supportTier;
    json...;
|};

# Base case.
public type Case record {|
    # Case ID
    string id;
    # Internal ID of the case
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
    ReferenceTableItem? assignedEngineer;
    # Associated project
    ReferenceTableItem? project;
    # Type of the case
    ReferenceTableItem? 'type;
    # Deployment information
    ReferenceTableItem? deployment;
    # Status information
    ChoiceListItem? state;
    # Severity information
    ChoiceListItem? severity;
    json...;
|};

# Choice list item information.
public type ChoiceListItem record {|
    # ID
    int id;
    # Label
    string label;
    json...;
|};

# Basic table information.
public type ReferenceTableItem record {|
    # ID
    string id;
    # Display name
    string name;
    json...;
|};

# Case search filters.
public type CaseSearchFilters record {|
    # List of project IDs to filter
    string[] projectIds?;
    # List of case types to filter
    string[] caseTypes?;
    # State ID
    int stateId?;
    # Severity ID
    int severityId?;
    # Deployment ID
    string deploymentId?;
|};

# Cases list response with pagination.
public type CaseSearchResponse record {|
    # List of cases
    Case[] cases;
    # Total records count
    int totalRecords;
    *Pagination;
    json...;
|};

# Payload for case search.
public type CaseSearchPayload record {|
    # Filter criteria
    CaseSearchFilters filters?;
    # Sort configuration
    SortBy sortBy?;
    # Pagination details
    Pagination pagination?;
|};

# Case information.
public type CaseResponse record {|
    *Case;
    # Last updated date and time
    string updatedOn;
    # SLA response time
    string slaResponseTime;
    # Product information
    record {
        *ReferenceTableItem;
        # Product version
        string? version;
    }? product;
    # Account information
    record {
        *ReferenceTableItem;
        # Account type
        string? 'type;
    }? account;
    # CS Manager information
    record {
        *ReferenceTableItem;
        # Email address
        string? email;
    }? csManager;
    json...;
|};

# Sort configuration.
public type SortBy record {|
    # Field to sort by
    string 'field;
    # Sort order
    SortOrder 'order;
|};

# Case metadata response.
public type CaseMetadataResponse record {|
    # List of available case states (eg: Open, Closed, etc.)
    ChoiceListItem[] states;
    # List of available case severities (eg: S0, S1, etc.)
    ChoiceListItem[] severities;
    # List of available case types (eg: Incident, Service Request, etc.)
    ReferenceTableItem[] caseTypes;
    json...;
|};

# Project statistics response from ServiceNow.
public type ProjectStatsResponse record {|
    # Total time logged
    decimal totalTimeLogged;
    # Billable hours
    decimal billableHours;
    # System health status
    string systemHealth;
    # SLA status
    string slaStatus;
    json...;
|};

# Active case count breakdown.
public type ActiveCaseCount record {|
    # Work in progress count
    int workInProgress;
    # Waiting on client count
    int waitingOnClient;
    # Waiting on WSO2 count
    int waitingOnWso2;
    # Total active count
    int total;
    json...;
|};

# Outstanding cases count breakdown.
public type OutstandingCasesCount record {|
    # Medium severity count
    int medium;
    # High severity count
    int high;
    # Critical severity count
    int critical;
    # Total count
    int total;
    json...;
|};

# Resolved case count breakdown.
public type ResolvedCaseCount record {|
    # Total resolved count
    int total;
    # Current month resolved count
    int currentMonth;
    json...;
|};

# Project cases statistics response.
public type ProjectCaseStatsResponse record {|
    # Total case count
    int totalCount;
    # Open case count
    int openCount;
    # Average response time
    decimal averageResponseTime;
    # Active case count breakdown
    ActiveCaseCount activeCount;
    # Outstanding cases count breakdown
    OutstandingCasesCount outstandingCasesCount;
    # Resolved case count breakdown
    ResolvedCaseCount resolvedCount;
    json...;
|};

# Project chats statistics response.
public type ProjectChatStatsResponse record {|
    # Active chat count
    int activeCount;
    # Session count
    int sessionCount;
    # Resolved count
    int resolvedCount;
|};

# Project deployment statistics response.
public type ProjectDeploymentStatsResponse record {|
    # Total deployment count
    int totalCount;
    # Last deployment date
    string? lastDeploymentOn;
    json...;
|};

# Comment information.
public type Comment record {|
    # ID
    string id;
    # Reference ID associated with the comment(query ID, incident ID, service request ID, etc.)
    string referenceId;
    # Content of the comment
    string content;
    # Type of the comment
    string 'type;
    # Created date and time
    string createdOn;
    # User who created the comment
    string createdBy;
    # Indicates if the comment is escalated
    boolean isEscalated;
    json...;
|};

# Comments response with pagination.
public type CommentsResponse record {|
    # List of comments
    Comment[] comments;
    # Total records count
    int totalRecords;
    *Pagination;
    json...;
|};

# Reference search payload to search comments, attachments, etc.
public type ReferenceSearchPayload record {|
    # Reference ID to filter related resources(query ID, incident ID, service request ID, etc.)
    string referenceId;
    # Pagination details
    Pagination pagination?;
|};

# Attachment data.
public type Attachment record {|
    # ID of the attachment
    string id;
    # Reference ID associated with the attachment(query ID, incident ID, service request ID, etc.)
    string referenceId;
    # File name
    string name;
    # MIME type of the file
    string 'type;
    # File size in bytes
    string sizeBytes;
    # User who created the attachment
    string createdBy;
    # Created date and time
    string createdOn;
    # Download URL
    string downloadUrl;
    json...;
|};

# Attachments response.
public type AttachmentsResponse record {|
    # List of attachments
    Attachment[] attachments;
    # Total records count
    int totalRecords;
    *Pagination;
|};
