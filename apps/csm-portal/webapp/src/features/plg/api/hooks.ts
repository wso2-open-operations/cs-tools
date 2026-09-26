import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";

import { usePlgApi } from "./client";
import type {
  CreatePlaybookRequest,
  DashboardAnalytics,
  LifecycleCatalogue,
  OrganizationDetail,
  PatchOrgPlatformRequest,
  PatchPlaybookRequest,
  PatchRunTaskRequest,
  Playbook,
  PlaybookTaskInput,
  Product,
  ProductDetail,
  SearchOrganizationsRequest,
  SearchOrganizationsResponse,
  SearchRegistrationsRequest,
  SearchRegistrationsResponse,
  UserRef,
  WorkQueueQuery,
  WorkQueueResponse,
} from "./types";

/**
 * Query keys.
 *
 * A pairing is keyed by (organisation, product) rather than by its own id,
 * because that is how the UI addresses it — the URL of the product tab is the
 * cache key.
 */
export const queryKeys = {
  me: ["me"] as const,
  products: ["products"] as const,
  csUsers: ["cs-users"] as const,
  lifecycle: ["lifecycle"] as const,
  organizations: (req: SearchOrganizationsRequest) => ["organizations", req] as const,
  organization: (id: string) => ["organization", id] as const,
  product: (orgId: string, code: string) => ["org-product", orgId, code] as const,
  registrations: (req: SearchRegistrationsRequest) => ["registrations", req] as const,
  playbooks: (productCode?: string) => ["playbooks", productCode ?? "all"] as const,
  playbook: (id: string) => ["playbook", id] as const,
  dashboard: (from?: string, to?: string) => ["dashboard", from ?? "", to ?? ""] as const,
  workQueue: (q: WorkQueueQuery) => ["work-queue", q] as const,
};

// ---------------------------------------------------------------------------
// Reference data
// ---------------------------------------------------------------------------

/** Reference data changes only with a migration, so it is cached for the session. */
const referenceOptions = { staleTime: Infinity } as const;

export function useMe(): UseQueryResult<UserRef> {
  const api = usePlgApi();
  return useQuery({
    queryKey: queryKeys.me,
    queryFn: () => api.get<UserRef>("/me"),
    ...referenceOptions,
  });
}

export function useProducts(): UseQueryResult<Product[]> {
  const api = usePlgApi();
  return useQuery({
    queryKey: queryKeys.products,
    queryFn: async () => (await api.get<{ products: Product[] }>("/products")).products,
    ...referenceOptions,
  });
}

export function useCSUsers(): UseQueryResult<UserRef[]> {
  const api = usePlgApi();
  return useQuery({
    queryKey: queryKeys.csUsers,
    queryFn: async () => (await api.get<{ users: UserRef[] }>("/cs-users")).users,
    ...referenceOptions,
  });
}

/**
 * The nine stages and the seven playbook paths.
 *
 * Everything that needs to know where playbooks may exist reads this one
 * endpoint — the diagram, the stage picker, the playbook editor — so none of
 * them can drift from the constraint the database enforces.
 */
export function useLifecycle(): UseQueryResult<LifecycleCatalogue> {
  const api = usePlgApi();
  return useQuery({
    queryKey: queryKeys.lifecycle,
    queryFn: () => api.get<LifecycleCatalogue>("/lifecycle"),
    staleTime: 60_000,
  });
}

// ---------------------------------------------------------------------------
// Organisations
// ---------------------------------------------------------------------------

export function useOrganizations(
  req: SearchOrganizationsRequest,
): UseQueryResult<SearchOrganizationsResponse> {
  const api = usePlgApi();
  return useQuery({
    queryKey: queryKeys.organizations(req),
    queryFn: () => api.post<SearchOrganizationsResponse>("/organizations/search", req),
    placeholderData: (previous) => previous,
  });
}

export function useOrganization(id: string | undefined): UseQueryResult<OrganizationDetail> {
  const api = usePlgApi();
  return useQuery({
    queryKey: queryKeys.organization(id ?? ""),
    queryFn: () => api.get<OrganizationDetail>(`/organizations/${id}`),
    enabled: Boolean(id),
  });
}

export function useSetOrganizationOwner(
  organizationId: string,
): UseMutationResult<OrganizationDetail, Error, string | null> {
  const api = usePlgApi();
  const client = useQueryClient();
  return useMutation({
    mutationFn: (ownerId: string | null) =>
      api.patch<OrganizationDetail>(`/organizations/${organizationId}`, { ownerId }),
    onSuccess: (detail) => {
      client.setQueryData(queryKeys.organization(organizationId), detail);
      void client.invalidateQueries({ queryKey: ["organizations"] });
      void client.invalidateQueries({ queryKey: ["work-queue"] });
    },
  });
}

// ---------------------------------------------------------------------------
// The product tab
// ---------------------------------------------------------------------------

export function useProductDetail(
  organizationId: string | undefined,
  productCode: string | undefined,
): UseQueryResult<ProductDetail> {
  const api = usePlgApi();
  return useQuery({
    queryKey: queryKeys.product(organizationId ?? "", productCode ?? ""),
    queryFn: () => api.get<ProductDetail>(`/organizations/${organizationId}/products/${productCode}`),
    enabled: Boolean(organizationId && productCode),
  });
}

/**
 * Every product-tab mutation resolves to the reloaded ProductDetail, so one
 * shared success handler puts it straight into the cache.
 *
 * That matters more here than usual: ticking "Initiate playbook" moves the run's
 * status, the pairing's progress counts and the work queue at once. Writing the
 * returned detail back is what keeps those three consistent without a refetch
 * storm.
 */
function useProductMutation<TVariables>(
  organizationId: string,
  productCode: string,
  mutationFn: (variables: TVariables) => Promise<ProductDetail>,
): UseMutationResult<ProductDetail, Error, TVariables> {
  const client = useQueryClient();
  return useMutation({
    mutationFn,
    onSuccess: (detail) => {
      client.setQueryData(queryKeys.product(organizationId, productCode), detail);
      void client.invalidateQueries({ queryKey: queryKeys.organization(organizationId) });
      void client.invalidateQueries({ queryKey: ["organizations"] });
      void client.invalidateQueries({ queryKey: ["registrations"] });
      void client.invalidateQueries({ queryKey: ["work-queue"] });
      void client.invalidateQueries({ queryKey: ["dashboard"] });
    },
  });
}

export function usePatchProduct(organizationId: string, productCode: string) {
  const api = usePlgApi();
  return useProductMutation<PatchOrgPlatformRequest>(organizationId, productCode, (body) =>
    api.patch<ProductDetail>(`/organizations/${organizationId}/products/${productCode}`, body),
  );
}

export function useAttachPlaybook(organizationId: string, productCode: string) {
  const api = usePlgApi();
  return useProductMutation<string>(organizationId, productCode, (playbookId) =>
    api.post<ProductDetail>(`/organizations/${organizationId}/products/${productCode}/playbook-runs`, {
      playbookId,
    }),
  );
}

export function useDetachRun(organizationId: string, productCode: string) {
  const api = usePlgApi();
  return useProductMutation<string>(organizationId, productCode, (runId) =>
    api.del<ProductDetail>(`/playbook-runs/${runId}`),
  );
}

export function usePatchRunTask(organizationId: string, productCode: string) {
  const api = usePlgApi();
  return useProductMutation<{ taskId: string; body: PatchRunTaskRequest }>(
    organizationId,
    productCode,
    ({ taskId, body }) => api.patch<ProductDetail>(`/playbook-run-tasks/${taskId}`, body),
  );
}

export function useCreateNote(organizationId: string, productCode: string) {
  const api = usePlgApi();
  return useProductMutation<string>(organizationId, productCode, (body) =>
    api.post<ProductDetail>(`/organizations/${organizationId}/products/${productCode}/notes`, { body }),
  );
}

/**
 * Correct the wording of a note.
 *
 * Addressed by note id, not by pairing, but still keyed to this product tab so
 * the reply — a whole ProductDetail — lands in the same cache entry the page is
 * reading. Only the note's author may edit it; the API answers 403 otherwise.
 */
export function useUpdateNote(organizationId: string, productCode: string) {
  const api = usePlgApi();
  return useProductMutation<{ noteId: string; body: string }>(
    organizationId,
    productCode,
    ({ noteId, body }) => api.patch<ProductDetail>(`/notes/${noteId}`, { body }),
  );
}

// ---------------------------------------------------------------------------
// Registrations
// ---------------------------------------------------------------------------

export function useRegistrations(
  req: SearchRegistrationsRequest,
): UseQueryResult<SearchRegistrationsResponse> {
  const api = usePlgApi();
  return useQuery({
    queryKey: queryKeys.registrations(req),
    queryFn: () => api.post<SearchRegistrationsResponse>("/registrations/search", req),
    placeholderData: (previous) => previous,
  });
}

export function useAcknowledge(): UseMutationResult<
  ProductDetail,
  Error,
  { orgPlatformId: string; ownerId?: string | null }
> {
  const api = usePlgApi();
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({ orgPlatformId, ownerId }) =>
      api.post<ProductDetail>(`/registrations/${orgPlatformId}/acknowledge`, { ownerId }),
    onSuccess: (detail) => {
      client.setQueryData(queryKeys.product(detail.organizationId, detail.product.code), detail);
      void client.invalidateQueries({ queryKey: ["registrations"] });
      void client.invalidateQueries({ queryKey: ["organizations"] });
      void client.invalidateQueries({ queryKey: queryKeys.organization(detail.organizationId) });
      void client.invalidateQueries({ queryKey: ["dashboard"] });
    },
  });
}

// ---------------------------------------------------------------------------
// Playbook manager
// ---------------------------------------------------------------------------

export function usePlaybooks(productCode?: string): UseQueryResult<Playbook[]> {
  const api = usePlgApi();
  return useQuery({
    queryKey: queryKeys.playbooks(productCode),
    queryFn: async () => {
      const suffix = productCode ? `?product=${encodeURIComponent(productCode)}` : "";
      return (await api.get<{ playbooks: Playbook[] }>(`/playbooks${suffix}`)).playbooks;
    },
  });
}

function usePlaybookMutation<TVariables>(
  mutationFn: (variables: TVariables) => Promise<unknown>,
): UseMutationResult<unknown, Error, TVariables> {
  const client = useQueryClient();
  return useMutation({
    mutationFn,
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: ["playbooks"] });
      // A template change alters what a pairing may add, so the product tab's
      // "available playbooks" list is stale too.
      void client.invalidateQueries({ queryKey: ["org-product"] });
    },
  });
}

export function useCreatePlaybook(productCode: string) {
  const api = usePlgApi();
  return usePlaybookMutation<CreatePlaybookRequest>((body) =>
    api.post<Playbook>(`/products/${productCode}/playbooks`, body),
  );
}

export function usePatchPlaybook(playbookId: string) {
  const api = usePlgApi();
  return usePlaybookMutation<PatchPlaybookRequest>((body) =>
    api.patch<Playbook>(`/playbooks/${playbookId}`, body),
  );
}

export function useReplacePlaybookTasks(playbookId: string) {
  const api = usePlgApi();
  return usePlaybookMutation<PlaybookTaskInput[]>((tasks) =>
    api.put<Playbook>(`/playbooks/${playbookId}/tasks`, { tasks }),
  );
}

export function useDeletePlaybook() {
  const api = usePlgApi();
  return usePlaybookMutation<string>((playbookId) => api.del<void>(`/playbooks/${playbookId}`));
}

// ---------------------------------------------------------------------------
// Analytics
// ---------------------------------------------------------------------------

export function useDashboard(from?: string, to?: string): UseQueryResult<DashboardAnalytics> {
  const api = usePlgApi();
  return useQuery({
    queryKey: queryKeys.dashboard(from, to),
    queryFn: () => {
      const params = new URLSearchParams();
      if (from) params.set("from", from);
      if (to) params.set("to", to);
      const suffix = params.toString() ? `?${params.toString()}` : "";
      return api.get<DashboardAnalytics>(`/analytics/dashboard${suffix}`);
    },
    placeholderData: (previous) => previous,
  });
}

export function useWorkQueue(query: WorkQueueQuery): UseQueryResult<WorkQueueResponse> {
  const api = usePlgApi();
  return useQuery({
    queryKey: queryKeys.workQueue(query),
    queryFn: () => {
      const params = new URLSearchParams();
      if (query.mine) params.set("mine", "true");
      if (query.owner) params.set("owner", query.owner);
      for (const [key, values] of Object.entries(query)) {
        if (Array.isArray(values)) {
          for (const value of values) params.append(key, value);
        }
      }
      const suffix = params.toString() ? `?${params.toString()}` : "";
      return api.get<WorkQueueResponse>(`/work-queue${suffix}`);
    },
    placeholderData: (previous) => previous,
  });
}
