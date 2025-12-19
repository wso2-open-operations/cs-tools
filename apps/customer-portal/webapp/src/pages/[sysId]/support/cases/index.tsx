import { Box, Card, Typography } from "@mui/material";
import React, { useMemo, useState } from "react";
import { useParams } from "react-router-dom";
import {
  AllCasesFilters,
  AllCasesHeader,
  AllCasesList,
} from "@/components/AllCases";
import PreLoader from "@/components/PreLoader/PreLoader";
import { Endpoints } from "@/services/endpoints";
import { useGet } from "@/services/useApi";
import { keepPreviousData } from "@tanstack/react-query";
import type { Case, CaseResponse } from "@/types/support.types";
import type { ProjectMetadataResponse } from "@/types/project-metadata.types";
import {
  CircleAlert,
  Clock,
  MessageCircle,
  CircleCheck,
  FileText,
} from "lucide-react";

interface FilterOptionsResponse {
  statuses: string[];
  severities: string[];
  categories: string[];
  products: string[];
  environments: string[];
}

const AllCasesPage: React.FC = () => {
  const { sysId } = useParams<{ sysId: string }>();

  // State for filters
  const [activeFilters, setActiveFilters] = useState({
    status: "all",
    severity: "all",
    category: "all",
    product: "all",
  });
  const [searchQuery, setSearchQuery] = useState("");
  const [page, setPage] = useState(0);
  const [sortBy, setSortBy] = useState("newest");
  const LIMIT = 10; // Default limit

  // Prepare params for API (Sort & Filter)
  const apiParams = useMemo(() => {
    const params: any = {};

    if (activeFilters.severity !== "all")
      params.severity = activeFilters.severity;
    if (activeFilters.status !== "all") params.status = activeFilters.status;
    if (activeFilters.category !== "all")
      params.category = activeFilters.category;
    if (activeFilters.product !== "all") params.product = activeFilters.product;

    // Sort logic
    if (sortBy === "newest") {
      params.sort = "sys_created_on";
      params.order = "desc";
    } else if (sortBy === "oldest") {
      params.sort = "sys_created_on";
      params.order = "asc";
    }

    return params;
  }, [activeFilters, sortBy]);

  // Scroll to top when component mounts
  // useEffect(() => {
  //   window.scrollTo(0, 0);
  // }, []);

  const { data, isLoading, error, isPlaceholderData } = useGet<CaseResponse>(
    ["getAllCases", sysId, page, activeFilters, sortBy],
    Endpoints.getAllCases(sysId || "", page * LIMIT, LIMIT),
    {
      enabled: !!sysId,
      placeholderData: keepPreviousData,
    },
    {
      params: apiParams,
    }
  );

  const { data: filterOptionsData } = useGet<FilterOptionsResponse>(
    ["getFilterOptions", sysId],
    Endpoints.getCaseFilterOptions(sysId || ""),
    {
      enabled: !!sysId,
    }
  );

  const { data: projectMetadata } = useGet<ProjectMetadataResponse>(
    ["project-metadata", sysId],
    Endpoints.getProjectMetaData(sysId || ""),
    {
      enabled: !!sysId,
    }
  );

  const filterOptions = useMemo(() => {
    if (!filterOptionsData) return undefined;
    return {
      statuses: filterOptionsData.statuses || [],
      severities: filterOptionsData.severities || [],
      categories: filterOptionsData.categories || [],
      projects: filterOptionsData.products || [],
      environments: filterOptionsData.environments || [],
    };
  }, [filterOptionsData]);

  const handleFilterChange = (type: string, value: string) => {
    // Map UI 'project' filter to 'product' state
    const stateKey = type === "project" ? "product" : type;
    setActiveFilters((prev) => ({ ...prev, [stateKey]: value }));
    setPage(0); // Reset to first page on filter change
  };

  const handleClearFilters = () => {
    setSearchQuery("");
    setActiveFilters({
      status: "all",
      severity: "all",
      category: "all",
      product: "all",
    });
    setPage(0);
  };

  // Map API response to UI model
  const filteredCases = useMemo(() => {
    if (!data?.cases) return [];

    const cases = data.cases;

    // Client-side search (filtering reduced set of current page)
    if (searchQuery) {
      return cases.filter((c: Case) => {
        return (
          searchQuery === "" ||
          c.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
          c.number.toLowerCase().includes(searchQuery.toLowerCase()) ||
          (c.description &&
            c.description.toLowerCase().includes(searchQuery.toLowerCase()))
        );
      });
    }

    return cases;
  }, [data, searchQuery]);

  if (isLoading) {
    return <PreLoader isLoading={isLoading} />;
  }

  if (error) {
    return <Box sx={{ p: 4 }}>Error loading cases.</Box>;
  }

  // Extract data from new structure
  // const stats = data?.stats;
  // const filterOptions = data?.filterOptions;

  return (
    <Box sx={{ pb: 4 }}>
      <Box sx={{ px: 4, py: 3 }}>
        <AllCasesHeader />

        {projectMetadata && (
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: {
                xs: "repeat(2, 1fr)",
                md: "repeat(5, 1fr)",
              },
              gap: 2,
              mb: 3,
              mt: 3,
            }}
          >
            {[
              {
                label: "Open",
                value: projectMetadata.projectStatistics.openCasesCount,
                icon: CircleAlert,
                color: "text-blue-600",
                iconColor: "#2563eb",
              },
              {
                label: "In Progress",
                value: projectMetadata.projectStatistics.inProgressCasesCount,
                icon: Clock,
                color: "text-orange-600",
                iconColor: "#ea580c",
              },
              {
                label: "Awaiting",
                value:
                  projectMetadata.projectStatistics.activeCaseCount
                    .awaitingCount,
                icon: MessageCircle,
                color: "text-yellow-600",
                iconColor: "#ca8a04",
              },
              {
                label: "Resolved",
                value: projectMetadata.projectStatistics.resolvedCasesCount,
                icon: CircleCheck,
                color: "text-green-600",
                iconColor: "#16a34a",
              },
              {
                label: "Total",
                value: projectMetadata.projectStatistics.totalCasesCount,
                icon: FileText,
                color: "text-gray-600",
                iconColor: "#4b5563",
              },
            ].map((stat, index) => (
              <Card
                key={index}
                variant="outlined"
                sx={{
                  p: 2,
                  display: "flex",
                  flexDirection: "column",
                  gap: 2,
                  borderRadius: "12px",
                }}
              >
                <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                  <stat.icon size={16} color={stat.iconColor} />
                  <Typography sx={{ fontSize: "0.75rem", color: "grey.600" }}>
                    {stat.label}
                  </Typography>
                </Box>
                <Typography
                  sx={{ fontSize: "1.5rem", color: "grey.900", lineHeight: 1 }}
                >
                  {stat.value}
                </Typography>
              </Card>
            ))}
          </Box>
        )}

        {filterOptions && (
          <AllCasesFilters
            filterOptions={filterOptions}
            activeFilters={{
              ...activeFilters,
              project: activeFilters.product, // Map product state back to project prop for UI
            }}
            onFilterChange={handleFilterChange}
            onClearFilters={handleClearFilters}
            totalCases={filteredCases.length}
            onSearch={setSearchQuery} // Updated to directly set search query
            searchValue={searchQuery}
            sortBy={sortBy}
            onSortChange={setSortBy}
          />
        )}

        <AllCasesList cases={filteredCases} />

        {/* Pagination Controls */}
        <Box
          sx={{
            display: "flex",
            justifyContent: "flex-end",
            alignItems: "center",
            mt: 3,
            gap: 2,
          }}
        >
          <button
            onClick={() => setPage((old) => Math.max(old - 1, 0))}
            disabled={page === 0}
            style={{
              padding: "8px 16px",
              borderRadius: "4px",
              border: "1px solid #e2e8f0",
              background: page === 0 ? "#f1f5f9" : "white",
              cursor: page === 0 ? "not-allowed" : "pointer",
              color: page === 0 ? "#94a3b8" : "#0f172a",
            }}
          >
            Previous Page
          </button>

          <Box
            component="span"
            sx={{ color: "grey.600", fontSize: "0.875rem" }}
          >
            Page {page + 1}
          </Box>

          <button
            onClick={() => {
              if (
                !isPlaceholderData &&
                (page + 1) * LIMIT < (data?.pagination?.totalRecords || 0)
              ) {
                setPage((old) => old + 1);
              }
            }}
            disabled={
              isPlaceholderData ||
              !data?.cases ||
              (page + 1) * LIMIT >= (data?.pagination?.totalRecords || 0)
            }
            style={{
              padding: "8px 16px",
              borderRadius: "4px",
              border: "1px solid #e2e8f0",
              background:
                isPlaceholderData ||
                !data?.cases ||
                (page + 1) * LIMIT >= (data?.pagination?.totalRecords || 0)
                  ? "#f1f5f9"
                  : "white",
              cursor:
                isPlaceholderData ||
                !data?.cases ||
                (page + 1) * LIMIT >= (data?.pagination?.totalRecords || 0)
                  ? "not-allowed"
                  : "pointer",
              color:
                isPlaceholderData ||
                !data?.cases ||
                (page + 1) * LIMIT >= (data?.pagination?.totalRecords || 0)
                  ? "#94a3b8"
                  : "#0f172a",
            }}
          >
            Next Page
          </button>
        </Box>
      </Box>
    </Box>
  );
};

export default AllCasesPage;
