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

import security_advisories_fileshare.file_storage;

import ballerina/http;
import ballerina/log;

@display {
    label: "Security Advisories File Share Backend",
    id: "security-advisories/files-backend"
}

service / on new http:Listener(9090) {

    # Service initialization - Runs health checks if enabled.
    function init() {
            error? fsHealth = file_storage:healthCheck();
            if fsHealth is error {
                log:printError("Startup failed: File storage health check failed", fsHealth);
                panic fsHealth;
            }
    }

    # Health check endpoint - Verifies service and all dependencies.
    #
    # + return - OK response if healthy, Service Unavailable if any dependency fails
    resource function get health() returns http:Ok|http:ServiceUnavailable {
        // Check file storage connectivity
        error? fileStorageHealth = file_storage:healthCheck();
        if fileStorageHealth is error {
            log:printError("Health check failed: File storage unavailable", fileStorageHealth);
            return <http:ServiceUnavailable>{
                body: {
                    status: "unhealthy",
                    message: "File storage service is unavailable"
                }
            };
        }

        return <http:Ok>{
            body: {
                status: "healthy",
                message: MSG_SERVICE_HEALTHY,
                dependencies: {
                    fileStorage: "healthy"
                }
            }
        };
    }

    # Get directory content from Azure File Share
    #
    # + path - Optional directory path to list contents from
    # + return - Array of FileShareItem or error response
    resource function get directory\-content(string? path) returns file_storage:FileShareItem[]|
    http:BadRequest|http:InternalServerError {
        // Use empty string if path is not provided
        string dirPath = path ?: "";
        
        // Validate path format
        if !dirPath.matches(re `${ALLOWED_PATH_PATTERN}`) {
            log:printError(string `Invalid path format: ${dirPath}`);
            return <http:BadRequest>{
                body: {message: ERR_MSG_INVALID_PATH}
            };
        }
        
        file_storage:FileShareItem[]|error items = file_storage:listItems(dirPath);
        if items is error {
            log:printError(string `Failed to list items at path: ${dirPath}`, items);
            return <http:InternalServerError>{
                body: {message: ERR_MSG_LIST_SECURITY_ADVISORIES}
            };
        }

        return items;
    }

    # Get file content from Azure File Share
    #
    # + path - Full path to the file
    # + return - HTTP response with file content or error
    resource function get file(string path) returns http:Response|
    http:BadRequest|http:InternalServerError {
        // Validate path format
        if !path.matches(re `${ALLOWED_PATH_PATTERN}`) {
            log:printError(string `Invalid path format: ${path}`);
            return <http:BadRequest>{
                body: {message: ERR_MSG_INVALID_PATH}
            };
        }

        byte[]|error fileBytes = file_storage:downloadFile(path);
        if fileBytes is error {
            log:printError(string `Failed to download file: ${path}`, fileBytes);
            return <http:InternalServerError>{body: {message: ERR_MSG_DOWNLOAD_SECURITY_ADVISORY}};
        }

        string contentType = file_storage:getContentType(path);
        string fileName = file_storage:getFileName(path);
        http:Response response = new;
        response.setPayload(fileBytes);
        response.setHeader("Content-Type", contentType);
        response.setHeader("Content-Disposition", string `inline; filename="${fileName}"`);
        return response;
    }
}
