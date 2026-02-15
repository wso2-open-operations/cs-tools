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
import ballerinax/azure_storage_service.files;

# Azure File Share client configuration.
public type AzureFileStorageConfig record {|
    # Azure storage account name
    string accountName;
    # Access key or SAS token
    string accessKeyOrSAS;
    # Authorization method (accessKey or SAS)
    files:AuthorizationMethod authorizationMethod;
|};

# Represents an item (file or directory) in Azure File Share.
public type FileShareItem record {|
    # Name of the item (without path prefix)
    string name;
    # Whether this item is a directory
    boolean isFolder;
    # Size in bytes (only for files)
    int size?;
    # MIME type (only for files)
    string contentType?;
|};

