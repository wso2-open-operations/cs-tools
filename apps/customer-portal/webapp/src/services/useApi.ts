import {
  type QueryKey,
  useMutation,
  type UseMutationResult,
  useQuery,
  useQueryClient,
  type UseQueryResult,
  type UseQueryOptions,
} from "@tanstack/react-query";
import { AxiosError, type AxiosRequestConfig } from "axios";
import { useEffect, useRef } from "react";
import apiClient from "./apiClient";
import { type Endpoint, type HTTPMethod } from "./endpoints";

// ErrorTypes
export const ErrorTypes = {
  CanceledError: "ERR_CANCELED",
  AbortError: "AbortError",
} as const;

// Type for our mutation function
type MutationFn<TData, TVariables> = (variables: TVariables) => Promise<TData>;

/**
 * Delay for the API (Mocking latency)
 */
const delay = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

/**
 * Construct the URL for the endpoint.
 */
const constructURL = (endpoint: Endpoint) => {
  return `${endpoint.baseUrl}${endpoint.path}`;
};

/**
 * Custom hook for GET requests using Tanstack Query.
 */
// Custom hook for GET requests using Tanstack Query
export const useGet = <TData>(
  queryKey: QueryKey,
  endpoint: Endpoint,
  queryOptions?: Omit<
    UseQueryOptions<TData, AxiosError>,
    "queryKey" | "queryFn"
  >,
  axiosOptions?: AxiosRequestConfig
): UseQueryResult<TData, AxiosError> => {
  return useQuery<TData, AxiosError>({
    queryKey,
    ...queryOptions,
    queryFn: async () => {
      // Use mocks if enabled
      const mock = endpoint.mock;
      if (import.meta.env.DEV && mock?.resource && mock.enabled) {
        if (mock?.delay) {
          await delay(mock?.delay);
        }

        console.log(`Using mock for ${endpoint.path}`);
        // Dynamically import the mock JSON file.
        try {
          const mockData = await import(`./mocks/${mock.resource}.json`);
          return mockData.default as TData;
        } catch (e) {
          console.error(`Failed to load mock: ${mock.resource}`, e);
          throw e;
        }
      }

      const url = constructURL(endpoint);
      try {
        const response = await apiClient.get<TData>(url, axiosOptions);
        return response.data;
      } catch (error: any) {
        console.error("API GET Error: ", error.message);
        throw error;
      }
    },
  });
};

/**
 * Custom hook for mutations.
 */
export const useAPI = <TData, TVariables>(
  endpoint: Endpoint,
  method: HTTPMethod
): UseMutationResult<TData, AxiosError, TVariables> => {
  const queryClient = useQueryClient();
  const abortControllerRef = useRef<AbortController | null>(null);

  const mutationFn: MutationFn<TData, TVariables> = async (variables) => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
    }

    const abortController = new AbortController();
    abortControllerRef.current = abortController;

    const url = constructURL(endpoint);
    const response = await apiClient.request<TData>({
      url,
      method,
      data: variables,
      signal: abortController.signal,
    });
    return response.data;
  };

  useEffect(() => {
    return () => {
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }
    };
  }, []);

  return useMutation<TData, AxiosError, TVariables>({
    mutationKey: [endpoint.path],
    mutationFn,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [endpoint.path] });
    },
    onError: (error) => {
      console.error("API Mutation Error.", error.message);
    },
  });
};
