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

  it("picks the matching URL from a comma-separated list by exact hostname", () => {
    const multi = "http://192.168.4.111:8080,http://localhost:8080,https://api.allopze.dev";
    expect(resolveApiUrl(multi, "192.168.4.111")).toBe("http://192.168.4.111:8080");
    expect(resolveApiUrl(multi, "localhost")).toBe("http://localhost:8080");
  });

  it("picks the public URL when accessed from a public domain", () => {
    const multi = "http://192.168.4.111:8080,http://localhost:8080,https://api.allopze.dev";
    expect(resolveApiUrl(multi, "reform.allopze.dev")).toBe("https://api.allopze.dev");
  });

  it("picks private URL when accessed from a different private IP", () => {
    const multi = "http://192.168.4.111:8080,https://api.allopze.dev";
    expect(resolveApiUrl(multi, "192.168.1.50")).toBe("http://192.168.4.111:8080");
  });

  it("replaces loopback host with current public hostname for single URL", () => {
    expect(resolveApiUrl("http://localhost:8080", "reform.allopze.dev")).toBe("http://reform.allopze.dev:8080");
  });

  it("treats same-origin API path values as an empty origin", () => {
    expect(resolveApiUrl("/api", "reform.allopze.dev")).toBe("");
    expect(resolveApiUrl("api", "reform.allopze.dev")).toBe("");
  });

  it("strips a trailing /api path because callers append the API prefix", () => {
    expect(resolveApiUrl("https://reform.allopze.dev/api", "reform.allopze.dev")).toBe("https://reform.allopze.dev");
    expect(resolveApiUrl("http://localhost:8080/api", "localhost")).toBe("http://localhost:8080");
  });
});
