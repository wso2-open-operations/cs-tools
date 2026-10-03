-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
--
-- This software is the property of WSO2 LLC. and its suppliers, if any.
-- Dissemination of any information or reproduction of any material contained
-- herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
-- You may not alter or remove any copyright or other notice from copies of this content.

-- The channel each outage communication was posted on.
--
-- The Postgres outage API (outage_repo.go) stores external, internal and
-- additional communications in outage_communication and reads them back by
-- channel -- the detail endpoint's per-channel counts, adding a
-- communication, listing them -- but no migration ever added the column, so
-- on any database built from migrations those endpoints fail with
-- `column "channel" does not exist`. Staging showed exactly that.
--
-- The table (000105) was built for the public status page and has only ever
-- held external entries, copied from ServiceNow's
-- u_external_outage_communications. 'external' is therefore their true value,
-- not a placeholder default.
--
-- Replaces the six-digit 000106 migration once proposed for this: make
-- migrate orders files by name, so a six-digit file runs before every
-- four-digit one and would have altered this table before 000105's
-- dependencies existed on a fresh database.
--
-- The outage_number_seq that 000106 also created is not repeated here:
-- 0180_native_number_series already creates it (START 10000).
BEGIN;

ALTER TABLE outage_communication
    ADD COLUMN IF NOT EXISTS channel VARCHAR(20) NOT NULL DEFAULT 'external';

DO $$ BEGIN
    ALTER TABLE outage_communication
        ADD CONSTRAINT outage_communication_channel_chk
        CHECK (channel IN ('external', 'internal', 'additional'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

COMMIT;
