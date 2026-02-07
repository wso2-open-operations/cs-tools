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

# Updates payload.
public type UpdatesPayload record {|
    # Product name
    string productName;
    # Product base version
    string productBaseVersion;
    # Channel
    string channel;
    # Update level
    int updateLevel = 0;
    # Added files
    string filesAdded;
    # Modified files
    string filesModified;
    # Removed files
    string filesRemoved;
    # Changes in bundles info
    string bundlesInfoChanges;
    # Dependant releases
    DependantRelease[] dependantReleases;
    # Update descriptions
    UpdateDescription[] updateDescriptions;
    # Life cycle state
    string lifeCycleState;
    # Updates count
    int updatesCount = 0;
    # Security updates count
    int securityUpdatesCount = 0;
|};

# Dependant release.
public type DependantRelease record {|
    # Repository
    string repository;
    # Release version
    string releaseVersion;
|};

# Update description.
public type UpdateDescription record {|
    # Product name
    string productName;
    # Product base version
    string productBaseVersion;
    # Channel
    string channel;
    # Update level
    string updateLevel;
    # Update number
    string updateNumber;
    # Description
    string description;
    # Instructions
    string instructions;
    # Bug fixes
    string bugFixes;
    # Added files
    string filesAdded;
    # Modified files
    string filesModified;
    # Removed files
    string filesRemoved;
    # Changes in bundles info
    string bundlesInfoChanges;
    # Dependant releases
    DependantRelease[] dependantReleases;
    # Security advisory descriptions
    SecurityAdvisoryDescription[] securityAdvisoryDescriptions;
    # Update type
    string updateType;
    # Timestamp
    int timestamp = 0;
|};

# Security advisory description.
public type SecurityAdvisoryDescription record {|
    # ID
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
    string jwtToken;
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
    record {|
        string value;
    |} changeType;
    # Order
    int 'order;
|};

# Update response.
public type UpdateResponse record {|
    # File changes
    FileChanges fileChanges;
    # JWT token
    string jwt;
    # Platform name
    string platformName;
    # Platform version
    string platformVersion;
    # Product name
    string productName;
    # Product base version
    string productBaseVersion;
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
    int[] appliedUpdateNumbers;
|};

# Update payload for listing updates.
public type ListUpdatePayload record {|
    # Product name
    string productName;
    # Product base version
    string productBaseVersion;
    # Channel
    string channel;
    # Starting update level
    string startingUpdateLevel;
    # Ending update level
    string endingUpdateLevel;
    # Hotfixes
    string[] hotfixes;
|};

# Update search payload.
public type UpdateSearchPayload record {|
    *ListUpdatePayload;
    # Whether the update is read only
    boolean? readOnly;
    # Update level state
    string? updateLevelState;
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
