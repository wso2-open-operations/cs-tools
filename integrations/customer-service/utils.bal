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
import ballerina/http;

# Maps the given error to an HTTP response.
#
# + err - The error to be mapped
# + return - HTTP error response
public isolated function mapErrorToHttp(error err) returns HttpErrorResponse {
    map<anydata|readonly> & readonly errorDetails = err.detail();
    int statusCode = extractStatusCode(errorDetails);
    string? entityMessage = extractEntityMessage(errorDetails);
    return createHttpErrorResponse(statusCode, entityMessage);
}

# Extracts the status code from error details.
#
# + errorDetails - Error details map
# + return - HTTP status code
isolated function extractStatusCode(map<anydata|readonly> & readonly errorDetails) returns int {
    anydata|readonly statusCodeValue = errorDetails["statusCode"] ?: ();
    return statusCodeValue is int ? statusCodeValue : http:STATUS_INTERNAL_SERVER_ERROR;
}

# Extracts the entity error message from error details.
#
# + errorDetails - Error details map
# + return - Entity error message or nil
isolated function extractEntityMessage(map<anydata|readonly> & readonly errorDetails) returns string? {
    anydata|readonly bodyValue = errorDetails["body"];
    if bodyValue !is map<anydata|readonly> {
        return;
    }
    anydata|readonly errorValue = bodyValue["error"];
    if errorValue !is map<anydata|readonly> {
        return;
    }
    anydata|readonly messageValue = errorValue["message"];
    return messageValue is string ? messageValue : ();
}

# Creates the appropriate HTTP error response based on status code.
#
# + statusCode - HTTP status code
# + entityMessage - Entity error message
# + return - HTTP error response
isolated function createHttpErrorResponse(int statusCode, string? entityMessage) returns HttpErrorResponse {
    string errorMessage = entityMessage ?: "An unexpected error occurred.";
    json body = {message: string `${ERR_MSG_CUSTOMER_SERVICE} ${errorMessage}`};

    match statusCode {
        http:STATUS_BAD_REQUEST => {
            return <http:BadRequest>{body};
        }
        http:STATUS_UNAUTHORIZED => {
            return <http:Unauthorized>{body};
        }
        http:STATUS_FORBIDDEN => {
            return <http:Forbidden>{body};
        }
        http:STATUS_NOT_FOUND => {
            return <http:NotFound>{body};
        }
        http:STATUS_BAD_GATEWAY => {
            return <http:BadGateway>{body};
        }
        http:STATUS_SERVICE_UNAVAILABLE => {
            return <http:ServiceUnavailable>{body};
        }
        http:STATUS_GATEWAY_TIMEOUT => {
            return <http:GatewayTimeout>{body};
        }
        _ => {
            return <http:InternalServerError>{body};
        }
    }
}
