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

import ballerina/lang.regexp;

public final regexp:RegExp REGEX_EMAIL_DOMAIN = re `^[a-zA-Z0-9][a-zA-Z0-9_\-\.]*@wso2\.com$`;
const ORGANIZATION_INTERNAL = "internal";
const ORGANIZATION_EXTERNAL = "external";
const DOMAIN_DEFAULT = "DEFAULT";
const ATTRIBUTE_PHONE_NUMBERS = "phoneNumbers";
