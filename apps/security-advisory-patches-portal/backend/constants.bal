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

# Message returned in `GET /health` when the file share dependency is healthy.
const string MSG_SERVICE_HEALTHY = "File Share backend service is running";

# Generic error message for `GET /file` when download fails for reasons other than Azure 404.
const string ERR_MSG_DOWNLOAD_SECURITY_ADVISORY = "Error occurred while downloading security advisory";

# Message for `GET /file` when Azure reports the path as missing (`NotFoundError`).
const string ERR_MSG_FILE_NOT_FOUND = "File or directory path not found in file share";

# Message for `GET /file` when the `path` query is missing, malformed, or fails percent-decoding.
const string ERR_MSG_INVALID_PATH = "Invalid path format";

# Allowlist regex for the `path` query (alphanumeric, hyphen, underscore, dot, space, slash, percent-encoding).
const string ALLOWED_PATH_PATTERN = "^[a-zA-Z0-9\\-_\\.\\s%0-9A-Fa-f/]*$";
