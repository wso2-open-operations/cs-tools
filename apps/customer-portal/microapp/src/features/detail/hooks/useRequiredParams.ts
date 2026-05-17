import { useParams } from "react-router-dom";

export function useRequiredParams() {
  const { id } = useParams();
  if (id === undefined) throw new Error(`Missing required param: id`);
  return { id };
}
