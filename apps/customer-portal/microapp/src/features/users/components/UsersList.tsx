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
import { UserItem, UserItemSkeleton } from "@features/users/components";
import { useUsersList } from "@features/users/hooks";

import { EmptyState } from "@shared/components/common";

export function UsersList() {
  const { data, isLoading } = useUsersList();

  if (isLoading) return <UsersListSkeleton />;

  if (!data?.length) return <EmptyState />;

  return (
    <>
      {data.map((props) => (
        <UserItem key={props.id} {...props} />
      ))}
    </>
  );
}

export function UsersListSkeleton() {
  return (
    <>
      {Array.from({ length: 10 }).map((_, index) => (
        <UserItemSkeleton key={index} />
      ))}
    </>
  );
}
