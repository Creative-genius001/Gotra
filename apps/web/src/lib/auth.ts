/** Auth API calls + client-side session store. */
import { create } from "zustand";
import { apiFetch, API_URL, setAccessToken } from "@/lib/api";

export interface User {
  id: string;
  email: string;
  name: string;
  avatar_url?: string;
  email_verified: boolean;
  created_at: string;
  updated_at: string;
}

export interface AuthResponse {
  access_token: string;
  expires_in: number;
  user: User;
  workspace_id: string;
  role: string;
}

// --- API calls --------------------------------------------------------------

export function register(input: { email: string; name: string; password: string }) {
  return apiFetch<AuthResponse>("/auth/register", { method: "POST", body: JSON.stringify(input) });
}

export function login(input: { email: string; password: string }) {
  return apiFetch<AuthResponse>("/auth/login", { method: "POST", body: JSON.stringify(input) });
}

export function refresh() {
  return apiFetch<AuthResponse>("/auth/refresh", { method: "POST" });
}

export function logout() {
  return apiFetch<{ status: string }>("/auth/logout", { method: "POST" });
}

export function verifyEmail(token: string) {
  return apiFetch<{ status: string }>("/auth/verify-email", {
    method: "POST",
    body: JSON.stringify({ token }),
  });
}

export function forgotPassword(email: string) {
  return apiFetch<{ status: string; dev_token?: string }>("/auth/password/forgot", {
    method: "POST",
    body: JSON.stringify({ email }),
  });
}

export function resetPassword(token: string, password: string) {
  return apiFetch<{ status: string }>("/auth/password/reset", {
    method: "POST",
    body: JSON.stringify({ token, password }),
  });
}

/** URL that begins a provider OAuth/SSO flow (full-page redirect). */
export function oauthURL(provider: "google" | "github" | "oidc") {
  return `${API_URL}/api/v1/auth/${provider}`;
}

// --- Session store ----------------------------------------------------------

export type AuthStatus = "loading" | "authenticated" | "unauthenticated";

interface AuthState {
  status: AuthStatus;
  user: User | null;
  workspaceId: string | null;
  role: string | null;
  /** Apply an auth response: store access token (in memory) + session. */
  setSession: (res: AuthResponse) => void;
  /** Clear the in-memory session. */
  clearSession: () => void;
  /** Attempt to restore a session via the refresh cookie. */
  bootstrap: () => Promise<void>;
  /** Log out: revoke the refresh session and clear local state. */
  signOut: () => Promise<void>;
}

export const useAuth = create<AuthState>((set) => ({
  status: "loading",
  user: null,
  workspaceId: null,
  role: null,

  setSession: (res) => {
    setAccessToken(res.access_token);
    set({
      status: "authenticated",
      user: res.user,
      workspaceId: res.workspace_id,
      role: res.role,
    });
  },

  clearSession: () => {
    setAccessToken(null);
    set({ status: "unauthenticated", user: null, workspaceId: null, role: null });
  },

  bootstrap: async () => {
    try {
      const res = await refresh();
      setAccessToken(res.access_token);
      set({
        status: "authenticated",
        user: res.user,
        workspaceId: res.workspace_id,
        role: res.role,
      });
    } catch {
      setAccessToken(null);
      set({ status: "unauthenticated", user: null, workspaceId: null, role: null });
    }
  },

  signOut: async () => {
    try {
      await logout();
    } catch {
      // ignore network errors on logout
    }
    setAccessToken(null);
    set({ status: "unauthenticated", user: null, workspaceId: null, role: null });
  },
}));
