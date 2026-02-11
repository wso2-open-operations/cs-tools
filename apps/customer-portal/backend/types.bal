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

import ballerina/constraint;

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
    # Deployed product information
    ReferenceItem? deployedProduct;
    # issueType of the case
    ReferenceItem? issueType;
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
    *entity:UserResponse;
    # Phone number of the user
    string phoneNumber?;
|};

# Payload for updating user information.
public type UserUpdatePayload record {|
    # Phone number of the user
    @constraint:String {
        pattern: {
            value: re `${PHONE_PATTERN_STRING}`,
            message: "Invalid phone number format. It should be in E.164 format."
        }
    }
    string phoneNumber?;
    # Timezone of the user
    string timeZone?;
|};

# Updated user information.
public type UpdatedUser record {|
    # Phone number of the user
    string phoneNumber?;
    # Timezone of the user
    string timeZone?;
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

# Case statistics for a project.
public type ProjectCaseStats record {|
    # Total case count
    int totalCases;
    # Open cases count
    int openCases;
    # Average response time
    decimal averageResponseTime;
    # Active case count breakdown
    entity:ActiveCaseCount activeCases;
    # Outstanding cases count breakdown
    entity:OutstandingCasesCount outstandingCases;
    # Resolved case count breakdown
    entity:ResolvedCaseCount resolvedCases;
|};

# Project support statistics.
public type ProjectSupportStats record {|
    # Total cases count
    int totalCases;
    # Active chats count
    int activeChats;
    # Session chats count
    int sessionChats;
    # Resolved chats count
    int resolvedChats;
|};

# Project statistics.
public type ProjectStats record {|
    # Open cases count
    int openCases;
    # Active chats count
    int activeChats;
    # Deployments count
    int deployments;
    # SLA status
    string slaStatus;
|};

# Recent activity details.
public type RecentActivity record {|
    # Total time logged
    decimal totalTimeLogged;
    # Billable hours
    decimal billableHours;
    # Last deployment date
    string? lastDeploymentOn;
    # System health status
    string systemHealth;
|};

# Project statistics response.
public type ProjectStatsResponse record {|
    # Project statistics
    ProjectStats projectStats;
    # Recent activity details
    RecentActivity recentActivity;
|};

# Comment information.
public type Comment record {|
    # ID
    string id;
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
|};

# Comments response with pagination.
public type CommentsResponse record {|
    # List of comments
    Comment[] comments;
    # Total records count
    int totalRecords;
    *entity:Pagination;
|};

# Attachment data.
public type Attachment record {|
    # ID of the attachment
    string id;
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
|};

# Attachments response.
public type AttachmentsResponse record {|
    # List of attachments
    Attachment[] attachments;
    # Total records count
    int totalRecords;
    *entity:Pagination;
|};

# Deployment information.
public type Deployment record {|
    # ID
    string id;
    # Name
    string name;
    # Created date and time
    string createdOn;
    # Updated date and time
    string updatedOn;
    # Description
    string? description;
    # URL
    string? url;
    # Associated project
    ReferenceItem? project;
    # Type
    ReferenceItem? 'type;
|};

# Deployed product data.
public type DeployedProduct record {|
    # ID
    string id;
    # Created date and time
    string createdOn;
    # Updated date and time
    string updatedOn;
    # Description
    string? description;
    # Associated product
    ReferenceItem? product;
    # Deployment
    ReferenceItem? deployment;
|};
