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
import security_advisories_fileshare.authorization;
import security_advisories_fileshare.file_storage;

import ballerina/http;
import ballerina/log;
import ballerina/url;
import ballerinax/azure_storage_service.files as azure_files;

public isolated service class ErrorInterceptor {
    *http:ResponseErrorInterceptor;

    isolated remote function interceptResponseError(error err, http:RequestContext ctx) returns http:BadRequest|error {
        if err is http:PayloadBindingError {
            string customError = "Payload binding failed!";
            log:printError(customError, err);
            return <http:BadRequest>{
                body: {
                    message: customError
                }
            };
        }
        return err;
    }
}

@display {
    label: "Security Advisories File Share Backend",
    id: "security-advisories/files-backend"
}

service http:InterceptableService / on new http:Listener(9090) {

    # Request interceptors.
    #
    # + return - Interceptor chain executed for every request
    public function createInterceptors() returns http:Interceptor[] {
        return [new authorization:JwtInterceptor(), new ErrorInterceptor()];
    }

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

    # Stream file bytes for a share-relative path carried in one URL path segment (`id`).
    #
    #
    # + ctx - Request context
    # + id - URL-decoded share-relative file path (after the framework decodes the path segment)
    # + return - Raw bytes with `Content-Type` and `Content-Disposition`, or `400` / `404` / `500` with JSON body
    resource function get files/[string id](http:RequestContext ctx) returns http:Response|http:BadRequest|
            http:NotFound|http:InternalServerError {

        authorization:CustomJwtPayload|error userInfo = ctx.getWithType(authorization:HEADER_USER_INFO);
        if userInfo is error {
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_USER_INFO_HEADER_NOT_FOUND
                }
            };
        }

        string|error decodedPath = url:decode(id, "UTF-8");
        if decodedPath is error {
            log:printError(string `Invalid file id encoding: ${id}`, decodedPath);
            return <http:BadRequest>{body: {message: ERR_MSG_INVALID_PATH}};
        }
        string filePath = decodedPath;

        error? pathValidation = file_storage:validatePath(filePath);
        if pathValidation is error {
            log:printError(string `Invalid path format: ${filePath}`, pathValidation);
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
        string|error encodedFileName = url:encode(fileName, "UTF-8");
        if encodedFileName is error {
            log:printError("Failed to encode filename for Content-Disposition", encodedFileName);
            return <http:InternalServerError>{body: {message: ERR_MSG_DOWNLOAD_SECURITY_ADVISORY}};
        }
        http:Response response = new;
        response.setPayload(fileBytes);
        response.setHeader("Content-Type", contentType);
        response.setHeader("Content-Disposition", string `inline; filename*=UTF-8''${encodedFileName}`);
        return response;
    }
}
