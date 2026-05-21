import { useQuery } from "@tanstack/react-query";

import { metadata } from "@features/metadata/api/metadata.queries";

export function useMetadata() {
  return useQuery(metadata.get());
}
