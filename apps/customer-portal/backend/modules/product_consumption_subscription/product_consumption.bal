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

# This module handles the product consumption subscription process and license retrieval.
#
# + payload - Project status request containing email, deploymentId, and projectId
# + return - License details or error
public isolated function processLicenseDownload(LicenseDownloadPayload payload) returns License|error {
    // Get current status
    Result statusRes = check productConsumptionClient->/projects/[payload.projectId]/consumption/status.post({
        email: payload.email,
        deploymentId: payload.deploymentId
    });

    int status = statusRes.result.status;
    string? applicationId = statusRes.result.applicationId;
    string? applicationName = statusRes.result.name;
    string? applicationDescription = statusRes.result.description;

    // Create application
    if status == STATUS_PENDING {
        if applicationName is () || applicationDescription is () {
            return error("Application name and description are required for application creation.");
        }
        ApplicationCreateResponse app = check productConsumptionClient->/applications.post({name: applicationName,
            description: applicationDescription});

        applicationId = app.applicationId;

        Result _ = check productConsumptionClient->/projects/[payload.projectId].patch({
            status: STATUS_CREATED, applicationId});

        status = STATUS_CREATED;
    }
    if applicationId is () {
        return error("Application ID is required.");
    }
    // Subscribe Application 
    if status == STATUS_CREATED {
        ApplicationSubscriptionResponse _ = check productConsumptionClient->/applications/[applicationId]/subscribe
            .post(applicationId);

        Result _ = check productConsumptionClient->/projects/[payload.projectId].patch({status: STATUS_SUBSCRIBED});

        status = STATUS_SUBSCRIBED;
    }

    // Generate Credentials
    if status == STATUS_SUBSCRIBED {
        ApplicationKeyGenerationResponse creds =
            check productConsumptionClient->/applications/[applicationId]/generate\-credentials.post({});

        Result _ = check productConsumptionClient->/projects/[payload.projectId].patch({
            status: STATUS_GENERATED_CREDENTIALS,
            consumerKey: creds.consumerKey,
            consumerSecret: creds.consumerSecret
        });

        status = STATUS_GENERATED_CREDENTIALS;
    }

    // Generate Secret Keys
    if status == STATUS_GENERATED_CREDENTIALS {
        SecretKeysResponse keys = check productConsumptionClient->/generate\-secret\-keys.post({});

        Result _ = check productConsumptionClient->/projects/[payload.projectId].patch({
            status: STATUS_GENERATED_SECRET_KEYS,
            primarySecretKey: keys.primarySecretKey,
            secondarySecretKey: keys.secondarySecretKey
        });

        status = STATUS_GENERATED_SECRET_KEYS;
    }

    // Download License
    if status == STATUS_GENERATED_SECRET_KEYS {
        LicenseResponse license =
        check productConsumptionClient->/projects/[payload.projectId]/deployments/[payload.deploymentId]/license.post(
            {email: payload.email}
        );

        return license.result.license;
    }
    return error("Unexpected application status: " + status.toString());
}
