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
import ballerina/constraint;
import ballerina/data.jsondata;

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

# Basic product information.
public type BasicProductInfo record {|
    # Product name
    @constraint:String {minLength: 1}
    string product\-name;
    # Product base version
    @constraint:String {minLength: 1}
    string product\-base\-version;
    # Channel
    @constraint:String {minLength: 1}
    string channel;
|};

# Recommended update level.
public type RecommendedUpdateLevel record {|
    # Product name
    @jsondata:Name {value: "product-name"}
    string productName;
    # Product base version
    @jsondata:Name {value: "product-base-version"}
    string productBaseVersion;
    # Channel
    string channel;
    # Starting update level
    @jsondata:Name {value: "starting-update-level"}
    int startingUpdateLevel;
    # Ending update level
    @jsondata:Name {value: "ending-update-level"}
    int endingUpdateLevel;
    # Installed updates count
    @jsondata:Name {value: "installed-updates-count"}
    int installedUpdatesCount;
    # Installed security updates count
    @jsondata:Name {value: "installed-security-updates-count"}
    int installedSecurityUpdatesCount;
    # Timestamp
    int timestamp;
    # Recommended update level
    @jsondata:Name {value: "recommended-update-level"}
    int recommendedUpdateLevel;
    # Available updates count
    @jsondata:Name {value: "available-updates-count"}
    int availableUpdatesCount;
    # Available security updates count
    @jsondata:Name {value: "available-security-updates-count"}
    int availableSecurityUpdatesCount;
|};

# File changes.
public type FileChanges record {|
    # Added files
    @jsondata:Name {value: "added-files"}
    BasicFileInfo[] addedFiles;
    # Modified files
    @jsondata:Name {value: "modified-files"}
    ModifiedFileInfo[] modifiedFiles;
    # Removed files
    @jsondata:Name {value: "removed-files"}
    string[] removedFiles;
    # Changes in bundles info
    @jsondata:Name {value: "bundles-info-changes"}
    BundlesInfoChange[] bundlesInfoChanges;
|};

# Basic file information.
public type BasicFileInfo record {|
    # File path
    @jsondata:Name {value: "file-path"}
    string filePath;
    # MD5 checksum
    string md5sum;
    # SHA256 checksum
    string sha256;
    # JWT token
    @jsondata:Name {value: "jwt"}
    string jwtToken;
    # Download URL
    @jsondata:Name {value: "download-url"}
    string downloadUrl;
|};

# Modified file information.
public type ModifiedFileInfo record {|
    # Type
    string 'type;
    # Original file
    @jsondata:Name {value: "original-file"}
    BasicFileInfo originalFile;
    # New file
    @jsondata:Name {value: "new-file"}
    BasicFileInfo newFile;
|};

# Changes in bundles info.
public type BundlesInfoChange record {|
    # Bundles info path
    @jsondata:Name {value: "bundles-info-path"}
    string bundlesInfoPath;
    # Content change
    @jsondata:Name {value: "content-change"}
    string contentChange;
    # Change type
    @jsondata:Name {value: "change-type"}
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
    @jsondata:Name {value: "file-changes"}
    FileChanges fileChanges;
    # JWT token
    string jwt;
    # Platform name
    @jsondata:Name {value: "platform-name"}
    string platformName;
    # Platform version
    @jsondata:Name {value: "platform-version"}
    string platformVersion;
    # Product name
    @jsondata:Name {value: "product-name"}
    string productName;
    # Product base version
    @jsondata:Name {value: "product-base-version"}
    string productBaseVersion;
    # Starting update level
    @jsondata:Name {value: "starting-update-level"}
    string startingUpdateLevel;
    # Ending update level
    @jsondata:Name {value: "ending-update-level"}
    string endingUpdateLevel;
    # Environment
    string environment;
    # Update summary message
    @jsondata:Name {value: "update-summary-message"}
    string updateSummaryMessage;
    # Update security message
    @jsondata:Name {value: "update-security-message"}
    string updateSecurityMessage;
    # Total updates
    @jsondata:Name {value: "total-updates"}
    int totalUpdates;
    # Total security updates
    @jsondata:Name {value: "total-security-updates"}
    int totalSecurityUpdates;
    # Applied update numbers
    @jsondata:Name {value: "applied-update-numbers"}
    int[] appliedUpdateNumbers;
|};

# Update payload for listing updates.
public type ListUpdatePayload record {|
    *BasicProductInfo;
    # Starting update level
    @constraint:String {minLength: 1}
    string start\-update\-level;
    # Ending update level
    @constraint:String {minLength: 1}
    string end\-update\-level;
    # Hotfixes
    string[] hot\-fixes;
|};

# Product update level.
public type ProductUpdateLevel record {|
    # Product name
    @jsondata:Name {value: "product-name"}
    string productName;
    # Product update levels
    @jsondata:Name {value: "product-update-levels"}
    UpdateLevel[] productUpdateLevels;
|};

# Update level.
public type UpdateLevel record {|
    # Product base version
    @jsondata:Name {value: "product-base-version"}
    string productBaseVersion;
    # Channel
    string channel;
    # Update level
    @jsondata:Name {value: "update-level"}
    int[] updateLevels;
|};
