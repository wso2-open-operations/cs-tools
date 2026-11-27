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

package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AlertSchema is what the alert consumer's own connection sees of the alert table and sequence.
type AlertSchema struct {
	Database   string
	User       string
	SearchPath string
	HasSeq     bool
	HasTable   bool
	CanUseSeq  bool
	CanInsert  bool
}

// The privilege checks take the resolved oid, so a missing object yields NULL (false) instead of an error.
const alertSchemaQuery = `SELECT current_database(), current_user, current_setting('search_path'),
	to_regclass('alert_seq') IS NOT NULL,
	to_regclass('alert') IS NOT NULL,
	COALESCE(has_sequence_privilege(to_regclass('alert_seq')::oid, 'USAGE'), false),
	COALESCE(has_table_privilege(to_regclass('alert')::oid, 'INSERT'), false)`

// CheckAlertSchema reports whether the pool's connections can reach alert_seq and alert, and where they are looking.
func CheckAlertSchema(ctx context.Context, pool *pgxpool.Pool) (AlertSchema, error) {
	var s AlertSchema
	err := pool.QueryRow(ctx, alertSchemaQuery).Scan(&s.Database, &s.User, &s.SearchPath, &s.HasSeq, &s.HasTable, &s.CanUseSeq, &s.CanInsert)
	if err != nil {
		return AlertSchema{}, fmt.Errorf("check alert schema: %w", err)
	}
	return s, nil
}

// Problem describes what keeps the consumer from storing alerts, or returns "" when nothing does.
func (s AlertSchema) Problem() string {
	var missing []string
	if !s.HasSeq {
		missing = append(missing, "sequence alert_seq not found")
	} else if !s.CanUseSeq {
		missing = append(missing, "no USAGE on alert_seq")
	}
	if !s.HasTable {
		missing = append(missing, "table alert not found")
	} else if !s.CanInsert {
		missing = append(missing, "no INSERT on alert")
	}
	return strings.Join(missing, ", ")
}

// Where names the database, user and search_path, so a migration run somewhere else is easy to spot.
func (s AlertSchema) Where() string {
	return fmt.Sprintf("database %q as %q with search_path %q", s.Database, s.User, s.SearchPath)
}
