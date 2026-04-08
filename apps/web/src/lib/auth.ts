import { API_URL } from "./config";

export interface AuthUser {
  id: string;
  name: string;
  email: string;
  team: string;
  role: "admin" | "user";
  createdAt: string;
}

export interface AuthResult {
  user: AuthUser;
  token: string;
}

export interface RegisterPayload {
  name: string;
  email: string;
  team: string;
  password: string;
}

export interface LoginPayload {
  email: string;
  password: string;
}

async function request<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });

  const data = await res.json();

  if (!res.ok) {
    throw new Error(data.error || "Request failed");
  }

  return data as T;
}

export function register(payload: RegisterPayload): Promise<AuthResult> {
  return request<AuthResult>("/api/auth/register", payload);
}

export function login(payload: LoginPayload): Promise<AuthResult> {
  return request<AuthResult>("/api/auth/login", payload);
}

const TOKEN_KEY = "reform_token";

export function saveToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token);
}

export function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(TOKEN_KEY);
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY);
}
