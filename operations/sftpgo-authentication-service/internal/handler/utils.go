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

package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/log"
	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/models"
	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/util"
)

const (
	// Authentication result codes for SFTPGo
	AuthResultFailure    = -1
	AuthResultIncomplete = 0
	AuthResultSuccess    = 1

	// Default permissions
	PermissionList = "list"
)

var (
	generalFileMgtPermissions = []string{"upload", "list", "download", "create_dirs", "delete", "overwrite", "rename"}
)

// writeJSONResponse is a helper to standardize writing JSON responses.
func writeJSONResponse(w http.ResponseWriter, status int, data interface{}, logger *log.AppLogger) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Error("Error writing JSON response: %v", err)
	}
}

// sanitizeUsername replaces special characters in a username with underscores.
func sanitizeUsername(u string) string {
	return util.SanitizeUsername(u)
}

// validateUsername performs basic validation on username
func validateUsername(username string) error {
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}
	if len(username) > 254 { // RFC 5321 limit for email
		return fmt.Errorf("username too long")
	}
	// Check for basic injection attempts
	if strings.Contains(username, "\n") || strings.Contains(username, "\r") {
		return fmt.Errorf("invalid characters in username")
	}
	return nil
}

func (h *Handler) handleAuthStep1(resp *models.KeyIntResponse, req *models.KeyIntRequest) {
	if req.Username == "" {
		resp.AuthResult = AuthResultFailure
		resp.Instruction = "Authentication failed: Username not provided."
		return
	}

	// Validate username before initiating IdP flow
	if err := validateUsername(req.Username); err != nil {
		resp.AuthResult = AuthResultFailure
		resp.Instruction = "Authentication failed: Invalid username."
		h.logger.Error("Invalid username in auth step 1: %v", err)
		return
	}

	initFlowResp, err := h.idp.InitFlow(req.Username)
	if err != nil || initFlowResp.NextStep == nil || len(initFlowResp.NextStep.Authenticators) == 0 {
		resp.AuthResult = AuthResultFailure
		resp.Instruction = "Authentication failed: Error initiating flow."
		h.logger.Error("Failed to get initial flow from IdP: %v", err)
		return
	}

	// First step is typically identifier-first
	discoveryPayload := map[string]interface{}{
		"flowId": initFlowResp.FlowID,
		"selectedAuthenticator": map[string]interface{}{
			"authenticatorId": initFlowResp.NextStep.Authenticators[0].AuthenticatorID,
			"params":          map[string]interface{}{"username": req.Username},
		},
	}

	idpResp, err := h.idp.PostToAuthnEndpoint(discoveryPayload, util.IsInternalUser(req.Username, h.cfg.InternalUserSuffix))
	if err != nil || idpResp.FlowStatus == "FAILED" || idpResp.NextStep == nil {
		resp.AuthResult = AuthResultFailure
		resp.Instruction = "Authentication failed."
		h.logger.Error("IdP discovery failed: %v", err)
		return
	}

	h.db.SaveSession(req.RequestID, models.SessionData{
		FlowID:   idpResp.FlowID,
		NextStep: idpResp.NextStep,
	})

	resp.Instruction, resp.Questions, resp.Echos = generatePromptFromAuthenticators(*idpResp)
	resp.AuthResult = AuthResultIncomplete // Incomplete
}

func (h *Handler) handleAuthSubsequentSteps(resp *models.KeyIntResponse, req *models.KeyIntRequest, session models.SessionData) {

	if session.NextStep == nil || len(session.NextStep.Authenticators) == 0 {
		resp.AuthResult = AuthResultFailure
		resp.Instruction = "Authentication session expired or invalid."
		return
	}

	selectedAuth := session.NextStep.Authenticators[0]
	params := make(map[string]interface{})

	if len(session.NextStep.Authenticators) > 1 {
		if len(req.Answers) == 0 {
			resp.AuthResult = AuthResultFailure
			resp.Instruction = "Authentication failed: No selection provided."
			return
		}
		selection, err := strconv.Atoi(req.Answers[0])
		if err != nil || selection < 1 || selection > len(session.NextStep.Authenticators) {
			resp.AuthResult = AuthResultFailure
			resp.Instruction = "Authentication failed: Invalid selection."
			return
		}
		selectedAuth = session.NextStep.Authenticators[selection-1]
	} else {
		// Populate params from user answers
		counter := 0
		for _, param := range selectedAuth.Metadata.Params {
			if param.ParamName != "username" && len(req.Answers) > counter {
				params[param.ParamName] = req.Answers[counter]
				counter++
			}
		}
	}

	// Support BasicAuthenticator where username is mandatory
	if selectedAuth.AuthenticatorID == "QmFzaWNBdXRoZW50aWNhdG9yOkxPQ0FM" {
		if _, hasPassword := params["password"]; hasPassword && len(params) == 1 {
			params["username"] = req.Username
		}
	}

	payload := map[string]interface{}{
		"flowId": session.FlowID,
		"selectedAuthenticator": map[string]interface{}{
			"authenticatorId": selectedAuth.AuthenticatorID,
			"params":          params,
		},
	}

	idpResp, err := h.idp.PostToAuthnEndpoint(payload, util.IsInternalUser(req.Username, h.cfg.InternalUserSuffix))
	if err != nil {
		resp.AuthResult = AuthResultFailure
		resp.Instruction = "Authentication failed."
		h.logger.Error("IdP authn step failed for user %s: %v", req.Username, err)
		return
	}

	switch idpResp.FlowStatus {
	case "SUCCESS_COMPLETED":
		h.handleAuthSuccess(resp, req)
	case "INCOMPLETE", "FAIL_INCOMPLETE":
		h.db.SaveSession(req.RequestID, models.SessionData{FlowID: session.FlowID, NextStep: idpResp.NextStep})
		resp.Instruction, resp.Questions, resp.Echos = generatePromptFromAuthenticators(*idpResp)
		resp.AuthResult = AuthResultIncomplete
	default:
		resp.AuthResult = AuthResultFailure
		resp.Instruction = "Authentication failed."
		h.db.DeleteSession(req.RequestID)
	}
}

func (h *Handler) handleAuthSuccess(resp *models.KeyIntResponse, req *models.KeyIntRequest) {

	resp.AuthResult = AuthResultSuccess
	h.db.DeleteSession(req.RequestID)
}

func generatePromptFromAuthenticators(idpResp models.IdPResponse) (string, []string, []bool) {
	if idpResp.NextStep == nil || len(idpResp.NextStep.Authenticators) == 0 {
		return "No auth methods available.", nil, nil
	}
	if idpResp.NextStep.StepType == "MULTI_OPTIONS_PROMPT" && len(idpResp.NextStep.Authenticators) > 1 {
		// If multiple authenticators, prompt for selection
		instruction := "Select an authentication method:"
		selectionPrompt := ""
		for i, auth := range idpResp.NextStep.Authenticators {
			selectionPrompt += fmt.Sprintf("%d for %s ", i+1, auth.DisplayName)
		}
		selectionPrompt += "Enter selection: "
		questions := []string{selectionPrompt}
		echos := []bool{true}
		return instruction, questions, echos
	}

	auth := idpResp.NextStep.Authenticators[0]
	var questions []string
	var echos []bool
	for _, param := range auth.Metadata.Params {
		if param.ParamName != "username" {
			questions = append(questions, fmt.Sprintf("%s: ", param.DisplayName))
			echos = append(echos, !param.IsConfidential)
		}
	}
	instruction := "Please provide the following information:"
	if len(idpResp.NextStep.Messages) > 0 {
		instruction = idpResp.NextStep.Messages[0].Message
	}
	return instruction, questions, echos
}
