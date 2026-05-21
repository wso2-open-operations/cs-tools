import { useSearchParams } from "react-router-dom";

export function useFilters() {
  const [searchParams, setSearchParams] = useSearchParams();

  const filters = {
    search: searchParams.get("search") ?? "",
  };

  const patch = (partial: Partial<{ search: string }>) => {
    setSearchParams((prev) => {
      if (partial.search !== undefined) prev.set("search", partial.search);
      return prev;
    });
  };

  return { filters, set: patch };
}
