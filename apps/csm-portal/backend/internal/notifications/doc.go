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

// Package notifications groups every outbound notification channel used by
// the CSM portal backend. Each channel gets its own config/client pair in its
// own file — e.g. email.go's EmailConfig/EmailClient — because channels are
// expected to differ in upstream auth scheme and base URL (email today via
// OAuth2 client credentials; SMS and voice/Twilio calls are expected to
// follow, likely with their own auth schemes such as Twilio's Account
// SID/Auth Token).
package notifications
