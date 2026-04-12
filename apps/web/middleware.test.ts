import { describe, expect, it, vi, afterEach } from "vitest";
import { NextRequest } from "next/server";
import { middleware } from "./middleware";

describe("middleware", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("adds a nonce-based CSP header to public pages", () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("NEXT_PUBLIC_API_URL", "https://api.reformlab.app");

    const response = middleware(new NextRequest("https://reformlab.app/"));
    const policy = response.headers.get("Content-Security-Policy");

    expect(policy).toContain("script-src 'self' 'nonce-");
    expect(policy).toContain("'strict-dynamic'");
    expect(policy).not.toContain("'unsafe-inline'");
  });

  it("keeps protected-route redirects intact", () => {
    vi.stubEnv("NODE_ENV", "production");

    const response = middleware(new NextRequest("https://reformlab.app/admin"));

    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe(
      "https://reformlab.app/acceso?from=%2Fadmin"
    );
  });
});
