import {
  useCaseNavigation,
  useChangeRequestNavigation,
  useCoreNavigation,
  useMultipleNavigation,
  useServiceRequestNavigation,
} from "@shared/hooks/navigation";

export const useNavigation = () => ({
  ...useCoreNavigation(),
  ...useCaseNavigation(),
  ...useServiceRequestNavigation(),
  ...useChangeRequestNavigation(),
  ...useMultipleNavigation(),
});
