const DEFAULT_API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

function isLoopbackHost(hostname: string) {
  return hostname === "localhost" || hostname === "127.0.0.1";
}

function isPrivateHost(hostname: string) {
  return /^(10\.|172\.(1[6-9]|2\d|3[01])\.|192\.168\.)/.test(hostname);
}

function hostCategory(hostname: string): "loopback" | "private" | "public" {
  if (isLoopbackHost(hostname)) return "loopback";
  if (isPrivateHost(hostname)) return "private";
  return "public";
}

function resolveSingleUrl(url: string, currentHostname?: string): string {
  try {
    const parsedUrl = new URL(url);

    if (!currentHostname || isLoopbackHost(currentHostname) || !isLoopbackHost(parsedUrl.hostname)) {
      return parsedUrl.toString().replace(/\/$/, "");
    }

    parsedUrl.hostname = currentHostname;
    return parsedUrl.toString().replace(/\/$/, "");
  } catch {
    return url.replace(/\/$/, "");
  }
}

export function resolveApiUrl(configuredUrl: string, currentHostname?: string): string {
  const candidates = configuredUrl.split(",").map((u) => u.trim()).filter(Boolean);

  if (candidates.length <= 1) {
    return resolveSingleUrl(candidates[0] ?? configuredUrl, currentHostname);
  }

  if (currentHostname) {
    const category = hostCategory(currentHostname);

    for (const candidate of candidates) {
      try {
        const parsed = new URL(candidate);
        if (parsed.hostname === currentHostname) {
          return parsed.toString().replace(/\/$/, "");
        }
      } catch {
        continue;
      }
    }

    for (const candidate of candidates) {
      try {
        const parsed = new URL(candidate);
        if (hostCategory(parsed.hostname) === category) {
          return resolveSingleUrl(candidate, currentHostname);
        }
      } catch {
        continue;
      }
    }
  }

  return resolveSingleUrl(candidates[0], currentHostname);
}

export const API_URL = resolveApiUrl(
  DEFAULT_API_URL,
  typeof window === "undefined" ? undefined : window.location.hostname
);
