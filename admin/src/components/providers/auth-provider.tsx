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
import { getAccessToken, clearTokens } from "@/lib/auth";
import type { Admin } from "@/types";

interface AuthContextType {
  admin: Admin | null;
  subjectType: "admin" | null;
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
  const [subjectType, setSubjectType] = useState<"admin" | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    const token = getAccessToken();
    if (!token) {
      setAdmin(null);
      setSubjectType(null);
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
    await clearTokens();
  }, []);

  return (
    <AuthContext.Provider value={{ admin, subjectType, loading, signOut, refreshSubject }}>
      {children}
    </AuthContext.Provider>
  );
}
