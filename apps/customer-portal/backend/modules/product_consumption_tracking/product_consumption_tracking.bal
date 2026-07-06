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

# Import deployment usage data from a zip file.
#
# + email - Email address of the importing user
# + zipFile - Zip file content as bytes
# + return - Deployment usage import response or error
public isolated function importDeploymentUsage(string email, byte[] zipFile)
    returns DeploymentUsageImportResponse|error {

    ImportRequest payload = {
        email: email,
        zip: zipFile.toBase64()
    };

    return productConsumptionTrackingClient->/deployment\-usages.post(payload);

}
