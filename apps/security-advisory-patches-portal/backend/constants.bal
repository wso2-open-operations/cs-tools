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

// Success messages
const string MSG_SERVICE_HEALTHY = "File Share backend service is running";

// Error messages
const string ERR_MSG_LIST_SECURITY_ADVISORIES = "Error occurred while retrieving security advisories";
const string ERR_MSG_DOWNLOAD_SECURITY_ADVISORY = "Error occurred while downloading security advisory";
const string ERR_MSG_INVALID_PATH = "Invalid path format";

// Path validation regex - allows alphanumeric, hyphens, underscores, dots, spaces, URL-encoded chars, and forward slashes
const string ALLOWED_PATH_PATTERN = "^[a-zA-Z0-9\\-_\\.\\s%0-9A-Fa-f/]*$";

