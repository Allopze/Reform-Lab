"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import {
  getAdminOverview,
  getFooterMessage,
  getUploadPolicy,
  updateFooterMessage,
  updateUploadPolicy,
  type AdminDashboardData,
  type UploadPolicy,
} from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import {
  DEFAULT_FOOTER_MESSAGE,
  emitFooterMessageUpdated,
} from "@/lib/footer-message";
import SMTPSettingsSection from "@/components/smtp-settings";
import EmailTemplatesSection from "@/components/email-templates";

const BYTES_PER_MB = 1024 * 1024;

const auditFilters = [
  { value: "all", label: "Todos" },
  { value: "upload", label: "Uploads" },
  { value: "job_created", label: "Jobs creados" },
  { value: "job_started", label: "Jobs iniciados" },
  { value: "job_completed", label: "Jobs completados" },
  { value: "job_failed", label: "Jobs fallidos" },
  { value: "job_cancelled", label: "Jobs cancelados" },
  { value: "job_retried", label: "Jobs reintentados" },
  { value: "artifact_created", label: "Artefactos" },
] as const;

function formatDate(value: string): string {
  return new Intl.DateTimeFormat("es-ES", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

function formatSeconds(value: number): string {
  if (value <= 0) return "Sin datos";
  if (value < 60) return `${value.toFixed(1)} s`;
  return `${(value / 60).toFixed(1)} min`;
}

function bytesToMegabytes(bytes: number): number {
  return Math.round(bytes / BYTES_PER_MB);
}

function formatMegabytes(bytes: number): string {
  return `${bytesToMegabytes(bytes)} MB`;
}

function parseLimitDraft(value: string): number | null {
  const trimmed = value.trim();
  if (!trimmed || !/^\d+$/.test(trimmed)) return null;

  const parsed = Number(trimmed);
  if (!Number.isInteger(parsed) || parsed < 1 || parsed > 500) return null;
  return parsed;
}

function auditLabel(eventType: string): string {
  switch (eventType) {
    case "upload":
      return "Archivo subido";
    case "job_created":
      return "Job creado";
    case "job_started":
      return "Job iniciado";
    case "job_completed":
      return "Job completado";
    case "job_failed":
      return "Job fallido";
    case "job_cancelled":
      return "Job cancelado";
    case "job_retried":
      return "Job reintentado";
    case "artifact_created":
      return "Artefacto creado";
    default:
      return eventType;
  }
}

function auditDetails(event: NonNullable<AdminDashboardData>["recentAudit"][number]): string {
  const details = event.details ?? {};
  if (event.eventType === "artifact_created") {
    const fileName = typeof details.fileName === "string" ? details.fileName : null;
    return fileName ? `Salida ${fileName}` : "Artefacto persistido";
  }
  if (event.eventType === "job_failed") {
    const error = typeof details.error === "string" ? details.error : null;
    return error || "Fallo registrado";
  }
  if (event.eventType === "job_created") {
    const capability = typeof details.capabilityId === "string" ? details.capabilityId : null;
    return capability ? `Capability ${capability}` : "Solicitud registrada";
  }
  if (event.eventType === "job_retried") {
    const sourceJobId = typeof details.sourceJobId === "string" ? details.sourceJobId : null;
    return sourceJobId ? `Reintento desde ${sourceJobId.slice(0, 8)}` : "Reintento registrado";
  }
  if (event.eventType === "upload") {
    const originalName = typeof details.originalName === "string" ? details.originalName : null;
    return originalName || "Upload registrado";
  }
  return "Evento registrado";
}

export default function AdminDashboard() {
  const router = useRouter();
  const { user, loading } = useAuth();
  const [data, setData] = useState<AdminDashboardData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [auditFilter, setAuditFilter] = useState<(typeof auditFilters)[number]["value"]>("all");
  const [footerMessage, setFooterMessage] = useState(DEFAULT_FOOTER_MESSAGE);
  const [footerDraft, setFooterDraft] = useState(DEFAULT_FOOTER_MESSAGE);
  const [footerError, setFooterError] = useState<string | null>(null);
  const [footerStatus, setFooterStatus] = useState<string | null>(null);
  const [footerSaving, setFooterSaving] = useState(false);
  const [uploadPolicy, setUploadPolicy] = useState<UploadPolicy | null>(null);
  const [guestLimitDraft, setGuestLimitDraft] = useState("500");
  const [registeredLimitDraft, setRegisteredLimitDraft] = useState("500");
  const [uploadPolicyError, setUploadPolicyError] = useState<string | null>(null);
  const [uploadPolicyStatus, setUploadPolicyStatus] = useState<string | null>(null);
  const [uploadPolicySaving, setUploadPolicySaving] = useState(false);

  useEffect(() => {
    if (loading) return;
    if (!user) {
      router.push("/acceso");
      return;
    }
    if (user.role !== "admin") {
      router.push("/usuario");
      return;
    }

    getAdminOverview()
      .then(setData)
      .catch((err) => {
        setError(err instanceof Error ? err.message : "No se pudo cargar el panel admin.");
      });

    getFooterMessage()
      .then((message) => {
        setFooterMessage(message);
        setFooterDraft(message);
      })
      .catch((err) => {
        setFooterError(err instanceof Error ? err.message : "No se pudo cargar el footer actual.");
      });

    getUploadPolicy()
      .then((policy) => {
        setUploadPolicy(policy);
        setGuestLimitDraft(String(bytesToMegabytes(policy.guestMaxBytes)));
        setRegisteredLimitDraft(String(bytesToMegabytes(policy.registeredMaxBytes)));
      })
      .catch((err) => {
        setUploadPolicyError(err instanceof Error ? err.message : "No se pudo cargar la politica de uploads.");
      });
  }, [loading, router, user]);

  if (loading || (!data && !error)) {
    return <p className="text-sm text-stone-500">Cargando panel admin...</p>;
  }

  if (error) {
    return <p className="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{error}</p>;
  }

  if (!data) return null;

  const visibleAudit =
    auditFilter === "all"
      ? data.recentAudit
      : data.recentAudit.filter((event) => event.eventType === auditFilter);
  const normalizedFooterDraft = footerDraft.trim();
  const footerDirty = normalizedFooterDraft !== footerMessage;
  const guestLimitMb = parseLimitDraft(guestLimitDraft);
  const registeredLimitMb = parseLimitDraft(registeredLimitDraft);
  const uploadPolicyDirty =
    !!uploadPolicy &&
    guestLimitMb !== null &&
    registeredLimitMb !== null &&
    (guestLimitMb * BYTES_PER_MB !== uploadPolicy.guestMaxBytes ||
      registeredLimitMb * BYTES_PER_MB !== uploadPolicy.registeredMaxBytes);

  async function handleFooterSave() {
    if (!normalizedFooterDraft) {
      setFooterError("El mensaje del footer no puede quedar vacio.");
      setFooterStatus(null);
      return;
    }

    setFooterSaving(true);
    setFooterError(null);
    setFooterStatus(null);

    try {
      const savedMessage = await updateFooterMessage(normalizedFooterDraft);
      setFooterMessage(savedMessage);
      setFooterDraft(savedMessage);
      emitFooterMessageUpdated(savedMessage);
      setFooterStatus("Mensaje del footer actualizado.");
    } catch (err) {
      setFooterError(err instanceof Error ? err.message : "No se pudo guardar el footer.");
    } finally {
      setFooterSaving(false);
    }
  }

  async function handleUploadPolicySave() {
    if (guestLimitMb === null || registeredLimitMb === null) {
      setUploadPolicyError("Los limites deben ser numeros enteros entre 1 MB y 500 MB.");
      setUploadPolicyStatus(null);
      return;
    }

    setUploadPolicySaving(true);
    setUploadPolicyError(null);
    setUploadPolicyStatus(null);

    try {
      const savedPolicy = await updateUploadPolicy({
        guestMaxBytes: guestLimitMb * BYTES_PER_MB,
        registeredMaxBytes: registeredLimitMb * BYTES_PER_MB,
      });
      setUploadPolicy(savedPolicy);
      setGuestLimitDraft(String(bytesToMegabytes(savedPolicy.guestMaxBytes)));
      setRegisteredLimitDraft(String(bytesToMegabytes(savedPolicy.registeredMaxBytes)));
      setUploadPolicyStatus("Politica de uploads actualizada.");
    } catch (err) {
      setUploadPolicyError(err instanceof Error ? err.message : "No se pudo guardar la politica de uploads.");
    } finally {
      setUploadPolicySaving(false);
    }
  }

  return (
    <div className="mt-6 space-y-6">
      <div className="grid gap-6 lg:grid-cols-[minmax(0,1.35fr)_minmax(320px,0.65fr)]">
        <section className="overflow-hidden rounded-xl border border-stone-200 bg-white">
          <div className="border-b border-stone-200 px-5 py-4">
            <h2 className="text-base font-semibold text-stone-900">Jobs recientes</h2>
            <p className="mt-1 text-sm text-stone-500">Ultima actividad operativa visible para administracion.</p>
          </div>
          <table className="w-full border-collapse text-left">
            <thead className="bg-stone-50 text-xs font-medium text-stone-500">
              <tr>
                <th className="px-5 py-3">Job</th>
                <th className="px-5 py-3">Archivo</th>
                <th className="px-5 py-3">Usuario</th>
                <th className="px-5 py-3">Salida</th>
                <th className="px-5 py-3">Estado</th>
                <th className="px-5 py-3">Actualizado</th>
              </tr>
            </thead>
            <tbody>
              {data.recentJobs.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-5 py-8 text-sm text-stone-500">
                    No hay jobs registrados todavia.
                  </td>
                </tr>
              ) : (
                data.recentJobs.map((job) => (
                  <tr key={job.jobId} className="border-t border-stone-200 text-sm text-stone-700">
                    <td className="px-5 py-4 font-medium text-stone-900">{job.jobId.slice(0, 8)}</td>
                    <td className="px-5 py-4">{job.fileName}</td>
                    <td className="px-5 py-4">
                      <div className="font-medium text-stone-900">{job.userName}</div>
                      <div className="text-xs text-stone-500">{job.userEmail}</div>
                    </td>
                    <td className="px-5 py-4">{job.outputFormat.toUpperCase()}</td>
                    <td className="px-5 py-4 capitalize">{job.status}</td>
                    <td className="px-5 py-4 text-stone-500">{formatDate(job.updatedAt)}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </section>

        <aside className="space-y-4">
          <section className="rounded-xl border border-stone-200 bg-white px-5 py-4">
            <h2 className="text-base font-semibold text-stone-900">Limite de archivo</h2>
            <p className="mt-1 text-sm text-stone-500">
              Ajusta el maximo por archivo para invitados y usuarios registrados. El tope tecnico global sigue siendo 500 MB.
            </p>

            <div className="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-1">
              <label className="block">
                <span className="mb-1.5 block text-[13px] font-medium text-stone-600">Invitados</span>
                <div className="relative">
                  <input
                    type="number"
                    min={1}
                    max={500}
                    step={1}
                    inputMode="numeric"
                    value={guestLimitDraft}
                    onChange={(event) => {
                      setGuestLimitDraft(event.target.value);
                      if (uploadPolicyError) setUploadPolicyError(null);
                      if (uploadPolicyStatus) setUploadPolicyStatus(null);
                    }}
                    className="h-11 w-full rounded-xl border border-stone-200 bg-stone-50/60 px-3.5 pr-12 text-sm text-stone-900 transition-colors duration-150 focus:border-coral-400 focus:bg-white"
                    aria-label="Limite para invitados en MB"
                  />
                  <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs font-medium text-stone-400">
                    MB
                  </span>
                </div>
              </label>

              <label className="block">
                <span className="mb-1.5 block text-[13px] font-medium text-stone-600">Registrados</span>
                <div className="relative">
                  <input
                    type="number"
                    min={1}
                    max={500}
                    step={1}
                    inputMode="numeric"
                    value={registeredLimitDraft}
                    onChange={(event) => {
                      setRegisteredLimitDraft(event.target.value);
                      if (uploadPolicyError) setUploadPolicyError(null);
                      if (uploadPolicyStatus) setUploadPolicyStatus(null);
                    }}
                    className="h-11 w-full rounded-xl border border-stone-200 bg-stone-50/60 px-3.5 pr-12 text-sm text-stone-900 transition-colors duration-150 focus:border-coral-400 focus:bg-white"
                    aria-label="Limite para registrados en MB"
                  />
                  <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs font-medium text-stone-400">
                    MB
                  </span>
                </div>
              </label>
            </div>

            <div className="mt-3 flex items-center justify-between gap-3">
              <p className="text-xs text-stone-500">
                {uploadPolicy
                  ? `Actual: invitados ${formatMegabytes(uploadPolicy.guestMaxBytes)} · registrados ${formatMegabytes(uploadPolicy.registeredMaxBytes)}`
                  : "Cargando politica actual..."}
              </p>
              <button
                type="button"
                onClick={() => void handleUploadPolicySave()}
                disabled={uploadPolicySaving || !uploadPolicyDirty}
                className="inline-flex h-10 items-center rounded-lg bg-coral-500 px-4 text-sm font-medium text-white transition-colors duration-150 hover:bg-coral-600 disabled:cursor-not-allowed disabled:bg-coral-200"
              >
                {uploadPolicySaving ? "Guardando..." : "Guardar limites"}
              </button>
            </div>

            {uploadPolicyStatus ? (
              <p className="mt-3 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                {uploadPolicyStatus}
              </p>
            ) : null}

            {uploadPolicyError ? (
              <p className="mt-3 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
                {uploadPolicyError}
              </p>
            ) : null}
          </section>

          <section className="rounded-xl border border-stone-200 bg-white px-5 py-4">
            <h2 className="text-base font-semibold text-stone-900">Footer publico</h2>
            <p className="mt-1 text-sm text-stone-500">
              Edita el mensaje visible al pie de todas las pantallas publicas y privadas.
            </p>

            <label className="mt-4 block">
              <span className="mb-1.5 block text-[13px] font-medium text-stone-600">Mensaje actual</span>
              <textarea
                value={footerDraft}
                onChange={(event) => {
                  setFooterDraft(event.target.value);
                  if (footerError) setFooterError(null);
                  if (footerStatus) setFooterStatus(null);
                }}
                rows={3}
                maxLength={240}
                className="min-h-24 w-full rounded-xl border border-stone-200 bg-stone-50/60 px-3.5 py-3 text-sm text-stone-900 placeholder:text-stone-400 transition-colors duration-150 focus:border-coral-400 focus:bg-white"
                placeholder="Escribe el mensaje del footer"
              />
            </label>

            <div className="mt-3 flex items-center justify-between gap-3">
              <p className="text-xs text-stone-500">{footerDraft.length}/240</p>
              <button
                type="button"
                onClick={() => void handleFooterSave()}
                disabled={footerSaving || !footerDirty || !normalizedFooterDraft}
                className="inline-flex h-10 items-center rounded-lg bg-coral-500 px-4 text-sm font-medium text-white transition-colors duration-150 hover:bg-coral-600 disabled:cursor-not-allowed disabled:bg-coral-200"
              >
                {footerSaving ? "Guardando..." : "Guardar footer"}
              </button>
            </div>

            {footerStatus ? (
              <p className="mt-3 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                {footerStatus}
              </p>
            ) : null}

            {footerError ? (
              <p className="mt-3 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
                {footerError}
              </p>
            ) : null}
          </section>

          <SMTPSettingsSection />

          <section className="rounded-xl border border-stone-200 bg-white px-5 py-4">
            <h2 className="text-base font-semibold text-stone-900">Resumen operativo</h2>
            <div className="mt-4 space-y-3 text-sm text-stone-600">
              <p>Usuarios: {data.totalUsers}</p>
              <p>Archivos: {data.totalFiles}</p>
              <p>Jobs: {data.totalJobs}</p>
              <p>En cola: {data.queuedJobs}</p>
              <p>En ejecucion: {data.runningJobs}</p>
              <p>Exitosos: {data.succeededJobs}</p>
              <p>Fallidos: {data.failedJobs}</p>
              <p>Cancelados: {data.cancelledJobs}</p>
            </div>
          </section>

          <section className="rounded-xl border border-stone-200 bg-white px-5 py-4">
            <h2 className="text-base font-semibold text-stone-900">Indicadores</h2>
            <div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-1">
              <div className="rounded-lg border border-stone-200 bg-stone-50 px-4 py-3">
                <p className="text-xs uppercase tracking-[0.12em] text-stone-500">Tasa de exito</p>
                <p className="mt-2 text-2xl font-semibold text-stone-900">{data.successRatePct.toFixed(1)}%</p>
              </div>
              <div className="rounded-lg border border-stone-200 bg-stone-50 px-4 py-3">
                <p className="text-xs uppercase tracking-[0.12em] text-stone-500">Duracion media</p>
                <p className="mt-2 text-2xl font-semibold text-stone-900">{formatSeconds(data.averageDurationSec)}</p>
              </div>
              <div className="rounded-lg border border-stone-200 bg-stone-50 px-4 py-3">
                <p className="text-xs uppercase tracking-[0.12em] text-stone-500">Engines disponibles</p>
                <p className="mt-2 text-2xl font-semibold text-stone-900">
                  {data.availableEngines}/{data.totalEngines}
                </p>
              </div>
            </div>

            {data.unavailableEngines.length > 0 && (
              <div className="mt-4 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
                Engines faltantes: {data.unavailableEngines.join(", ")}
              </div>
            )}
          </section>

          <section className="rounded-xl border border-stone-200 bg-white px-5 py-4">
            <h2 className="text-base font-semibold text-stone-900">Uso por engine</h2>
            <div className="mt-4 space-y-3">
              {data.engineUsage.length === 0 ? (
                <p className="text-sm text-stone-500">Todavia no hay uso registrado.</p>
              ) : (
                data.engineUsage.map((item) => (
                  <div key={item.key} className="flex items-center justify-between rounded-lg border border-stone-200 px-3 py-2 text-sm text-stone-700">
                    <span className="font-medium text-stone-900">{item.key}</span>
                    <span>{item.count} jobs</span>
                  </div>
                ))
              )}
            </div>
          </section>
        </aside>
      </div>

      <EmailTemplatesSection />

      <section className="rounded-xl border border-stone-200 bg-white">
        <div className="border-b border-stone-200 px-5 py-4">
          <h2 className="text-base font-semibold text-stone-900">Auditoria reciente</h2>
          <p className="mt-1 text-sm text-stone-500">Eventos recientes del sistema con filtro rapido por tipo.</p>
          <div className="mt-4 flex flex-wrap gap-2">
            {auditFilters.map((filter) => (
              <button
                key={filter.value}
                type="button"
                onClick={() => setAuditFilter(filter.value)}
                className={
                  auditFilter === filter.value
                    ? "rounded-full border border-stone-900 bg-stone-900 px-3 py-1 text-xs font-medium text-white"
                    : "rounded-full border border-stone-300 bg-white px-3 py-1 text-xs font-medium text-stone-600"
                }
              >
                {filter.label}
              </button>
            ))}
          </div>
        </div>

        <div className="divide-y divide-stone-200">
          {visibleAudit.length === 0 ? (
            <p className="px-5 py-8 text-sm text-stone-500">No hay eventos para ese filtro.</p>
          ) : (
            visibleAudit.map((event) => (
              <div key={event.id} className="grid gap-2 px-5 py-4 text-sm text-stone-700 sm:grid-cols-[180px_minmax(0,1fr)_180px] sm:items-center">
                <div>
                  <p className="font-medium text-stone-900">{auditLabel(event.eventType)}</p>
                  <p className="mt-1 text-xs text-stone-500">{event.eventType}</p>
                </div>
                <div>
                  <p>{auditDetails(event)}</p>
                  <p className="mt-1 text-xs text-stone-500">
                    {event.jobId ? `Job ${event.jobId.slice(0, 8)}` : "Sin job"}
                    {event.fileId ? ` · File ${event.fileId.slice(0, 8)}` : ""}
                  </p>
                </div>
                <p className="text-xs text-stone-500 sm:text-right">{formatDate(event.createdAt)}</p>
              </div>
            ))
          )}
        </div>
      </section>
    </div>
  );
}
