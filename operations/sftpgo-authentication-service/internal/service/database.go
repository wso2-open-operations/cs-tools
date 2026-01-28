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

package service

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/config"

	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/log"
	"github.com/wso2-open-operations/cs-tools/operations/sftpgo-authentication-service/internal/models"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
)

var ErrSessionNotFound = errors.New("session not found or expired")

// DBService handles all database interactions.
type DBService struct {
	db     *sql.DB
	logger *log.AppLogger
}

// NewDBService creates a new DBService and establishes a database connection.
func NewDBService(cfg *config.Config, logger *log.AppLogger) (*DBService, error) {
	if cfg.DBConnString == "" {
		return nil, logger.Errorf("DB_CONN_STRING is not set. Service requires a database for session persistence.")
	}

	connStr := cfg.DBConnString
	if !strings.Contains(connStr, "parseTime=true") {
		if strings.Contains(connStr, "?") {
			connStr += "&parseTime=true"
		} else {
			connStr += "?parseTime=true"
		}
	}

	db, err := sql.Open("mysql", connStr)
	if err != nil {
		return nil, logger.Errorf("failed to open database connection: %v", err)
	}

	if err = db.Ping(); err != nil {
		return nil, logger.Errorf("failed to ping database: %v", err)
	}

	// Connection Pooling Configuration
	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxLifetime(cfg.DBConnMaxLifetime)

	logger.Info("Successfully connected to the MySQL database.")
	return &DBService{db: db, logger: logger}, nil
}

// SaveSession saves session data to the database.
func (s *DBService) SaveSession(requestID string, data models.SessionData) error {
	if s.db == nil {
		s.logger.Warn("Database not configured. Cannot save session %s.", requestID)
		return nil // Not a fatal error
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return s.logger.Errorf("failed to marshal session data: %v", err)
	}

	expiresAt := time.Now().Add(15 * time.Minute)
	query := `INSERT INTO sessions (request_id, session_data, expires_at)
              VALUES (?, ?, ?)
              ON DUPLICATE KEY UPDATE session_data = VALUES(session_data), expires_at = VALUES(expires_at), updated_at = NOW()`

	_, err = s.db.Exec(query, requestID, jsonData, expiresAt)
	if err != nil {
		return s.logger.Errorf("failed to save session to database: %v", err)
	}
	s.logger.Debug("Session %s saved successfully to the database.", requestID)
	return nil
}

// GetSession retrieves session data from the database.
func (s *DBService) GetSession(requestID string) (models.SessionData, error) {
	if s.db == nil {
		s.logger.Warn("Database not configured. Cannot get session %s.", requestID)
		return models.SessionData{}, ErrSessionNotFound
	}

	var jsonData []byte
	var expiresAt time.Time
	data := models.SessionData{}

	query := `SELECT session_data, expires_at FROM sessions WHERE request_id = ?`
	row := s.db.QueryRow(query, requestID)

	if err := row.Scan(&jsonData, &expiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return data, ErrSessionNotFound
		}
		return data, s.logger.Errorf("failed to retrieve session data: %v", err)
	}

	if time.Now().After(expiresAt) {
		_ = s.DeleteSession(requestID)
		return data, ErrSessionNotFound
	}

	if err := json.Unmarshal(jsonData, &data); err != nil {
		return data, s.logger.Errorf("failed to unmarshal session data: %v", err)
	}
	s.logger.Debug("Session %s retrieved successfully.", requestID)
	return data, nil
}

// DeleteSession removes a session from the database.
func (s *DBService) DeleteSession(requestID string) error {
	if s.db == nil {
		return nil
	}
	query := `DELETE FROM sessions WHERE request_id = ?`
	_, err := s.db.Exec(query, requestID)
	if err != nil {
		return s.logger.Errorf("failed to delete session: %v", err)
	}
	s.logger.Debug("Session %s deleted successfully.", requestID)
	return nil
}
