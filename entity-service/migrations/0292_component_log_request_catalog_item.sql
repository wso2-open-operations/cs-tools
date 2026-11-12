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

-- The "Component Log Request" catalog item: a service request that collects
-- a Choreo component's logs for a time window and shares them with named
-- people. When staff raise one, the CSM portal backend starts the collection
-- through the Engineering Entity Service.
--
-- The first catalog item created in Postgres rather than copied from
-- ServiceNow. Every row has a fixed id so it is the same in every
-- environment (the CSM portal backend is configured with the item id) and the
-- migration is safe to re-run.
--
-- Staff only (csm_only, migration 0291): customers never see it, and a
-- request raised from it by anyone but CSM-portal staff is refused.
--
-- Listed under the existing "Information Request" category, next to
-- "Request Product Logs" and "Request LB Logs". The owner is the owner of
-- those items. Both are looked up rather than assumed: if a database has no
-- such category (a fresh local database has no catalog at all), the item is
-- still created but is not linked, and a NOTICE says so.
--
-- Question text matters to the portal: "Choreo Environment" is deliberately
-- not "Environment", which the portal treats as a context field and never
-- shows or submits (catalogVariables.ts). Types are the names the portal renders
-- (catalogVariables.ts); date fields are also recognised by their text.
--
-- The Cluster question gets no choices here: they differ per environment
-- (each is a collector runner label). Until clusters are stored per
-- deployment, each environment's choices are added separately.
--
-- Routing rules (who sees the item) are in migration 0293.

INSERT INTO catalog_item (id, created_on, updated_on, created_by, updated_by, name, owner_id, csm_only)
SELECT '9d48f18f-0ae3-466b-b0ab-fdd14038864b', now(), now(), 'system', 'system',
       'Component Log Request',
       (SELECT id FROM "user" WHERE id = 'f742aa3f-1b8c-0610-264c-997a234bcb95'),
       TRUE
ON CONFLICT (id) DO NOTHING;

DO $$
DECLARE
    information_request_id UUID;
BEGIN
    SELECT id INTO information_request_id
    FROM sr_category
    WHERE id = '63cafc50-1ba2-0210-609e-8666624bcb38'
       OR (name = 'Information Request' AND catalog = 'SERVICE_CATALOG')
    ORDER BY (id = '63cafc50-1ba2-0210-609e-8666624bcb38') DESC
    LIMIT 1;

    IF information_request_id IS NULL THEN
        RAISE NOTICE 'No "Information Request" category; "Component Log Request" is not linked to a category';
        RETURN;
    END IF;

    INSERT INTO catalog_item_category (id, created_on, updated_on, created_by, updated_by,
                                       catalog_item_id, sr_category_id)
    VALUES ('ba9840c8-c63d-4e44-9cfd-00b82709c935', now(), now(), 'system', 'system',
            '9d48f18f-0ae3-466b-b0ab-fdd14038864b', information_request_id)
    ON CONFLICT (id) DO NOTHING;
END $$;

INSERT INTO catalog_variable (id, created_on, updated_on, created_by, updated_by, catalog_item_id,
                              name, question_text, type, "order", is_mandatory, is_active,
                              read_only, hidden, max_length,
                              validation_name, validation_regex, validation_message)
VALUES
    ('f239e6be-0b03-49e9-8d66-4ea06858f161', now(), now(), 'system', 'system', '9d48f18f-0ae3-466b-b0ab-fdd14038864b',
     'cluster', 'Cluster', 'Select Box', 100, TRUE, TRUE,
     FALSE, FALSE, NULL, NULL, NULL, NULL),
    ('81f21d11-4ecd-43cb-ad44-256532137748', now(), now(), 'system', 'system', '9d48f18f-0ae3-466b-b0ab-fdd14038864b',
     'choreo_environment', 'Choreo Environment', 'Select Box', 200, TRUE, TRUE,
     FALSE, FALSE, NULL, NULL, NULL, NULL),
    ('dd6c260d-049b-4dd4-969e-0d9acce0fbb1', now(), now(), 'system', 'system', '9d48f18f-0ae3-466b-b0ab-fdd14038864b',
     'namespace', 'Namespace', 'Single Line Text', 300, FALSE, TRUE,
     FALSE, FALSE, 63,
     'k8s_name', '^[a-z0-9]([-a-z0-9]*[a-z0-9])?$', 'Lower-case letters, digits and hyphens only'),
    ('b7dcd5d4-2d62-4406-8dbd-1c7b4cd3bd78', now(), now(), 'system', 'system', '9d48f18f-0ae3-466b-b0ab-fdd14038864b',
     'pod_name', 'Pod Name', 'Single Line Text', 400, TRUE, TRUE,
     FALSE, FALSE, 253,
     'k8s_name', '^[a-z0-9]([-a-z0-9]*[a-z0-9])?$', 'Lower-case letters, digits and hyphens only'),
    ('48bcac1b-0c37-4882-bc7e-6f5afc699e9d', now(), now(), 'system', 'system', '9d48f18f-0ae3-466b-b0ab-fdd14038864b',
     'start_time', 'Start Time', 'Date/Time', 500, TRUE, TRUE,
     FALSE, FALSE, NULL, NULL, NULL, NULL),
    ('d3a37969-c277-4ab5-ba95-0df23a0f8478', now(), now(), 'system', 'system', '9d48f18f-0ae3-466b-b0ab-fdd14038864b',
     'end_time', 'End Time', 'Date/Time', 600, TRUE, TRUE,
     FALSE, FALSE, NULL, NULL, NULL, NULL),
    ('1cae4e36-b64d-48e6-b196-8f62e0cfe044', now(), now(), 'system', 'system', '9d48f18f-0ae3-466b-b0ab-fdd14038864b',
     'people_who_need_access', 'People Who Need Access', 'Multi Line Text', 700, TRUE, TRUE,
     FALSE, FALSE, NULL, NULL, NULL, NULL),
    ('f8f732aa-42d6-4da6-8872-2f3bc5bd0217', now(), now(), 'system', 'system', '9d48f18f-0ae3-466b-b0ab-fdd14038864b',
     'component', 'Component', 'Single Line Text', 800, FALSE, TRUE,
     FALSE, FALSE, 64,
     'component', '^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$', 'Letters, digits, hyphens and underscores only'),
    ('fccb11bc-8911-4245-bf0b-f406f4840821', now(), now(), 'system', 'system', '9d48f18f-0ae3-466b-b0ab-fdd14038864b',
     'search_text', 'Search Text', 'Single Line Text', 900, FALSE, TRUE,
     FALSE, FALSE, 255, NULL, NULL, NULL)
ON CONFLICT (id) DO NOTHING;

INSERT INTO catalog_variable_choice (id, created_on, updated_on, created_by, updated_by,
                                     catalog_variable_id, value, text, "order", is_inactive)
VALUES
    ('2a5faf4d-3d1b-4c23-ae2c-85954f3e518f', now(), now(), 'system', 'system',
     '81f21d11-4ecd-43cb-ad44-256532137748', 'development', 'Development', 100, FALSE),
    ('a8a036b7-65f2-449d-8636-0bcc067c2b14', now(), now(), 'system', 'system',
     '81f21d11-4ecd-43cb-ad44-256532137748', 'staging', 'Staging', 200, FALSE),
    ('9b61e289-ac4a-4532-b008-3ec5525104a5', now(), now(), 'system', 'system',
     '81f21d11-4ecd-43cb-ad44-256532137748', 'production', 'Production', 300, FALSE)
ON CONFLICT (id) DO NOTHING;
