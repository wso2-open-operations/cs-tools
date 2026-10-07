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

-- LOCAL DEVELOPMENT ONLY. Not a migration: no database outside this compose stack runs it.
--
-- GET /metadata reads its time-zone list from the `timezone` reference table
-- (reference_data_repo.go ListTimeZones: SELECT value, label FROM timezone). The sync-owned
-- migration 0121_timezone_table.sql creates the table (and 0122_user_add_timezone.sql the
-- "user".timezone column that references it), but only the table: its rows come from
-- ServiceNow, so a database built here has an empty list and /metadata answers `timeZones: []`.
-- Both portals call /metadata on load, and a profile's time-zone picker needs entries.
--
-- This loads a small, curated stand-in list, straight after the 0121 file that creates the
-- table. It declares no schema: the table is the sync's, whatever its column types are on a
-- given volume. Safe to re-run (ON CONFLICT DO NOTHING).
INSERT INTO timezone (value, label, utc_offset, dst) VALUES
  ('Africa/Cairo',                    'Cairo',                                    '+02:00', TRUE),
  ('Africa/Johannesburg',             'Johannesburg / Pretoria',                  '+02:00', FALSE),
  ('Africa/Lagos',                    'West Central Africa',                      '+01:00', FALSE),
  ('Africa/Nairobi',                  'Nairobi',                                  '+03:00', FALSE),
  ('America/Anchorage',               'Alaska',                                   '-09:00', TRUE),
  ('America/Argentina/Buenos_Aires',  'Buenos Aires',                             '-03:00', FALSE),
  ('America/Chicago',                 'Central Time (US & Canada)',               '-06:00', TRUE),
  ('America/Denver',                  'Mountain Time (US & Canada)',              '-07:00', TRUE),
  ('America/Halifax',                 'Atlantic Time (Canada)',                   '-04:00', TRUE),
  ('America/Los_Angeles',             'Pacific Time (US & Canada)',               '-08:00', TRUE),
  ('America/New_York',                'Eastern Time (US & Canada)',               '-05:00', TRUE),
  ('America/Sao_Paulo',               'Brasilia',                                 '-03:00', FALSE),
  ('Asia/Bangkok',                    'Bangkok / Hanoi / Jakarta',                '+07:00', FALSE),
  ('Asia/Colombo',                    'Sri Jayawardenepura (Colombo)',            '+05:30', FALSE),
  ('Asia/Dhaka',                      'Dhaka',                                    '+06:00', FALSE),
  ('Asia/Dubai',                      'Abu Dhabi / Muscat',                       '+04:00', FALSE),
  ('Asia/Hong_Kong',                  'Hong Kong',                                '+08:00', FALSE),
  ('Asia/Karachi',                    'Islamabad / Karachi',                      '+05:00', FALSE),
  ('Asia/Kolkata',                    'Chennai / Kolkata / Mumbai / New Delhi',   '+05:30', FALSE),
  ('Asia/Seoul',                      'Seoul',                                    '+09:00', FALSE),
  ('Asia/Singapore',                  'Singapore / Malaysia / Philippines',       '+08:00', FALSE),
  ('Asia/Tokyo',                      'Osaka / Sapporo / Tokyo',                  '+09:00', FALSE),
  ('Atlantic/Azores',                 'Azores',                                   '-01:00', TRUE),
  ('Australia/Perth',                 'Perth',                                    '+08:00', FALSE),
  ('Australia/Sydney',                'Canberra / Melbourne / Sydney',            '+10:00', TRUE),
  ('Europe/Amsterdam',                'Amsterdam / Berlin / Rome / Stockholm',    '+01:00', TRUE),
  ('Europe/Athens',                   'Athens / Bucharest',                       '+02:00', TRUE),
  ('Europe/Dublin',                   'Dublin',                                   '+00:00', TRUE),
  ('Europe/Helsinki',                 'Helsinki / Kyiv / Riga',                   '+02:00', TRUE),
  ('Europe/Istanbul',                 'Istanbul',                                 '+03:00', FALSE),
  ('Europe/Lisbon',                   'Lisbon',                                   '+00:00', TRUE),
  ('Europe/London',                   'Edinburgh / London',                       '+00:00', TRUE),
  ('Europe/Madrid',                   'Brussels / Copenhagen / Madrid / Paris',   '+01:00', TRUE),
  ('Europe/Moscow',                   'Moscow / St. Petersburg',                  '+03:00', FALSE),
  ('Pacific/Auckland',                'Auckland / Wellington',                    '+12:00', TRUE),
  ('Pacific/Honolulu',                'Hawaii',                                   '-10:00', FALSE),
  ('UTC',                             'UTC',                                      '+00:00', FALSE)
ON CONFLICT (value) DO NOTHING;
