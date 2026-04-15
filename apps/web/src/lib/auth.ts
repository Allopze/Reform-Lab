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
}

export interface RegisterPayload {
  name: string;
  email: string;
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
    credentials: "include",
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

export async function logout(): Promise<void> {
  await fetch(`${API_URL}/api/auth/logout`, {
    method: "POST",
    credentials: "include",
  });
}
