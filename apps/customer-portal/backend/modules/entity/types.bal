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

# Case filters.
public type CaseFiltersResponse record {|
    # Categories filter options
    string[] categories;
    # Products filter options
    string[] products;
    # Statuses filter options
    string[] statuses;
    # Severities filter options
    string[] severities;
    # Environments filter options
    string[] environments;
    json...;
|};

# Incident count by severity.
public type IncidentCount record {|
    # Severity 0
    int s0;
    # Severity 1
    int s1;
    # Severity 2
    int s2;
    # Severity 3
    int s3;
    # Severity 4
    int s4;
|};

# Active case count by status.
public type ActiveCaseCount record {|
    # Awaiting count
    int awaitingCount;
    # Waiting on WSO2 count
    int waitingOnWSO2Count;
    # Work in progress count
    int workInProgressCount;
|};

# Project statistics.
public type ProjectStatistics record {|
    # Open cases count
    int openCasesCount;
    # Active chats count
    int activeChatsCount;
    # Deployments count
    int deploymentsCount;
    # Total cases count
    int totalCasesCount;
    # In progress cases count
    int inProgressCasesCount;
    # Awaiting cases count
    int awaitingCasesCount;
    # Resolved cases count
    int resolvedCasesCount;
    # Current month resolved cases count
    int currentMonthResolvedCasesCount;
    # Average response time
    string avgResponseTime;
    # Incident count by severity
    IncidentCount incidentCount;
    # Active case count by status
    ActiveCaseCount activeCaseCount;
|};

# Recent activity.
public type RecentActivity record {|
    # Total time logged
    int totalTimeLogged;
    # Billable hours
    int billableHours;
    # Last deployment timestamp
    string lastDeploymentOn;
    # System health status
    string systemHealth;
|};

# Project overview response.
public type ProjectOverviewResponse record {|
    # Project statistics
    ProjectStatistics projectStatistics;
    # Recent activity
    RecentActivity recentActivity;
|};

# Comment author information.
public type CommentAuthor record {|
    # Name of the author
    string name;
    # Role of the author
    string role;
|};

# Case comment.
public type CaseComment record {|
    # Comment ID
    string id;
    # Author information
    CommentAuthor author;
    # Comment content
    string content;
    # Timestamp of the comment
    string timestamp;
    # Whether the comment is large
    boolean isLarge;
    # Whether the comment contains code
    boolean hasCode;
|};

# Case attachment.
public type CaseAttachment record {|
    # System ID of the attachment
    string sysId;
    # Name of the attachment file
    string name;
    # Size in bytes
    int sizeBytes;
    # Uploaded by user email
    string uploadedBy;
    # Upload date
    string uploadedDate;
|};

# Call request.
public type CallRequest record {|
    # System ID of the call request
    string sysId;
    # Call request number
    string number;
    # Requested date
    string requestedDate;
    # State of the call request
    string state;
    # Reason for the call
    string reason;
    # Duration of the call
    string duration;
    # Preferred time for the call
    string preferredTime;
|};

# Knowledge base article.
public type KbArticle record {|
    # Article ID
    string id;
    # Article title
    string title;
    # Article summary
    string summary;
    # Article category
    string category;
    # Suggested by (AI or user)
    string suggestedBy;
    # Date when suggested
    string suggestedDate;
    # Number of views
    int views;
|};

# Base case.
public type Case record {|
    # System ID of the case
    string sysId;
    # Case number
    string number;
    # Case title
    string title;
    # Case description
    string description;
    # Case status
    string status;
    # Case severity
    string severity;
    # Created date and time
    string createdOn;
    # Updated date and time
    string updatedOn;
    # Assigned engineer name
    string assignedEngineer;
    # Case category
    string category;
    # Product information
    string product;
|};

# Detailed case response.
public type CaseDetailsResponse record {|
    *Case;
    # Environment type
    string environment;
    # Account type
    string accountType;
    # Organization name
    string organization;
    # SLA information
    string sla;
    # Initial comments list
    CaseComment[] initialComments;
    # Attachments list
    CaseAttachment[] attachments;
    # Call requests list
    CallRequest[] callRequests;
    # Knowledge base articles list
    KbArticle[] kbArticles;
|};

# Cases list response with pagination.
public type CasesResponse record {|
    # List of cases
    Case[] cases;
    # Pagination details
    Pagination pagination;
|};

# Request body for fetching cases with filters.
public type CaseFiltersRequest record {|
    # Pagination offset
    int offset;
    # Pagination limit
    int 'limit;
    # Optional contact name
    string contact?;
    # Optional case status
    string status?;
    # Optional severity level
    string severity?;
    # Optional product name
    string product?;
    # Optional category
    string category?;
|};
