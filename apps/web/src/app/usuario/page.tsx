import type { Metadata } from "next";
import Link from "next/link";
import Header from "@/components/header";
import Footer from "@/components/footer";
import UserDashboard from "@/components/user-dashboard";

export const metadata: Metadata = {
  title: "Mis Archivos | Reform Lab",
  description: "Historial reciente de archivos y conversiones del usuario.",
};

export default function UsuarioPage() {
  return (
    <div className="flex min-h-screen flex-col">
      <Header />

      <main className="flex-1 px-5 pb-10 pt-4 sm:px-8 sm:pb-12">
        <div className="mx-auto max-w-6xl">
          <div className="flex flex-col gap-4 border-b border-stone-200 pb-5 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <h1 className="text-[28px] font-semibold tracking-[-0.02em] text-stone-900">
                Mis archivos
              </h1>
              <p className="mt-1 text-sm text-stone-500">
                Consulta el formato detectado, la salida elegida y el estado de cada conversion reciente.
              </p>
            </div>

            <Link
              href="/"
              className="inline-flex h-10 items-center rounded-lg border border-stone-300 bg-white px-4 text-sm font-medium text-stone-700 transition-colors duration-150 hover:border-stone-400 hover:text-stone-900"
            >
              Nueva conversion
            </Link>
          </div>

          <UserDashboard />
        </div>
      </main>

      <Footer />
    </div>
  );
}