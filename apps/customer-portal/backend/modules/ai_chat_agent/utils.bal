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

# Build a recommendation request from a single user/assistant and fetch recommendations.
#
# + userMessage - User's message
# + assistantMessage - Assistant's response
# + envProducts - Environment products context
# + region - Customer region
# + tier - Support tier
# + return - Recommendation response or error
public isolated function getRecommendations(string userMessage, string assistantMessage,
        map<string[]> envProducts, string region, string tier) returns RecommendationResponse|error {

    Message user = {role: USER, content: userMessage, timestamp: ""};
    Message assistant = {role: ASSISTANT, content: assistantMessage, timestamp: ""};
    ConversationData conversationData = {
        chatHistory: string `${user.role}: ${user.content}\n${assistant.role}: ${assistant.content}`,
        envProducts,
        region,
        tier
    };

    RecommendationRequest recommendationRequest = {chatHistory: [user, assistant], conversationData};
    return getRecommendation(recommendationRequest);
}

