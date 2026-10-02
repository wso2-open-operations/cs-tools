-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
--
-- This software is the property of WSO2 LLC. and its suppliers, if any.
-- Dissemination of any information or reproduction of any material contained
-- herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
-- You may not alter or remove any copyright or other notice from copies of this content.

-- Unique keys so allocation-event upserts (ON CONFLICT) cannot duplicate on repeats or races.
-- Existing duplicates are not deleted: the check below stops with their keys; resolve them first.

DO $$
DECLARE
    dup_engagements TEXT;
    dup_allocations TEXT;
BEGIN
    SELECT string_agg(engagement_id || ' x' || n, ', ') INTO dup_engagements
    FROM (SELECT engagement_id, count(*) AS n FROM customer_engagement
          WHERE engagement_id IS NOT NULL GROUP BY engagement_id HAVING count(*) > 1) d;
    SELECT string_agg(engagement_id || '/' || allocation_id || ' x' || n, ', ') INTO dup_allocations
    FROM (SELECT engagement_id, allocation_id, count(*) AS n FROM customer_engagement_allocation_resource
          WHERE engagement_id IS NOT NULL AND allocation_id IS NOT NULL
          GROUP BY engagement_id, allocation_id HAVING count(*) > 1) d;
    IF dup_engagements IS NOT NULL OR dup_allocations IS NOT NULL THEN
        RAISE EXCEPTION 'migration 0178: duplicates must be resolved first. customer_engagement.engagement_id: [%]; customer_engagement_allocation_resource (engagement_id/allocation_id): [%]',
            coalesce(dup_engagements, 'none'), coalesce(dup_allocations, 'none');
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_customer_engagement_engagement_id
    ON customer_engagement (engagement_id)
    WHERE engagement_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_cea_resource_engagement_allocation
    ON customer_engagement_allocation_resource (engagement_id, allocation_id)
    WHERE engagement_id IS NOT NULL AND allocation_id IS NOT NULL;
