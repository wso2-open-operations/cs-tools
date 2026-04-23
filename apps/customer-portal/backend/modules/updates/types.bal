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

# Recommended update level.
public type RecommendedUpdateLevel record {|
    # Product name
    string product\-name;
    # Product base version
    string product\-base\-version;
    # Channel
    string channel;
    # Starting update level
    int starting\-update\-level;
    # Ending update level
    int ending\-update\-level;
    # Installed updates count
    int installed\-updates\-count;
    # Installed security updates count
    int installed\-security\-updates\-count;
    # Timestamp
    int timestamp;
    # Recommended update level
    int recommended\-update\-level;
    # Available updates count
    int available\-updates\-count;
    # Available security updates count
    int available\-security\-updates\-count;
    json...;
|};

# File changes.
public type FileChanges record {|
    # Added files
    BasicFileInfo[] added\-files;
    # Modified files
    ModifiedFileInfo[] modified\-files;
    # Removed files
    string[] removed\-files;
    # Changes in bundles info
    BundlesInfoChange[] bundles\-info\-changes;
|};

# Basic file information.
public type BasicFileInfo record {|
    # File path
    string file\-path;
    # MD5 checksum
    string md5sum;
    # SHA256 checksum
    string sha256;
    # JWT token
    string jwt;
    # Download URL
    string download\-url;
|};

# Modified file information.
public type ModifiedFileInfo record {|
    # Type
    string 'type;
    # Original file
    BasicFileInfo original\-file;
    # New file
    BasicFileInfo new\-file;
|};

# Changes in bundles info.
public type BundlesInfoChange record {|
    # Bundles info path
    string bundles\-info\-path;
    # Content change
    string content\-change;
    # Change type
    ChangeType change\-type;
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
    FileChanges file\-changes;
    # Product name
    string product\-name;
    # Product version
    string product\-version;
    # Starting update level
    string starting\-update\-level;
    # Ending update level
    string ending\-update\-level;
    # Environment
    string environment;
    # Update summary message
    string update\-summary\-message;
    # Update security message
    string update\-security\-message;
    # Total updates
    int total\-updates;
    # Total security updates
    int total\-security\-updates;
    # Applied update numbers
    int[] applied\-updates\-numbers;
    json...;
|};

# Product update level.
public type ProductUpdateLevel record {|
    # Product name
    string product\-name;
    # Product update levels
    UpdateLevel[] product\-update\-levels;
    json...;
|};

# Update level.
public type UpdateLevel record {|
    # Product base version
    string product\-base\-version;
    # Channel
    string channel;
    # Update level
    int[] update\-levels;
    json...;
|};

# Update description payload.
public type UpdateDescriptionRequest record {|
    # Product name
    string product\-name;
    # Product version
    string product\-version;
    # Channel
    string channel;
    # Starting update level
    int starting\-update\-level;
    # Ending update level
    int ending\-update\-level;
    # User email
    string user\-email;
|};

# Update description.
public type UpdateDescription record {|
    # Product name
    string product\-name;
    # Product version
    string product\-version;
    # Channel
    string channel;
    # Update level
    int update\-level;
    # Update number
    int update\-number;
    # Description
    string? description?;
    # Instructions
    string? instructions?;
    # Bug fixes
    string? bug\-fixes?;
    # Files added
    string? files\-added?;
    # Files modified
    string? files\-modified?;
    # Files removed
    string? files\-removed?;
    # Bundles info changes
    string? bundles\-info\-changes?;
    # Dependant releases
    DependantRelease[]? dependant\-releases?;
    # Update type
    string update\-type;
    # Timestamp
    int timestamp;
    # Security advisories
    SecurityAdvisoryDescription[] security\-advisories;
    json...;
|};

# Dependant release information.
public type DependantRelease record {|
    # Repository
    string repository;
    # Release version
    string release\-version;
    json...;
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
    json...;
|};
