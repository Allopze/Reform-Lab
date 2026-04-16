import { NextRequest, NextResponse } from "next/server";
import { jwtVerify } from "jose";
import { buildContentSecurityPolicy } from "@/lib/security/csp";

const PROTECTED_PREFIXES = ["/usuario", "/admin"];
const ADMIN_PREFIX = "/admin";

async function extractRole(
  token: string,
): Promise<string | null> {
  const secret = process.env.JWT_SECRET;
  if (!secret) return null;
  try {
    const { payload } = await jwtVerify(
      token,
      new TextEncoder().encode(secret),
      { algorithms: ["HS256"] },
    );
    return (payload as Record<string, unknown>).role as string ?? null;
  } catch {
    return null;
  }
}

export async function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const nonce = Buffer.from(crypto.randomUUID()).toString("base64");
  const contentSecurityPolicy = buildContentSecurityPolicy({
    nonce,
    apiUrl: process.env.NEXT_PUBLIC_API_URL,
    hasSentry: !!process.env.NEXT_PUBLIC_SENTRY_DSN,
    isDev: process.env.NODE_ENV === "development",
  });
  const requestHeaders = new Headers(request.headers);
  requestHeaders.set("x-nonce", nonce);
  requestHeaders.set("Content-Security-Policy", contentSecurityPolicy);

  const isProtected = PROTECTED_PREFIXES.some((prefix) =>
    pathname.startsWith(prefix)
  );

  if (!isProtected) {
    const response = NextResponse.next({
      request: {
        headers: requestHeaders,
      },
    });
    response.headers.set("Content-Security-Policy", contentSecurityPolicy);
    return response;
  }

  const session = request.cookies.get("reform_session");

  if (!session?.value) {
    const loginUrl = new URL("/acceso", request.url);
    loginUrl.searchParams.set("from", pathname);
    const response = NextResponse.redirect(loginUrl);
    response.headers.set("Content-Security-Policy", contentSecurityPolicy);
    return response;
  }

  // Harden /admin: reject non-admin users at the edge
  if (pathname.startsWith(ADMIN_PREFIX)) {
    const role = await extractRole(session.value);
    if (role !== "admin") {
      const homeUrl = new URL("/usuario", request.url);
      const response = NextResponse.redirect(homeUrl);
      response.headers.set("Content-Security-Policy", contentSecurityPolicy);
      return response;
    }
  }

  const response = NextResponse.next({
    request: {
      headers: requestHeaders,
    },
  });
  response.headers.set("Content-Security-Policy", contentSecurityPolicy);
  return response;
}

export const config = {
  matcher: [
    {
      source:
        "/((?!api|_next/static|_next/image|favicon.ico|manifest.webmanifest).*)",
      missing: [
        { type: "header", key: "next-router-prefetch" },
        { type: "header", key: "purpose", value: "prefetch" },
      ],
    },
  ],
};
