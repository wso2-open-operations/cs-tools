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

package models

import "time"

// SFTPGoUser represents a user object from SFTPGo for pre-login hook.
type SFTPGoUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

// UserVirtualFolder represents a virtual folder for a user in SFTPGo.
type UserVirtualFolder struct {
	Name        string `json:"name"`
	VirtualPath string `json:"virtual_path"`
	MappedPath  string `json:"mapped_path"`
}

// MinimalSFTPGoUser represents the minimal SFTPGo user structure for the hook response.
type MinimalSFTPGoUser struct {
	Username       string              `json:"username"`
	HomeDir        string              `json:"home_dir"`
	Permissions    map[string][]string `json:"permissions"`
	Status         int                 `json:"status"`
	VirtualFolders []UserVirtualFolder `json:"virtual_folders,omitempty"`
}

// TokenResponse represents the structure of an OAuth2 token response.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

// CustomUserExtension represents the "urn:scim:schemas:extension:custom:User" object from Asgardeo SCIM.
type CustomUserExtension struct {
	SFTPProjects string `json:"SFTPProjects"`
	SFTPFolders  string `json:"SFTPFolders"`
}

// Meta represents the "meta" object from Asgardeo SCIM.
type Meta struct {
	Created      time.Time `json:"created"`
	Location     string    `json:"location"`
	LastModified time.Time `json:"lastModified"`
	ResourceType string    `json:"resourceType"`
}

// Role represents an item in the "roles" array from Asgardeo SCIM.
type Role struct {
	Display string `json:"display"`
	Value   string `json:"value"`
}

// AsgardeoUser represents the main user object in the Asgardeo SCIM response.
type AsgardeoUser struct {
	Emails              []string            `json:"emails"`
	CustomUserExtension CustomUserExtension `json:"urn:scim:schemas:extension:custom:User"`
	Meta                Meta                `json:"meta"`
	Roles               []Role              `json:"roles"`
	ID                  string              `json:"id"`
	UserName            string              `json:"userName"`
}

// KeyIntRequest represents the incoming request for keyboard-interactive auth.
type KeyIntRequest struct {
	RequestID string   `json:"RequestID"`
	Step      int      `json:"Step"`
	Username  string   `json:"Username"`
	Answers   []string `json:"Answers"`
}

// KeyIntResponse represents the response for keyboard-interactive auth.
type KeyIntResponse struct {
	AuthResult    int      `json:"auth_result"`
	Instruction   string   `json:"instruction"`
	Questions     []string `json:"questions"`
	CheckPassword int      `json:"check_password"`
	Echos         []bool   `json:"echos"`
}

// RequiredParam represents a detailed parameter required by an authenticator.
type RequiredParam struct {
	ParamName      string `json:"param"`
	ParamType      string `json:"type"`
	IsConfidential bool   `json:"confidential"`
	DisplayName    string `json:"displayName"`
	Order          int    `json:"order"`
}

// AuthenticatorMetadata represents the metadata section of an authenticator.
type AuthenticatorMetadata struct {
	Params []RequiredParam `json:"params"`
}

// Authenticator represents an authentication method returned by the IdP.
type Authenticator struct {
	AuthenticatorID string                `json:"authenticatorId"`
	DisplayName     string                `json:"authenticator"`
	Metadata        AuthenticatorMetadata `json:"metadata"`
}

// NextStep represents the next step in the authentication flow from IdP.
type NextStep struct {
	StepType       string          `json:"stepType"`
	Authenticators []Authenticator `json:"authenticators"`
	Messages       []Message       `json:"messages"`
}

// Message represents an item in the "messages" array from the IdP.
type Message struct {
	Message string `json:"message"`
}

// IdPResponse represents the top-level response from the IdP's authentication endpoint.
type IdPResponse struct {
	FlowStatus        string    `json:"flowStatus"`
	FlowID            string    `json:"flowId"`
	NextStep          *NextStep `json:"nextStep"`
	AuthorizationCode string    `json:"authorizationCode"`
	Error             string    `json:"error"`
}

// SessionData stores state for each ongoing authentication flow.
type SessionData struct {
	FlowID   string    `json:"flowId"`
	NextStep *NextStep `json:"nextStep"`
}

// FolderResponse is the successful response from the subscription API.
type FolderResponse struct {
	IsValidCustomer bool     `json:"isValidCustomer"`
	ProjectKeys     []string `json:"projectKeys"`
}

// ErrorMessage represents a generic JSON error message.
type ErrorMessage struct {
	Message string `json:"message"`
}
