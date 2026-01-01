import { Search } from "@mui/icons-material";
import { InputAdornment, Stack, Tab, Tabs, InputBase as TextField } from "@mui/material";
import { useLocation, useNavigate, useSearchParams } from "react-router-dom";
import type { Status } from "./ItemCard";

export interface FilterTab {
  label: string;
  value: Status | "all";
}

export interface FilterSlotBuilderProps {
  tabs: FilterTab[];
  searchPlaceholder?: string;
}

export function FilterSlotBuilder({ tabs, searchPlaceholder }: FilterSlotBuilderProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams] = useSearchParams();

  const searchParamName = "search",
    filterParamName = "filter";

  const baseRoute = location.pathname;
  const activeFilter = searchParams.get(filterParamName) ?? "all";
  const searchValue = searchParams.get(searchParamName) ?? "";

  const updateParams = (updates: Record<string, string | null>) => {
    const next = new URLSearchParams(searchParams);

    Object.entries(updates).forEach(([key, value]) => {
      if (!value) {
        next.delete(key);
      } else {
        next.set(key, value);
      }
    });

    navigate(`${baseRoute}?${next.toString()}`, { replace: true });
  };

  return (
    <Stack gap={2} pb={1}>
      <TextField
        fullWidth
        type="search"
        placeholder={searchPlaceholder}
        value={searchValue}
        onChange={(e) => updateParams({ [searchParamName]: e.target.value || null })}
        startAdornment={
          <InputAdornment position="start">
            <Search />
          </InputAdornment>
        }
        sx={{
          mt: 1,
          bgcolor: "background.paper",
        }}
      />
      <Tabs
        value={activeFilter}
        scrollButtons={false}
        variant="scrollable"
        onChange={(_, value) => updateParams({ [filterParamName]: value })}
        sx={(theme) => ({
          minHeight: "unset",

          "& .MuiTabs-flexContainer": {
            gap: 1.2,
          },
          "& .MuiButtonBase-root": {
            minHeight: "unset",
            minWidth: "unset",
            p: "4px 12px",
            fontSize: theme.typography.subtitle1,
            color: "text.tertiary",
            fontWeight: "medium",
            textTransform: "revert",
            borderRadius: 999,
            backgroundColor: "background.default",
          },

          "& .MuiTab-root.Mui-selected": {
            color: theme.palette.primary.contrastText,
            backgroundColor: theme.palette.primary.main,
            fontSize: theme.typography.body1,
            fontWeight: "bold",
          },

          "& .MuiTabs-indicator": {
            display: "none",
          },
        })}
      >
        <Tab label="All" value="all" disableRipple />
        {tabs.map((tab, index) => (
          <Tab key={index} label={tab.label} value={tab.value} disableRipple />
        ))}
      </Tabs>
    </Stack>
  );
}
