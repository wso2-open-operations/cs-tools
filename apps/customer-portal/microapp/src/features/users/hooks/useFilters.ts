import { useSearchParams } from "react-router-dom";

interface UsersFilterParams {
  search?: string;
}

export function useFilters() {
  const [searchParams, setSearchParams] = useSearchParams();

  const filters: UsersFilterParams = {
    search: searchParams.get("search") ?? "",
  };

  const patch = (partial: Partial<UsersFilterParams>) => {
    setSearchParams((prev) => {
      if (partial.search !== undefined) prev.set("search", partial.search);
      return prev;
    });
  };

  return {
    filters,
    set: patch,
  };
}
