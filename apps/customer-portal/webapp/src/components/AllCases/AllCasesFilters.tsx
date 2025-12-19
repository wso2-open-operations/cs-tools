import {
  Box,
  Button,
  Chip,
  Divider,
  InputBase,
  MenuItem,
  Select,
  Typography,
} from "@mui/material";
import React from "react";
import {
  ChevronDownIcon,
  ChevronUpIcon,
  SearchIcon,
  SlidersIcon,
  CloseIcon,
} from "@/assets/icons/common-icons";

interface FilterOptions {
  statuses: string[];
  severities: string[];
  categories: string[];
  projects: string[];
}

interface ActiveFilters {
  status: string;
  severity: string;
  category: string;
  project: string;
}

interface AllCasesFiltersProps {
  filterOptions: FilterOptions;
  activeFilters: ActiveFilters;
  onFilterChange: (type: string, value: string) => void;
  onClearFilters: () => void;
  totalCases: number;
  onSearch: (value: string) => void;
  searchValue: string;
  sortBy: string;
  onSortChange: (value: string) => void;
}

export const AllCasesFilters: React.FC<AllCasesFiltersProps> = ({
  filterOptions,
  activeFilters,
  onFilterChange,
  onClearFilters,
  totalCases,
  onSearch,
  searchValue,
  sortBy,
  onSortChange,
}) => {
  const [showFilters, setShowFilters] = React.useState(true);

  // Calculate active filters count
  const activeFiltersCount = [
    activeFilters.status !== "all",
    activeFilters.severity !== "all",
    activeFilters.category !== "all",
    activeFilters.project !== "all",
    searchValue !== "",
  ].filter(Boolean).length;

  const SelectButton = ({
    label,
    value,
    options,
    onChange,
  }: {
    label: string;
    value: string;
    options: string[];
    onChange: (val: string) => void;
  }) => (
    <Box>
      <Typography
        variant="body2"
        color="text.secondary"
        sx={{ mb: 1, fontSize: "0.875rem" }}
      >
        {label}
      </Typography>
      <Select
        value={value}
        displayEmpty
        onChange={(e) => onChange(e.target.value as string)}
        renderValue={(selected) => {
          if (selected === "all") {
            if (label === "Category") return "All Categories";
            if (label === "Severity") return "All Severities";
            if (label === "Status") return "All Statuses";
            return `All ${label}s`;
          }
          return selected;
        }}
        sx={{
          height: 36,
          fontSize: "0.875rem",
          backgroundColor: "#F9FAFB", // bg-gray-50
          "& .MuiOutlinedInput-notchedOutline": {
            borderColor: "grey.200",
          },
          "&:hover .MuiOutlinedInput-notchedOutline": {
            borderColor: "grey.300",
          },
          width: "100%",
          borderRadius: "6px",
        }}
        IconComponent={(props) => (
          <Box
            {...props}
            sx={{
              ...props.sx,
              right: 7,
              top: "calc(50% - 0.5em)",
              pointerEvents: "none",
            }}
          >
            <ChevronDownIcon width={16} height={16} color="#9ca3af" />
          </Box>
        )}
      >
        <MenuItem value="all">
          {label === "Category"
            ? "All Categories"
            : label === "Severity"
            ? "All Severities"
            : label === "Status"
            ? "All Statuses"
            : `All ${label}s`}
        </MenuItem>
        {options.map((opt) => (
          <MenuItem key={opt} value={opt}>
            {opt}
          </MenuItem>
        ))}
      </Select>
    </Box>
  );

  return (
    <>
      <Box
        sx={{
          p: 3, // p-6
          border: "1px solid",
          borderColor: "grey.200",
          borderRadius: "12px",
          mb: 3,
          backgroundColor: "background.paper",
        }}
      >
        <Box sx={{ display: "flex", gap: 1.5, mb: showFilters ? 2 : 0 }}>
          <Box
            sx={{
              position: "relative",
              flex: 1,
            }}
          >
            <Box
              sx={{
                position: "absolute",
                left: 12,
                top: "50%",
                transform: "translateY(-50%)",
                color: "grey.400",
                display: "flex",
                zIndex: 1,
              }}
            >
              <SearchIcon width={16} height={16} />
            </Box>
            <InputBase
              placeholder="Search cases by ID, title, or description..."
              value={searchValue}
              onChange={(e) => onSearch(e.target.value)}
              sx={{
                width: "100%",
                height: 36,
                pl: 5,
                pr: 2,
                fontSize: "0.875rem",
                border: "1px solid",
                borderColor: "grey.200",
                borderRadius: "6px",
                transition: "box-shadow 0.2s",
                backgroundColor: "#F9FAFB",
                "&.Mui-focused": {
                  boxShadow: "0 0 0 3px rgba(37, 99, 235, 0.2)",
                  borderColor: "primary.main",
                },
              }}
            />
          </Box>
          <Button
            variant="outlined"
            onClick={() => setShowFilters(!showFilters)}
            sx={{
              borderColor: "grey.200",
              color: "text.primary",
              textTransform: "none",
              height: 36,
              px: 2,
              backgroundColor: "background.paper",
              gap: 1,
              "&:hover": {
                backgroundColor: "action.hover",
              },
            }}
          >
            <SlidersIcon width={16} height={16} />
            Filters
            {activeFiltersCount > 0 && (
              <Chip
                label={activeFiltersCount}
                size="small"
                sx={{
                  height: 20,
                  minWidth: 20,
                  fontSize: "0.75rem",
                  backgroundColor: "grey.200",
                  color: "text.primary",
                  "& .MuiChip-label": { px: 0.8 },
                }}
              />
            )}
            {showFilters ? (
              <ChevronUpIcon width={16} height={16} />
            ) : (
              <ChevronDownIcon width={16} height={16} />
            )}
          </Button>
        </Box>

        {showFilters && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
            <Box
              sx={{
                display: "grid",
                gridTemplateColumns: { xs: "1fr", md: "repeat(4, 1fr)" },
                gap: 2,
              }}
            >
              <SelectButton
                label="Status"
                value={activeFilters.status}
                options={filterOptions.statuses}
                onChange={(val) => onFilterChange("status", val)}
              />
              <SelectButton
                label="Severity"
                value={activeFilters.severity}
                options={filterOptions.severities}
                onChange={(val) => onFilterChange("severity", val)}
              />
              <SelectButton
                label="Category"
                value={activeFilters.category}
                options={filterOptions.categories}
                onChange={(val) => onFilterChange("category", val)}
              />
              <SelectButton
                label="Project"
                value={activeFilters.project}
                options={filterOptions.projects}
                onChange={(val) => onFilterChange("project", val)}
              />
            </Box>

            {activeFiltersCount > 0 && (
              <>
                <Divider sx={{ my: 0 }} />
                <Box
                  sx={{
                    display: "flex",
                    justifyContent: "space-between",
                    alignItems: "center",
                  }}
                >
                  <Typography variant="body2" color="text.secondary">
                    {activeFiltersCount} filter
                    {activeFiltersCount !== 1 ? "s" : ""} active
                  </Typography>
                  <Button
                    onClick={onClearFilters}
                    startIcon={<CloseIcon width={14} height={14} />}
                    sx={{
                      textTransform: "none",
                      color: "text.secondary",
                      fontSize: "0.875rem",
                      "&:hover": {
                        backgroundColor: "action.hover",
                        color: "text.primary",
                      },
                    }}
                  >
                    Clear all filters
                  </Button>
                </Box>
              </>
            )}
          </Box>
        )}
      </Box>

      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          mb: 2,
        }}
      >
        <Typography variant="body2" color="text.secondary">
          Showing {totalCases} of {totalCases} cases
        </Typography>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Typography variant="body2" color="text.secondary">
            Sort by:
          </Typography>
          <Select
            value={sortBy}
            onChange={(e) => onSortChange(e.target.value as string)}
            variant="outlined"
            sx={{
              height: 36,
              fontSize: "0.875rem",
              backgroundColor: "#F9FAFB", // bg-gray-50
              "& .MuiOutlinedInput-notchedOutline": {
                borderColor: "grey.200",
              },
              borderRadius: "6px",
              width: 180,
            }}
            IconComponent={(props) => (
              <Box
                {...props}
                sx={{
                  ...props.sx,
                  right: 7,
                  top: "calc(50% - 0.5em)",
                  pointerEvents: "none",
                }}
              >
                <ChevronDownIcon width={16} height={16} color="#9ca3af" />
              </Box>
            )}
          >
            <MenuItem value="newest">Newest First</MenuItem>
            <MenuItem value="oldest">Oldest First</MenuItem>
          </Select>
        </Box>
      </Box>
    </>
  );
};
