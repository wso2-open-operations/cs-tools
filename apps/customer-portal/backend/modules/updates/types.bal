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
    # JWT token
    string jwt;
    # Platform name
    string platform\-name;
    # Platform version
    string platform\-version;
    # Product name
    string product\-name;
    # Product base version
    string product\-base\-version;
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
    int[] applied\-update\-numbers;
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
    string product\-name;
    # Product update levels
    UpdateLevel[] product\-update\-levels;
|};

# Update level.
public type UpdateLevel record {|
    # Product base version
    string product\-base\-version;
    # Channel
    string channel;
    # Update level
    int[] update\-levels;
|};
