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
import ballerina/url;

# Validate share-relative path syntax and length before Azure SDK calls.
#
# + path - Path to validate
# + return - `()` if valid, or an error if the path is empty, too long, or malformed
public isolated function validatePath(string path) returns error? {
    if path == "" {
        return error(ERR_MSG_INVALID_PATH);
    }

    if path.length() > MAX_PATH_LENGTH {
        return error(ERR_MSG_PATH_TOO_LONG);
    }

    if path.includes("\\") || path.includes("\u{0000}") {
        return error(ERR_MSG_INVALID_PATH);
    }

    if path.startsWith(FOLDER_DELIMITER) || path.endsWith(FOLDER_DELIMITER) || path.includes("//") {
        return error(ERR_MSG_INVALID_PATH);
    }

    string[] segments = re `${FOLDER_DELIMITER}`.split(path);
    foreach string segment in segments {
        if segment == "" || segment == "." || segment == ".." {
            return error(ERR_MSG_INVALID_PATH);
        }
    }

    return;
}

# Normalize a directory path for Azure SDK calls or internal use.
#
# + path - Directory path (may include leading `/`)
# + forSdkCall - When `true`, strip trailing slash and URL-encode each segment for the Azure Files REST API
# + return - Normalized path, or an error from encoding
public isolated function normalizePath(string path, boolean forSdkCall = false) returns string|error {
    if path == "" {
        return "";
    }

    string normalized = path.startsWith(FOLDER_DELIMITER) ? path.substring(1) : path;

    if !forSdkCall && !normalized.endsWith(FOLDER_DELIMITER) {
        normalized = normalized + FOLDER_DELIMITER;
    }

    if forSdkCall {
        normalized = check encodePathSegments(normalized);
    }

    return normalized;
}

# Return the last path segment (file name), ignoring a trailing `/`.
#
# + filePath - Full share-relative path
# + return - File name portion
public isolated function getFileName(string filePath) returns string {
    if filePath == "" {
        return "";
    }
    string fp = filePath.endsWith(FOLDER_DELIMITER) ? filePath.substring(0, filePath.length() - 1) : filePath;
    int? slash = fp.lastIndexOf(FOLDER_DELIMITER);
    if slash is int && slash < fp.length() - 1 {
        return fp.substring(slash + 1, fp.length());
    }
    return fp;
}

# Return the directory prefix of a full path, including trailing `/`, or `""` for root-level files.
#
# + fullPath - Full share-relative path
# + return - Directory path prefix
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

# Extension to MIME map used by `getContentType`.
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

# Resolve a MIME type from the file extension (case-insensitive); unknown extensions default to `application/octet-stream`.
#
# + fileName - File name or path ending with a name that includes an extension
# + return - Content type string
public isolated function getContentType(string fileName) returns string {
    string lowerFileName = fileName.toLowerAscii();

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

# URL-encode each `/`-separated segment for Azure File Share REST paths (skips empty segments).
#
# + path - Unencoded path using `/` as the delimiter
# + return - Encoded path, or an error from `url:encode`
public isolated function encodePathSegments(string path) returns string|error {
    string[] parts = re `${FOLDER_DELIMITER}`.split(path);
    string[] encoded = [];

    foreach string p in parts {
        if p.length() == 0 {
            continue;
        }
        encoded.push(check url:encode(p, "UTF-8"));
    }

    return string:'join(FOLDER_DELIMITER, ...encoded);
}
