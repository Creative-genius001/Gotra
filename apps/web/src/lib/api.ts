/** Minimal typed client for the Gotra core API. */

export const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

// In-memory access token. Refresh tokens live in an HttpOnly cookie; the access
// token is deliberately kept out of localStorage to limit XSS exposure.
let accessToken: string | null = null;

export function setAccessToken(token: string | null) {
  accessToken = token;
}

export function getAccessToken(): string | null {
  return accessToken;
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}


export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers);
  headers.set("Content-Type", "application/json");
  if (accessToken) headers.set("Authorization", `Bearer ${accessToken}`);

  const res = await fetch(`${API_URL}/api/v1${path}`, {
    ...init,
    headers,
    credentials: "include",
  });

  if (!res.ok) {
    let message = res.statusText;
    try {
      const body = (await res.json()) as { error?: string };
      if (body.error) message = body.error;
    } catch {
      // non-JSON error body; keep statusText
    }
    throw new ApiError(res.status, message);
  }

  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

/** Liveness check against the API root health endpoint. */
export async function health(): Promise<{ status: string; service: string }> {
  const res = await fetch(`${API_URL}/health`);
  return res.json();
}
