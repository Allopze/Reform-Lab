interface CspOptions {
  apiUrl?: string;
  hasSentry?: boolean;
  isDev?: boolean;
  nonce: string;
}

function buildConnectSrc({
  apiUrl,
  hasSentry = false,
  isDev = false,
}: Omit<CspOptions, "nonce">): string[] {
  const connectSrc = ["'self'"];

  if (apiUrl) {
    for (const candidate of apiUrl.split(",")) {
      try {
        connectSrc.push(new URL(candidate.trim()).origin);
      } catch {
        // Ignore invalid user-provided URLs and keep the default safe policy.
      }
    }
  }

  if (isDev) {
    connectSrc.push("http:", "ws:");
  }

  if (hasSentry) {
    connectSrc.push("https://*.ingest.sentry.io");
  }

  return connectSrc;
}

export function buildContentSecurityPolicy({
  apiUrl,
  hasSentry = false,
  isDev = false,
  nonce,
}: CspOptions): string {
  const cspHeader = `
    default-src 'self';
    script-src 'self' 'nonce-${nonce}' 'strict-dynamic'${isDev ? " 'unsafe-eval'" : ""};
    style-src 'self' 'nonce-${nonce}';
    img-src 'self';
    font-src 'self';
    connect-src ${buildConnectSrc({ apiUrl, hasSentry, isDev }).join(" ")};
    object-src 'none';
    base-uri 'self';
    form-action 'self';
    frame-ancestors 'none';
    ${isDev ? "" : "upgrade-insecure-requests;"}
  `;

  return cspHeader.replace(/\s{2,}/g, " ").trim();
}
