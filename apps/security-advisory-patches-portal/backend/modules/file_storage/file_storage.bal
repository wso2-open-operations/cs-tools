// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
//
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

# Verify that the configured file share is reachable by listing the share root.
#
# + return - `()` on success, or an error if the share cannot be accessed
public isolated function healthCheck() returns error? {
    _ = check fileClient->getDirectoryList(fileShareName);
    return;
}

# Download a file from the share using a share-relative path (`dir/file.pdf` or `file.pdf` at root).
#
# + filePath - Full path to the file within the share
# + return - File bytes, or an error if validation or Azure access fails
public isolated function downloadFile(string filePath) returns byte[]|error {
    check validatePath(filePath);

    string dirPath = getDirectoryPath(filePath);
    string fileName = getFileName(filePath);
    string encodedName = check encodePathSegments(fileName);

    if dirPath == "" {
        return check fileClient->getFileAsByteArray(fileShareName, encodedName);
    }

    string normalizedDir = check normalizePath(dirPath, forSdkCall = true);
    return check fileClient->getFileAsByteArray(fileShareName, encodedName, normalizedDir);
}
