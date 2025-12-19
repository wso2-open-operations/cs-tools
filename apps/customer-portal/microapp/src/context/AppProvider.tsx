import LayoutProvider from "./layout/LayoutProvider";

export default function AppProvider({ children }: { children: React.ReactNode }) {
  return <LayoutProvider>{children}</LayoutProvider>;
}
