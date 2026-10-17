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

DO $$
DECLARE
    max_num BIGINT;
BEGIN
    -- Extract maximum numeric suffix from existing ServiceNow deployments
    SELECT COALESCE(MAX(SUBSTRING(number FROM '[0-9]+')::BIGINT), 0)
    INTO max_num
    FROM deployment
    -- Only the generated DEP + up-to-9-digit format: a malformed or over-long number
    -- must not push the sequence out of range or overflow the BIGINT cast.
    WHERE number ~ '^DEP[0-9]{1,9}$';

    -- Initialize sequence to start at MAX + 1 (e.g. 2183 -> 2184)
    IF NOT EXISTS (SELECT 1 FROM pg_sequences WHERE sequencename = 'portal_deployment_number_seq') THEN
        EXECUTE 'CREATE SEQUENCE portal_deployment_number_seq START WITH ' || (max_num + 1);
    ELSE
        IF max_num > 0 THEN
            PERFORM setval('portal_deployment_number_seq', max_num, true);
        END IF;
    END IF;

    -- Also advance dep_number_seq if present to maintain consistency across sequences
    IF EXISTS (SELECT 1 FROM pg_sequences WHERE sequencename = 'dep_number_seq') THEN
        IF max_num > 0 THEN
            PERFORM setval('dep_number_seq', max_num, true);
        END IF;
    END IF;
END $$;

-- Standard ServiceNow 9-digit zero-padded format generator (e.g. DEP000002184)
CREATE OR REPLACE FUNCTION next_portal_deployment_number() RETURNS VARCHAR(40) AS $$
DECLARE
    next_val BIGINT;
BEGIN
    next_val := nextval('portal_deployment_number_seq');
    RETURN 'DEP' || lpad(next_val::text, 9, '0');
END;
$$ LANGUAGE plpgsql;
