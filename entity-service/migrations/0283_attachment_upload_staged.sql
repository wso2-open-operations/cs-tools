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


-- An upload that may be sent straight to Azure is put together under a staging key
-- and copied to its own key at confirm. Whether an upload is staged is decided
-- when it starts and kept here, so the key and the size limit stay the same for the
-- whole upload even if direct transfers are switched on or off meanwhile.
ALTER TABLE work_item_attachment ADD COLUMN IF NOT EXISTS staged BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE comment_attachment ADD COLUMN IF NOT EXISTS staged BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE deployment_attachment ADD COLUMN IF NOT EXISTS staged BOOLEAN NOT NULL DEFAULT FALSE;
