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
import type { Admin, User } from "@/types";

interface AuthContextType {
  user: User | null;
  admin: Admin | null;
  subjectType: "user" | "admin" | null;
  loading: boolean;
  signOut: () => Promise<void>;
  refreshSubject: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType>({
  user: null,
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
  const [user, setUser] = useState<User | null>(null);
  const [admin, setAdmin] = useState<Admin | null>(null);
  const [subjectType, setSubjectType] = useState<"user" | "admin" | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    const token = getAccessToken();
    if (!token) {
      setUser(null);
      setAdmin(null);
      setSubjectType(null);
      setLoading(false);
      return;
    }

    try {
      const { data } = await api.get<{ data: Admin }>("/api/admin/me");
      setAdmin(data.data);
      setUser(null);
      setSubjectType("admin");
      setLoading(false);
      return;
    } catch {
      // Not an admin, try user
    }

    try {
      const { data } = await api.get<{ data: User }>("/api/users/me");
      setUser(data.data);
      setAdmin(null);
      setSubjectType("user");
      setLoading(false);
      return;
    } catch {
      clearTokens();
      setUser(null);
      setAdmin(null);
      setSubjectType(null);
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load(); // eslint-disable-line react-hooks/set-state-in-effect -- auth initialization must fetch and set state on mount
  }, [load]);

  return { user, admin, subjectType, loading, load };
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const { user, admin, subjectType, loading, load: refreshSubject } = useSubjectLoader();

  const signOut = useCallback(async () => {
    try {
      await api.post("/api/auth/logout");
    } catch {
      // Ignore logout API errors
    }
    await clearTokens();
  }, []);

  return (
    <AuthContext.Provider value={{ user, admin, subjectType, loading, signOut, refreshSubject }}>
      {children}
    </AuthContext.Provider>
  );
}