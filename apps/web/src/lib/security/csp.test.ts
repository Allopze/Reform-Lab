import { describe, expect, it } from "vitest";
import { buildContentSecurityPolicy } from "./csp";

describe("buildContentSecurityPolicy", () => {
  it("uses a nonce-based script policy without unsafe-inline", () => {
    const policy = buildContentSecurityPolicy({
      nonce: "test-nonce",
      apiUrl: "https://api.reformlab.app",
      hasSentry: true,
    });

    expect(policy).toContain("script-src 'self' 'nonce-test-nonce' 'strict-dynamic';");
    expect(policy).not.toContain("'unsafe-inline'");
    expect(policy).toContain("img-src 'self';");
    expect(policy).not.toContain("blob:");
    expect(policy).not.toContain("data:");
    expect(policy).toContain("connect-src 'self' https://api.reformlab.app https://*.ingest.sentry.io;");
  });

  it("keeps dev-only allowances scoped to development", () => {
    const policy = buildContentSecurityPolicy({
      nonce: "dev-nonce",
      isDev: true,
    });

    expect(policy).toContain("'unsafe-eval'");
    expect(policy).toContain("connect-src 'self' http: ws:;");
    expect(policy).not.toContain("upgrade-insecure-requests;");
  });
});
