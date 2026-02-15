import LayoutProvider from "./layout/LayoutProvider";
import ProjectProvider from "./project/ProjectProvider";

export default function AppProvider({ children }: { children: React.ReactNode }) {
  return (
    <LayoutProvider>
      <ProjectProvider>{children}</ProjectProvider>
    </LayoutProvider>
  );
}
