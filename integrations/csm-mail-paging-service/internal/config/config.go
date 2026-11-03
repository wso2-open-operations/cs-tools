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

// Package config reads the service's environment configuration. Every value
// is a flat string, per the repo convention for the deployment platform.
package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"
)

// Config is the resolved configuration.
type Config struct {
	GmailClientID     string
	GmailClientSecret string
	GmailRefreshToken string
	ProcessedLabel    string
	Lookback          string
	PollInterval      time.Duration
	RulesJSON         string
	KafkaBroker       string
	KafkaConnection   string
	KafkaTopic        string
	// Port is the health endpoint's port (PORT, default 8080).
	Port string
}

var lookbackRe = regexp.MustCompile(`^[0-9]{1,3}[dhm]$`)

// Load reads and validates the environment.
func Load() (Config, error) {
	c := Config{
		GmailClientID:     os.Getenv("GMAIL_OAUTH_CLIENT_ID"),
		GmailClientSecret: os.Getenv("GMAIL_OAUTH_CLIENT_SECRET"),
		GmailRefreshToken: os.Getenv("GMAIL_OAUTH_REFRESH_TOKEN"),
		ProcessedLabel:    getenv("GMAIL_PROCESSED_LABEL", "csm-mail-paging-processed"),
		Lookback:          getenv("MAIL_LOOKBACK", "2d"),
		RulesJSON:         os.Getenv("MAIL_TRIGGER_RULES"),
		KafkaBroker:       os.Getenv("EVENT_BUS_BROKER"),
		KafkaConnection:   os.Getenv("EVENT_BUS_CONNECTION_STRING"),
		KafkaTopic:        os.Getenv("EVENT_BUS_TOPIC"),
	}
	c.Port = getenv("PORT", "8080")
	if p, err := strconv.Atoi(c.Port); err != nil || p < 1 || p > 65535 {
		return Config{}, errors.New("PORT must be a plain port number such as 8080")
	}
	interval, err := time.ParseDuration(getenv("MAIL_POLL_INTERVAL", "30s"))
	if err != nil || interval < time.Second {
		return Config{}, fmt.Errorf("MAIL_POLL_INTERVAL must be a duration of at least 1s")
	}
	c.PollInterval = interval
	if !lookbackRe.MatchString(c.Lookback) {
		return Config{}, errors.New("MAIL_LOOKBACK must look like 2d, 12h or 30m")
	}
	for name, v := range map[string]string{
		"GMAIL_OAUTH_CLIENT_ID": c.GmailClientID, "GMAIL_OAUTH_CLIENT_SECRET": c.GmailClientSecret,
		"GMAIL_OAUTH_REFRESH_TOKEN": c.GmailRefreshToken,
		"MAIL_TRIGGER_RULES":        c.RulesJSON, "EVENT_BUS_BROKER": c.KafkaBroker,
		"EVENT_BUS_CONNECTION_STRING": c.KafkaConnection, "EVENT_BUS_TOPIC": c.KafkaTopic,
	} {
		if v == "" {
			return Config{}, fmt.Errorf("%s is required", name)
		}
	}
	return c, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
