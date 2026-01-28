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
	"path/filepath"
	"strings"

	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/config"
	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/log"
	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/models"
	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/service"
	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/util"
)

// Handler holds all the services and dependencies for the HTTP handlers.
type Handler struct {
	cfg          *config.Config
	logger       *log.AppLogger
	db           *service.DBService
	idp          *service.IdPService
	sftpgo       *service.SFTPGoService
	subscription *service.SubscriptionService
}

// NewHandler creates a new Handler with all its dependencies.
func NewHandler(
	cfg *config.Config,
	logger *log.AppLogger,
	db *service.DBService,
	idp *service.IdPService,
	sftpgo *service.SFTPGoService,
	subscription *service.SubscriptionService,
) *Handler {
	return &Handler{
		cfg:          cfg,
		logger:       logger,
		db:           db,
		idp:          idp,
		sftpgo:       sftpgo,
		subscription: subscription,
	}
}

func (h *Handler) auditLog(r *http.Request, username string, action string, status string, detail string) {
	h.logger.Info("AUDIT: ip=%s user=%s action=%s status=%s detail=%s", r.RemoteAddr, username, action, status, detail)
}

// PreLoginHook handles SFTPGo's pre-login hook to provision users and folders.
// It validates the user against the IdP and configures appropriate permissions and virtual folders.
func (h *Handler) PreLoginHook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.authenticate(r, w) {
		return
	}
	defer r.Body.Close()

	var u models.SFTPGoUser
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		h.logger.Error("Invalid payload in pre-login hook: %v", err)
		h.auditLog(r, "unknown", "pre-login-hook", "failure", "invalid payload")
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	if u.Username == "" {
		h.logger.Error("No username in pre-login hook payload.")
		h.auditLog(r, "unknown", "pre-login-hook", "failure", "no username")
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if err := validateUsername(u.Username); err != nil {
		h.logger.Error("Invalid username '%s': %v", u.Username, err)
		h.auditLog(r, u.Username, "pre-login-hook", "failure", "invalid username")
		http.Error(w, "Invalid username", http.StatusBadRequest)
		return
	}

	h.logger.Debug("Processing pre-login hook for username: %s", u.Username)

	isInternal := util.IsInternalUser(u.Username, h.cfg.InternalUserSuffix)
	providedProjectKey := ""

	if isInternal {
		parts := strings.Split(u.Username, "@")
		if len(parts) == 3 {
			providedProjectKey = parts[0]
			username := parts[1] + "@" + parts[2]
			h.logger.Debug("Extracted project key: %s, username: %s", providedProjectKey, username)
			u.Username = username
		}
	}

	asgUser, err := h.idp.GetAsgardeoUser(u.Username)
	if err != nil {
		h.logger.Debug("User '%s' not found in IdP, denying access.", u.Username)

		// Return anonymous user to prevent username enumeration attacks.
		// Consistent response regardless of IdP user existence protects valid usernames.
		anonymousUsername := "anonymous_user@wso2.org"
		home := filepath.Join(h.cfg.DIRPath, sanitizeUsername(anonymousUsername))
		perms := make(map[string][]string)
		var vfs []models.UserVirtualFolder
		perms["/"] = []string{PermissionList}
		res := models.MinimalSFTPGoUser{
			Username:       anonymousUsername,
			HomeDir:        home,
			Permissions:    perms,
			Status:         1,
			VirtualFolders: vfs,
		}

		h.auditLog(r, u.Username, "pre-login-hook", "denied", "user not found in IdP, anonymous user returned")
		writeJSONResponse(w, http.StatusOK, res, h.logger)
		return
	}

	var isInternalRole bool
	if isInternal {
		for _, role := range asgUser.Roles {
			if role.Display == h.cfg.CheckRole {
				isInternalRole = true
				break
			}
		}
	}
	h.logger.Debug("User '%s' internal check: Is Internal? %t, Has CheckRole? %t", u.Username, isInternal, isInternalRole)

	home := filepath.Join(h.cfg.DIRPath, sanitizeUsername(u.Username))
	perms := make(map[string][]string)
	var vfs []models.UserVirtualFolder

	perms["/"] = []string{PermissionList}
	folders := []string{}
	if isInternalRole {
		h.logger.Debug("User '%s' identified as internal user. Checking provided project key.", u.Username)
		if providedProjectKey != "" {
			if h.subscription.IsValidProjectKey(providedProjectKey) {
				h.logger.Debug("Project key '%s' successfully validated for user '%s'.", providedProjectKey, u.Username)
				folders = []string{providedProjectKey}
			} else {
				h.logger.Warn("Validation failed for project key '%s' provided by user '%s'. No project folders assigned.", providedProjectKey, u.Username)
				folders = []string{}
			}
		} else {
			h.logger.Debug("No project key provided for internal user '%s'. Defaulting to no project folders.", u.Username)
			folders = []string{}
		}
	} else {
		h.logger.Debug("User '%s' identified as external user. Checking subscription folders.", u.Username)
		folders = h.subscription.GetUserFolderList(u.Username)
	}
	if len(folders) == 0 && asgUser.CustomUserExtension.SFTPFolders != "" {
		folders = strings.Split(strings.ToLower(asgUser.CustomUserExtension.SFTPFolders), ",")
		h.logger.Debug("User '%s' has custom folders: %v", u.Username, folders)
	}

	if len(folders) == 0 {
		h.logger.Debug("No folders for external user '%s'. Returning no content.", u.Username)
		h.auditLog(r, u.Username, "pre-login-hook", "denied", "no folders found")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := h.sftpgo.ProvisionFolders(folders); err != nil {
		h.logger.Error("Failed to provision folders for user '%s': %v", u.Username, err)
	}

	for _, folder := range folders {
		virtualPath := "/" + folder
		mappedPath := filepath.Join(h.cfg.FolderPath, folder)
		perms[virtualPath] = generalFileMgtPermissions
		vfs = append(vfs, models.UserVirtualFolder{Name: folder, VirtualPath: virtualPath, MappedPath: mappedPath})
	}

	res := models.MinimalSFTPGoUser{
		Username:       u.Username,
		HomeDir:        home,
		Permissions:    perms,
		Status:         1,
		VirtualFolders: vfs,
	}

	detail := fmt.Sprintf("isInternalRole: %t", isInternalRole)
	if len(vfs) > 0 {
		var folderNames []string
		for _, v := range vfs {
			folderNames = append(folderNames, v.Name)
		}
		detail += fmt.Sprintf(" folders: %s", strings.Join(folderNames, ","))
	}
	h.auditLog(r, u.Username, "pre-login-hook", "success", detail)

	if h.logger.IsDebugEnabled() {
		resBody, _ := json.Marshal(res)
		h.logger.Debug("Pre-login hook response for user '%s': %s", u.Username, string(resBody))
	}

	writeJSONResponse(w, http.StatusOK, res, h.logger)
}

// AuthHandler handles the keyboard-interactive authentication flow.
// It integrates with the IdP to perform multi-step authentication.
func (h *Handler) AuthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.authenticate(r, w) {
		return
	}
	defer r.Body.Close()

	var req models.KeyIntRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	h.logger.Debug("AuthHandler: step=%d id=%s user=%s", req.Step, req.RequestID, req.Username)
	resp := &models.KeyIntResponse{}

	if req.Step == 1 {
		h.handleAuthStep1(resp, &req)
	} else {
		session, err := h.db.GetSession(req.RequestID)
		if err != nil {
			resp.AuthResult = AuthResultFailure
			resp.Instruction = "Authentication session expired or invalid."
			h.logger.Error("Failed to get session for ID %s: %v", req.RequestID, err)
		} else {
			h.handleAuthSubsequentSteps(resp, &req, session)
		}
	}

	writeJSONResponse(w, http.StatusOK, resp, h.logger)

	status := "incomplete"
	if resp.AuthResult == AuthResultSuccess {
		status = "success"
	} else if resp.AuthResult == AuthResultFailure {
		status = "failure"
	}
	detail := fmt.Sprintf("step=%d id=%s", req.Step, req.RequestID)
	detail += fmt.Sprintf(" instruction=%s questions=%s", resp.Instruction, strings.Join(resp.Questions, ","))
	h.auditLog(r, req.Username, "auth-hook", status, detail)
}

// authenticate checks if the request has a valid API key.
// It returns true if authentication is successful or not configured, false otherwise.
func (h *Handler) authenticate(r *http.Request, w http.ResponseWriter) bool {
	if h.cfg.HookAPIKey == "" {
		return true
	}

	apiKey := r.Header.Get("API-Key")
	if apiKey != h.cfg.HookAPIKey {
		h.logger.Warn("Unauthorized access attempt from %s: invalid or missing API key", r.RemoteAddr)
		h.auditLog(r, "unknown", r.URL.Path, "unauthorized", "invalid or missing api key")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}
