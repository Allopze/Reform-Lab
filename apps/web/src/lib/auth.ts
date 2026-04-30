import { API_URL } from "./config";
import { csrfHeaders } from "./csrf";

export interface AuthUser {
  id: string;
  name: string;
  email: string;
  team: string;
  role: "admin" | "user";
  emailVerifiedAt?: string;
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

export interface PasswordResetRequestPayload {
  email: string;
}

export interface PasswordResetConfirmPayload {
  token: string;
  password: string;
}

export interface EmailVerificationConfirmPayload {
  token: string;
}

async function request<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    method: "POST",
    headers: { ...csrfHeaders(), "Content-Type": "application/json" },
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

export async function requestPasswordReset(payload: PasswordResetRequestPayload): Promise<{ status: string }> {
  return request<{ status: string }>("/api/auth/password-reset/request", payload);
}

export async function confirmPasswordReset(payload: PasswordResetConfirmPayload): Promise<{ status: string }> {
  return request<{ status: string }>("/api/auth/password-reset/confirm", payload);
}

export async function requestEmailVerification(): Promise<{ status: string }> {
  return request<{ status: string }>("/api/auth/email-verification/request", {});
}

export async function confirmEmailVerification(payload: EmailVerificationConfirmPayload): Promise<{ status: string }> {
  return request<{ status: string }>("/api/auth/email-verification/confirm", payload);
}

export async function logout(): Promise<void> {
  await fetch(`${API_URL}/api/auth/logout`, {
    method: "POST",
    headers: csrfHeaders(),
    credentials: "include",
  });
}
