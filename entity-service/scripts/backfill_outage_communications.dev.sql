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

-- One-time backfill of outage_communication, for the DEV instance ONLY.
--
-- *** THIS IS NOT A MIGRATION AND MUST NOT BECOME ONE. *** The rows below are
-- dev content -- lorem ipsum, localhost links, one author. Running them
-- against production would insert dev text onto the public status page.
--
-- Produced by discovery PASS 17 (the outage cloud-status flows pass) of the
-- backing-system discovery scripts under integrations/csm-flow-service/docs/,
-- which reads each outage's external-communication journal entries and emits
-- these statements. Run that pass against whichever backing-system instance a
-- given environment is migrating from, and apply ITS output there.
--
-- The author address and outage ids below are placeholders: replace them with
-- that pass's own output before running this anywhere.
--
-- Why a table rather than the existing outage.external_outage_communications
-- column: PASS 17 measured 17 entries across 7 outages, 4 of which carry
-- more than one and one of which carries five. A single column cannot hold
-- a list, and the endpoint publishes them newest-first with timestamps.
-- *** ONE TRANSACTION, BECAUSE THE FIRST STATEMENT IS A DELETE. *** Every
-- INSERT below references an outage through a foreign key. Without a
-- transaction, one missing outage fails its INSERT after the DELETE has
-- already committed, leaving the table empty or half filled -- and the
-- migration's own comment calls this table first-class data after cutover.
-- Run against the wrong database and an unwrapped script wipes real
-- customer-facing updates with no way back.
BEGIN;

DELETE FROM outage_communication;

INSERT INTO outage_communication (outage_id, comment, created_on, created_by) VALUES ('00000000-0000-0000-0000-000000000001'::uuid, '[code]<p><strong>Investigating</strong></p>
<p><span style="color: rgb(0, 0, 0); font-family: ''Open Sans'', Arial, sans-serif; font-size: 14px; text-align: justify; white-space: normal; background-color: rgb(255, 255, 255); float: none; display: inline;">It is a long established fact that a reader will be distracted by the readable content of a page when looking at its layout. The point of using Lorem Ipsum is that it has a more-or-less normal distribution of letters, as opposed to using ''Content here, content here'', making it look like readable English. Many desktop publishing packages and web page editors now use Lorem Ipsum as their default model text, and a search for ''lorem ipsum'' will uncover many web sites still in their infancy. Various versions have evolved over the years, sometimes by accident, sometimes on purpose (injected humour and the like).</span></p>[/code]', '2025-11-10 05:09:38'::timestamptz, 'jane.doe@example.com');
INSERT INTO outage_communication (outage_id, comment, created_on, created_by) VALUES ('00000000-0000-0000-0000-000000000002'::uuid, '[code]<p>Mitigations were applied by the Engineering Team to the respective feature.</p>[/code]', '2026-05-11 06:52:59'::timestamptz, 'jane.doe@example.com');
INSERT INTO outage_communication (outage_id, comment, created_on, created_by) VALUES ('00000000-0000-0000-0000-000000000002'::uuid, '[code]<p>The incident was discovered by the Engineering team responsible for this feature.</p>[/code]', '2026-05-11 06:51:49'::timestamptz, 'jane.doe@example.com');
INSERT INTO outage_communication (outage_id, comment, created_on, created_by) VALUES ('00000000-0000-0000-0000-000000000003'::uuid, '[code]<p>New Outage</p>[/code]', '2025-11-20 05:30:34'::timestamptz, 'jane.doe@example.com');
INSERT INTO outage_communication (outage_id, comment, created_on, created_by) VALUES ('00000000-0000-0000-0000-000000000004'::uuid, '[code]<p><strong>Update</strong></p>
<p><strong style="margin: 0px; padding: 0px; color: rgb(0, 0, 0); font-family: ''Open Sans'', Arial, sans-serif; font-size: 14px; text-align: justify; white-space: normal; background-color: rgb(255, 255, 255);">Lorem Ipsum</strong><span style="color: rgb(0, 0, 0); font-family: ''Open Sans'', Arial, sans-serif; font-size: 14px; text-align: justify; white-space: normal; background-color: rgb(255, 255, 255); float: none; display: inline;">&nbsp;is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry''s standard dummy text ever since the 1500s, when an unknown printer took a galley of type and scrambled it to make a type specimen book. It has survived not only five centuries, but also the leap into electronic typesetting, remaining essentially unchanged. It was popularised in the 1960s with the release of Letraset sheets containing Lorem Ipsum passages, and more recently with desktop publishing software like Aldus PageMaker including versions of Lorem Ipsum.</span></p>
<p><span style="color: rgb(0, 0, 0); font-family: ''Open Sans'', Arial, sans-serif; font-size: 14px; text-align: justify; white-space: normal; background-color: rgb(255, 255, 255); float: none; display: inline;"><span style="color: rgb(0, 0, 0); font-family: ''Open Sans'', Arial, sans-serif; font-size: 14px; text-align: justify; white-space: normal; background-color: rgb(255, 255, 255); float: none; display: inline;">Contrary to popular belief, Lorem Ipsum is not simply random text. It has roots in a piece of classical Latin literature from 45 BC, making it over 2000 years old. Richard McClintock, a Latin professor at Hampden-Sydney College in Virginia, looked up one of the more obscure Latin words, consectetur, from a Lorem Ipsum passage, and going through the cites of the word in classical literature, discovered the undoubtable source. Lorem Ipsum comes from sections 1.10.32 and 1.10.33 of "de Finibus Bonorum et Malorum" (The Extremes of Good and Evil) by Cicero, written in 45 BC. This book is a treatise on the theory of ethics, very popular during the Renaissance. The first line of Lorem Ipsum, "Lorem ipsum dolor sit amet..", comes from a line in section 1.10.32.</span></span></p>[/code]', '2025-11-07 02:36:44'::timestamptz, 'jane.doe@example.com');
INSERT INTO outage_communication (outage_id, comment, created_on, created_by) VALUES ('00000000-0000-0000-0000-000000000004'::uuid, '[code]<p><span style="font-family: verdana, geneva;"><strong>Resolved</strong></span></p>
<p><span style="font-family: verdana, geneva;">The incident has been mitigated by applying hot-fixes to the necessary components.</span></p>
<p><span style="font-family: verdana, geneva;">More details can be found in <a title="Status Page" href="http://localhost:3000/history" target="_blank" rel="noopener">here</a>.</span></p>[/code]', '2025-11-07 02:29:51'::timestamptz, 'jane.doe@example.com');
INSERT INTO outage_communication (outage_id, comment, created_on, created_by) VALUES ('00000000-0000-0000-0000-000000000004'::uuid, '[code]<p style="box-sizing: border-box; margin-top: 0px; color: rgb(21, 25, 32); font-family: Lato, Arial, sans-serif; font-size: 16px; white-space: normal; background-color: rgb(255, 255, 255);"><span style="font-family: ''times new roman'', times;"><strong style="box-sizing: border-box;">Update</strong></span></p>
<p style="box-sizing: border-box; color: rgb(21, 25, 32); font-family: Lato, Arial, sans-serif; font-size: 16px; white-space: normal; background-color: rgb(255, 255, 255);"><span style="font-family: ''times new roman'', times;">This is to provide an update on the ongoing service disruption.</span></p>
<p style="box-sizing: border-box; margin-bottom: 0px; color: rgb(21, 25, 32); font-family: Lato, Arial, sans-serif; font-size: 16px; white-space: normal; background-color: rgb(255, 255, 255);"><span style="font-family: ''times new roman'', times;">We''ll provide an update soon.</span></p>[/code]', '2025-11-07 02:25:22'::timestamptz, 'jane.doe@example.com');
INSERT INTO outage_communication (outage_id, comment, created_on, created_by) VALUES ('00000000-0000-0000-0000-000000000004'::uuid, '[code]<p>This another comment from the Outage itself. Providing an update about the outage and its status. This update message contains spaces,&nbsp;</p>
<p>Newlines</p>
<p><strong>Bold,&nbsp;</strong><em>Italic,&nbsp;</em>and <span style="text-decoration: underline;">underline.</span> Also, this contains the following points</p>
<ol>
<li>One</li>
<li>Two</li>
</ol>
<p>Also, some bullet points</p>
<ul>
<li>One</li>
<li>Two</li>
<li>Three</li>
</ul>
<p>This to check the rendering at the Frontend when using the HTML inputs for the Journals.</p>
<p>Thanks</p>[/code]', '2025-11-06 10:18:04'::timestamptz, 'jane.doe@example.com');
INSERT INTO outage_communication (outage_id, comment, created_on, created_by) VALUES ('00000000-0000-0000-0000-000000000004'::uuid, '[code]<p>This is an external comment added in Outage: OUT0001856</p>[/code]', '2025-11-06 09:54:40'::timestamptz, 'jane.doe@example.com');
INSERT INTO outage_communication (outage_id, comment, created_on, created_by) VALUES ('00000000-0000-0000-0000-000000000005'::uuid, '[code]<p><a title="Azure Status Title" href="https://azure.status.microsoft/en-gb/status">Azure Status</a></p>[/code]', '2025-11-10 08:29:59'::timestamptz, 'jane.doe@example.com');
INSERT INTO outage_communication (outage_id, comment, created_on, created_by) VALUES ('00000000-0000-0000-0000-000000000005'::uuid, '[code]<p><strong>Update</strong></p>
<p><span style="color: rgb(0, 0, 0); font-family: ''Open Sans'', Arial, sans-serif; font-size: 14px; text-align: justify; white-space: normal; background-color: rgb(255, 255, 255); float: none; display: inline;">Contrary to popular belief, Lorem Ipsum is not simply random text. It has roots in a piece of classical Latin literature from 45 BC, making it over 2000 years old. Richard McClintock, a Latin professor at Hampden-Sydney College in Virginia, looked up one of the more obscure Latin words, consectetur, from a Lorem Ipsum passage, and going through the cites of the word in classical literature, discovered the undoubtable source. Lorem Ipsum comes from sections 1.10.32 and 1.10.33 of "de Finibus Bonorum et Malorum" (The Extremes of Good and Evil) by Cicero, written in 45 BC. This book is a treatise on the theory of ethics, very popular during the Renaissance. The first line of Lorem Ipsum, "Lorem ipsum dolor sit amet..", comes from a line in section 1.10.32.</span></p>
<p><span style="color: rgb(0, 0, 0); font-family: ''Open Sans'', Arial, sans-serif; font-size: 14px; text-align: justify; white-space: normal; background-color: rgb(255, 255, 255); float: none; display: inline;"><a title="Azure Status" href="https://azure.status.microsoft/en-gb/status">https://azure.status.microsoft/en-gb/status</a></span></p>[/code]', '2025-11-10 08:29:10'::timestamptz, 'jane.doe@example.com');
INSERT INTO outage_communication (outage_id, comment, created_on, created_by) VALUES ('00000000-0000-0000-0000-000000000005'::uuid, '[code]<p><strong>Update&nbsp;</strong></p>
<p><strong style="margin: 0px; padding: 0px; color: rgb(0, 0, 0); font-family: ''Open Sans'', Arial, sans-serif; font-size: 14px; text-align: justify; white-space: normal; background-color: rgb(255, 255, 255);">Lorem Ipsum</strong><span style="color: rgb(0, 0, 0); font-family: ''Open Sans'', Arial, sans-serif; font-size: 14px; text-align: justify; white-space: normal; background-color: rgb(255, 255, 255); float: none; display: inline;">&nbsp;is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry''s standard dummy text ever since the 1500s, when an unknown printer took a galley of type and scrambled it to make a type <em>specimen book</em>. It has survived not only five centuries, but also the leap into electronic typesetting, remaining essentially unchanged. It was popularised in the 1960s with the <span style="text-decoration: underline;">release of Letraset </span>sheets containing Lorem Ipsum passages, and more recently with desktop publishing </span></p>
<ol>
<li style="font-size: 14px;"><span style="color: rgb(0, 0, 0); font-family: ''Open Sans'', Arial, sans-serif; font-size: 14px; text-align: justify; white-space: normal; background-color: rgb(255, 255, 255); float: none; display: inline;">Here you go&nbsp;</span></li>
</ol>
<p><span style="color: rgb(0, 0, 0); font-family: ''Open Sans'', Arial, sans-serif; font-size: 14px; text-align: justify; white-space: normal; background-color: rgb(255, 255, 255); float: none; display: inline;">software like Aldus PageMaker including versions of Lorem Ipsum.</span></p>[/code]', '2025-11-10 08:26:01'::timestamptz, 'jane.doe@example.com');
INSERT INTO outage_communication (outage_id, comment, created_on, created_by) VALUES ('00000000-0000-0000-0000-000000000005'::uuid, '[code]<p><strong><span style="color: rgb(0, 0, 0); font-family: ''Open Sans'', Arial, sans-serif; font-size: 14px; text-align: justify; white-space: normal; background-color: rgb(255, 255, 255); float: none; display: inline;">Investigating</span></strong></p>
<p><span style="color: rgb(0, 0, 0); font-family: ''Open Sans'', Arial, sans-serif; font-size: 14px; text-align: justify; white-space: normal; background-color: rgb(255, 255, 255); float: none; display: inline;">It is a long established fact that a reader will be distracted by the readable content of a page when looking at its layout. The point of using Lorem Ipsum is that it has a more-or-less normal distribution of letters, as opposed to using ''Content here, content here'', making it look like readable English. Many desktop publishing packages and web page editors now use Lorem Ipsum as their default model text, and a search for ''lorem ipsum'' will uncover many web sites still in their infancy. Various versions have evolved over the years, sometimes by accident, sometimes on purpose (injected humour and the like).</span></p>[/code]', '2025-11-10 08:19:21'::timestamptz, 'jane.doe@example.com');
INSERT INTO outage_communication (outage_id, comment, created_on, created_by) VALUES ('00000000-0000-0000-0000-000000000006'::uuid, '[code]<p>This is another external communication</p>[/code]', '2025-11-20 05:26:38'::timestamptz, 'jane.doe@example.com');
INSERT INTO outage_communication (outage_id, comment, created_on, created_by) VALUES ('00000000-0000-0000-0000-000000000006'::uuid, '[code]<p>Another External Communications</p>[/code]', '2025-11-20 05:21:06'::timestamptz, 'jane.doe@example.com');
INSERT INTO outage_communication (outage_id, comment, created_on, created_by) VALUES ('00000000-0000-0000-0000-000000000006'::uuid, '[code]<p>This is an external communication</p>[/code]', '2025-11-20 05:15:08'::timestamptz, 'jane.doe@example.com');
INSERT INTO outage_communication (outage_id, comment, created_on, created_by) VALUES ('00000000-0000-0000-0000-000000000007'::uuid, '[code]<p><strong>Investigating</strong></p>
<p><span style="color: rgb(0, 0, 0); font-family: ''Open Sans'', Arial, sans-serif; font-size: 14px; text-align: justify; white-space: normal; background-color: rgb(255, 255, 255); float: none; display: inline;">It is a long established fact that a reader will be distracted by the readable content of a page when looking at its layout. The point of using Lorem Ipsum is that it has a more-or-less normal distribution of letters, as opposed to using ''Content here, content here'', making it look like readable English. Many desktop publishing packages and web page editors now use Lorem Ipsum as their default model text, and a search for ''lorem ipsum'' will uncover many web sites still in their infancy. Various versions have evolved over the years, sometimes by accident, sometimes on purpose (injected humour and the like).</span></p>[/code]', '2025-11-10 05:04:14'::timestamptz, 'jane.doe@example.com');

COMMIT;
