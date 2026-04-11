import type { Metadata } from "next";
import { Suspense } from "react";
import AccessShell from "@/components/access-shell";
import Footer from "@/components/footer";

export const metadata: Metadata = {
  title: "Acceso | Reform Lab",
  description: "Pantalla de acceso y registro para usuarios y administradores de Reform Lab.",
};

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