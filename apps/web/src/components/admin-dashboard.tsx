"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { useTranslations } from "next-intl";
import {
  getAdminOverview,
  getFooterMessage,
  getHealthInfo,
  getUploadPolicy,
  updateFooterMessage,
  updateUploadPolicy,
  type AdminDashboardData,
  type HealthInfo,
  type UploadPolicy,
} from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import {
  DEFAULT_FOOTER_MESSAGE,
  emitFooterMessageUpdated,
} from "@/lib/footer-message";
import SMTPSettingsSection from "@/components/smtp-settings";
import EmailTemplatesSection from "@/components/email-templates";
import WebhookSettings from "@/components/webhook-settings";

const BYTES_PER_MB = 1024 * 1024;

const STATUS_BADGE_CLASS: Record<string, string> = {
  queued: "border-amber-200 bg-amber-50 text-amber-700",
  running: "border-amber-200 bg-amber-50 text-amber-700",
  succeeded: "border-emerald-200 bg-emerald-50 text-emerald-700",
  failed: "border-rose-200 bg-rose-50 text-rose-700",
  cancelled: "border-stone-200 bg-stone-100 text-stone-500",
  expired: "border-stone-200 bg-stone-100 text-stone-500",
};

const STATUS_BADGE_FALLBACK = "border-stone-200 bg-stone-100 text-stone-600";

const AUDIT_BORDER_CLASS: Record<string, string> = {
  upload: "border-l-sky-400",
  job_created: "border-l-stone-300",
  job_started: "border-l-amber-400",
  job_completed: "border-l-emerald-400",
  job_failed: "border-l-rose-400",
  job_cancelled: "border-l-stone-400",
  job_retried: "border-l-amber-400",
  artifact_created: "border-l-emerald-400",
  artifact_downloaded: "border-l-emerald-300",
  session_login: "border-l-sky-300",
  session_login_failed: "border-l-rose-300",
  session_logout: "border-l-stone-300",
  admin_footer_updated: "border-l-violet-400",
  admin_upload_policy_updated: "border-l-violet-400",
  admin_smtp_updated: "border-l-violet-400",
  admin_smtp_test: "border-l-violet-400",
  admin_template_created: "border-l-violet-400",
  admin_template_updated: "border-l-violet-400",
  admin_template_deleted: "border-l-violet-400",
  admin_webhook_created: "border-l-violet-400",
  admin_webhook_updated: "border-l-violet-400",
  admin_webhook_deleted: "border-l-violet-400",
  admin_role_changed: "border-l-violet-400",
  admin_queue_paused: "border-l-violet-400",
  admin_queue_resumed: "border-l-violet-400",
  admin_queue_drained: "border-l-violet-400",
  admin_workers_pruned: "border-l-violet-400",
};

const AUDIT_BORDER_FALLBACK = "border-l-stone-200";

type SidebarTab = "operativo" | "config";

const auditFilterKeys = [
  "all",
  "upload",
  "job_created",
  "job_started",
  "job_completed",
  "job_failed",
  "job_cancelled",
  "job_retried",
  "artifact_created",
  "admin",
] as const;

function formatDate(value: string): string {
  return new Intl.DateTimeFormat("es-ES", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

function formatSeconds(value: number, noDataLabel: string): string {
  if (value <= 0) return noDataLabel;
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

export default function AdminDashboard() {
  const router = useRouter();
  const { user, loading } = useAuth();
  const t = useTranslations("adminDashboard");
  const tCommon = useTranslations("common");

  function auditDetails(event: NonNullable<AdminDashboardData>["recentAudit"][number]): string {
    const details = event.details ?? {};
    if (event.eventType === "artifact_created") {
      const fileName = typeof details.fileName === "string" ? details.fileName : null;
      return fileName ? t("auditDetail.output", { fileName }) : t("auditDetail.artifactPersisted");
    }
    if (event.eventType === "job_failed") {
      const error = typeof details.error === "string" ? details.error : null;
      return error || t("auditDetail.failureRecorded");
    }
    if (event.eventType === "job_created") {
      const capability = typeof details.capabilityId === "string" ? details.capabilityId : null;
      return capability ? t("auditDetail.capability", { id: capability }) : t("auditDetail.requestRecorded");
    }
    if (event.eventType === "job_retried") {
      const sourceJobId = typeof details.sourceJobId === "string" ? details.sourceJobId : null;
      return sourceJobId ? t("auditDetail.retryFrom", { jobId: sourceJobId.slice(0, 8) }) : t("auditDetail.retryRecorded");
    }
    if (event.eventType === "upload") {
      const originalName = typeof details.originalName === "string" ? details.originalName : null;
      return originalName || t("auditDetail.uploadRecorded");
    }
    return t("auditDetail.eventRecorded");
  }

  const [data, setData] = useState<AdminDashboardData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [auditFilter, setAuditFilter] = useState<(typeof auditFilterKeys)[number]>("all");
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
  const [sidebarTab, setSidebarTab] = useState<SidebarTab>("operativo");
  const [healthInfo, setHealthInfo] = useState<HealthInfo | null>(null);

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
        setError(err instanceof Error ? err.message : t("loadError"));
      });

    getFooterMessage()
      .then((message) => {
        setFooterMessage(message);
        setFooterDraft(message);
      })
      .catch((err) => {
        setFooterError(err instanceof Error ? err.message : t("footerLoadError"));
      });

    getUploadPolicy()
      .then((policy) => {
        setUploadPolicy(policy);
        setGuestLimitDraft(String(bytesToMegabytes(policy.guestMaxBytes)));
        setRegisteredLimitDraft(String(bytesToMegabytes(policy.registeredMaxBytes)));
      })
      .catch((err) => {
        setUploadPolicyError(err instanceof Error ? err.message : t("policyLoadError"));
      });

    getHealthInfo()
      .then(setHealthInfo)
      .catch(() => {/* health is optional, don't block dashboard */});
  }, [loading, router, user, t]);

  if (loading || (!data && !error)) {
    return <p className="text-sm text-stone-500">{t("loading")}</p>;
  }

  if (error) {
    return <p className="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{error}</p>;
  }

  if (!data) return null;

  const visibleAudit =
    auditFilter === "all"
      ? data.recentAudit
      : auditFilter === "admin"
        ? data.recentAudit.filter((event) => event.eventType.startsWith("admin_"))
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
      setFooterError(t("footerEmpty"));
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
      setFooterStatus(t("footerUpdated"));
    } catch (err) {
      setFooterError(err instanceof Error ? err.message : t("footerSaveError"));
    } finally {
      setFooterSaving(false);
    }
  }

  async function handleUploadPolicySave() {
    if (guestLimitMb === null || registeredLimitMb === null) {
      setUploadPolicyError(t("limitsValidation"));
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
      setUploadPolicyStatus(t("policyUpdated"));
    } catch (err) {
      setUploadPolicyError(err instanceof Error ? err.message : t("policySaveError"));
    } finally {
      setUploadPolicySaving(false);
    }
  }

  return (
    <div className="mt-6 space-y-6">
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <Link href="/admin/users" className="rounded-2xl border border-stone-200 bg-white px-4 py-3 shadow-[0_1px_3px_rgba(15,23,42,0.04)] hover:border-stone-300 transition-colors">
          <p className="text-xs uppercase tracking-[0.12em] text-stone-500">{t("totalUsers", { count: "" }).replace(/:\s*$/, "").trim()}</p>
          <p className="mt-1.5 text-2xl font-semibold text-stone-900">{data.totalUsers}</p>
        </Link>
        <div className="rounded-2xl border border-stone-200 bg-white px-4 py-3 shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
          <p className="text-xs uppercase tracking-[0.12em] text-stone-500">{t("totalFiles", { count: "" }).replace(/:\s*$/, "").trim()}</p>
          <p className="mt-1.5 text-2xl font-semibold text-stone-900">{data.totalFiles}</p>
        </div>
        <div className="rounded-2xl border border-stone-200 bg-white px-4 py-3 shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
          <p className="text-xs uppercase tracking-[0.12em] text-stone-500">{t("successRate")}</p>
          <p className="mt-1.5 text-2xl font-semibold text-stone-900">{data.successRatePct.toFixed(1)}%</p>
        </div>
        <div className="rounded-2xl border border-stone-200 bg-white px-4 py-3 shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
          <p className="text-xs uppercase tracking-[0.12em] text-stone-500">{t("avgDuration")}</p>
          <p className="mt-1.5 text-2xl font-semibold text-stone-900">{formatSeconds(data.averageDurationSec, t("noData"))}</p>
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1.35fr)_minmax(320px,0.65fr)]">
        <section className="overflow-hidden rounded-2xl border border-stone-200 bg-white shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
          <div className="flex items-center justify-between border-b border-stone-200 px-5 py-4">
            <div>
              <h2 className="text-base font-semibold text-stone-900">{t("recentJobs")}</h2>
              <p className="mt-1 text-sm text-stone-500">{t("recentJobsDescription")}</p>
            </div>
            <Link
              href="/admin/jobs"
              className="text-sm font-medium text-coral-500 hover:text-coral-600"
            >
              {t("viewAllJobs")}
            </Link>
          </div>
          <div className="overflow-x-auto">
          <table className="w-full min-w-175 border-collapse text-left">
            <thead className="bg-stone-50 text-xs font-medium text-stone-500">
              <tr>
                <th className="px-5 py-3">{t("jobHeader")}</th>
                <th className="px-5 py-3">{t("fileHeader")}</th>
                <th className="px-5 py-3">{t("userHeader")}</th>
                <th className="px-5 py-3">{t("capabilityHeader")}</th>
                <th className="px-5 py-3">{t("outputHeader")}</th>
                <th className="px-5 py-3">{t("statusHeader")}</th>
                <th className="px-5 py-3">{t("updatedHeader")}</th>
              </tr>
            </thead>
            <tbody>
              {data.recentJobs.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-5 py-8 text-sm text-stone-500">
                    {t("emptyJobs")}
                  </td>
                </tr>
              ) : (
                data.recentJobs.map((job) => (
                  <tr key={job.jobId} className="border-t border-stone-200 text-sm text-stone-700">
                    <td className="px-5 py-4 font-medium text-stone-900">{job.jobId.slice(0, 8)}</td>
                    <td className="px-5 py-4">{job.fileName}</td>
                    <td className="px-5 py-4">
                      {job.userEmail === "sin-propietario@local" ? (
                        <span className="inline-block rounded-full border border-stone-200 bg-stone-100 px-2.5 py-0.5 text-xs font-medium text-stone-500">
                          {t("guestBadge")}
                        </span>
                      ) : (
                        <>
                          <div className="font-medium text-stone-900">{job.userName}</div>
                          <div className="text-xs text-stone-500">{job.userEmail}</div>
                        </>
                      )}
                    </td>
                    <td className="px-5 py-4 text-xs text-stone-500">{job.capabilityId}</td>
                    <td className="px-5 py-4">{job.outputFormat.toUpperCase()}</td>
                    <td className="px-5 py-4">
                      <span className={`inline-block rounded-full border px-2.5 py-0.5 text-xs font-medium ${STATUS_BADGE_CLASS[job.status] ?? STATUS_BADGE_FALLBACK}`}>
                        {job.status}
                      </span>
                      {job.error && (
                        <p className="mt-1 max-w-48 truncate text-xs text-rose-600" title={job.error}>
                          {job.error}
                        </p>
                      )}
                    </td>
                    <td className="px-5 py-4 text-stone-500">{formatDate(job.updatedAt)}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
          </div>
        </section>

        <aside className="space-y-4">
          <div className="flex gap-1 rounded-[22px] bg-stone-100 p-1">
            <button
              type="button"
              onClick={() => setSidebarTab("operativo")}
              className={`flex-1 rounded-[18px] py-2 text-sm font-medium outline-none transition-colors duration-150 focus-visible:ring-2 focus-visible:ring-coral-400/40 focus-visible:ring-offset-1 ${
                sidebarTab === "operativo" ? "bg-white text-stone-900 shadow-sm" : "text-stone-500 hover:text-stone-700"
              }`}
            >
              {t("summaryTitle")}
            </button>
            <button
              type="button"
              onClick={() => setSidebarTab("config")}
              className={`flex-1 rounded-[18px] py-2 text-sm font-medium outline-none transition-colors duration-150 focus-visible:ring-2 focus-visible:ring-coral-400/40 focus-visible:ring-offset-1 ${
                sidebarTab === "config" ? "bg-white text-stone-900 shadow-sm" : "text-stone-500 hover:text-stone-700"
              }`}
            >
              {t("fileLimitTitle")}
            </button>
          </div>

          {sidebarTab === "config" ? (
            <>
          <section className="rounded-2xl border border-stone-200 bg-white px-5 py-4 shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
            <h2 className="text-base font-semibold text-stone-900">{t("fileLimitTitle")}</h2>
            <p className="mt-1 text-sm text-stone-500">
              {t("fileLimitDescription")}
            </p>

            <div className="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-1">
              <label className="block">
                <span className="mb-1.5 block text-[13px] font-medium text-stone-600">{t("guestsLabel")}</span>
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
                    aria-label={t("guestLimitAria")}
                  />
                  <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs font-medium text-stone-400">
                    MB
                  </span>
                </div>
              </label>

              <label className="block">
                <span className="mb-1.5 block text-[13px] font-medium text-stone-600">{t("registeredLabel")}</span>
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
                    aria-label={t("registeredLimitAria")}
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
                  ? t("currentLimits", { guestLimit: formatMegabytes(uploadPolicy.guestMaxBytes), registeredLimit: formatMegabytes(uploadPolicy.registeredMaxBytes) })
                  : t("loadingPolicy")}
              </p>
              <button
                type="button"
                onClick={() => void handleUploadPolicySave()}
                disabled={uploadPolicySaving || !uploadPolicyDirty}
                className="inline-flex h-10 items-center rounded-lg bg-coral-500 px-4 text-sm font-medium text-white transition-colors duration-150 hover:bg-coral-600 disabled:cursor-not-allowed disabled:bg-coral-200"
              >
                {uploadPolicySaving ? tCommon("saving") : t("saveLimits")}
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

          <section className="rounded-2xl border border-stone-200 bg-white px-5 py-4 shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
            <h2 className="text-base font-semibold text-stone-900">{t("footerTitle")}</h2>
            <p className="mt-1 text-sm text-stone-500">
              {t("footerDescription")}
            </p>

            <label className="mt-4 block">
              <span className="mb-1.5 block text-[13px] font-medium text-stone-600">{t("currentMessage")}</span>
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
                placeholder={t("footerPlaceholder")}
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
                {footerSaving ? tCommon("saving") : t("saveFooter")}
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

          <WebhookSettings />
            </>
          ) : (
            <>
          <section className="rounded-2xl border border-stone-200 bg-white px-5 py-4 shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
            <h2 className="text-base font-semibold text-stone-900">{t("summaryTitle")}</h2>
            <div className="mt-4 space-y-2 text-sm text-stone-600">
              <p>{t("totalJobs", { count: data.totalJobs })}</p>
              <p>{t("queuedJobs", { count: data.queuedJobs })}</p>
              <p>{t("runningJobs", { count: data.runningJobs })}</p>
              <p>{t("succeededJobs", { count: data.succeededJobs })}</p>
              <p>{t("failedJobs", { count: data.failedJobs })}</p>
              <p>{t("cancelledJobs", { count: data.cancelledJobs })}</p>
            </div>
          </section>

          <section className="rounded-2xl border border-stone-200 bg-white px-5 py-4 shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
            <h2 className="text-base font-semibold text-stone-900">{t("availableEngines")}</h2>
            <p className="mt-2 text-2xl font-semibold text-stone-900">
              {data.availableEngines}/{data.totalEngines}
            </p>

            {data.unavailableEngines.length > 0 && (
              <div className="mt-3 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
                {t("missingEngines", { engines: data.unavailableEngines.join(", ") })}
              </div>
            )}
          </section>

          <section className="rounded-2xl border border-stone-200 bg-white px-5 py-4 shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
            <h2 className="text-base font-semibold text-stone-900">{t("engineUsageTitle")}</h2>
            <div className="mt-4 space-y-3">
              {data.engineUsage.length === 0 ? (
                <p className="text-sm text-stone-500">{t("noEngineUsage")}</p>
              ) : (
                data.engineUsage.map((item) => (
                  <div key={item.key} className="flex items-center justify-between rounded-lg border border-stone-200 px-3 py-2 text-sm text-stone-700">
                    <span className="font-medium text-stone-900">{item.key}</span>
                    <span>{t("jobs", { count: item.count })}</span>
                  </div>
                ))
              )}
            </div>
          </section>

          {healthInfo && (
            <section className="rounded-2xl border border-stone-200 bg-white px-5 py-4 shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
              <div className="flex items-center justify-between gap-3">
                <h2 className="text-base font-semibold text-stone-900">{t("systemHealth")}</h2>
                <Link
                  href="/admin/system"
                  className="text-sm font-medium text-coral-500 hover:text-coral-600"
                >
                  {t("viewSystemModule")}
                </Link>
              </div>
              <div className="mt-4 space-y-3 text-sm text-stone-600">
                <div>
                  <p className="text-xs font-medium uppercase tracking-[0.12em] text-stone-500">{t("retentionPolicy")}</p>
                  <p className="mt-1">{t("defaultTTL", { hours: healthInfo.retention.artifactTTLHours })}</p>
                  {Object.keys(healthInfo.retention.artifactTTLHoursByFamily).length > 0 && (
                    <div className="mt-2 space-y-1">
                      {Object.entries(healthInfo.retention.artifactTTLHoursByFamily).map(([family, hours]) => (
                        <p key={family} className="text-xs text-stone-500">{family}: {hours}h</p>
                      ))}
                    </div>
                  )}
                </div>
                {(healthInfo.featureFlags.disabledCapabilities.length > 0 || healthInfo.featureFlags.disabledEngines.length > 0) && (
                  <div>
                    <p className="text-xs font-medium uppercase tracking-[0.12em] text-stone-500">{t("featureFlags")}</p>
                    {healthInfo.featureFlags.disabledEngines.length > 0 && (
                      <p className="mt-1 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">
                        {t("disabledEngines", { engines: healthInfo.featureFlags.disabledEngines.join(", ") })}
                      </p>
                    )}
                    {healthInfo.featureFlags.disabledCapabilities.length > 0 && (
                      <p className="mt-1 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">
                        {t("disabledCapabilities", { capabilities: healthInfo.featureFlags.disabledCapabilities.join(", ") })}
                      </p>
                    )}
                  </div>
                )}
              </div>
            </section>
          )}
            </>
          )}
        </aside>
      </div>

      <EmailTemplatesSection />

      <section className="rounded-2xl border border-stone-200 bg-white shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
        <div className="border-b border-stone-200 px-5 py-4">
          <div className="flex items-center justify-between gap-3">
            <h2 className="text-base font-semibold text-stone-900">{t("auditTitle")}</h2>
            <Link
              href="/admin/audit"
              className="text-sm font-medium text-coral-500 hover:text-coral-600"
            >
              {t("viewAuditModule")}
            </Link>
          </div>
          <p className="mt-1 text-sm text-stone-500">{t("auditDescription")}</p>
          <div className="mt-4 flex flex-wrap gap-2">
            {auditFilterKeys.map((key) => (
              <button
                key={key}
                type="button"
                onClick={() => setAuditFilter(key)}
                className={
                  auditFilter === key
                    ? "rounded-full border border-coral-500 bg-coral-500 px-3 py-1 text-xs font-medium text-white"
                    : "rounded-full border border-stone-300 bg-white px-3 py-1 text-xs font-medium text-stone-600 transition-colors duration-150 hover:border-stone-400"
                }
              >
                {t(`auditFilter.${key}`)}
              </button>
            ))}
          </div>
        </div>

        <div className="divide-y divide-stone-200">
          {visibleAudit.length === 0 ? (
            <p className="px-5 py-8 text-sm text-stone-500">{t("noAuditEvents")}</p>
          ) : (
            visibleAudit.map((event) => (
              <div key={event.id} className={`grid gap-2 border-l-3 px-5 py-4 text-sm text-stone-700 sm:grid-cols-[180px_minmax(0,1fr)_180px] sm:items-center ${AUDIT_BORDER_CLASS[event.eventType] ?? AUDIT_BORDER_FALLBACK}`}>
                <div>
                  <p className="font-medium text-stone-900">{t(`auditLabel.${event.eventType}`)}</p>
                  <p className="mt-1 text-xs text-stone-500">{event.eventType}</p>
                </div>
                <div>
                  <p>{auditDetails(event)}</p>
                  <p className="mt-1 text-xs text-stone-500">
                    {event.jobId ? `Job ${event.jobId.slice(0, 8)}` : t("sinJob")}
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
