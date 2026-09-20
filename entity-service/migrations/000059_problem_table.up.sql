-- Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
--
-- WSO2 LLC. licenses this file to you under the Apache License,
-- Version 2.0 (the "License"); you may not use this file except
-- in compliance with the License.
-- You may obtain a copy of the License at
--
-- http://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing,
-- software distributed under the License is distributed on an
-- "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
-- KIND, either express or implied.  See the License for the
-- specific language governing permissions and limitations
-- under the License.

-- work_item type extension for PROBLEM, same shared-primary-key pattern as
-- incident (000058_incident_table.up.sql): id IS work_item.id, ON DELETE
-- CASCADE, no audit columns - those live on work_item and are reachable via
-- join.
DO $$ BEGIN
    CREATE TYPE problem_state_enum AS ENUM (
        'NEW', 'ASSESS', 'ROOT_CAUSE_ANALYSIS', 'FIX_IN_PROGRESS', 'RESOLVED', 'CLOSED'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE problem_priority_enum AS ENUM ('CRITICAL', 'HIGH', 'MODERATE', 'LOW', 'PLANNING');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- ServiceNow's problem table carries both "state" and the legacy
-- "problem_state" field as separate choice lists with overlapping but
-- distinct value sets - kept as two separate enum types/columns to match.
DO $$ BEGIN
    CREATE TYPE problem_problem_state_enum AS ENUM (
        'NEW', 'OPEN', 'ASSESS', 'KNOWN_ERROR', 'PENDING_CHANGE', 'ROOT_CAUSE_ANALYSIS',
        'FIX_IN_PROGRESS', 'CLOSED_RESOLVED', 'RESOLVED', 'CLOSED'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE problem_category_enum AS ENUM ('SOFTWARE', 'HARDWARE', 'NETWORK', 'DATABASE');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE problem_resolution_code_enum AS ENUM (
        'FIX_APPLIED', 'RISK_ACCEPTED', 'DUPLICATE', 'CANCELED'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- subcategory is a ServiceNow dependent choice list, same shape as
-- incident_subcategory (000058) - a lookup table with its own surrogate id,
-- not a flat enum, since valid values depend on category. value is the raw
-- ServiceNow choice value (lookup-by-name match target); label is the
-- display text.
CREATE TABLE IF NOT EXISTS problem_subcategory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category problem_category_enum NOT NULL,
    value TEXT NOT NULL,
    label TEXT NOT NULL,
    UNIQUE (category, value)
);

INSERT INTO problem_subcategory (category, value, label) VALUES
    ('NETWORK', 'dhcp', 'DHCP'),
    ('NETWORK', 'ip address', 'IP Address'),
    ('NETWORK', 'dns', 'DNS'),
    ('NETWORK', 'vpn', 'VPN'),
    ('NETWORK', 'wireless', 'Wireless'),
    ('HARDWARE', 'cpu', 'CPU'),
    ('HARDWARE', 'keyboard', 'Keyboard'),
    ('HARDWARE', 'memory', 'Memory'),
    ('HARDWARE', 'mouse', 'Mouse'),
    ('HARDWARE', 'disk', 'Disk'),
    ('HARDWARE', 'monitor', 'Monitor'),
    ('DATABASE', 'oracle', 'Oracle'),
    ('DATABASE', 'sql server', 'MS SQL Server'),
    ('DATABASE', 'db2', 'DB2'),
    ('SOFTWARE', 'os', 'Operating System'),
    ('SOFTWARE', 'email', 'Email')
ON CONFLICT (category, value) DO NOTHING;

CREATE TABLE IF NOT EXISTS problem (
    id UUID PRIMARY KEY REFERENCES work_item(id) ON DELETE CASCADE,
    opened_by_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    opened_on TIMESTAMPTZ,
    state problem_state_enum,
    is_active BOOLEAN,
    priority problem_priority_enum,
    problem_state problem_problem_state_enum,
    category problem_category_enum,
    subcategory_id UUID REFERENCES problem_subcategory(id) ON DELETE SET NULL,
    incident_id UUID REFERENCES incident(id) ON DELETE SET NULL,
    change_request_id UUID REFERENCES change_request(id) ON DELETE SET NULL,
    resolution_code problem_resolution_code_enum,
    cause_notes TEXT,
    fix_notes TEXT,
    workaround TEXT,
    resolved_on TIMESTAMPTZ,
    resolved_by_id UUID REFERENCES "user"(id) ON DELETE SET NULL,
    closed_on TIMESTAMPTZ,
    due_on TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_problem_opened_by_id ON problem (opened_by_id);
CREATE INDEX IF NOT EXISTS idx_problem_subcategory_id ON problem (subcategory_id);
CREATE INDEX IF NOT EXISTS idx_problem_incident_id ON problem (incident_id);
CREATE INDEX IF NOT EXISTS idx_problem_change_request_id ON problem (change_request_id);
CREATE INDEX IF NOT EXISTS idx_problem_resolved_by_id ON problem (resolved_by_id);
