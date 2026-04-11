import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { Suspense } from "react";
import AccessShell from "@/components/access-shell";
import Footer from "@/components/footer";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("metadata");
  return {
    title: t("accesoTitle"),
    description: t("accesoDescription"),
  };
}

export default function AccesoPage() {
  return (
    <div className="flex min-h-screen flex-col">
      <Suspense>
        <AccessShell />
      </Suspense>

      <Footer />
    </div>
  );
}