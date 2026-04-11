import { describe, expect, it } from "vitest";
import { resolveApiUrl } from "./config";

describe("resolveApiUrl", () => {
  it("keeps the configured URL when the current hostname is localhost", () => {
    expect(resolveApiUrl("http://localhost:4040", "localhost")).toBe("http://localhost:4040");
  });

  it("reuses the current hostname when the configured API host is loopback", () => {
    expect(resolveApiUrl("http://localhost:4040", "192.168.4.111")).toBe("http://192.168.4.111:4040");
    expect(resolveApiUrl("http://127.0.0.1:4040", "cloudbox-labs.local")).toBe("http://cloudbox-labs.local:4040");
  });

  it("preserves explicit non-loopback API hosts", () => {
    expect(resolveApiUrl("https://api.reform.lab", "192.168.4.111")).toBe("https://api.reform.lab");
  });
});