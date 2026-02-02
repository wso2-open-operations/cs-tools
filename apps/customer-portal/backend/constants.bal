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

// Error messages.
const ERR_MSG_USER_INFO_HEADER_NOT_FOUND = "User information header not found!";
const UNEXPECTED_ERROR_MSG = "An unexpected error occurred.";
const ERR_MSG_PROJECT_ACCESS_FORBIDDEN = "Access to the requested project is forbidden!";
const ERR_MSG_CASE_ACCESS_FORBIDDEN = "Access to the requested case is forbidden!";
const ERR_MSG_FETCHING_PROJECT_DETAILS = "Error retrieving project details!";
const ERR_MSG_PROJECT_ID_EMPTY = "Project ID cannot be empty!";
const ERR_MSG_CASE_ID_EMPTY = "Case ID cannot be empty!";
const ERR_LIMIT_OFFSET_INVALID = "Limit must be between 1 and 50. Offset must be a non-negative integer!";
const ERR_MSG_UNAUTHORIZED_ACCESS = "You're not authorized to access this service. Please sign in again." +
    "If the issue continues, contact support.";

// Default Pagination Values
public const int DEFAULT_OFFSET = 0;
public const int DEFAULT_LIMIT = 10;

public const ERR_STATUS_CODE = "statusCode";
public const PHONE_PATTERN_STRING = "^\\+\\d{10,14}$";
public const ERR_BODY = "body";
