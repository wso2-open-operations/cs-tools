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

# Client retry configuration
const TIMEOUT = 300.0d;
const RETRY_COUNT = 3;
const RETRY_INTERVAL = 2.0d;
const RETRY_BACKOFF_FACTOR = 2.0f;
const RETRY_MAX_INTERVAL = 20.0d;

# Email constraint for email validation.
public final string:RegExp EMAIL_CONSTRAINT =
    re `^[A-Za-z0-9]+([._%+-][A-Za-z0-9]+)*@[A-Za-z0-9]+([.-][A-Za-z0-9]+)*\.[A-Za-z]{2,}$`;

# Sales entity default pagination values
public const SALES_DEFAULT_RECORD_LIMIT = 100;
public const SALES_DEFAULT_RECORD_OFFSET = 0;
public const SALES_MAX_RECORD_LIMIT = 1000;

# CS Entity default pagination values
public const CS_DEFAULT_OFFSET = 0;
public const CS_DEFAULT_LIMIT = 20;
