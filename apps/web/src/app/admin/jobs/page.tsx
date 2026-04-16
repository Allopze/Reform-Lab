import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import Header from "@/components/header";
import Footer from "@/components/footer";
import AdminJobsTable from "@/components/admin-jobs-table";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("metadata");
  return {
    title: t("adminTitle"),
    description: t("adminDescription"),
  };
}

export default async function AdminJobsPage() {
  const t = await getTranslations("adminJobs");
  return (
    <div className="flex min-h-screen flex-col">
      <Header />

      <main className="flex-1 px-5 pb-10 pt-4 sm:px-8 sm:pb-12">
        <div className="mx-auto max-w-6xl">
          <div className="border-b border-stone-200 pb-5">
            <h1 className="text-[28px] font-semibold tracking-[-0.02em] text-stone-900">
              {t("title")}
            </h1>
            <p className="mt-1 text-sm text-stone-500">
              {t("description")}
            </p>
          </div>

          <AdminJobsTable />
        </div>
      </main>

      <Footer />
    </div>
  );
}
