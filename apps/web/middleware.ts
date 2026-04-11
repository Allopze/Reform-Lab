import { NextRequest, NextResponse } from "next/server";

const PROTECTED_PREFIXES = ["/usuario", "/admin"];

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;

  const isProtected = PROTECTED_PREFIXES.some((prefix) =>
    pathname.startsWith(prefix)
  );

  if (!isProtected) {
    return NextResponse.next();
  }

  const session = request.cookies.get("reform_session");

  if (!session?.value) {
    const loginUrl = new URL("/acceso", request.url);
    loginUrl.searchParams.set("from", pathname);
    return NextResponse.redirect(loginUrl);
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/usuario/:path*", "/admin/:path*"],
};
