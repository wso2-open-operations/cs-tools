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

package main

import (
	"fmt"
	"os"
)

// toolConfig holds this backfill's own env-driven configuration. It
// deliberately does not reuse entity-service's internal/config.Config.Validate
// (that also requires SERVICENOW_INTEGRATION_SERVICE_* fields, which are a
// completely different thing -- Choreo-proxied SN access for the live API,
// not this tool's direct OAuth client_credentials read path). This tool
// still reuses config.Config/db.NewPoolFromConfig for entity-service's own
// DB_* variables, since those describe the same database either way.
type toolConfig struct {
	// ServiceNow direct access (NOT the SERVICENOW_INTEGRATION_SERVICE_*
	// vars in entity-service/.env.example, which are Choreo-proxied and
	// unrelated to this tool's OAuth client_credentials flow).
	ServiceNowInstance     string // e.g. https://wso2sndev.service-now.com -- DEV tenant only, never prod
	ServiceNowClientID     string
	ServiceNowClientSecret string

	// CSMSyncDatabaseURL is a full postgres:// DSN for
	// digiops-cs/operations/csm-sync-service's OWN, separate database --
	// read-only here, used only to join a migrated SN sys_user's sys_id to
	// its email (see resolve.go).
	CSMSyncDatabaseURL string
}

// loadToolConfig reads this tool's own env vars and returns an error naming
// every missing one at once (not fail-fast on the first), so a misconfigured
// run doesn't require several restart-and-retry cycles just to enumerate
// what's missing.
func loadToolConfig() (toolConfig, error) {
	cfg := toolConfig{
		ServiceNowInstance:     os.Getenv("SERVICENOW_INSTANCE"),
		ServiceNowClientID:     os.Getenv("SERVICENOW_CLIENT_ID"),
		ServiceNowClientSecret: os.Getenv("SERVICENOW_CLIENT_SECRET"),
		CSMSyncDatabaseURL:     os.Getenv("CSM_SYNC_DATABASE_URL"),
	}

	var missing []string
	if cfg.ServiceNowInstance == "" {
		missing = append(missing, "SERVICENOW_INSTANCE")
	}
	if cfg.ServiceNowClientID == "" {
		missing = append(missing, "SERVICENOW_CLIENT_ID")
	}
	if cfg.ServiceNowClientSecret == "" {
		missing = append(missing, "SERVICENOW_CLIENT_SECRET")
	}
	if cfg.CSMSyncDatabaseURL == "" {
		missing = append(missing, "CSM_SYNC_DATABASE_URL")
	}
	if len(missing) > 0 {
		return toolConfig{}, fmt.Errorf("missing required env vars: %v", missing)
	}
	return cfg, nil
}
