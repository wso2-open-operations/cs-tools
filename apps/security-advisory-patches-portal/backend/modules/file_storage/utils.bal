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
import ballerina/url;
import ballerinax/azure_storage_service.files as files;

# Validate file path.
#
# + path - Path to validate
# + return - Error if path is invalid
public isolated function validatePath(string path) returns error? {
    // Empty path is valid (represents root directory)
    if path == "" {
        return;
    }

    if path.length() > MAX_PATH_LENGTH {
        return error(ERR_MSG_PATH_TOO_LONG);
    }

    return;
}

# Normalizes a directory path for use with Azure SDK or general purposes.
#
# + path - Directory path to normalize
# + forSdkCall - If true, normalizes for Azure SDK (removes trailing delimiter and URL-encodes); if false, keeps trailing delimiter and does not encode
# + return - Normalized (and optionally encoded) path or error
public isolated function normalizePath(string path, boolean forSdkCall = false) returns string|error {
    if path == "" {
        return "";
    }

    // Remove leading delimiter if present
    string normalized = path.startsWith(FOLDER_DELIMITER) ? path.substring(1) : path;

    // Add trailing delimiter if not present
    if !normalized.endsWith(FOLDER_DELIMITER) {
        normalized = normalized + FOLDER_DELIMITER;
    }

    // For SDK calls, remove the trailing delimiter and URL-encode
    if forSdkCall {
        if normalized.endsWith(FOLDER_DELIMITER) {
            normalized = normalized.substring(0, normalized.length() - 1);
        }
        // URL encode for Azure API
        normalized = check encodePathSegments(normalized);
    }

    return normalized;
}

# Extract file name from full path.
#
# + filePath - Full path
# + return - File name
public isolated function getFileName(string filePath) returns string {
    string[] parts = re `${FOLDER_DELIMITER}`.split(filePath);
    return parts.length() > 0 ? parts[parts.length() - 1] : filePath;
}

# Extract directory path from full path.
#
# + fullPath - Full path
# + return - Directory path
public isolated function getDirectoryPath(string fullPath) returns string {
    if fullPath == "" {
        return "";
    }
    string normalized = fullPath.endsWith(FOLDER_DELIMITER) ? fullPath.substring(0, fullPath.length() - 1) : fullPath;
    int? idx = normalized.lastIndexOf(FOLDER_DELIMITER);
    if idx is int {
        return idx > 0 ? normalized.substring(0, idx + 1) : "";
    }
    return "";
}

# Content type mapping for file extensions.
const map<string> CONTENT_TYPE_MAP = {
    "pdf": "application/pdf",
    "png": "image/png",
    "jpg": "image/jpeg",
    "jpeg": "image/jpeg",
    "gif": "image/gif",
    "webp": "image/webp",
    "svg": "image/svg+xml",
    "txt": "text/plain",
    "json": "application/json",
    "xml": "application/xml",
    "html": "text/html",
    "htm": "text/html",
    "zip": "application/zip",
    "xls": "application/vnd.ms-excel",
    "xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    "doc": "application/msword",
    "docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
};

# Determine content type based on file extension (case-insensitive).
#
# + fileName - File name
# + return - MIME content type
public isolated function getContentType(string fileName) returns string {
    // Normalize to lower case for case-insensitive matching
    string lowerFileName = fileName.toLowerAscii();

    // Find the last dot to extract extension
    int? lastDotIndex = lowerFileName.lastIndexOf(".");
    if lastDotIndex is int && lastDotIndex < lowerFileName.length() - 1 {
        string extension = lowerFileName.substring(lastDotIndex + 1);
        string? contentType = CONTENT_TYPE_MAP[extension];
        if contentType is string {
            return contentType;
        }
    }

    return "application/octet-stream";
}

# URL encode path segments for Azure File Share API.
# Azure requires URL encoding for paths containing special characters.
#
# + path - Path to encode
# + return - URL-encoded path
public isolated function encodePathSegments(string path) returns string|error {
    string[] parts = re `${FOLDER_DELIMITER}`.split(path);
    string[] encoded = [];

    foreach string p in parts {
        string ep = check url:encode(p, "UTF-8");
        encoded.push(ep);
    }

    return string:'join(FOLDER_DELIMITER, ...encoded);
}

# Create a directory item.
#
# + directoryName - Directory name without path prefix
# + return - FileShareItem representing a directory
public isolated function createDirectoryItem(string directoryName) returns FileShareItem {
    return {
        name: directoryName,
        isFolder: true
    };
}

# Create a file item with metadata extracted from File object.
#
# + file - File object from Azure SDK
# + return - FileShareItem representing a file
public isolated function createFileItem(files:File file) returns FileShareItem {
    int fileSize = 0;
    string contentType = getContentType(file.Name);

    // Extract metadata from File Properties if available
    files:PropertiesFileItem|""? props = file.Properties;
    if props is files:PropertiesFileItem {
        fileSize = extractFileSize(props);

        string? extractedContentType = extractContentType(props);
        if extractedContentType is string {
            contentType = extractedContentType;
        }
    }

    return {
        name: file.Name,
        isFolder: false,
        size: fileSize,
        contentType: contentType
    };
}

# Extract file size from Properties object.
#
# + props - Properties object from Azure SDK
# + return - File size in bytes
public isolated function extractFileSize(files:PropertiesFileItem props) returns int {
    anydata|""? contentLengthVal = props["Content-Length"];

    if contentLengthVal is int {
        return contentLengthVal;
    } else if contentLengthVal is string {
        int|error parsedSize = int:fromString(contentLengthVal);
        return parsedSize is int ? parsedSize : 0;
    }

    return 0;
}

# Extract content type from Properties object.
#
# + props - Properties object from Azure SDK
# + return - Content type or nil
public isolated function extractContentType(files:PropertiesFileItem props) returns string? {
    anydata|""? contentTypeVal = props["Content-Type"];
    return contentTypeVal is string ? contentTypeVal : ();
}

