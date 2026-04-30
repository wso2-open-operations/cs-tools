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
import ballerinax/azure_storage_service.files as files;

# Perform health check by verifying connectivity to Azure File Share.
#
# + return - Error if health check fails, nil if successful
public isolated function healthCheck() returns error? {
    // Attempt to list directories at root to verify connectivity
    // Using check to propagate error to service level
    _ = check fileClient->getDirectoryList(fileShareName);
    return;
}

# List immediate children (directories and files) for a given path in Azure File Share.
#
# + path - Optional directory path to list contents from
# + return - Array of FileShareItem or error
public isolated function listItems(string path = "") returns FileShareItem[]|error {
    // Normalize and encode path for SDK call
    string normalizedPath = check normalizePath(path, forSdkCall = true);

    FileShareItem[] items = [];

    // List directories - empty result is not an error
    files:DirectoryList|error dirResult;
    if normalizedPath == "" {
        dirResult = fileClient->getDirectoryList(fileShareName);
    } else {
        dirResult = fileClient->getDirectoryList(fileShareName, normalizedPath);
    }

    if dirResult is files:DirectoryList {
        files:Directory|files:Directory[] directories = dirResult.Directory;
        files:Directory[] dirArray = directories is files:Directory[] ? directories : [directories];

        foreach files:Directory d in dirArray {
            items.push(createDirectoryItem(d.Name));
        }
    }
    // Note: If dirResult is error, we ignore it (treat empty as success)

    // List files
    files:FileList|error fileResult;
    if normalizedPath == "" {
        fileResult = fileClient->getFileList(fileShareName);
    } else {
        fileResult = fileClient->getFileList(fileShareName, normalizedPath);
    }

    if fileResult is files:FileList {
        files:File|files:File[] fileList = fileResult.File;
        files:File[] fileArray = fileList is files:File[] ? fileList : [fileList];

        foreach files:File f in fileArray {
            items.push(createFileItem(f));
        }
    }
    // Note: If fileResult is error, we ignore it (treat empty as success)

    // If both directory and file listing failed, return the file error as it's more likely to be meaningful
    if dirResult is error && fileResult is error {
        return fileResult;
    }

    return items;
}

# Download file content from Azure File Share.
#
# + filePath - Full path to the file
# + return - File content as byte array or error
public isolated function downloadFile(string filePath) returns byte[]|error {
    check validatePath(filePath);

    string dirPath = getDirectoryPath(filePath);
    string fileName = getFileName(filePath);

    // Encode file name for Azure API
    string encodedName = check encodePathSegments(fileName);

    if dirPath == "" {
        return check fileClient->getFileAsByteArray(fileShareName, encodedName);
    }

    // Normalize and encode directory path for SDK call
    string normalizedDir = check normalizePath(dirPath, forSdkCall = true);

    return check fileClient->getFileAsByteArray(fileShareName, encodedName, normalizedDir);
}
