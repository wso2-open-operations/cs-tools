import { useNavigate } from "react-router-dom";

export const useCoreNavigation = () => {
  const navigate = useNavigate();

  return {
    back: () => navigate(-1),
  };
};
