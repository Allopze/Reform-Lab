// @vitest-environment node
import { describe, expect, it, vi, afterEach } from "vitest";
import { NextRequest } from "next/server";
import { SignJWT } from "jose";
import { middleware } from "./middleware";

const TEST_JWT_SECRET = "test-secret-for-middleware";

async function makeAdminToken(): Promise<string> {
  return new SignJWT({ role: "admin", sub: "00000000-0000-0000-0000-000000000001" })
    .setProtectedHeader({ alg: "HS256" })
    .setIssuedAt()
    .setExpirationTime("1h")
    .sign(new TextEncoder().encode(TEST_JWT_SECRET));
}

async function makeUserToken(): Promise<string> {
  return new SignJWT({ role: "user", sub: "00000000-0000-0000-0000-000000000002" })
    .setProtectedHeader({ alg: "HS256" })
    .setIssuedAt()
    .setExpirationTime("1h")
    .sign(new TextEncoder().encode(TEST_JWT_SECRET));
}

describe("middleware", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("adds a nonce-based CSP header to public pages", async () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("NEXT_PUBLIC_API_URL", "https://api.reformlab.app");

    const response = await middleware(new NextRequest("https://reformlab.app/"));
    const policy = response.headers.get("Content-Security-Policy");

    expect(policy).toContain("script-src 'self' 'nonce-");
    expect(policy).toContain("'strict-dynamic'");
    expect(policy).not.toContain("'unsafe-inline'");
  });

  it("redirects unauthenticated users to login", async () => {
    vi.stubEnv("NODE_ENV", "production");

    const response = await middleware(new NextRequest("https://reformlab.app/admin"));

    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe(
      "https://reformlab.app/acceso?from=%2Fadmin"
    );
  });

  it("redirects non-admin users away from /admin", async () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("JWT_SECRET", TEST_JWT_SECRET);

    const token = await makeUserToken();
    const req = new NextRequest("https://reformlab.app/admin");
    req.cookies.set("reform_session", token);

    const response = await middleware(req);

    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe(
      "https://reformlab.app/usuario"
    );
  });

  it("allows admin users to access /admin", async () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("JWT_SECRET", TEST_JWT_SECRET);

    const token = await makeAdminToken();
    const req = new NextRequest("https://reformlab.app/admin");
    req.cookies.set("reform_session", token);

    const response = await middleware(req);

    expect(response.status).toBe(200);
  });

  it("allows authenticated users to access /usuario", async () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("JWT_SECRET", TEST_JWT_SECRET);

    const token = await makeUserToken();
    const req = new NextRequest("https://reformlab.app/usuario");
    req.cookies.set("reform_session", token);

    const response = await middleware(req);

    expect(response.status).toBe(200);
  });
});
