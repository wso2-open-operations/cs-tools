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

const MOCK_HEADER_NAME = "x-jwt-assertion";
const MOCK_ERR_MSG_MISSING_INVOKER_HEADER = "Missing invoker info header!";
const MOCK_USER_INFO_RESPONSE = {
    "id": "mockSysId",
    "email": "mock-user@wso2.com",
    "firstName": "Mock",
    "lastName": "User",
    "timezone": "Europe/London"
};
const MOCK_PROJECTS_SEARCH_RESPONSE = {
    "projects": [
        {
            "sysId": "project1",
            "name": "Project One",
            "description": "Description for Project One",
            "projectKey": "PROJ1",
            "createdOn": "2026-01-01T10:00:00Z"
        },
        {
            "sysId": "project2",
            "name": "Project Two",
            "description": "Description for Project Two",
            "projectKey": "PROJ2",
            "createdOn": "2026-01-02T11:00:00Z"
        }
    ],
    "totalRecords": 2,
    "offset": 0,
    "limit": 10
};
