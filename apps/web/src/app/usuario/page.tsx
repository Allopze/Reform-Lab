import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import Link from "next/link";
import Header from "@/components/header";
import Footer from "@/components/footer";
import UserDashboard from "@/components/user-dashboard";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("metadata");
  return {
    title: t("usuarioTitle"),
    description: t("usuarioDescription"),
  };
}

export default async function UsuarioPage() {
  const t = await getTranslations("usuarioPage");
  return (
    <div className="flex min-h-screen flex-col">
      <Header />

      <main className="flex-1 px-5 pb-10 pt-4 sm:px-8 sm:pb-12">
        <div className="mx-auto max-w-6xl">
          <div className="flex flex-col gap-4 border-b border-stone-200 pb-5 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <h1 className="text-[28px] font-semibold tracking-[-0.02em] text-stone-900">
                {t("title")}
              </h1>
              <p className="mt-1 text-sm text-stone-500">
                {t("description")}
              </p>
            </div>

            <Link
              href="/"
              className="inline-flex h-10 items-center rounded-lg border border-stone-300 bg-white px-4 text-sm font-medium text-stone-700 transition-colors duration-150 hover:border-stone-400 hover:text-stone-900"
            >
              {t("newConversion")}
            </Link>
          </div>

          <UserDashboard />
        </div>
      </main>

      <Footer />
    </div>
  );
}