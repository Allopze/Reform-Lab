import Link from "next/link";
import { getTranslations } from "next-intl/server";

export default async function NotFound() {
  const t = await getTranslations("notFound");
  const tc = await getTranslations("common");

  return (
    <div className="flex min-h-screen flex-col items-center justify-center px-5">
      <div className="mx-auto max-w-md text-center">
        <h1 className="text-[28px] font-semibold tracking-[-0.02em] text-stone-900">
          {t("title")}
        </h1>
        <p className="mt-2 text-sm text-stone-500">
          {t("description")}
        </p>
        <div className="mt-6">
          <Link
            href="/"
            className="rounded-lg bg-stone-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-stone-800"
          >
            {tc("backToHome")}
          </Link>
        </div>
      </div>
    </div>
  );
}
