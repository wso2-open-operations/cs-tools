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
import ballerina/test;

http:Client testClient = check new ("http://localhost:9090");

configurable string mockIdToken = ?;

@test:Config
public function testGetLoggedInUserInformation() returns error? {
    http:Response response = check testClient->/users/me.get(headers = {MOCK_HEADER_NAME: mockIdToken});
    json payload = check response.getJsonPayload();
    test:assertEquals(response.statusCode, http:STATUS_OK);
    test:assertEquals(payload, MOCK_USER_INFO_RESPONSE);

    response = check testClient->/users/me.get();
    test:assertEquals(response.statusCode, http:STATUS_INTERNAL_SERVER_ERROR);
    test:assertEquals(response.getTextPayload(), ERR_MSG_USER_INFO_HEADER_NOT_FOUND);
}

@test:Config
public function testSearchLoggedInUserProjects() returns error? {
    // TODO: Add mock search payload
    http:Response response = check testClient->/projects/search.post({},
        headers = {MOCK_HEADER_NAME: mockIdToken});
    json payload = check response.getJsonPayload();
    test:assertEquals(response.statusCode, http:STATUS_OK);
    test:assertEquals(payload, MOCK_PROJECTS_SEARCH_RESPONSE);
}

@test:Config
public function testGetProjectDetails() returns error? {
    // TODO: Add mock project ID
    http:Response response = check testClient->/projects/[1].get(headers = {MOCK_HEADER_NAME: mockIdToken});
    json payload = check response.getJsonPayload();
    test:assertEquals(response.statusCode, http:STATUS_OK);
    test:assertEquals(payload, {}); // TODO: Add mock response

    response = check testClient->/projects/[" "].get(headers = {MOCK_HEADER_NAME: mockIdToken});
    test:assertEquals(response.statusCode, http:STATUS_BAD_REQUEST);
    test:assertEquals(response.getTextPayload(), "Project ID cannot be empty or whitespace");
}
