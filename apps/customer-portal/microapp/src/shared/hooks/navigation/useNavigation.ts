import {
  useCaseNavigation,
  useChangeRequestNavigation,
  useMultipleNavigation,
  useServiceRequestNavigation,
} from "@shared/hooks/navigation";

export const useNavigation = () => ({
  ...useCaseNavigation(),
  ...useServiceRequestNavigation(),
  ...useChangeRequestNavigation(),
  ...useMultipleNavigation(),
});
