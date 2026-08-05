"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { api } from "@/lib/api";
import { getAccessToken, getSubjectHint, clearTokens } from "@/lib/auth";
import type { Admin, User } from "@/types";

type SubjectType = "admin" | "platform_admin" | "user" | null;

interface AuthContextType {
  admin: Admin | null;
  user: User | null;
  subjectType: SubjectType;
  loading: boolean;
  signOut: () => Promise<void>;
  refreshSubject: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType>({
  admin: null,
  user: null,
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
  const [user, setUser] = useState<User | null>(null);
  const [subjectType, setSubjectType] = useState<SubjectType>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    const token = getAccessToken();
    const hint = getSubjectHint();

    if (!token || !hint) {
      setAdmin(null);
      setUser(null);
      setSubjectType(null);
      setLoading(false);
      return;
    }

    if (hint === "platform_admin") {
      // No /api/platform/me endpoint yet - trust the stored hint so the
      // platform shell can render; real profile loading lands with the
      // platform-console task.
      setAdmin(null);
      setUser(null);
      setSubjectType("platform_admin");
      setLoading(false);
      return;
    }

    try {
      if (hint === "user") {
        const { data } = await api.get<{ data: User }>("/api/users/me");
        setAdmin(null);
        setUser(data.data);
        setSubjectType("user");
      } else {
        const { data } = await api.get<{ data: Admin }>("/api/admin/me");
        setAdmin(data.data);
        setUser(null);
        setSubjectType("admin");
      }
    } catch {
      clearTokens();
      setAdmin(null);
      setUser(null);
      setSubjectType(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load(); // eslint-disable-line react-hooks/set-state-in-effect -- auth initialization must fetch and set state on mount
  }, [load]);

  return { admin, user, subjectType, loading, load };
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const { admin, user, subjectType, loading, load: refreshSubject } = useSubjectLoader();

  const signOut = useCallback(async () => {
    try {
      await api.post("/api/auth/logout");
    } catch {
      // Ignore logout API errors
    }
    clearTokens();
  }, []);

  const value = useMemo(
    () => ({ admin, user, subjectType, loading, signOut, refreshSubject }),
    [admin, user, subjectType, loading, signOut, refreshSubject]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
