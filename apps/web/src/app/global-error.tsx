"use client";

import * as Sentry from "@sentry/nextjs";
import Link from "next/link";
import { useEffect } from "react";

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    Sentry.captureException(error);
  }, [error]);

  // global-error replaces the root layout — no NextIntlClientProvider available.
  // Strings stay hardcoded intentionally.
  return (
    <html lang="es">
      <body className="min-h-screen font-sans text-stone-950">
        <div className="flex min-h-screen flex-col items-center justify-center px-5">
          <div className="mx-auto max-w-md text-center">
            <h1 className="text-[28px] font-semibold tracking-[-0.02em] text-stone-900">
              Error crítico
            </h1>
            <p className="mt-2 text-sm text-stone-500">
              La aplicación encontró un error inesperado. Intenta recargar la
              página.
            </p>
            <div className="mt-6 flex items-center justify-center gap-3">
              <button
                onClick={reset}
                className="rounded-lg bg-stone-900 px-4 py-2 text-sm font-medium text-white"
              >
                Intentar de nuevo
              </button>
              <Link
                href="/"
                className="rounded-lg border border-stone-200 px-4 py-2 text-sm font-medium text-stone-700"
              >
                Volver al inicio
              </Link>
            </div>
          </div>
        </div>
      </body>
    </html>
  );
}
