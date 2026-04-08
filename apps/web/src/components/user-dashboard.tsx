"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { downloadArtifact, getMyDashboard, retryJob, type UserDashboardData } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";

function formatDate(value: string): string {
  return new Intl.DateTimeFormat("es-ES", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

function statusLabel(status: string): string {
  switch (status) {
    case "queued":
      return "En cola";
    case "running":
      return "En ejecucion";
    case "succeeded":
      return "Listo";
    case "failed":
      return "Fallido";
    case "expired":
      return "Expirado";
    case "cancelled":
      return "Cancelado";
    default:
      return status;
  }
}

export default function UserDashboard() {
  const router = useRouter();
  const { user, loading } = useAuth();
  const [data, setData] = useState<UserDashboardData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [downloadingId, setDownloadingId] = useState<string | null>(null);
  const [retryingId, setRetryingId] = useState<string | null>(null);

  const refreshDashboard = useCallback(async () => {
    const nextData = await getMyDashboard();
    setData(nextData);
  }, []);

  useEffect(() => {
    if (loading) return;
    if (!user) {
      router.push("/acceso");
      return;
    }

    refreshDashboard()
      .catch((err) => {
        setError(err instanceof Error ? err.message : "No se pudo cargar tu panel.");
      });
  }, [loading, refreshDashboard, router, user]);

  if (loading || (!data && !error)) {
    return <p className="text-sm text-stone-500">Cargando actividad...</p>;
  }

  if (error) {
    return <p className="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{error}</p>;
  }

  if (!data) return null;

  return (
    <div className="mt-6 grid gap-6 lg:grid-cols-[minmax(0,1.45fr)_minmax(280px,0.55fr)]">
      <section className="overflow-hidden rounded-xl border border-stone-200 bg-white">
        <table className="w-full border-collapse text-left">
          <thead className="bg-stone-50 text-xs font-medium text-stone-500">
            <tr>
              <th className="px-5 py-3">Archivo</th>
              <th className="px-5 py-3">Detectado</th>
              <th className="px-5 py-3">Salida</th>
              <th className="px-5 py-3">Estado</th>
              <th className="px-5 py-3">Actualizado</th>
              <th className="px-5 py-3">Accion</th>
            </tr>
          </thead>
          <tbody>
            {data.recentJobs.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-5 py-8 text-sm text-stone-500">
                  Todavia no tienes conversiones registradas.
                </td>
              </tr>
            ) : (
              data.recentJobs.map((job) => (
                <tr key={job.jobId} className="border-t border-stone-200 text-sm text-stone-700">
                  <td className="px-5 py-4 font-medium text-stone-900">{job.fileName}</td>
                  <td className="px-5 py-4 capitalize">{job.detectedFamily}</td>
                  <td className="px-5 py-4">{job.outputFormat.toUpperCase()}</td>
                  <td className="px-5 py-4">
                    {(job.status === "running" || job.status === "queued") && job.progress > 0 ? (
                      <div className="flex items-center gap-2">
                        <div className="h-1.5 w-16 rounded-full bg-stone-200">
                          <div
                            className="h-full rounded-full bg-coral-600 transition-all"
                            style={{ width: `${job.progress}%` }}
                          />
                        </div>
                        <span className="text-xs text-stone-500">{job.progress}%</span>
                      </div>
                    ) : (
                      statusLabel(job.status)
                    )}
                  </td>
                  <td className="px-5 py-4 text-stone-500">{formatDate(job.updatedAt)}</td>
                  <td className="px-5 py-4">
                    {job.artifactId && job.status === "succeeded" ? (
                      <div className="flex flex-col gap-1">
                        <button
                          type="button"
                          onClick={async () => {
                            try {
                              setError(null);
                              setDownloadingId(job.jobId);
                              await downloadArtifact(job.artifactId!, job.artifactFileName);
                            } catch (err) {
                              setError(err instanceof Error ? err.message : "No se pudo descargar.");
                            } finally {
                              setDownloadingId(null);
                            }
                          }}
                          className="font-medium text-coral-700 underline underline-offset-2"
                        >
                          {downloadingId === job.jobId ? "Descargando..." : "Descargar"}
                        </button>
                        {job.expiresAt && (
                          <span className="text-xs text-stone-400">
                            Expira {formatDate(job.expiresAt)}
                          </span>
                        )}
                      </div>
                    ) : job.status === "failed" ? (
                      <button
                        type="button"
                        onClick={async () => {
                          try {
                            setError(null);
                            setRetryingId(job.jobId);
                            await retryJob(job.jobId);
                            await refreshDashboard();
                          } catch (err) {
                            setError(err instanceof Error ? err.message : "No se pudo reintentar.");
                          } finally {
                            setRetryingId(null);
                          }
                        }}
                        className="font-medium text-coral-700 underline underline-offset-2"
                      >
                        {retryingId === job.jobId ? "Reintentando..." : "Reintentar"}
                      </button>
                    ) : (
                      <span className="text-stone-400">-</span>
                    )}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </section>

      <aside className="space-y-4">
        <section className="rounded-xl border border-stone-200 bg-white px-5 py-4">
          <h2 className="text-base font-semibold text-stone-900">Resumen</h2>
          <div className="mt-4 space-y-3 text-sm text-stone-600">
            <p>Archivos propios: {data.totalFiles}</p>
            <p>Jobs registrados: {data.totalJobs}</p>
            <p>Jobs activos: {data.activeJobs}</p>
            <p>Exitosos: {data.succeededJobs}</p>
            <p>Fallidos: {data.failedJobs}</p>
          </div>
        </section>

        <section className="rounded-xl border border-stone-200 bg-white px-5 py-4">
          <h2 className="text-base font-semibold text-stone-900">Retencion</h2>
          <p className="mt-3 text-sm leading-6 text-stone-600">
            Los artefactos completados quedan disponibles hasta su expiracion real en backend. Cuando expiran, se purgan y el job pasa a estado expirada.
          </p>
        </section>
      </aside>
    </div>
  );
}