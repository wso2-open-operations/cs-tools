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

# [Configurable] Client credentials grant type oauth2 configuration.
type ClientCredentialsOauth2Config record {|
    # OAuth2 token endpoint
    string tokenUrl;
    # OAuth2 client ID
    string clientId;
    # OAuth2 client secret
    string clientSecret;
    # OAuth2 scopes
    string[] scopes = [];
|};

public enum Role {
    USER = "user",
    ASSISTANT = "assistant"
};

# Case classification payload.
public type CaseClassificationPayload record {|
    # Chat history
    string chatHistory;
    # Environment products
    map<string[]> envProducts;
    # Region
    string region;
    # Tier
    string tier;
    # Project type ID
    string projectTypeId;
    json...;
|};

# Chat case information.
public type ChatCaseInfo record {|
    # Description
    string description;
    # Short description
    string shortDescription;
    # Product name
    string productName;
    # Product version
    string productVersion;
    # Environment
    string environment;
    # Tier
    string tier;
    # Region
    string region;
    json...;
|};

# Case classification response.
public type CaseClassificationResponse record {|
    # Issue type
    string issueType;
    # Severity level
    string severityLevel;
    # Case information
    ChatCaseInfo caseInfo;
    json...;
|};

# Chat payload for creating a chat message.
public type ChatPayload record {|
    # User message
    string message;
    # Account ID
    string accountId;
    # Conversation ID
    string conversationId;
    # Deployments with related products
    map<string[]> envProducts?;
    json...;
|};

# Conversation payload for creating a conversation thread.
public type ConversationPayload record {|
    # User message
    string message;
    # Deployments with related products
    map<string[]> envProducts?;
    # Region (optional context for recommendations)
    string region = "";
    # Support tier (optional context for recommendations)
    string tier = "";
|};

# Intent classification result for UI rendering.
public type DetectedIntent record {|
    # Global intent identifier
    string intentId;
    # Human-readable intent name
    string intentLabel;
    # Classification confidence 0-1
    float confidence;
    # Case severity (S1, S4, SR, General, Security)
    string severity = "";
    # Case creation logic to invoke
    string caseType = "";
    json...;
|};

# Available options for a slot — rendered as dropdown/selector by UI.
public type SlotOption record {|
    # Slot name, e.g. 'environment', 'product', 'version'
    string slot;
    # Human-readable label, e.g. 'Environment'
    string label;
    # Available values to choose from
    string[] options = [];
    # Input type: 'select' for dropdown, 'text' for free input
    string 'type = "select";
    json...;
|};

# Current slot-filling progress for UI rendering.
public type SlotState record {|
    # Intent being filled
    string intentId;
    # Slots already collected
    map<string> filledSlots = {};
    # Slots still needed
    string[] missingSlots = [];
    # True when all mandatory slots are filled
    boolean isComplete = false;
    # Available options per missing slot for UI dropdowns
    SlotOption[]? slotOptions = ();
    json...;
|};

# Action button rendered by the UI.
public type Action record {|
    # Action type, e.g. 'Create case'
    string 'type;
    # Button label
    string label;
    # Button style: primary | danger
    string style = "primary";
    # Data sent when button is clicked
    map<json> payload = {};
    json...;
|};

# Recommendation item.
public type RecommendationItem record {|
    # Article title
    string title;
    # Article ID
    string articleId;
    # Similarity score
    float score;
    json...;
|};

# Recommendation response.
public type RecommendationResponse record {|
    # Original query (shortDescription from classification)
    string query;
    # Top matching articles
    RecommendationItem[] recommendations = [];
    json...;
|};

# Chat response from the agent.
public type ChatResponse record {|
    # Message
    string message;
    # Session identifier
    string sessionId;
    # Conversation thread ID
    string conversationId;
    # Detected global intent, if any
    DetectedIntent? intent = ();
    # Current slot-filling progress, if any
    SlotState? slotState = ();
    # UI action buttons, if any
    Action[]? actions = ();
    # Recommendations (only included on the first chat invocation)
    RecommendationResponse? recommendations = ();
    # True when user indicates their issue is resolved
    boolean? resolved = ();
    json...;
|};

# Metadata for a conversation thread.
public type ConversationMetadata record {|
    # Unique conversation ID
    string conversationId;
    # Conversation title (auto-generated from first message)
    string title;
    # ISO timestamp when conversation was created
    string createdAt;
    # ISO timestamp when conversation was last updated
    string updatedAt;
    # Number of messages in conversation
    int messageCount = 0;
    json...;

|};

# List of all conversations for a user.
public type ConversationListResponse record {|
    # User account identifier
    string accountId;
    # List of conversations
    ConversationMetadata[] conversations = [];
    # Total number of conversations
    int totalCount = 0;
    json...;
|};

# Single chat message for UI rendering.
public type Message record {|
    # Message role: 'user' or 'assistant'
    Role role;
    # Message content
    string content;
    # ISO timestamp
    string timestamp;
    json...;
|};

# Response after deleting a conversation.
public type DeleteConversationResponse record {|
    # Success message
    string message;
    # Account ID
    string accountId;
    # Conversation ID
    string conversationId;
    json...;
|};

# Chat history response.
public type ChatHistoryResponse record {|
    # Session identifier
    string sessionId;
    # Conversation thread ID
    string conversationId;
    # Chat messages
    Message[] messages = [];
    # Total number of messages
    int messageCount = 0;
    json...;
|};

# Conversation data for recommendations.
public type ConversationData record {|
    # Chat history as a string
    string chatHistory;
    # Environment products
    map<string[]> envProducts;
    # Region
    string region;
    # Tier
    string tier;
    json...;
|};

# Recommendation request.
public type RecommendationRequest record {|
    # Chat history
    Message[] chatHistory;
    # Customer question or issue description
    ConversationData conversationData;
|};

# Conversation summary information.
public type ConversationSummaryResponse record {|
    # Account ID
    string accountId;
    # Conversation ID
    string conversationId;
    # Number of messages exchanged in the conversation
    int messagesExchanged;
    # Number of troubleshooting attempts in the conversation
    int troubleshootingAttempts;
    # Number of KB articles reviewed in the conversation
    int kbArticlesReviewed;
    json...;
|};
