// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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
import security_advisories_fileshare.file_storage;

import ballerina/http;
import ballerina/log;
import ballerina/url;
import ballerinax/azure_storage_service.files as azure_files;

# HTTP API: health check and authenticated file download for the Security Advisory Patches SPA.
@http:ServiceConfig {
    cors: {
        allowOrigins: [
            "http://localhost:3000",
            "http://127.0.0.1:3000",
            "http://patches.wso2.com:3000"
        ],
        allowMethods: ["GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"],
        allowCredentials: true,
        allowHeaders: ["x-jwt-assertion", "Authorization", "Content-Type"],
        exposeHeaders: [],
        maxAge: 84900
    }
}
service / on new http:Listener(9090) {

    # Fail fast at startup if the file share is not reachable.
    function init() {
        error? fsHealth = file_storage:healthCheck();
        if fsHealth is error {
            log:printError("Startup failed: File storage health check failed", fsHealth);
            panic fsHealth;
        }
    }

    # Liveness: returns whether the Azure file share is reachable.
    #
    # + return - `200` with status payload, or `503` if the share check fails
    resource function get health() returns http:Ok|http:ServiceUnavailable {
        error? fileStorageHealth = file_storage:healthCheck();
        if fileStorageHealth is error {
            string customError = "Health check failed: File storage unavailable";
            log:printError(customError, fileStorageHealth);
            return <http:ServiceUnavailable>{
                body: {
                    status: "unhealthy",
                    message: customError
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

    # Stream file bytes for `path` (share-relative). Query value is UTF-8 percent-decoded before validation.
    #
    # + path - Share-relative file path (query parameter)
    # + return - Raw bytes with `Content-Type` and `Content-Disposition`, or `400` / `404` / `500` with JSON body
    resource function get file(string path) returns http:Response|
            http:BadRequest|http:NotFound|http:InternalServerError {
        string|error decodedPath = url:decode(path, "UTF-8");
        if decodedPath is error {
            log:printError(string `Invalid path query encoding: ${path}`, decodedPath);
            return <http:BadRequest>{body: {message: ERR_MSG_INVALID_PATH}};
        }
        string filePath = decodedPath;

        if !filePath.matches(re `${ALLOWED_PATH_PATTERN}`) {
            log:printError(string `Invalid path format: ${filePath}`);
            return <http:BadRequest>{
                body: {message: ERR_MSG_INVALID_PATH}
            };
        }

        byte[]|error fileBytes = file_storage:downloadFile(filePath);
        if fileBytes is azure_files:NotFoundError {
            log:printWarn(string `Advisory path not found in Azure Files (404): ${filePath}`, fileBytes);
            return <http:NotFound>{body: {message: ERR_MSG_FILE_NOT_FOUND}};
        }
        if fileBytes is error {
            log:printError(string `Failed to download file: ${filePath}`, fileBytes);
            return <http:InternalServerError>{body: {message: ERR_MSG_DOWNLOAD_SECURITY_ADVISORY}};
        }

        string contentType = file_storage:getContentType(filePath);
        string fileName = file_storage:getFileName(filePath);
        http:Response response = new;
        response.setPayload(fileBytes);
        response.setHeader("Content-Type", contentType);
        response.setHeader("Content-Disposition", string `inline; filename="${fileName}"`);
        return response;
    }
}
