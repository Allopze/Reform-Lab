const CSRF_COOKIE = "reform_csrf";

export function csrfHeaders(): HeadersInit {
  if (typeof document === "undefined") {
    return {};
  }

  const token = document.cookie
    .split(";")
    .map((part) => part.trim())
    .find((part) => part.startsWith(`${CSRF_COOKIE}=`))
    ?.slice(CSRF_COOKIE.length + 1);

  return token ? { "X-CSRF-Token": decodeURIComponent(token) } : {};
}

