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
