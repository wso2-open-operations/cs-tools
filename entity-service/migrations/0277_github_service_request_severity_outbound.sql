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

-- Dispatches csm-severity-update to the issue when a service request's severity is set or changes; the WHEN guard stops echoes.

CREATE OR REPLACE FUNCTION trg_github_service_request_severity()
RETURNS TRIGGER AS $$
DECLARE
    gh RECORD;
BEGIN
    -- Same repository resolution as trg_github_record_created (0206).
    SELECT COALESCE(linked_wi.github_owner, agr.owner) AS owner,
           COALESCE(linked_wi.github_repository, agr.repository) AS repository,
           linked_wi.github_issue_number,
           linked_wi.number
      INTO gh
      FROM work_item linked_wi
      LEFT JOIN account_github_repo agr ON agr.account_id = linked_wi.account_id AND agr.is_active
     WHERE linked_wi.id = NEW.id
       AND linked_wi.github_issue_number IS NOT NULL
       AND (linked_wi.github_owner IS NOT NULL OR agr.account_id IS NOT NULL);

    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    -- API severity values, the same vocabulary PUT .../severity takes.
    INSERT INTO github_outbound_queue (event, work_item_id, owner, repository, issue_number, payload)
    VALUES ('severity_changed', NEW.id, gh.owner, gh.repository, gh.github_issue_number,
            jsonb_build_object(
                'action',              CASE WHEN TG_OP = 'INSERT' THEN 'created' ELSE 'updated' END,
                'github_issue_number', gh.github_issue_number,
                'case_number',         gh.number,
                'case_sys_id',         NEW.id,
                'severity',            CASE NEW.severity
                                           WHEN 'S1' THEN 'critical'
                                           WHEN 'S2' THEN 'high'
                                           WHEN 'S3' THEN 'medium'
                                           WHEN 'S4' THEN 'low'
                                       END));
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS service_request_severity_github_insert ON service_request;
CREATE TRIGGER service_request_severity_github_insert
    AFTER INSERT ON service_request
    FOR EACH ROW EXECUTE FUNCTION trg_github_service_request_severity();

DROP TRIGGER IF EXISTS service_request_severity_github_update ON service_request;
CREATE TRIGGER service_request_severity_github_update
    AFTER UPDATE OF severity ON service_request
    FOR EACH ROW
    WHEN (OLD.severity IS DISTINCT FROM NEW.severity)
    EXECUTE FUNCTION trg_github_service_request_severity();
