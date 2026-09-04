import { QueryClient } from "@tanstack/react-query";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000, // data is fine for 30s; avoids refetch storms while clicking around
      refetchOnWindowFocus: false,
      retry: 1,
    },
  },
});
