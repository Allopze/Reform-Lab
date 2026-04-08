import type { Metadata } from "next";
import Header from "@/components/header";
import Footer from "@/components/footer";
import AdminDashboard from "@/components/admin-dashboard";

export const metadata: Metadata = {
  title: "Panel Admin | Reform Lab",
  description: "Vista administrativa de trabajos, workers y politicas activas.",
};

export default function AdminPage() {
  return (
    <div className="flex min-h-screen flex-col">
      <Header />

      <main className="flex-1 px-5 pb-10 pt-4 sm:px-8 sm:pb-12">
        <div className="mx-auto max-w-6xl">
          <div className="border-b border-stone-200 pb-5">
            <h1 className="text-[28px] font-semibold tracking-[-0.02em] text-stone-900">
              Panel admin
            </h1>
            <p className="mt-1 text-sm text-stone-500">
              Supervisa la cola de trabajos, el estado de los workers y las politicas activas del sistema.
            </p>
          </div>

          <AdminDashboard />
        </div>
      </main>

      <Footer />
    </div>
  );
}