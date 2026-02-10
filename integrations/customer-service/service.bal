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

import customer_service.entity;

import ballerina/cache;
import ballerina/http;
import ballerina/log;

final cache:Cache cache = new ({
    capacity: 2000,
    defaultMaxAge: 1800.0,
    cleanupInterval: 900.0
});

@display {
    label: "Customer Service",
    id: "integrations/customer-service"
}
service / on new http:Listener(9090) {

    # Initialize the service.
    #
    function init() returns error? {
        log:printInfo("Customer Service started...");
    }

    # Search contacts for given filters.
    #
    # + filter - Contact search payload
    # + return - List of contacts or http:InternalServerError
    resource function post contacts/search(entity:ContactSearchPayload filter) returns Contact[]|http:InternalServerError {
        entity:Contact[]|error contacts = entity:searchContacts(filter);
        if contacts is error {
            log:printError(ERR_MSG_GET_CONTACTS, contacts);
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_GET_CONTACTS
                }
            };
        }
        return from entity:Contact {id, email, account} in contacts
            let Account? sanitizedAccount = account is entity:Account ? {id: account.id} : ()
            select {id, email, account: sanitizedAccount};
    }
}
