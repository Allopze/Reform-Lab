import type { Metadata } from "next";
import Header from "@/components/header";
import Footer from "@/components/footer";

const jobs = [
  {
    id: "job_4821",
    source: "video-demo.mov",
    worker: "video-transcoder",
    status: "En ejecucion",
    updatedAt: "Hace 2 min",
  },
  {
    id: "job_4817",
    source: "manual-operacion.pdf",
    worker: "pdf-transform",
    status: "En cola",
    updatedAt: "Hace 5 min",
  },
  {
    id: "job_4808",
    source: "audio-soporte.wav",
    worker: "audio-converter",
    status: "Error",
    updatedAt: "Hace 18 min",
  },
];

const policies = [
  {
    name: "Deteccion de formato",
    value: "Activa",
    detail: "El frontend solo refleja el tipo detectado por el sistema.",
  },
  {
    name: "Tamano maximo",
    value: "500 MB",
    detail: "Aplicado solo a video y cargas compatibles con el flujo publico.",
  },
  {
    name: "Retencion de artefactos",
    value: "7 dias",
    detail: "Los artefactos derivados expiran y se purgan automaticamente.",
  },
];

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

          <div className="mt-6 grid gap-6 lg:grid-cols-[minmax(0,1.4fr)_minmax(300px,0.6fr)]">
            <section className="overflow-hidden rounded-xl border border-stone-200 bg-white">
              <table className="w-full border-collapse text-left">
                <thead className="bg-stone-50 text-xs font-medium text-stone-500">
                  <tr>
                    <th className="px-5 py-3">Job</th>
                    <th className="px-5 py-3">Archivo</th>
                    <th className="px-5 py-3">Worker</th>
                    <th className="px-5 py-3">Estado</th>
                    <th className="px-5 py-3">Actualizado</th>
                  </tr>
                </thead>
                <tbody>
                  {jobs.map((job) => (
                    <tr key={job.id} className="border-t border-stone-200 text-sm text-stone-700">
                      <td className="px-5 py-4 font-medium text-stone-900">{job.id}</td>
                      <td className="px-5 py-4">{job.source}</td>
                      <td className="px-5 py-4">{job.worker}</td>
                      <td className="px-5 py-4">{job.status}</td>
                      <td className="px-5 py-4 text-stone-500">{job.updatedAt}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </section>

            <section className="rounded-xl border border-stone-200 bg-white px-5 py-4">
              <h2 className="text-base font-semibold text-stone-900">Politicas activas</h2>
              <div className="mt-4 space-y-4">
                {policies.map((policy) => (
                  <div key={policy.name} className="border-b border-stone-200 pb-4 last:border-b-0 last:pb-0">
                    <div className="flex items-center justify-between gap-4">
                      <span className="text-sm font-medium text-stone-900">{policy.name}</span>
                      <span className="text-sm text-stone-500">{policy.value}</span>
                    </div>
                    <p className="mt-2 text-sm leading-6 text-stone-600">{policy.detail}</p>
                  </div>
                ))}
              </div>
            </section>
          </div>
        </div>
      </main>

      <Footer />
    </div>
  );
}