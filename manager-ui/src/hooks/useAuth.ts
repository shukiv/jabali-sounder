import { useState, useCallback } from "react";
import apiClient from "../apiClient";

export interface AuthState {
  token: string | null;
  username: string | null;
  role: string | null;
}

const ROLE_RANK: Record<string, number> = { viewer: 1, operator: 2, owner: 3 };

// currentRole returns the signed-in operator's role, read from persisted auth.
export function currentRole(): string {
  return loadAuth().role ?? "";
}

// roleAtLeast reports whether the current role meets a minimum (viewer <
// operator < owner).
export function roleAtLeast(min: "viewer" | "operator" | "owner"): boolean {
  return (ROLE_RANK[currentRole()] ?? 0) >= ROLE_RANK[min];
}

const STORAGE_KEY = "jabali-sounder-auth";

export function loadAuth(): AuthState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      return {
        token: parsed.token || null,
        username: parsed.username || null,
        role: parsed.role || null,
      };
    }
  } catch {
    // ignore
  }
  return { token: null, username: null, role: null };
}

export function saveAuth(state: AuthState) {
  if (state.token) {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  } else {
    localStorage.removeItem(STORAGE_KEY);
  }
}

export function useAuth() {
  const [auth, setAuth] = useState<AuthState>(loadAuth);

  const acceptAuthResponse = useCallback((data: { token: string; admin: { username: string; role?: string } }) => {
    const newState = {
      token: data.token,
      username: data.admin.username,
      role: data.admin.role ?? null,
    };
    setAuth(newState);
    saveAuth(newState);
    apiClient.defaults.headers.common["Authorization"] = `Bearer ${data.token}`;
    return newState;
  }, []);

  const login = useCallback(
    async (username: string, password: string, totpCode?: string) => {
      const resp = await apiClient.post("/auth/login", {
        username,
        password,
        ...(totpCode ? { totp_code: totpCode } : {}),
      });
      if (resp.data?.two_factor_required) {
        return { twoFactorRequired: true as const };
      }
      return { twoFactorRequired: false as const, state: acceptAuthResponse(resp.data) };
    },
    [acceptAuthResponse],
  );

  const setup = useCallback(async (username: string, password: string) => {
    const resp = await apiClient.post("/auth/setup", { username, password });
    return acceptAuthResponse(resp.data);
  }, [acceptAuthResponse]);

  const logout = useCallback(() => {
    const newState = { token: null, username: null, role: null };
    setAuth(newState);
    saveAuth(newState);
    delete apiClient.defaults.headers.common["Authorization"];
  }, []);

  // Set auth header on load if token exists.
  if (auth.token) {
    apiClient.defaults.headers.common["Authorization"] = `Bearer ${auth.token}`;
  }

  return { auth, login, setup, logout };
}
