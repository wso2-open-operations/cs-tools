// HTTP Methods
export type HTTPMethod = "POST" | "GET" | "PUT" | "PATCH" | "DELETE";

export interface Endpoint {
  baseUrl: string;
  path: string;
  httpMethod: HTTPMethod;
  mock?: MockEndpoint;
  cacheEnabled?: boolean;
}

// Used to stub the API response
export interface MockEndpoint {
  enabled: boolean;
  resource: string;
  delay?: number;
}

const BASE_URL = import.meta.env.VITE_APP_BACKEND_BASE_URL;

/**
 * A helper function to build a URL with query parameters.
 */
export const buildPathWithParams = (
  basePath: string,
  params?: Record<string, string | number | boolean>
): string => {
  if (!params) {
    return basePath;
  }

  const searchParams = new URLSearchParams();
  for (const key in params) {
    if (Object.prototype.hasOwnProperty.call(params, key)) {
      const value = params[key];
      if (value !== undefined && value !== null) {
        searchParams.append(key, String(value));
      }
    }
  }

  const queryString = searchParams.toString();
  return queryString ? `${basePath}?${queryString}` : basePath;
};

// Endpoints
export const Endpoints = {
  // Projects
  getAllProjects: (offset: number = 0, limit: number = 10): Endpoint => ({
    baseUrl: BASE_URL,
    path: buildPathWithParams("/projects", { offset, limit }),
    httpMethod: "GET",
    cacheEnabled: true,
  }),

  // Cases
  getAllCases: (
    projectId: string,
    offset: number = 0,
    limit: number = 10
  ): Endpoint => ({
    baseUrl: BASE_URL,
    path: buildPathWithParams(`/projects/${projectId}/cases`, {
      offset,
      limit,
    }),
    httpMethod: "GET",
    cacheEnabled: true,
  }),

  getCaseFilterOptions: (projectId: string): Endpoint => ({
    baseUrl: BASE_URL,
    path: `/projects/${projectId}/cases/filters`,
    httpMethod: "GET",
    cacheEnabled: true,
  }),

  getCaseDetails: (projectId: string, caseId: string): Endpoint => ({
    baseUrl: BASE_URL,
    path: `/projects/${projectId}/cases/${caseId}`,
    httpMethod: "GET",
    cacheEnabled: true,
  }),

  getProjectMetaData: (projectId: string): Endpoint => ({
    baseUrl: BASE_URL,
    path: `/projects/${projectId}/overview`,
    httpMethod: "GET",
    cacheEnabled: true,
  }),

  postCase: (projectId: string): Endpoint => ({
    baseUrl: BASE_URL,
    path: `/projects/${projectId}/case`,
    httpMethod: "POST",
    cacheEnabled: true,
  }),
};
