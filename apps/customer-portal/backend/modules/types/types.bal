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
import customer_portal.registry;

import ballerina/constraint;

public const PHONE_PATTERN_STRING = "^\\+\\d{10,14}$";

# [Configurable] Feature flags.
public type FeatureFlags record {|
    # Indicates if the usage and metrics page is enabled or not
    boolean usageMetricsEnabled;
|};

# Metadata.
public type MetadataResponse record {|
    # List of available time zones
    ReferenceItem[] timeZones;
    # List of available project types
    ReferenceItem[] projectTypes;
    # Indicate which features are enabled
    FeatureFlags featureFlags;
|};

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
    # Search query for case number, title and description
    string searchQuery?;
    # Issue ID
    int issueId?;
    # Status IDs
    int[] statusIds?;
    # List of case types
    entity:CaseType[] caseTypes?;
    # Severity ID
    int severityId?;
    # Deployment ID
    string deploymentId?;
    # Case created by the logged in user
    boolean createdByMe?;
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
    # Created by (User email)
    string createdBy;
    # Case title
    string? title;
    # Case description
    string? description;
    # Duration
    string? duration;
    # issueType of the case
    ReferenceItem? issueType;
    # Severity of the case
    ReferenceItem? severity;
    # State of the case
    ReferenceItem? status;
    # Catalog information (if the case is a service request)
    ReferenceItem? catalog?;
    # Catalog item information (if the case is a service request)
    ReferenceItem? catalogItem?;
    # Assigned team
    ReferenceItem? assignedTeam;
    # Product information
    ReferenceItem? product;
    # Engagement type information
    ReferenceItem engagementType?;
|};

# Case information.
public type CaseResponse record {|
    *Case;
    # Last updated date and time
    string updatedOn;
    # SLA response time
    string? slaResponseTime;
    # Project
    ReferenceItem? project;
    # Case type information (eg: incident, service request, etc.)
    ReferenceItem? 'type;
    # Assigned engineer
    ReferenceItem? assignedEngineer;
    # Parent case information
    ReferenceItem? parentCase;
    # Related case information
    ReferenceItem? relatedCase;
    # Conversation information
    ReferenceItem? conversation;
    # Deployment information
    record {
        *ReferenceItem;
        # Deployment type
        string? 'type;
    }? deployment;
    # Deployed product information
    record {
        *ReferenceItem;
        # Deployed product version
        string? version;
    }? deployedProduct;
    # Account information
    record {
        *ReferenceItem;
        # Account type
        string? 'type;
    }? account;
    # CS Manager information
    record {
        *ReferenceItem;
        # Email address
        string? email;
    }? csManager;
    # Case closed date and time
    string? closedOn?;
    # User who closed the case
    ReferenceItem? closedBy?;
    # Close notes for the case closure
    string? closeNotes?;
    # Indicates if the case is auto closed
    boolean? hasAutoClosed?;
    # Change requests (only for service requests)
    ReferenceItem[]? changeRequests?;
    # Engagement payment type information
    ReferenceItem engagementPaymentType?;
    # Engagement start date
    entity:Date? engagementStartDate?;
    # Engagement end date
    entity:Date? engagementEndDate?;
    # Variables for service request
    entity:ServiceRequestVariable[]? variables?;
|};

# Reference item.
public type ReferenceItem record {|
    # ID
    string id;
    # Label
    string label;
    # Count
    int count?;
    # Number
    string? number?;
    # Abbreviation
    string? abbreviation?;
    # Release date (for product versions)
    entity:Date? releasedOn?;
    # End of life date (for product versions)
    entity:Date? endOfLifeOn?;
|};

# Case metadata information.
public type CaseMetaData record {|
    *Case;
    # Associated project
    ReferenceItem? project;
    # Case type information (eg: incident, service request, etc.)
    ReferenceItem? 'type;
    # Deployment information
    ReferenceItem? deployment;
    # Deployed product information
    ReferenceItem? deployedProduct;
    # Assigned engineer
    ReferenceItem? assignedEngineer;
    # Parent case information
    ReferenceItem? parentCase;
    # Related case information
    ReferenceItem? relatedCase;
    # Conversation information
    ReferenceItem? conversation;
|};

# Cases list response with pagination.
public type CaseSearchResponse record {|
    # List of cases with associated information
    CaseMetaData[] cases;
    # Total records count
    int totalRecords;
    *entity:Pagination;
|};

# User.
public type User record {|
    *entity:UserResponse;
    # Phone number of the user
    string? phoneNumber?;
    # Last password update time
    string? lastPasswordUpdateTime?;
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

# Project filter options.
public type ProjectFilterOptions record {|
    # List of case states
    ReferenceItem[] caseStates;
    # List of case severities
    ReferenceItem[] severities;
    # List of issue types
    ReferenceItem[] issueTypes;
    # List of deployment types
    ReferenceItem[] deploymentTypes;
    # List of available call request states
    ReferenceItem[] callRequestStates;
    # List of available change request states
    ReferenceItem[] changeRequestStates;
    # List of available change request impacts
    ReferenceItem[] changeRequestImpacts;
    # List of available conversation states
    ReferenceItem[] conversationStates;
    # List of available case types
    ReferenceItem[] caseTypes;
    # List of available time card states
    ReferenceItem[] timeCardStates;
    # List of available engagement types
    ReferenceItem[] engagementTypes;
    # List of available engagement payment types
    ReferenceItem[] engagementPaymentTypes;
    # Severity based allocation time mapping (severity ID to allocation time in minutes)
    map<int> severityBasedAllocationTime;
|};

# Project data.
public type Project record {|
    # ID
    entity:IdString id;
    # Name
    string name;
    # Project key
    string key;
    # Created date and time
    string createdOn;
    # Description
    string? description;
    # Project type
    ReferenceItem 'type;
    # Closure state
    string? closureState;
    # Indicates whether the project has a PDP subscription
    boolean hasPdpSubscription;
    # Novera agent enabled status for the project
    boolean hasAgent;
    # Knowledge base references enabled status for the project
    boolean hasKbReferences;
    # Active cases count
    int activeCasesCount;
    # Active chats/conversations count
    int activeChatsCount;
    # SLA status (e.g., "Needs Attention")
    string slaStatus;
|};

# Project information.
public type ProjectResponse record {|
    # ID of the project
    entity:IdString id;
    # Name of the project
    string name;
    # Project key
    string key;
    # Created date and time
    string createdOn;
    # Description of the project
    string? description;
    # Project type
    ReferenceItem 'type;
    # Salesforce ID
    string sfId;
    # Closure state
    string? closureState;
    # Indicates whether the project has a PDP subscription
    boolean hasPdpSubscription;
    # Project start date
    entity:Date? startDate;
    # Project end date 
    entity:Date? endDate;
    # Account information
    record {|
        # ID of the account
        entity:IdString id;
        # Indicates whether the agent is enabled for the account
        boolean hasAgent;
        # Indicates whether the account enabled knowledge base references
        boolean hasKbReferences;
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
    |} account;
    # Query hour information
    decimal? totalQueryHours;
    # Consumed query hours
    decimal? consumedQueryHours;
    # Remaining query hours
    decimal? remainingQueryHours;
    # Go-live date
    entity:Date? goLiveDate;
    # Go-live plan date
    entity:Date? goLivePlanDate;
    # Onboarding hour information
    decimal? totalOnboardingHours;
    # Consumed onboarding hours
    decimal? consumedOnboardingHours;
    # Remaining onboarding hours
    decimal? remainingOnboardingHours;
    # Onboarding expiry date
    entity:Date? onboardingExpiryDate;
    # Onboarding status
    string? onboardingStatus;
|};

# Projects response.
public type ProjectsResponse record {|
    # List of projects
    Project[] projects;
    # Total records count
    int totalRecords;
    *entity:Pagination;
|};

# Case statistics for a project.
public type ProjectCaseStats record {|
    # Total count(last 30d)
    int totalCount;
    # Active case count (cases that are not in closed state)
    int activeCount;
    # Outstanding case count (cases that are not solution proposed or closed)
    int outstandingCount;
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
    |} resolvedCases;
    # Change rate of engagements past thirty days breakdown
    record {|
        # Change rate of resolved engagements
        decimal resolvedEngagements;
        # Change rate of average response time
        decimal averageResponseTime;
        json...;
    |} changeRate;
    # Count of cases by state
    ReferenceItem[] stateCount;
    # Count of cases by severity
    ReferenceItem[] severityCount;
    # Outstanding cases count by severity
    ReferenceItem[] outstandingSeverityCount;
    # Count of cases by type
    ReferenceItem[] caseTypeCount;
    # Cases trend
    entity:CasesTrend[] casesTrend;
    # Count of cases by engagement type
    ReferenceItem[] engagementTypeCount;
    # Outstanding cases count by engagement type
    ReferenceItem[] outstandingEngagementTypeCount;
|};

# Project support statistics.
public type ProjectSupportStats record {|
    # Ongoing cases count
    int? ongoingCases?;
    # Active chats count
    int activeChats?;
    # Resolved past 30 days cases count
    int resolvedPast30DaysCasesCount?;
    # Resolved chats count
    int resolvedChats?;
|};

# Project statistics.
public type ProjectStats record {|
    # Open cases count
    int openCases?;
    # Active chats count
    int activeChats?;
    # Deployments count
    int deployments?;
    # SLA status
    string slaStatus?;
|};

# Recent activity details.
public type RecentActivity record {|
    # Total hours
    decimal totalHours?;
    # Billable hours
    decimal billableHours?;
    # Last deployment date
    string? lastDeploymentOn?;
|};

# Project statistics response.
public type ProjectStatsResponse record {|
    # Project statistics
    ProjectStats projectStats;
    # Recent activity details
    RecentActivity recentActivity;
|};

# Created case details.
public type CreatedCase record {|
    # System ID of the created case
    string id;
    # WSO2 internal ID of the case
    string internalId;
    # Case number
    string number;
    # User who created the case
    string createdBy;
    # Created date and time
    string createdOn;
    # Status
    ReferenceItem state;
    # Case type information (eg: incident, query, etc.)
    ReferenceItem 'type;
    json...;
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
    # Indicates if the comment has inline attachments
    boolean hasInlineAttachments;
    # List of inline attachments
    entity:InlineAttachment[] inlineAttachments;
    # First name of the user who created the comment
    string? createdByFirstName;
    # Last name of the user who created the comment
    string? createdByLastName;
|};

# Comments response with pagination.
public type CommentsResponse record {|
    # List of comments
    Comment[] comments;
    # Total records count
    int totalRecords;
    *entity:Pagination;
|};

# Created attachment details.
public type CreatedAttachment record {|
    # System ID of the created attachment
    string id;
    # File size(in Bytes)
    int size;
    # Created date and time
    string createdOn;
    # User who created the attachment
    string createdBy;
    # Download URL
    string downloadUrl?;
|};

# Attachment data.
public type Attachment record {|
    # ID of the attachment
    string id;
    # File name
    string name;
    # MIME type of the file
    string 'type;
    # File size(in Bytes)
    int size;
    # User who created the attachment
    string createdBy;
    # Created date and time
    string createdOn;
    # Download URL
    string? downloadUrl;
    # Preview URL for image attachments
    string? previewUrl;
    # Description of the attachment
    string? description;
|};

# Attachments response.
public type AttachmentsResponse record {|
    # List of attachments
    Attachment[] attachments;
    # Total records count
    int totalRecords;
    *entity:Pagination;
|};

# Payload for updating an attachment.
public type AttachmentUpdatePayload record {|
    # File name
    string? name?;
    # Description of the attachment (only for deployment type)
    string? description?;
|};

# Request payload for searching deployments.
public type DeploymentSearchPayload record {|
    # Pagination details
    entity:Pagination pagination?;
|};

# Deployment information.
public type Deployment record {|
    # ID
    string id;
    # Number of the deployment
    string? number;
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

# Deployments response.
public type DeploymentsResponse record {|
    # List of deployments
    Deployment[] deployments;
    # Total records count
    int totalRecords;
    *entity:Pagination;
|};

# Deployed product search payload
public type DeployedProductSearchPayload record {|
    # Pagination details
    entity:Pagination pagination?;
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
    # Category
    ReferenceItem? category;
    # Product
    ReferenceItem? product;
    # Deployment
    ReferenceItem? deployment;
    # Product version
    ReferenceItem? version;
    # Product updates
    entity:ProductUpdate[]? updates;
    # Cores allocated for the product
    int? cores;
    # TPS allocated for the product
    decimal? tps;
|};

# Instance search filters.
public type InstanceSearchPayload record {|
    # Filter criteria
    record {|
        # Start date of consumption
        entity:Date startDate?;
        # End date of consumption
        entity:Date endDate?;
    |} filters?;
    # Pagination details
    entity:Pagination pagination?;
|};

# Instance data.
public type Instance record {|
    # ID
    entity:IdString id;
    # Key
    string key;
    # Associated project information
    ReferenceItem? project;
    # Associated deployment information
    ReferenceItem? deployment;
    # Associated product information
    ReferenceItem? product;
    # Associated deployed product information
    ReferenceItem? deployedProduct;
    # Created date and time
    string createdOn;
    # Updated date and time
    string updatedOn;
    # Instance metadata
    entity:InstanceMetadata? metadata;
    json...;
|};

# Instances response.
public type InstancesResponse record {|
    # List of instances
    Instance[] instances;
    # Total records count
    int totalRecords;
    *entity:Pagination;
|};

# Deployed products response.
public type DeployedProductsResponse record {|
    # List of deployed products
    DeployedProduct[] deployedProducts;
    # Total records count
    int totalRecords;
    *entity:Pagination;
|};

# Request payload for creating a deployed product.
public type DeployedProductCreatePayload record {|
    # Project ID
    string projectId;
    # Product ID
    string productId;
    # Product version ID
    string versionId;
    # Cores allocated for the product
    int? cores?;
    # TPS allocated for the product
    decimal? tps?;
    # Description of the deployed product
    string? description?;
|};

# Payload for creating a comment.
public type CommentCreatePayload record {|
    # Comment content
    @constraint:String {minLength: 1} // TODO: Remove max length until the byte array support is added
    string content;
    # Comment type (eg: case, change request, etc.)
    entity:CommentType 'type;
|};

# Payload for creating an attachment.
public type AttachmentCreatePayload record {|
    # File name
    string name;
    # MIME type of the file
    string 'type;
    # Base 64 encoded content
    string content;
    # Description of the attachment
    string? description?;
|};

# Product vulnerability metadata response.
public type ProductVulnerabilityMetaResponse record {|
    # List of vulnerability severities
    ReferenceItem[] severities;
    json...;
|};

# Product vulnerability search filters.
public type ProductVulnerabilitySearchFilters record {|
    # Search query for CVE ID, Vulnerability ID, Component Name and etc.
    string searchQuery?;
    # Status ID
    int statusId?;
    # Severity ID
    int severityId?;
|};

# Base product vulnerability.
public type ProductVulnerability record {|
    # ID
    string id;
    # CVE identifier
    string cveId;
    # Vulnerability identifier
    string vulnerabilityId;
    # Severity level
    ReferenceItem severity;
    # Name of the component
    string componentName;
    # Version of the component
    string version;
    # Type
    string 'type;
    # Use case description
    string? useCase;
    # Justification
    string? justification;
    # Resolution details
    string? resolution;
    json...;
|};

# Product vulnerability information.
public type ProductVulnerabilityResponse record {|
    *ProductVulnerability;
    # Type of the component
    string componentType?;
    # Update level for the vulnerability
    string updateLevel?;
|};

# Product vulnerabilities response with pagination.
public type ProductVulnerabilitySearchResponse record {|
    # List of product vulnerabilities
    ProductVulnerability[] productVulnerabilities;
    # Total records count
    int totalRecords;
    *entity:Pagination;
|};

# Recommended update level.
public type RecommendedUpdateLevel record {|
    # Product name
    string productName;
    # Product base version
    string productBaseVersion;
    # Channel
    string channel;
    # Starting update level
    int startingUpdateLevel;
    # Ending update level
    int endingUpdateLevel;
    # Installed updates count
    int installedUpdatesCount;
    # Installed security updates count
    int installedSecurityUpdatesCount;
    # Timestamp
    int timestamp;
    # Recommended update level
    int recommendedUpdateLevel;
    # Available updates count
    int availableUpdatesCount;
    # Available security updates count
    int availableSecurityUpdatesCount;
|};

# File changes.
public type FileChanges record {|
    # Added files
    BasicFileInfo[] addedFiles;
    # Modified files
    ModifiedFileInfo[] modifiedFiles;
    # Removed files
    string[] removedFiles;
    # Changes in bundles info
    BundlesInfoChange[] bundlesInfoChanges;
|};

# Basic file information.
public type BasicFileInfo record {|
    # File path
    string filePath;
    # MD5 checksum
    string md5sum;
    # SHA256 checksum
    string sha256;
    # JWT token
    string jwt;
    # Download URL
    string downloadUrl;
|};

# Modified file information.
public type ModifiedFileInfo record {|
    # Type
    string 'type;
    # Original file
    BasicFileInfo originalFile;
    # New file
    BasicFileInfo newFile;
|};

# Changes in bundles info.
public type BundlesInfoChange record {|
    # Bundles info path
    string bundlesInfoPath;
    # Content change
    string contentChange;
    # Change type
    ChangeType changeType;
    # Order
    int 'order;
|};

# Change type.
public type ChangeType record {|
    # Value
    string value;
|};

# Update response.
public type UpdateResponse record {|
    # File changes
    FileChanges fileChanges;
    # Product name
    string productName;
    # Product version
    string productVersion;
    # Starting update level
    string startingUpdateLevel;
    # Ending update level
    string endingUpdateLevel;
    # Environment
    string environment;
    # Update summary message
    string updateSummaryMessage;
    # Update security message
    string updateSecurityMessage;
    # Total updates
    int totalUpdates;
    # Total security updates
    int totalSecurityUpdates;
    # Applied update numbers
    int[] appliedUpdatesNumbers;
|};

# Update payload for listing updates.
public type ListUpdatePayload record {|
    # Product name
    @constraint:String {minLength: 1}
    string productName;
    # Product version
    @constraint:String {minLength: 1}
    string productVersion;
    # Channel
    @constraint:String {minLength: 1}
    string channel;
    # Starting update level
    @constraint:String {minLength: 1}
    string startUpdateLevel;
    # Ending update level
    @constraint:String {minLength: 1}
    string endUpdateLevel;
    # Hotfixes
    string[] hotFixes;
|};

# Product update level.
public type ProductUpdateLevel record {|
    # Product name
    string productName;
    # Product update levels
    UpdateLevel[] productUpdateLevels;
|};

# Update level.
public type UpdateLevel record {|
    # Product base version
    string productBaseVersion;
    # Channel
    string channel;
    # Update level
    int[] updateLevels;
|};

# Payload for creating a deployment.
public type DeploymentCreatePayload record {|
    # Name
    string name;
    # Type key
    int deploymentTypeKey;
    # Description
    string description;
    json...;
|};

# The request payload to be validated.
public type ContactOnboardPayload record {|
    # Email address of the Contact
    @constraint:String {pattern: re `^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$`}
    string contactEmail;
    # First name of the Contact
    string contactFirstName;
    # Last name of the Contact
    string contactLastName;
    # Whether the contact is System User or not
    boolean isCsIntegrationUser;
    # Whether the contact is Security Contact or not
    boolean isSecurityContact = false;
|};

# Payload for updating membership security flag.
public type MembershipSecurityPayload record {|
    # Whether the contact is a security contact or not
    boolean isSecurityContact;
|};

# The request payload to be validated.
public type ValidationPayload record {|
    # Contact email
    string contactEmail;
|};

# Request payload for searching call requests.
public type CallRequestSearchPayload record {|
    # Filter criteria
    record {
        # List of state keys to filter
        int[] stateKeys?;
    } filters?;
    # Pagination details
    entity:Pagination pagination?;
|};

# Call request data from ServiceNow.
public type CallRequest record {|
    # ID
    string id;
    # Number of the call request
    string number;
    # Associated case information
    ReferenceItem case;
    # Reason for the call request
    string? reason;
    # Preferred times for the call
    string[] preferredTimes;
    # Duration in minutes
    int durationMin;
    # Scheduled time for the call
    string? scheduleTime;
    # Meeting link for the scheduled call
    string? meetingLink;
    # Created date and time
    string createdOn;
    # Updated date and time
    string updatedOn;
    # State information
    ReferenceItem state;
|};

# Call requests response.
public type CallRequestsResponse record {|
    # List of call requests
    CallRequest[] callRequests;
    # Total records count
    int totalRecords;
    *entity:Pagination;
|};

# Request payload for creating a call request.
public type CallRequestCreatePayload record {|
    # Reason for the call request
    string reason;
    # Preferred UTC times for the call
    @constraint:Array {minLength: 1}
    entity:DateTime[] utcTimes;
    # Duration in minutes
    @constraint:Int {minValue: 1}
    int durationInMinutes;
|};

# Product version data.
public type ProductVersion record {|
    # ID
    string id;
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
    # Product information
    ReferenceItem? product;
    json...;
|};

# Product versions response.
public type ProductVersionsResponse record {|
    # List of product versions
    ProductVersion[] versions;
    # Total records count
    int totalRecords;
    *entity:Pagination;
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
    # State information (e.g., "Approved", "Submitted")
    ReferenceItem? state;
    # User who approved the time card
    ReferenceItem? approvedBy;
    # User who reported the time card
    ReferenceItem? reportedBy;
    # Associated project
    ReferenceItem? project;
    # Associated case
    record {|
        *ReferenceItem;
        # Case number
        string number;
    |}? case;
    json...;
|};

# Time cards response.
public type TimeCardsResponse record {|
    # List of time cards
    TimeCard[] timeCards;
    # Total records count
    int totalRecords;
    *entity:Pagination;
|};

# Conversation data.
public type Conversation record {|
    # ID
    entity:IdString id;
    # Conversation number
    string? number;
    # Initial message of the conversation
    string? initialMessage;
    # Message count
    int messageCount;
    # Created date and time
    string createdOn;
    # User who created the conversation
    string createdBy;
    # Project information
    ReferenceItem? project;
    # Case information
    ReferenceItem? case;
    # State information
    ReferenceItem? state;
    json...;
|};

# Conversations response.
public type ConversationSearchResponse record {|
    # List of conversations
    Conversation[] conversations;
    # Total records count
    int totalRecords;
    *entity:Pagination;
|};

# Request payload for searching time cards.
public type TimeCardSearchPayload record {|
    # Filter criteria
    record {
        # Start date for filtering time cards (ISO 8601 format)
        entity:Date startDate?;
        # End date for filtering time cards (ISO 8601 format)
        entity:Date endDate?;
        # List of time card states to filter (e.g., "Approved", "Submitted", etc.)
        entity:TimeCardState[] states?;
    } filters?;
    # Pagination details
    entity:Pagination pagination?;
|};

# Payload for searching updates between update levels.
public type UpdateDescriptionPayload record {|
    # Product name
    string productName;
    # Product version
    string productVersion;
    # Starting update level
    int startingUpdateLevel;
    # Ending update level
    int endingUpdateLevel;
|};

# Security advisory description.
public type SecurityAdvisoryDescription record {|
    # Advisory ID
    string id;
    # Overview
    string overview;
    # Severity
    string severity;
    # Description
    string description;
    # Impact
    string impact;
    # Solution
    string solution;
    # Notes
    string notes;
    # Credits
    string credits;
|};

# Update description information.
public type UpdateDescription record {|
    # Update level
    int updateLevel;
    # Product name
    string productName;
    # Product version
    string productVersion;
    # Channel
    string channel;
    # Update type
    string updateType;
    # Update number
    int updateNumber;
    # Description
    string? description?;
    # Instructions
    string? instructions?;
    # Bug fixes
    string? bugFixes?;
    # Files added
    string? filesAdded?;
    # Files modified
    string? filesModified?;
    # Files removed
    string? filesRemoved?;
    # Bundles info changes
    string? bundlesInfoChanges?;
    # Dependant releases
    DependantRelease[]? dependantReleases?;
    # Timestamp
    int timestamp;
    # Security advisories
    SecurityAdvisoryDescription[] securityAdvisories;
|};

# Group of update descriptions by update level.
public type UpdateLevelGroup record {|
    # Update type
    string updateType;
    # Update description levels
    UpdateDescription[] updateDescriptionLevels;
|};

# Dependant release information.
public type DependantRelease record {|
    # Repository
    string repository;
    # Release version
    string releaseVersion;
|};

# Payload for searching conversations.
public type ConversationSearchPayload record {|
    # Filter criteria
    record {
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
        entity:ConversationSortField 'field;
        # Sort order
        entity:SortOrder 'order;
    } sortBy?;
    # Pagination details
    entity:Pagination pagination?;
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

# Overall conversation statistics for a project.
public type OverallConversationStats record {|
    # Chat sessions count
    int sessionCount?;
    # Resolved chats count
    int resolvedCount?;
    # Open chats count
    int openCount?;
    # Abandoned chats count
    int abandonedCount?;
    # Total messages count
    int totalCount?;
    # Active chats count
    int activeCount?;
    # Converted chats count
    int convertedCount?;
|};

# Conversation statistics by state for a project.
public type ConversationStats record {|
    # Resolved chats count
    int resolvedCount?;
    # Open chats count
    int openCount?;
    # Abandoned chats count
    int abandonedCount?;
    # Active chats count
    int activeCount?;
|};

# Change request data.
public type ChangeRequest record {|
    # ID
    entity:IdString id;
    # Change request number
    string number;
    # Change request title
    string? title;
    # Project
    ReferenceItem? project;
    # Service request information (case)
    ReferenceItem? case;
    # Deployment information
    ReferenceItem? deployment;
    # Deployed product information
    ReferenceItem? deployedProduct;
    # Product information
    ReferenceItem? product;
    # Planned start date and time
    entity:Date? startDate;
    # Planned end date and time
    entity:Date? endDate;
    # Duration
    string? duration;
    # Indicates if the change request has a service outage
    boolean hasServiceOutage = false;
    # Impact information
    ReferenceItem? impact;
    # State information
    ReferenceItem? state;
    # Type information
    ReferenceItem? 'type;
    # Assigned engineer
    ReferenceItem? assignedEngineer;
    # Assigned team
    ReferenceItem? assignedTeam;
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
    *entity:Pagination;
|};

# Catalog data.
public type Catalog record {|
    # ID
    entity:IdString id;
    # Name of the catalog
    string name;
    # List of catalog items
    ReferenceItem[] catalogItems;
    json...;
|};

# Catalog search response.
public type CatalogSearchResponse record {|
    # List of catalogs
    Catalog[] catalogs;
    # Total records count
    int totalRecords;
    *entity:Pagination;
|};

# Request payload for searching change requests.
public type ChangeRequestSearchPayload record {|
    # Filter criteria
    record {|
        # Search query for change request number and title
        string searchQuery?;
        # List of change request state keys
        int[] stateKeys?;
        # Change request impact key
        int impactKey?;
    |} filters?;
    # Pagination details
    entity:Pagination pagination?;
|};

# Request payload for searching catalogs.
public type CatalogSearchPayload record {|
    # Pagination details
    entity:Pagination pagination?;
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
    # Indicates if the customer has permission to approve
    boolean hasCustomerApproved;
    # Indicates if the customer has permission to review
    boolean hasCustomerReviewed;
    # Internal approval details
    ReferenceItem? approvedBy;
    # Internal approval date and time
    string? approvedOn;
|};

# Change request statistics.
public type ProjectChangeRequestStatsResponse record {|
    # Total change request count
    int totalCount;
    # Active change request count (change requests that are not in rollback or closed or cancelled state)
    int activeCount;
    # Outstanding change request count
    int outstandingCount;
    # Count of change requests by state
    ReferenceItem[] stateCount;
|};

# Registry token creation payload.
public type RegistryTokenCreatePayload record {|
    # Registry token name (provided by the user)
    @constraint:String {
        pattern: {
            value: re `^[a-zA-Z0-9\-]+$`,
            message: "Name can only contain alphanumeric characters and dashes"
        }
    }
    string robotName;
    # Token Type
    registry:TokenType tokenType;
    # Created for user email
    string createdFor?;
|};

# Usage and metrics statistics for a project.
public type UsageStats record {|
    # Deployment count associated with the project
    int deploymentCount;
    # Deployed product count associated with the project
    int deployedProductCount;
    # Instance count associated with the project
    int instanceCount;
|};

# Payload for fetching instance metrics.
public type InstanceMetricsPayload record {|
    # Filter criteria
    record {|
        # Start date
        entity:Date startDate;
        # End date
        entity:Date endDate;
    |} filters;
|};

# Per-node metrics entry.
public type InstanceMetric record {|
    # ID
    string instanceId;
    # Instance key
    string instanceKey;
    # Associated project information
    ReferenceItem? project;
    # Associated deployment information
    ReferenceItem? deployment;
    # Associated product information
    ReferenceItem? product;
    # Associated deployed product information
    ReferenceItem? deployedProduct;
    # Data points ordered newest to oldest; empty if no changes in window
    entity:InstanceDataPoint[] dataPoints;
|};

# Metrics response.
public type InstanceMetricsResponse record {|
    # List of per-node metric entries
    InstanceMetric[] metrics;
    # Total number of nodes
    int totalInstances;
    # Start date of the queried range
    string startDate;
    # End date of the queried range
    string endDate;
|};

# Per-node usage entry.
public type InstanceUsageEntry record {|
    # ID
    string instanceId;
    # Instance key
    string instanceKey;
    # Associated project information
    ReferenceItem? project;
    # Associated deployment information
    ReferenceItem? deployment;
    # Associated product information
    ReferenceItem? product;
    # Associated deployed product information
    ReferenceItem? deployedProduct;
    # Summaries ordered by date; empty if no rows in the date range
    entity:InstanceSummary[] periodSummaries;
|};

# Usage summary response.
public type InstanceUsageResponse record {|
    # List of per-node usage entries
    InstanceUsageEntry[] usages;
    # Total number of nodes
    int totalInstances;
    # Start date of the queried range
    string startDate;
    # End date of the queried range
    string endDate;
|};
