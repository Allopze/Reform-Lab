const DEFAULT_API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

function isLoopbackHost(hostname: string) {
  return hostname === "localhost" || hostname === "127.0.0.1";
}

export function resolveApiUrl(configuredUrl: string, currentHostname?: string): string {
  try {
    const parsedUrl = new URL(configuredUrl);

    if (!currentHostname || isLoopbackHost(currentHostname) || !isLoopbackHost(parsedUrl.hostname)) {
      return parsedUrl.toString().replace(/\/$/, "");
    }

    parsedUrl.hostname = currentHostname;
    return parsedUrl.toString().replace(/\/$/, "");
  } catch {
    return configuredUrl.replace(/\/$/, "");
  }
}

export const API_URL = resolveApiUrl(
  DEFAULT_API_URL,
  typeof window === "undefined" ? undefined : window.location.hostname
);
