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

-- A customer allocation is either on site or off site, and which one matters:
-- an engineer at a customer's office is unreachable in a way one working from
-- their desk is not, so a lead looking at cover needs to tell them apart.
--
-- This is why absence kinds became a table in 000090. Splitting a category is
-- two inserts and a backfill; against the ENUM it replaced it would have been
-- ALTER TYPE, a migration and a deploy.
INSERT INTO schedule_absence_kind (code, short_code, label, bucket, colour_token, sort_order, created_by, updated_by)
VALUES
    ('CUSTOMER_ONSITE',  'CUS-ON',  'Customer allocation — on site',  'ALLOCATION', 'EXT', 41, 'migration', 'migration'),
    ('CUSTOMER_OFFSITE', 'CUS-OFF', 'Customer allocation — off site', 'ALLOCATION', 'EXT', 42, 'migration', 'migration')
ON CONFLICT (code) DO NOTHING;

-- The original CUSTOMER row stays and stays usable: existing rows point at it,
-- and "we do not know which" is a truthful answer for them. It sorts first of
-- the three so a lead marking new time sees the specific options beside it.
UPDATE schedule_absence_kind
   SET label = 'Customer allocation — unspecified', updated_on = NOW()
 WHERE code = 'CUSTOMER';
