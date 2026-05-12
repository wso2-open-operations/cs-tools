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
configurable TokenValidatorConfig tokenValidatorConfig = ?;
configurable boolean isTokenValidatorEnabled = true;

final jwt:ValidatorConfig & readonly jwtConfig = {
    issuer: tokenValidatorConfig.issuer,
    audience: tokenValidatorConfig.audience,
    clockSkew: tokenValidatorConfig.clockSkew,
    signatureConfig: {
        jwksConfig: {url: tokenValidatorConfig.jwksEndPoint}
    }
};

# Extracts and validates user info from JWT headers in an HTTP request.
# This function is used by both the HTTP interceptor and the WebSocket upgrade resource.
#
# + req - The HTTP request containing JWT headers
# + return - UserInfoPayload on success or error on validation failure
public isolated function getUserInfoFromRequest(http:Request req) returns UserInfoPayload|error {
    string|error idToken = req.getHeader(JWT_ASSERTION_HEADER);
    if idToken is error {
        string errorMsg = "Missing invoker info header!";
        log:printError(errorMsg, idToken);
        return error(errorMsg);
    }

    string|error userIdToken = req.getHeader(USER_ID_TOKEN_HEADER);
    if userIdToken is error {
        string errorMsg = "Missing user id token info header!";
        log:printError(errorMsg, userIdToken);
        return error(errorMsg);
    }

    if !isTokenValidatorEnabled {
        [jwt:Header, jwt:Payload]|jwt:Error result = jwt:decode(idToken);
        if result is jwt:Error {
            string errorMsg = "Error while reading the Invoker info!";
            log:printError(errorMsg, result);
            return error(errorMsg);
        }

        CustomJwtPayload|error payloadData = result[1].cloneWithType(CustomJwtPayload);
        if payloadData is error {
            string errorMsg = "Malformed JWT payload!";
            log:printError(errorMsg, payloadData);
            return error(errorMsg);
        }

        return {
            email: payloadData.email,
            groups: payloadData.groups,
            userId: payloadData.userid,
            idToken: userIdToken
        };
    }

    jwt:Payload|error payload = jwt:validate(idToken, jwtConfig.cloneReadOnly());
    if payload is error {
        string errorMsg = "Invalid or expired token!";
        log:printError(errorMsg, payload);
        return error(errorMsg);
    }

    CustomJwtPayload|error payloadData = payload.cloneWithType(CustomJwtPayload);
    if payloadData is error {
        string errorMsg = "Malformed JWT payload!";
        log:printError(errorMsg, payloadData);
        return error(errorMsg);
    }

    return {
        email: payloadData.email,
        groups: payloadData.groups,
        userId: payloadData.userid,
        idToken: userIdToken
    };
}

# Extracts user info from the user ID token.
# Used when tokens are received outside of standard HTTP headers (e.g., via Sec-WebSocket-Protocol).
# The token authenticity is guaranteed by Choreo gateway having validated the access token during the handshake.
#
# + userIdToken - The user ID token (x-user-id-token)
# + return - UserInfoPayload on success or error on extraction failure
public isolated function getUserInfoFromTokens(string userIdToken) returns UserInfoPayload|error {
    [jwt:Header, jwt:Payload]|jwt:Error result = jwt:decode(userIdToken);
    if result is jwt:Error {
        string errorMsg = "Error while decoding the user ID token!";
        log:printError(errorMsg, result);
        return error(errorMsg);
    }

    CustomJwtPayload|error payloadData = result[1].cloneWithType(CustomJwtPayload);
    if payloadData is error {
        string errorMsg = "Malformed JWT payload!";
        log:printError(errorMsg, payloadData);
        return error(errorMsg);
    }

    return {
        email: payloadData.email,
        groups: payloadData.groups,
        userId: payloadData.userid,
        idToken: userIdToken
    };
}

# To handle authorization for each resource function invocation.
public isolated service class JwtInterceptor {

    *http:RequestInterceptor;

    isolated resource function default [string... path](http:RequestContext ctx, http:Request req)
        returns http:NextService|http:Unauthorized|http:InternalServerError|error? {

        if req.method == http:GET && path.length() == 1 && path[0] == "health" {
            // Skip authorization for health check endpoint
            return ctx.next();
        }
        
        UserInfoPayload|error userInfo = getUserInfoFromRequest(req);
        if userInfo is error {
            return <http:InternalServerError>{body: {message: userInfo.message()}};
        }

        ctx.set(HEADER_USER_INFO, userInfo);
        return ctx.next();
    }
}

# Response interceptor to add security headers for the response.
public isolated service class ResponseInterceptor {
    *http:ResponseInterceptor;

    isolated remote function interceptResponse(http:RequestContext ctx, http:Response res)
        returns http:NextService|error? {

        res.setHeader("X-Content-Type-Options", "nosniff");
        res.setHeader("Content-Security-Policy", "upgrade-insecure-requests");
        res.setHeader("Strict-Transport-Security", "max-age=31536000; includeSubDomains");
        return ctx.next();
    }
}
