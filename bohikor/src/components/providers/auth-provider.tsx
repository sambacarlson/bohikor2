"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { api } from "@/lib/api";
import { getAccessToken, getSubjectHint, clearTokens } from "@/lib/auth";
import type { Admin } from "@/types";

type SubjectType = "admin" | "platform_admin" | null;

interface AuthContextType {
  admin: Admin | null;
  subjectType: SubjectType;
  loading: boolean;
  signOut: () => Promise<void>;
  refreshSubject: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType>({
  admin: null,
  subjectType: null,
  loading: true,
  signOut: async () => {},
  refreshSubject: async () => {},
});

export function useAuth() {
  return useContext(AuthContext);
}

function useSubjectLoader() {
  const [admin, setAdmin] = useState<Admin | null>(null);
  const [subjectType, setSubjectType] = useState<SubjectType>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    const token = getAccessToken();
    const hint = getSubjectHint();

    if (!token || !hint) {
      setAdmin(null);
      setSubjectType(null);
      setLoading(false);
      return;
    }

    if (hint === "platform_admin") {
      // No /api/platform/me endpoint yet - trust the stored hint so the
      // platform shell can render; real profile loading lands with the
      // platform-console task.
      setAdmin(null);
      setSubjectType("platform_admin");
      setLoading(false);
      return;
    }

    try {
      const { data } = await api.get<{ data: Admin }>("/api/admin/me");
      setAdmin(data.data);
      setSubjectType("admin");
    } catch {
      clearTokens();
      setAdmin(null);
      setSubjectType(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load(); // eslint-disable-line react-hooks/set-state-in-effect -- auth initialization must fetch and set state on mount
  }, [load]);

  return { admin, subjectType, loading, load };
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const { admin, subjectType, loading, load: refreshSubject } = useSubjectLoader();

  const signOut = useCallback(async () => {
    try {
      await api.post("/api/auth/logout");
    } catch {
      // Ignore logout API errors
    }
    clearTokens();
  }, []);

  return (
    <AuthContext.Provider value={{ admin, subjectType, loading, signOut, refreshSubject }}>
      {children}
    </AuthContext.Provider>
  );
}
