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
import ballerina/http;
import ballerina/jwt;
import ballerina/log;

public configurable AppRoles authorizedRoles = ?;
configurable AuthorizationConfig authorizationConfig = ?;

final jwt:ValidatorConfig & readonly jwtConfig = {
    issuer: authorizationConfig.jwtIssuer,
    audience: authorizationConfig.jwtAudience,
    clockSkew: 60,
    signatureConfig: {
        jwksConfig: {url: authorizationConfig.jwksEndPoint}
    }
};

# To handle authorization for each resource function invocation.
public isolated service class JwtInterceptor {

    *http:RequestInterceptor;

    isolated resource function default [string... path](http:RequestContext ctx, http:Request req)
        returns http:NextService|http:Unauthorized|http:Forbidden|http:InternalServerError|error? {

        string|error idToken = req.getHeader(JWT_ASSERTION_HEADER);
        if idToken is error {
            string errorMsg = "Missing invoker info header!";
            log:printError(errorMsg, idToken);
            return <http:InternalServerError>{
                body: {
                    message: errorMsg
                }
            };
        }

        // TODO: Remove this if the token issuer issue get resolved.
        string|error userIdToken = req.getHeader(USER_ID_TOKEN_HEADER);
        if userIdToken is error {
            string errorMsg = "Missing user id token info header!";
            log:printError(errorMsg, userIdToken);
            return <http:InternalServerError>{
                body: {
                    message: errorMsg
                }
            };
        }

        [jwt:Header, jwt:Payload]|jwt:Error result = jwt:decode(idToken);
        if result is jwt:Error {
            string errorMsg = "Error while reading the Invoker info!";
            log:printError(errorMsg, result);
            return <http:InternalServerError>{body: {message: errorMsg}};
        }

        jwt:Payload|error payload = jwt:validate(idToken, jwtConfig.cloneReadOnly());
        if payload is error {
            string errorMsg = "Invalid JWT";
            log:printError(errorMsg, payload);
            return <http:Unauthorized>{body: {message: errorMsg}};
        }

        jwt:Payload jwtPayload = result[1];
        CustomJwtPayload|error payloadData = jwtPayload.cloneWithType(CustomJwtPayload);
        if payloadData is error {
            string errorMsg = "Malformed JWT payload!";
            log:printError(errorMsg, payloadData);
            return <http:InternalServerError>{body: {message: errorMsg}};
        }

        UserInfoPayload userInfo = {
            email: payloadData.email,
            groups: payloadData.groups,
            userId: payloadData.userid,
            idToken: userIdToken
        };

        ctx.set(HEADER_USER_INFO, userInfo);
        return ctx.next();
    }
}
