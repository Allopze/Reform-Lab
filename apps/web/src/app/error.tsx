"use client";

import * as Sentry from "@sentry/nextjs";
import Link from "next/link";
import { useEffect } from "react";
import { useTranslations } from "next-intl";

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  const t = useTranslations("error");
  const tc = useTranslations("common");

  useEffect(() => {
    Sentry.captureException(error);
  }, [error]);

  return (
    <div className="flex min-h-screen flex-col items-center justify-center px-5">
      <div className="mx-auto max-w-md text-center">
        <h1 className="text-[28px] font-semibold tracking-[-0.02em] text-stone-900">
          {t("title")}
        </h1>
        <p className="mt-2 text-sm text-stone-500">
          {t("description")}
        </p>
        <div className="mt-6 flex items-center justify-center gap-3">
          <button
            onClick={reset}
            className="rounded-lg bg-stone-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-stone-800"
          >
            {tc("tryAgain")}
          </button>
          <Link
            href="/"
            className="rounded-lg border border-stone-200 px-4 py-2 text-sm font-medium text-stone-700 transition-colors hover:bg-stone-50"
          >
            {tc("backToHome")}
          </Link>
        </div>
      </div>
    </div>
  );
}
