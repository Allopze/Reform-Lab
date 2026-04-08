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
import {
  type AuthUser,
  getToken,
  saveToken,
  clearToken,
  login as apiLogin,
  register as apiRegister,
  type LoginPayload,
  type RegisterPayload,
} from "./auth";
import { API_URL } from "./config";

interface AuthState {
  user: AuthUser | null;
  loading: boolean;
}

interface AuthContextValue extends AuthState {
  login: (payload: LoginPayload) => Promise<void>;
  register: (payload: RegisterPayload) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({ user: null, loading: true });

  // On mount, check token and fetch user profile
  useEffect(() => {
    const token = getToken();
    if (!token) {
      setState({ user: null, loading: false });
      return;
    }

    fetch(`${API_URL}/api/auth/me`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((res) => {
        if (!res.ok) throw new Error("invalid token");
        return res.json();
      })
      .then((user: AuthUser) => setState({ user, loading: false }))
      .catch(() => {
        clearToken();
        setState({ user: null, loading: false });
      });
  }, []);

  const login = useCallback(async (payload: LoginPayload) => {
    const result = await apiLogin(payload);
    saveToken(result.token);
    setState({ user: result.user, loading: false });
  }, []);

  const register = useCallback(async (payload: RegisterPayload) => {
    const result = await apiRegister(payload);
    saveToken(result.token);
    setState({ user: result.user, loading: false });
  }, []);

  const logout = useCallback(() => {
    clearToken();
    setState({ user: null, loading: false });
  }, []);

  const value = useMemo(
    () => ({ ...state, login, register, logout }),
    [state, login, register, logout]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
