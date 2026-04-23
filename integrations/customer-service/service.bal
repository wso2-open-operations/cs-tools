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

service class ErrorInterceptor {
    *http:ResponseErrorInterceptor;

    # Intercepts the response error.
    #
    # + err - The error occurred during request processing
    # + return - Internal server error response
    remote function interceptResponseError(error err) returns http:InternalServerError {
        if err is http:PayloadBindingError {
            string customError = string `${ERR_MSG_CUSTOMER_SERVICE} Failed to bind payload to the expected schema.`;
            log:printError(customError, err);
            return <http:InternalServerError>{
                body: {
                    message: customError
                }
            };
        }

        string customError = string `${ERR_MSG_CUSTOMER_SERVICE} An unexpected error occurred.`;
        log:printError(customError, err);
        return <http:InternalServerError>{
            body: {
                message: customError
            }
        };
    }
}

@display {
    label: "Customer Service",
    id: "integrations/customer-service"
}
service / on new http:Listener(9090) {

    # Response error interceptor.
    #
    # + return - ErrorInterceptor
    public function createInterceptors() returns http:Interceptor[] => [new ErrorInterceptor()];

    # Initialize the service.
    #
    function init() returns error? {
        log:printInfo("Customer Service started...");
    }

    # Search contacts for given filters.
    #
    # + filter - Contact search payload
    # + return - List of contacts or http:InternalServerError
    resource function post contacts/search(entity:ContactSearchPayload filter)
        returns http:Ok|http:InternalServerError {

        entity:Contact[]|error contacts = entity:searchContacts(filter);
        if contacts is error {
            log:printError(ERR_MSG_GET_CONTACTS, contacts);
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_GET_CONTACTS
                }
            };
        }

        return <http:Ok>{
            body: from entity:Contact {id, email, account} in contacts
                let Account? sanitizedAccount = account is entity:Account ? {id: account.id} : ()
                select {id, email, account: sanitizedAccount}
        };
    }

    # Retrieve the contact by email.
    #
    # + filter - Contact search payload
    # + return - Contact | InternalServerError | BadRequest
    resource function post contacts/find(entity:ContactSearchPayload filter)
        returns http:Ok|http:InternalServerError|http:BadRequest {

        if filter.email !is string {
            log:printWarn(ERR_MSG_CONTACTS_BAD_REQUEST);
            return <http:BadRequest>{
                body: {
                    message: ERR_MSG_CONTACTS_BAD_REQUEST
                }
            };
        }

        entity:Contact[]|error contacts = entity:searchContacts(filter);
        if contacts is error {
            log:printError(ERR_MSG_GET_CONTACTS, contacts);
            return <http:InternalServerError>{
                body: {
                    message: ERR_MSG_GET_CONTACTS
                }
            };
        }
        if contacts.length() == 0 {
            log:printError(ERR_MSG_CONTACTS_NOTFOUND);
            return <http:Ok>{
                body: {
                    isUserExist: false
                }
            };
        }
        log:printDebug(`Account ID: ${contacts[0].account?.id}`);
        entity:Account? account = contacts[0].account;

        return <http:Ok>{
            body: {
                id: contacts[0].id,
                accountId: account is entity:Account ? account.id : "",
                isUserExist: true
            }
        };
    }

    # Search deployments with filters.
    #
    # + req - HTTP request object
    # + payload - Deployment search request payload with filters
    # + return - http:Ok with deployments search results or Error
    resource function post deployments/search(http:Request req, entity:DeploymentSearchPayload payload)
        returns http:Ok|HttpErrorResponse {

        string|http:HeaderNotFoundError token = req.getHeader(USER_ID_TOKEN);
        if token is http:HeaderNotFoundError {
            log:printError(string `${ERR_MSG_CUSTOMER_SERVICE} ${ERR_MSG_INVOKER_HEADER}`);
            return <http:Unauthorized>{
                body: {
                    message: string `${ERR_MSG_CUSTOMER_SERVICE} ${ERR_MSG_INVOKER_HEADER}`
                }
            };
        }

        entity:DeploymentsResponse|error deployments = entity:searchDeployments(token, payload);
        if deployments is error {
            log:printError("Error while searching deployments", deployments);
            return mapErrorToHttp(deployments);
        }
        return <http:Ok>{body: deployments};
    }

    # Search deployed products by deployment ID.
    #
    # + req - HTTP request object
    # + payload - Deployed product search request payload with deployment ID
    # + return - http:Ok with deployed products search results or Error
    resource function post deployed\-products/search(http:Request req, entity:DeployedProductSearchPayload payload)
        returns http:Ok|HttpErrorResponse {

        string|http:HeaderNotFoundError token = req.getHeader(USER_ID_TOKEN);
        if token is http:HeaderNotFoundError {
            log:printError(string `${ERR_MSG_CUSTOMER_SERVICE} ${ERR_MSG_INVOKER_HEADER}`);
            return <http:Unauthorized>{
                body: {
                    message: string `${ERR_MSG_CUSTOMER_SERVICE} ${ERR_MSG_INVOKER_HEADER}`
                }
            };
        }

        entity:DeployedProductsResponse|error deployedProducts = entity:searchDeployedProducts(token, payload);
        if deployedProducts is error {
            log:printError("Error while searching deployed products", deployedProducts);
            return mapErrorToHttp(deployedProducts);
        }
        return <http:Ok>{body: deployedProducts};
    }
}
