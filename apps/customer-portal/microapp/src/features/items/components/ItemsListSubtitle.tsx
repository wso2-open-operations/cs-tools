// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
import { Skeleton, Typography } from "@wso2/oxygen-ui";

export function ItemsListSubtitle({ count, total }: { count?: number; total?: number }) {
  if (!count || !total) return <Skeleton variant="text" width={50} height={20} />;
  return (
    <Typography component="div" variant="subtitle2" fontWeight="regular" color="text.secondary">
      {count} of {total}
    </Typography>
  );
}
