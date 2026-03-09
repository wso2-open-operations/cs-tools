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

# Common ID string type with length constraint.
@constraint:String {
    pattern: re `^[A-Fa-f0-9]{32}$`
}
public type IdString string;

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
    IdString id;
    # Email address
    string email;
    # Last name
    string lastName;
    # First name
    string? firstName;
    # Time zone
    string? timeZone;
    # User roles
    string[] roles = [];
    json...;
|};

# Request payload for updating user.
public type UserUpdatePayload record {|
    # Time zone to update for the user
    string timeZone;
|};

# Response payload for user update.
public type UserUpdateResponse record {|
    # Success message
    string message;
    # Updated user details
    UpdatedUser user;
|};

# Updated user details.
public type UpdatedUser record {|
    # ID of the user
    IdString id;
    # User who performed the update
    string updatedBy;
    # Updated date and time
    DateTimeWithoutTimezone updatedOn;
    json...;
|};

# Project data.
public type Project record {|
    # ID
    IdString id;
    # Name
    string name;
    # Project key
    string key;
    # Created date and time
    string createdOn;
    # Description
    string? description;
    # Project type
    ReferenceTableItem 'type;
    json...;
|};

# Payload for searching projects.
public type ProjectSearchPayload record {|
    # Filter criteria
    record {
        # Search query for projects
        string searchQuery?;
    } filters?;
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
    # Salesforce ID
    string sfId;
    # Indicates if the project has service requests
    boolean hasSr;
    # Project start date
    Date? startDate;
    # Project end date 
    Date? endDate;
    # Account information
    Account account;
    json...;
|};

# Account Information.
public type Account record {|
    # ID of the account
    IdString id;
    # Indicates whether the agent is enabled for the account
    boolean hasAgent;
    # Name of the account
    string? name;
    # Activation date
    string? activationDate;
    # Deactivation date
    string? deactivationDate;
    # Support tier
    string? supportTier;
    # Region
    string? region;
    # Owner email
    string? ownerEmail;
    # Technical owner email
    string? technicalOwnerEmail;
    json...;
|};

# Payload for updating a project.
public type ProjectUpdatePayload record {|
    # Indicates whether the agent is enabled for the project
    boolean hasAgent;
|};

# Response from updating a project.
public type ProjectUpdateResponse record {|
    # Success message
    string message;
    # Updated project metadata
    UpdatedProject project;
    json...;
|};

# Updated project details.
public type UpdatedProject record {|
    # ID of the project
    IdString id;
    # User who updated the project
    string updatedBy;
    # Updated date and time
    DateTimeWithoutTimezone updatedOn;
    json...;
|};

# Payload for creating a case.
public type CaseCreatePayload record {|
    # Case type
    CaseType 'type;
    # Project ID
    IdString projectId;
    # Deployment ID
    IdString deploymentId;
    # Deployed product ID
    IdString deployedProductId?;
    # Case title (required for DEFAULT_CASE and SECURITY_REPORT_ANALYSIS)
    string title?;
    # Case description (required for DEFAULT_CASE and SECURITY_REPORT_ANALYSIS)
    string description?;
    # Issue type ID (required for DEFAULT_CASE)
    int issueTypeKey?;
    # Severity key (required for DEFAULT_CASE)
    int severityKey?;
    # Related case ID (if the case is related to an existing case)
    IdString parentCaseId?;
    # Conversation ID (if the case is related to a conversation)
    IdString conversationId?;
    # Catalog ID (required for SERVICE_REQUEST)
    IdString catalogId?;
    # Catalog Item ID (required for SERVICE_REQUEST)
    IdString catalogItemId?;
    # Variables for service request (required for SERVICE_REQUEST)
    Variable[] variables?;
    # List of attachments
    CaseCreateAttachment[] attachments?;
|};

# Attachment for creating a case.
public type CaseCreateAttachment record {|
    # File name
    string name;
    # Base64 encoded file content
    string file;
|};

# Response from creating a case.
public type CaseCreateResponse record {|
    # Success message
    string message;
    # Created case details
    CreatedCase case;
|};

# Created case details.
public type CreatedCase record {|
    # ID of the created case
    IdString id;
    # WSO2 internal ID of the case
    string internalId;
    # Case number
    string number;
    # User who created the case
    string createdBy;
    # Created date and time
    string createdOn;
    # Status
    ChoiceListItem state;
    # Case type information (eg: incident, query, etc.)
    ReferenceTableItem 'type;
    json...;
|};

# Request payload for updating a case.
public type CaseUpdatePayload record {|
    # State key to update
    int stateKey;
|};

# Response from updating a case.
public type CaseUpdateResponse record {|
    # Success message
    string message;
    # Updated case details
    UpdatedCase case;
|};

# Updated case details.
public type UpdatedCase record {|
    # ID of the updated case
    IdString id;
    # Updated date and time
    string updatedOn;
    # User who updated the case
    string updatedBy;
    # Updated state information
    ChoiceListItem state;
    # Case type information
    ReferenceTableItem 'type;
    json...;
|};

# Base case.
public type Case record {|
    # Case ID
    IdString id;
    # Internal ID of the case
    string internalId;
    # Case number
    string number;
    # Created date and time
    string createdOn;
    # Created by (User email)
    string createdBy;
    # Case title
    string? title;
    # Case description
    string? description;
    # issue type of the case
    ChoiceListItem? issueType;
    # Status information
    ChoiceListItem? state;
    # Severity information
    ChoiceListItem? severity;
    # Catalog information (if the case is a service request)
    ReferenceTableItem? catalog;
    # Catalog item information (if the case is a service request)
    ReferenceTableItem? catalogItem;
    # Assigned team
    ReferenceTableItem? assignedTeam;
    json...;
|};

# Choice list item information.
public type ChoiceListItem record {|
    # Choice list item value
    int|string id;
    # Choice list item label
    string label;
    # Count
    int count?;
    json...;
|};

# Basic table information.
public type ReferenceTableItem record {|
    # ID
    string id;
    # Display name
    string name;
    # Number
    string? number?;
    # Count value
    int count?;
    json...;
|};

# Case search filters.
public type CaseSearchFilters record {|
    # List of project IDs to filter
    string[] projectIds?;
    # List of case types to filter
    CaseType[] caseTypes?;
    # Search query for case number, title and description
    string searchQuery?;
    # List of issue types to filter
    int[] issueTypeKeys?;
    # State key
    int[] stateKeys?;
    # Severity key
    int severityKey?;
    # Deployment ID
    string deploymentId?;
    # Case created by the logged in user
    boolean createdByMe?;
|};

# Case metadata information.
public type CaseMetaData record {|
    *Case;
    # Associated project
    ReferenceTableItem? project;
    # Case type information (eg: incident, service request, etc.)
    ReferenceTableItem? caseType;
    # Deployment information
    ReferenceTableItem? deployment;
    # Deployed product information
    ReferenceTableItem? deployedProduct;
    # Assigned engineer
    ReferenceTableItem? assignedEngineer;
    # Related case information (if the case is related to an existing case)
    ReferenceTableItem? parentCase;
    # Conversation information (if the case is related to a conversation)
    ReferenceTableItem? conversation;
|};

# Cases list response with pagination.
public type CaseSearchResponse record {|
    # List of cases with associated information
    CaseMetaData[] cases;
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

# Case response.
public type CaseResponse record {|
    *Case;
    # Last updated date and time
    string updatedOn;
    # SLA response time
    string slaResponseTime;
    # Associated project
    ReferenceTableItem? project;
    # Case type information (eg: incident, service request, etc.)
    ReferenceTableItem? caseType;
    # Assigned engineer
    ReferenceTableItem? assignedEngineer;
    # Related case information (if the case is related to an existing case)
    ReferenceTableItem? parentCase;
    # Conversation information (if the case is related to a conversation)
    ReferenceTableItem? conversation;
    # Deployment information
    record {
        *ReferenceTableItem;
        # Deployment type
        string? 'type;
    }? deployment;
    # Deployed product information
    record {
        *ReferenceTableItem;
        # Deployed product version
        string? version;
    }? deployedProduct;
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
    # Case closed date and time
    string? closedOn?;
    # User who closed the case
    ReferenceTableItem? closedBy?;
    # Close notes for the case closure
    string? closeNotes?;
    # Indicates if the case is auto closed
    boolean? hasAutoClosed?;
    # Associated change requests (only for service requests)
    ReferenceTableItem[]? changeRequests?;
    # Variables for service request
    ServiceRequestVariable[]? variables?;
    json...;
|};

# Service request variables.
public type ServiceRequestVariable record {|
    # Variable name
    string name;
    # Variable value
    string value;
|};

# Sort configuration.
public type SortBy record {|
    # Field to sort by
    CaseSortField 'field;
    # Sort order
    SortOrder 'order;
    json...;
|};

# Project metadata response.
public type ProjectMetadataResponse record {|
    # List of available case states (eg: Open, Closed, etc.)
    ChoiceListItem[] caseStates;
    # List of available time zones (eg: UTC, GMT, etc.)
    ChoiceListItem[] timeZones;
    # List of available case severities (eg: S0, S1, etc.)
    ChoiceListItem[] severities;
    # List of available issue types (eg: Error, Total Outage, etc.)
    ChoiceListItem[] issueTypes;
    # List of available deployment types (eg: Development, QA, etc.)
    ChoiceListItem[] deploymentTypes;
    # List of available call request states
    ChoiceListItem[] callRequestStates;
    # List of available change request states
    ChoiceListItem[] changeRequestStates;
    # List of available time card states
    ChoiceListItem[] timeCardStates;
    # List of available change request impacts
    ChoiceListItem[] changeRequestImpacts;
    # List of available conversation states
    ChoiceListItem[] conversationStates;
    # List of available case types
    ReferenceTableItem[] caseTypes;
    # Severity based allocation time mapping (severity ID to allocation time in minutes)
    map<int> severityBasedAllocationTime;
    json...;
|};

# Project statistics response.
public type ProjectStatsResponse record {|
    # Total hours logged
    decimal totalHours?;
    # Billable hours
    decimal billableHours?;
    # SLA status
    string slaStatus?;
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

# Cases trend by time unit.
public type CasesTrend record {|
    # Time unit identifier (e.g., "2025 - Q1", "2025 - M1")
    string period;
    # Severity breakdown for the time unit
    ChoiceListItem[] severities;
    json...;
|};

# Project cases statistics response.
public type ProjectCaseStatsResponse record {|
    # Total case count
    int totalCount;
    # Average response time
    decimal averageResponseTime;
    # Resolved case count breakdown
    record {|
        # Total resolved count
        int total;
        # Current month resolved count
        int currentMonth;
        # Past thirty days resolved count
        int pastThirtyDays;
        json...;
    |} resolvedCount;
    # Change rate of engagements past thirty days breakdown
    record {|
        # Change rate of resolved engagements
        decimal resolvedEngagements;
        # Change rate of average response time
        decimal averageResponseTime;
        json...;
    |} changeRate;
    # Count of cases by state
    ChoiceListItem[] stateCount;
    # Count of cases by severity
    ChoiceListItem[] severityCount;
    # Outstanding cases count by severity
    ChoiceListItem[] outstandingSeverityCount;
    # Count of cases by type
    ReferenceTableItem[] caseTypeCount;
    # Cases trend
    CasesTrend[] casesTrend;
    json...;
|};

# Project conversation statistics response.
public type ProjectConversationStatsResponse record {|
    # Count of conversations by state
    ChoiceListItem[] stateCount;
    json...;
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
    IdString id;
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
    # Indicates if the comment has inline attachments
    boolean hasInlineAttachments;
    # List of inline attachments
    InlineAttachment[] inlineAttachments;
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
    # Reference type
    ReferenceType referenceType;
    # Pagination details
    Pagination pagination?;
    # Filter criteria
    record {|
        # Type filter criteria (e.g., "comments", "work_note")
        string 'type?;
    |} filters?;
|};

# Attachment data.
public type Attachment record {|
    # ID of the attachment
    IdString id;
    # Reference ID associated with the attachment(query ID, incident ID, service request ID, etc.)
    string referenceId;
    # File name
    string name;
    # MIME type of the file
    string 'type;
    # File size
    int sizeBytes;
    # User who created the attachment
    string createdBy;
    # Created date and time
    string createdOn;
    # Download URL
    string downloadUrl;
    # Base64 encoded file content (data URI format: data:@file/<type>;base64,<content>)
    string content;
    # Description of the attachment
    string? description;
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

# Request payload for updating an attachment.
public type AttachmentUpdatePayload record {|
    # Reference ID (case or deployment ID)
    IdString referenceId;
    # Reference type
    ReferenceType referenceType;
    # File name
    string? name?;
    # Description of the attachment (only for deployment type)
    string? description?;
    json...;
|};

# Response from updating an attachment.
public type AttachmentUpdateResponse record {|
    # Success message
    string message;
    # Updated attachment details
    UpdatedAttachment attachment;
|};

# Updated attachment details.
public type UpdatedAttachment record {|
    # ID of the updated attachment
    IdString id;
    # Updated date and time
    string updatedOn;
    # User who updated the attachment
    string updatedBy;
    json...;
|};

# Delete attachment response from ServiceNow.
public type AttachmentDeleteResponse record {|
    # Success message
    string message;
    # Deleted attachment details
    record {|
        # ID of the deleted attachment
        IdString id;
        # User who deleted the attachment
        string deletedBy;
        # Deleted date and time
        string deletedOn;
        json...;
    |} attachment;
|};

# Deployed product data.
public type DeployedProduct record {|
    # ID
    IdString id;
    # Created date and time
    string createdOn;
    # Updated date and time
    string updatedOn;
    # Description
    string? description;
    # Category of the product
    ReferenceTableItem? category;
    # Associated deployment
    ReferenceTableItem? deployment;
    # Product information
    ReferenceTableItem? product;
    # Product version
    ReferenceTableItem? version;
    # Cores allocated for the product
    int? cores;
    # TPS allocated for the product
    decimal? tps;
    # Release date of the product
    string? releasedOn;
    # End of life date of the product
    string? endOfLifeOn;
    # Update level of the product
    string? updateLevel;
    json...;
|};

# Deployed products response.
public type DeployedProductsResponse record {|
    # List of deployed products
    DeployedProduct[] deployedProducts;
|};

# Request payload for creating a deployed product.
public type DeployedProductCreatePayload record {|
    # Project ID
    IdString projectId;
    # Deployment ID
    IdString deploymentId;
    # Product ID
    IdString productId;
    # Product version ID
    IdString versionId;
    # Cores allocated for the product
    int? cores?;
    # TPS allocated for the product
    decimal? tps?;
    # Description of the deployed product
    string? description?;
|};

# Response from creating a deployed product.
public type DeployedProductCreateResponse record {|
    # Success message
    string message;
    # Created deployed product details
    CreatedDeployedProduct deployedProduct;
    json...;
|};

# Created deployed product details.
public type CreatedDeployedProduct record {|
    # ID
    IdString id;
    # Created date and time
    string createdOn;
    # User who created the deployed product
    string createdBy;
    json...;
|};

# Payload for updating a deployed product.
public type DeployedProductUpdatePayload record {|
    # Cores allocated for the product
    int? cores?;
    # TPS allocated for the product
    decimal? tps?;
    # Description of the deployed product
    string? description?;
    # Active status (can only be set to false to deactivate deployed product)
    boolean active?;
|};

# Response from updating a deployed product.
public type DeployedProductUpdateResponse record {|
    # Success message
    string message;
    # Updated deployed product details
    UpdatedDeployedProduct deployedProduct;
    json...;
|};

# Updated deployed product details.
public type UpdatedDeployedProduct record {|
    # ID of the updated deployed product
    IdString id;
    # Updated date and time
    string updatedOn;
    # User who updated the deployed product
    string updatedBy;
    json...;
|};

# Deployment data.
public type Deployment record {|
    # ID
    IdString id;
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
    ReferenceTableItem? project;
    # Type
    ChoiceListItem? 'type;
    json...;
|};

# Payload for creating a deployment.
public type DeploymentCreatePayload record {|
    # Project ID
    IdString projectId;
    # Name
    string name;
    # Type key
    int typeKey;
    # Description
    string description;
    json...;
|};

# Response from creating a deployment.
public type DeploymentCreateResponse record {|
    # Success message
    string message;
    # Created deployment details
    CreatedDeployment deployment;
|};

# Created deployment details.
public type CreatedDeployment record {|
    # ID of the created deployment
    IdString id;
    # Created date and time
    string createdOn;
    # User who created the deployment
    string createdBy;
    json...;
|};

# Deployments response.
public type DeploymentsResponse record {|
    # List of deployments
    Deployment[] deployments;
|};

# Payload for creating a comment.
public type CommentCreatePayload record {|
    # Reference ID (case or change request ID)
    IdString referenceId;
    # Reference type
    ReferenceType referenceType;
    # Comment content
    @constraint:String {minLength: 1} // TODO: Remove max length until the byte array support is added
    string content;
    # Comment type
    CommentType 'type;
    # User who created the comment
    string createdBy;
|};

# Created comment details.
public type CreatedComment record {|
    # ID of the created comment
    IdString id;
    # Created date and time
    string createdOn;
    # User who created the comment
    string createdBy;
    # HTML content of the comment
    string content;
    # Indicates if the comment has inline attachments
    boolean hasInlineAttachments;
    # Count of inline attachments
    int inlineImageCount;
    # List of inline attachments
    InlineAttachment[] inlineAttachments;
    json...;
|};

# Response from creating a comment.
public type CommentCreateResponse record {|
    # Success message
    string message;
    # Created comment details
    CreatedComment comment;
    json...;
|};

# Response from creating an attachment.
public type AttachmentCreateResponse record {|
    # Success message
    string message;
    # Created attachment details
    CreatedAttachment attachment;
    json...;
|};

# Created attachment details.
public type CreatedAttachment record {|
    # ID of the created attachment
    IdString id;
    # File size
    int sizeBytes;
    # Created date and time
    string createdOn;
    # User who created the attachment
    string createdBy;
    # Download URL
    string downloadUrl?;
    json...;
|};

# Payload for creating an attachment.
public type AttachmentCreatePayload record {|
    # Reference ID to which the attachment is associated (e.g., query ID, incident ID, etc)
    IdString referenceId;
    # Reference type
    ReferenceType referenceType;
    # File name
    string name;
    # MIME type of the file
    string 'type;
    # Content of the file as a byte array
    string file;
    # Description of the attachment
    string? description?;
|};

# Inline attachment.
public type InlineAttachment record {|
    # ID of the inline attachment
    string id;
    # File name
    string fileName;
    # Content type
    string contentType;
    # Download URL
    string downloadUrl;
    # Created date and time
    string createdOn;
    # User who created
    string createdBy;
|};

# Payload for searching product vulnerabilities.
public type ProductVulnerabilitySearchPayload record {|
    # Filter criteria
    record {
        # Search query for CVE ID, Vulnerability ID, Component Name, etc.
        string searchQuery?;
        # Status ID
        int statusId?;
        # Severity ID
        int severityId?;
    } filters?;
    # Sort configuration
    SortBy sortBy?; // TODO: Check the correct sort by fields for vulnerabilities
    # Pagination details
    Pagination pagination?;
|};

# Product vulnerability.
public type ProductVulnerability record {|
    # ID
    string id;
    # CVE identifier
    string cveId;
    # Vulnerability identifier
    string vulnerabilityId;
    # Severity level
    ChoiceListItem severity;
    # Name of the component
    string componentName;
    # Version of the component
    string version;
    # Type
    string 'type;
    # Use case description
    string? useCase;
    # Justification for the vulnerability
    string? justification;
    # Resolution details for the vulnerability
    string? resolution;
    json...;
|};

# Product vulnerability information.
public type ProductVulnerabilityResponse record {|
    *ProductVulnerability;
    # Type of the component
    string componentType?;
    # Update level for the vulnerability
    string updateLevel;
    json...;
|};

# Product vulnerabilities response with pagination.
public type ProductVulnerabilitySearchResponse record {|
    # List of product vulnerabilities
    ProductVulnerability[] productVulnerabilities;
    # Total records count
    int totalRecords;
    *Pagination;
    json...;
|};

# Vulnerability metadata response.
public type VulnerabilityMetaResponse record {|
    # List of vulnerability severities
    ChoiceListItem[] severities;
    json...;
|};

# Request payload for searching call requests.
public type CallRequestSearchPayload record {|
    # Case ID
    IdString caseId;
    # Filter criteria
    record {
        # List of state keys to filter
        int[] stateKeys?;
    } filters?;
    # Pagination details
    Pagination pagination?;
|};

# Call request data.
public type CallRequest record {|
    # ID
    IdString id;
    # Associated case information
    ReferenceTableItem case;
    # Reason for the call request
    string? reason;
    # Preferred times for the call
    string[] preferredTimes;
    # Duration in minutes
    int durationMin;
    # Scheduled time for the call
    string? scheduleTime;
    # Created date and time
    string createdOn;
    # Updated date and time
    string updatedOn;
    # State information
    ChoiceListItem state;
    json...;
|};

# Call requests response.
public type CallRequestsResponse record {|
    # List of call requests
    CallRequest[] callRequests;
    json...;
|};

# Date-time (UTC).
@constraint:String {
    pattern: {
        value: re `^\d{4}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])T([01]\d|2[0-3]):[0-5]\d(:[0-5]\d(\.\d{1,9})?)?(Z|[+-]([01]\d|2[0-3]):?[0-5]\d)$`,
        message: "Invalid date provided. Please provide a valid date value."
    }
}
public type DateTime string;

# Request payload for creating a call request.
public type CallRequestCreatePayload record {|
    # Case ID
    IdString caseId;
    # Reason for the call request
    string reason;
    # Preferred UTC times for the call
    @constraint:Array {minLength: 1}
    DateTime[] utcTimes;
    # Duration in minutes
    @constraint:Int {minValue: 1}
    int durationInMinutes;
|};

# Created call request details.
public type CreatedCallRequest record {|
    # ID
    IdString id;
    # Created date and time
    string createdOn;
    # User who created the call request
    string createdBy;
    # State information
    ChoiceListItem state;
    json...;
|};

# Response from creating a call request.
public type CallRequestCreateResponse record {|
    # Success message
    string message;
    # Created call request details
    CreatedCallRequest callRequest;
    json...;
|};

# Request payload for updating a call request.
public type CallRequestUpdatePayload record {|
    # State key
    int stateKey;
    # Reason for the update
    string reason;
    # New preferred UTC times for the call (mandatory when stateKey is 2)
    DateTime[] utcTimes?;
|};

# Updated call request details.
public type UpdatedCallRequest record {|
    # ID
    IdString id;
    # Updated date and time
    string updatedOn;
    # User who updated the call request
    string updatedBy;
    json...;
|};

# Response from updating a call request.
public type CallRequestUpdateResponse record {|
    # Success message
    string message;
    # Updated call request details
    UpdatedCallRequest callRequest;
    json...;
|};

# Request payload for updating a deployment.
public type DeploymentUpdatePayload record {|
    # Name
    string name?;
    # Type key
    int typeKey?;
    # Description of the deployment
    string? description?;
    # Active status (can only be set to false to deactivate deployment)
    boolean active?;
|};

# Response from updating a deployment.
public type DeploymentUpdateResponse record {|
    # Success message
    string message;
    # Updated deployment details
    UpdatedDeployment deployment;
|};

# Updated deployment details.
public type UpdatedDeployment record {|
    # ID of the updated deployment
    IdString id;
    # Updated date and time
    string updatedOn;
    # User who updated the deployment
    string updatedBy;
    json...;
|};

# Request payload for searching products.
public type ProductSearchPayload record {|
    # Pagination details
    Pagination pagination = {};
|};

# Product data.
public type Product record {|
    # ID
    IdString id;
    # Name
    string name;
    json...;
|};

# Products response.
public type ProductsResponse record {|
    # List of products
    Product[] products;
    json...; // TODO: Remove after adding pagination
|};

# Request payload for searching product versions.
public type ProductVersionSearchPayload record {|
    # Pagination details
    Pagination pagination = {};
|};

# Product version data.
public type ProductVersion record {|
    # ID
    IdString id;
    # Version number
    string version;
    # Current support status
    string? currentSupportStatus;
    # Release date
    string? releaseDate;
    # Support end of life date
    string? supportEolDate;
    # Earliest possible support end of life date
    string? earliestPossibleSupportEolDate;
    # Associated product information
    ReferenceTableItem? product;
    json...;
|};

# Product versions response.
public type ProductVersionsResponse record {|
    # List of product versions
    ProductVersion[] versions;
    json...; // TODO: Add pagination
|};

# Date.
@constraint:String {
    pattern: {
        value: re `^\d{4}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])$`,
        message: "Invalid date format."
    }
}
public type Date string;

# Request payload for searching time cards.
public type TimeCardSearchPayload record {|
    # Filter criteria
    record {
        # List of project IDs to filter
        string[] projectIds?;
        # Start date for filtering time cards (ISO 8601 format)
        Date startDate?;
        # End date for filtering time cards (ISO 8601 format)
        Date endDate?;
        # States of the time cards to filter (e.g., "Approved", "Submitted", etc.)
        TimeCardState[] states?;
    } filters?;
    # Pagination details
    Pagination pagination?;
|};

# Payload for searching conversations.
public type ConversationSearchPayload record {|
    # Filter criteria
    record {
        # List of project IDs to filter
        IdString[] projectIds?;
        # List of state keys to filter
        int[] stateKeys?;
        # Search query for conversations
        string searchQuery?;
        # Conversations created by logged in user
        boolean createdByMe?;
    } filters?;
    # Sort configuration
    record {
        # Field to sort by
        ConversationSortField 'field;
        # Sort order
        SortOrder 'order;
    } sortBy?;
    # Pagination details
    Pagination pagination?;
|};

# Time card data.
public type TimeCard record {|
    # ID
    string id;
    # Total time logged
    decimal totalTime;
    # Created date and time
    string createdOn;
    # Indicates if the time card has billable hours
    boolean hasBillable;
    # State information (e.g., "Approved", "Submitted", etc.)
    ChoiceListItem? state;
    # User who approved the time card
    ReferenceTableItem? approvedBy;
    # Associated project
    ReferenceTableItem? project;
    # Associated case
    TimeCardCase? case;
    json...;
|};

# Time card information associated with a case.
public type TimeCardCase record {|
    *ReferenceTableItem;
    # Case number
    string number;
|};

# Time cards response.
public type TimeCardsResponse record {|
    # List of time cards
    TimeCard[] timeCards;
    # Total records count
    int totalRecords;
    *Pagination;
|};

# Conversation data.
public type Conversation record {|
    # ID of the chat
    IdString id;
    # Chat number
    string? number;
    # Initial message of the chat
    string? initialMessage;
    # Message count
    int messageCount;
    # Created date and time
    string createdOn;
    # User who created the chat
    string createdBy;
    # Associated project
    ReferenceTableItem? project;
    # Associated case
    ReferenceTableItem? case;
    # State information
    ChoiceListItem? state;
    json...;
|};

# Conversation response.
public type ConversationSearchResponse record {|
    # List of conversations
    Conversation[] conversations;
    # Total records count
    int totalRecords;
    *Pagination;
|};

# Request payload for creating a conversation.
public type ConversationCreatePayload record {|
    # Project ID
    IdString projectId;
    # Initial message of the conversation
    string initialMessage;
|};

# Created conversation details.
public type CreatedConversation record {|
    # ID of the created conversation
    IdString id;
    # Conversation number
    string? number;
    # User who created the conversation
    string createdBy;
    # Created date and time
    string createdOn;
    # State information
    ChoiceListItem state;
    json...;
|};

# Response from creating a conversation.
public type ConversationCreateResponse record {|
    # Success message
    string message;
    # Created conversation details
    CreatedConversation conversation;
    json...;
|};

# Request payload for updating a conversation.
public type ConversationUpdatePayload record {|
    # State key to update
    int stateKey;
|};

# Updated conversation details.
public type UpdatedConversation record {|
    # ID of the updated conversation
    IdString id;
    # Conversation number
    string? number;
    # Updated date and time
    string updatedOn;
    # User who updated the conversation
    string updatedBy;
    # State information
    ChoiceListItem state;
    json...;
|};

# Response from updating a conversation.
public type ConversationUpdateResponse record {|
    # Success message
    string message;
    # Updated conversation details
    UpdatedConversation conversation;
    json...;
|};

# Conversation information.
public type ConversationResponse record {|
    *Conversation;
    # Updated date and time
    string updatedOn;
    # User who updated the conversation
    string updatedBy;
    json...;
|};

# Project time cards statistics response.
public type ProjectTimeCardStatsResponse record {|
    # Total hours logged
    decimal totalHours;
    # Billable hours
    decimal billableHours;
    # Non-billable hours
    decimal nonBillableHours;
    json...;
|};

# case state IDs.
public type CaseStateIds record {|
    # Open state ID
    int open;
    # Close state ID
    int closed;
    # Solution proposed ID
    int solutionProposed;
    # Work in progress state ID
    int workInProgress;
    # Awaiting info ID
    int awaitingInfo;
    # Waiting on WSO2 ID
    int waitingOnWso2;
    # Reopened state ID
    int reopened;
|};

# Conversation state IDs.
public type ConversationStateIds record {|
    # Open state ID
    int open;
    # Active state ID
    int active;
    # Converted state ID
    int converted;
    # Resolved state ID
    int resolved;
    # Abandoned state ID
    int abandonded;
|};

# Variable data for service request.
public type Variable record {|
    # Variable ID
    IdString id;
    # Variable value
    string value;
|};

# Request payload for searching change requests.
public type ChangeRequestSearchPayload record {|
    # Filter criteria
    record {|
        # List of project IDs to filter
        IdString[] projectIds?;
        # Search query for change request number and title
        string searchQuery?;
        # List of change request state keys
        int[] stateKeys?;
        # Change request impact key
        int impactKey?;
    |} filters?;
    # Pagination details
    Pagination pagination?;
|};

# Change request data.
public type ChangeRequest record {|
    # ID
    IdString id;
    # Change request number
    string number;
    # Change request title
    string? title;
    # Associated project information
    ReferenceTableItem? project;
    # Service request information (case)
    ReferenceTableItem? case;
    # Deployment information
    ReferenceTableItem? deployment;
    # Deployed product information
    ReferenceTableItem? deployedProduct;
    # Product information
    ReferenceTableItem? product;
    # Assigned engineer
    ReferenceTableItem? assignedEngineer;
    # Assigned team
    ReferenceTableItem? assignedTeam;
    # Planned start date and time
    Date? plannedStartOn;
    # Planned end date and time
    Date? plannedEndOn;
    # Duration
    string? duration;
    # Indicates if the change request has a service outage
    boolean hasServiceOutage = false;
    # Impact information
    ChoiceListItem? impact;
    # State information
    ChoiceListItem? state;
    # Type information
    ChoiceListItem? 'type;
    # Created date and time
    string createdOn;
    # Updated date and time
    string updatedOn;
    json...;
|};

# Change requests response.
public type ChangeRequestSearchResponse record {|
    # List of change requests
    ChangeRequest[] changeRequests;
    # Total records count
    int totalRecords;
    *Pagination;
|};

# Request payload for searching catalogs.
public type CatalogSearchPayload record {|
    # Deployed product ID
    IdString deployedProductId;
    # Pagination details (optional)
    Pagination pagination?;
|};

# Catalog data.
public type Catalog record {|
    # ID
    IdString id;
    # Name of the catalog
    string name;
    # List of catalog items
    ReferenceTableItem[] catalogItems;
    json...;
|};

# Catalog search response.
public type CatalogSearchResponse record {|
    # List of catalogs
    Catalog[] catalogs;
    # Total records count
    int totalRecords;
    *Pagination;
|};

# Catalog item variable information.
public type CatalogItemVariable record {|
    # Variable ID
    IdString id;
    # Question text for the variable
    string questionText;
    # Display order of the variable
    int 'order;
    # Type of the variable (e.g., "Single Line Text", "Multi Line Text")
    string 'type;
    json...;
|};

# Catalog item variables response.
public type CatalogItemVariablesResponse record {|
    # List of catalog item variables
    CatalogItemVariable[] variables;
|};

# Change request details information.
public type ChangeRequestResponse record {|
    *ChangeRequest;
    # Change request description
    string? description;
    # User who created the change request
    string createdBy;
    # Justification for the change request
    string? justification;
    # Impact description
    string? impactDescription;
    # Service outage details
    string? serviceOutage;
    # Communication plan
    string? communicationPlan;
    # Rollback plan
    string? rollbackPlan;
    # Test plan
    string? testPlan;
    # Indicates if the customer has approved
    boolean hasCustomerApproved;
    # Indicates if the customer has reviewed
    boolean hasCustomerReviewed;
    # Internal approval details
    ReferenceTableItem? approvedBy;
    # Internal approval date and time
    string? approvedOn;
    json...;
|};

# Change request statistics.
public type ProjectChangeRequestStatsResponse record {|
    # Total change request count
    int totalCount;
    # Count of change requests by state
    ChoiceListItem[] stateCount;
    json...;
|};

# DateTime string type with YYYY-MM-DD HH:MM:SS format constraint.
@constraint:String {
    pattern: re `^\d{4}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01]) ([01]\d|2[0-3]):[0-5]\d:[0-5]\d$`
}
public type DateTimeWithoutTimezone string;

# Request payload for updating a change request.
public type ChangeRequestUpdatePayload record {|
    # Planned start date and time (format: YYYY-MM-DD HH:MM:SS)
    DateTimeWithoutTimezone plannedStartOn;
|};

# Response from updating a change request.
public type ChangeRequestUpdateResponse record {|
    # Success message
    string message;
    # Updated change request details
    UpdatedChangeRequest changeRequest;
|};

# Updated change request details.
public type UpdatedChangeRequest record {|
    # ID of the updated change request
    IdString id;
    # Updated date and time
    string updatedOn;
    # User who updated the change request
    string updatedBy;
    json...;
|};
