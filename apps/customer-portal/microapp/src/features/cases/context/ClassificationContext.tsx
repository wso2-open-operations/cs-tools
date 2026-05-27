import { createContext, type ReactNode, useContext, useState } from "react";

interface ClassificationContextProps {
  classified: Set<string>;
  set: (fields: string[]) => void;
  remove: (fields: string[]) => void;
}

const ClassificationContext = createContext<ClassificationContextProps | undefined>(undefined);

export function ClassificationProvider({ children }: { children: ReactNode }) {
  const [classified, setClassified] = useState<Set<string>>(new Set());

  const set = (fields: string[]) => {
    setClassified((prev) => new Set([...prev, ...fields]));
  };

  const remove = (fields: string[]) => {
    setClassified((prev) => {
      const next = new Set(prev);
      fields.forEach((field) => next.delete(field));
      return next;
    });
  };

  return (
    <ClassificationContext.Provider value={{ classified, set, remove }}>{children}</ClassificationContext.Provider>
  );
}

export function useClassification() {
  const context = useContext(ClassificationContext);
  if (!context) {
    throw new Error("useClassification must be within a ClassificationProvider");
  }
  return context;
}
