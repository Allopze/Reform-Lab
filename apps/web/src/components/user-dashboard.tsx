"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import {
  cancelJob,
  cancelJobs,
  downloadArtifact,
  getMyDashboard,
  retryJob,
  retryJobs,
  type UserDashboardData,
  type UserDashboardJob,
} from "@/lib/api";
import { requestEmailVerification } from "@/lib/auth";
import { useAuth } from "@/lib/auth-context";

function formatDate(value: string): string {
  return new Intl.DateTimeFormat("es-ES", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

export default function UserDashboard() {
  const router = useRouter();
  const { user, loading } = useAuth();
  const t = useTranslations("userDashboard");
  const tCommon = useTranslations("common");
  const [data, setData] = useState<UserDashboardData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [downloadingId, setDownloadingId] = useState<string | null>(null);
  const [retryingId, setRetryingId] = useState<string | null>(null);
  const [cancellingId, setCancellingId] = useState<string | null>(null);
  const [verificationStatus, setVerificationStatus] = useState<string | null>(null);
  const [verificationSending, setVerificationSending] = useState(false);
  const [selectedJobIds, setSelectedJobIds] = useState<string[]>([]);

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
        setError(err instanceof Error ? err.message : t("loadError"));
      });
  }, [loading, refreshDashboard, router, user, t]);

  if (loading || (!data && !error)) {
    return <p className="text-sm text-stone-500">{t("loading")}</p>;
  }

  if (error) {
    return <p className="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{error}</p>;
  }

  if (!data) return null;

  const isCancelable = (job: UserDashboardJob) =>
    job.status === "queued" || job.status === "running";
  const isRetryable = (job: UserDashboardJob) => job.status === "failed";
  const isSelectable = (job: UserDashboardJob) =>
    isCancelable(job) || isRetryable(job);

  const selectableJobIds = data.recentJobs
    .filter(isSelectable)
    .map((job) => job.jobId);
  const selectedJobs = data.recentJobs.filter((job) =>
    selectedJobIds.includes(job.jobId),
  );
  const canBatchCancel =
    selectedJobs.length > 0 && selectedJobs.every(isCancelable);
  const canBatchRetry =
    selectedJobs.length > 0 && selectedJobs.every(isRetryable);
  const hasMixedSelection =
    selectedJobs.length > 0 && !canBatchCancel && !canBatchRetry;

  async function handleSingleCancel(jobId: string) {
    try {
      setError(null);
      setCancellingId(jobId);
      await cancelJob(jobId);
      await refreshDashboard();
      setSelectedJobIds((current) => current.filter((id) => id !== jobId));
    } catch (err) {
      setError(err instanceof Error ? err.message : t("cancelError"));
    } finally {
      setCancellingId(null);
    }
  }

  async function handleBatchCancel() {
    if (!canBatchCancel) {
      return;
    }

    try {
      setError(null);
      await cancelJobs(selectedJobIds);
      await refreshDashboard();
      setSelectedJobIds([]);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("cancelError"));
    }
  }

  async function handleBatchRetry() {
    if (!canBatchRetry) {
      return;
    }

    try {
      setError(null);
      await retryJobs(selectedJobIds);
      await refreshDashboard();
      setSelectedJobIds([]);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("retryError"));
    }
  }

  async function handleVerificationRequest() {
    try {
      setError(null);
      setVerificationStatus(null);
      setVerificationSending(true);
      const result = await requestEmailVerification();
      setVerificationStatus(
        result.status === "already_verified"
          ? t("emailAlreadyVerified")
          : t("emailVerificationSent"),
      );
    } catch (err) {
      setVerificationStatus(err instanceof Error ? err.message : t("emailVerificationError"));
    } finally {
      setVerificationSending(false);
    }
  }

  return (
    <div className="mt-6 grid gap-6 lg:grid-cols-[minmax(0,1.45fr)_minmax(280px,0.55fr)]">
      {user && !user.emailVerifiedAt ? (
        <section className="lg:col-span-2 rounded-xl border border-amber-200 bg-amber-50 px-5 py-4 text-sm text-amber-900">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <p className="font-medium">{t("emailVerificationTitle")}</p>
              <p className="mt-1 text-amber-800">{t("emailVerificationDescription")}</p>
              {verificationStatus ? (
                <p className="mt-2 font-medium">{verificationStatus}</p>
              ) : null}
            </div>
            <button
              type="button"
              onClick={() => void handleVerificationRequest()}
              disabled={verificationSending}
              className="h-10 rounded-lg border border-amber-300 bg-white px-4 text-sm font-medium text-amber-900 transition-colors duration-150 hover:border-amber-400 disabled:cursor-not-allowed disabled:text-amber-400"
            >
              {verificationSending ? tCommon("sending") : t("sendVerificationEmail")}
            </button>
          </div>
        </section>
      ) : null}
      <section className="overflow-hidden rounded-xl border border-stone-200 bg-white">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-stone-200 px-5 py-4">
          <div>
            <h2 className="text-base font-semibold text-stone-900">
              {t("batchActionsTitle")}
            </h2>
            <p className="mt-1 text-sm text-stone-500">
              {t("selectedCount", { count: selectedJobIds.length })}
            </p>
            {hasMixedSelection ? (
              <p className="mt-2 text-sm text-amber-700">
                {t("selectionConflict")}
              </p>
            ) : null}
          </div>
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={() => void handleBatchCancel()}
              disabled={!canBatchCancel}
              className="inline-flex h-10 items-center rounded-lg border border-stone-300 px-4 text-sm font-medium text-stone-700 transition-colors duration-150 hover:border-stone-400 hover:bg-stone-50 disabled:cursor-not-allowed disabled:border-stone-200 disabled:text-stone-400"
            >
              {t("cancelSelected")}
            </button>
            <button
              type="button"
              onClick={() => void handleBatchRetry()}
              disabled={!canBatchRetry}
              className="inline-flex h-10 items-center rounded-lg bg-coral-500 px-4 text-sm font-medium text-white transition-colors duration-150 hover:bg-coral-600 disabled:cursor-not-allowed disabled:bg-coral-200"
            >
              {t("retrySelected")}
            </button>
          </div>
        </div>

        <table className="w-full border-collapse text-left">
          <thead className="bg-stone-50 text-xs font-medium text-stone-500">
            <tr>
              <th className="px-5 py-3">
                <input
                  type="checkbox"
                  aria-label={t("selectAllAria")}
                  checked={
                    selectableJobIds.length > 0 &&
                    selectedJobIds.length === selectableJobIds.length
                  }
                  onChange={(event) => {
                    setSelectedJobIds(
                      event.target.checked ? selectableJobIds : [],
                    );
                  }}
                />
              </th>
              <th className="px-5 py-3">{t("fileHeader")}</th>
              <th className="px-5 py-3">{t("detectedHeader")}</th>
              <th className="px-5 py-3">{t("outputHeader")}</th>
              <th className="px-5 py-3">{t("statusHeader")}</th>
              <th className="px-5 py-3">{t("updatedHeader")}</th>
              <th className="px-5 py-3">{t("actionHeader")}</th>
            </tr>
          </thead>
          <tbody>
            {data.recentJobs.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-5 py-8 text-sm text-stone-500">
                  {t("emptyState")}
                </td>
              </tr>
            ) : (
              data.recentJobs.map((job) => (
                <tr key={job.jobId} className="border-t border-stone-200 text-sm text-stone-700">
                  <td className="px-5 py-4">
                    {isSelectable(job) ? (
                      <input
                        type="checkbox"
                        aria-label={t("selectJobAria", { fileName: job.fileName })}
                        checked={selectedJobIds.includes(job.jobId)}
                        onChange={(event) => {
                          setSelectedJobIds((current) =>
                            event.target.checked
                              ? [...current, job.jobId]
                              : current.filter((id) => id !== job.jobId),
                          );
                        }}
                      />
                    ) : (
                      <span className="text-stone-300">-</span>
                    )}
                  </td>
                  <td className="px-5 py-4 font-medium text-stone-900">{job.fileName}</td>
                  <td className="px-5 py-4 capitalize">{job.detectedFamily}</td>
                  <td className="px-5 py-4">{job.outputFormat.toUpperCase()}</td>
                  <td className="px-5 py-4">
                    {(job.status === "running" || job.status === "queued") && job.progress > 0 ? (
                      <div className="flex items-center gap-2">
                        <div className="h-1.5 w-16 rounded-full bg-stone-200">
                          <progress
                            aria-label={t(`status.${job.status}`)}
                            className="conversion-progress conversion-progress--compact"
                            max={100}
                            value={job.progress}
                          />
                        </div>
                        <span className="text-xs text-stone-500">{job.progress}%</span>
                      </div>
                    ) : (
                      t(`status.${job.status}`)
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
                              setError(err instanceof Error ? err.message : t("downloadError"));
                            } finally {
                              setDownloadingId(null);
                            }
                          }}
                          className="font-medium text-coral-700 underline underline-offset-2"
                        >
                          {downloadingId === job.jobId ? tCommon("downloading") : tCommon("download")}
                        </button>
                        {job.expiresAt && (
                          <span className="text-xs text-stone-400">
                            {t("expires", { date: formatDate(job.expiresAt) })}
                          </span>
                        )}
                      </div>
                    ) : isCancelable(job) ? (
                      <button
                        type="button"
                        onClick={() => {
                          void handleSingleCancel(job.jobId);
                        }}
                        className="font-medium text-stone-700 underline underline-offset-2"
                      >
                        {cancellingId === job.jobId
                          ? t("cancelling")
                          : tCommon("cancel")}
                      </button>
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
                            setError(err instanceof Error ? err.message : t("retryError"));
                          } finally {
                            setRetryingId(null);
                          }
                        }}
                        className="font-medium text-coral-700 underline underline-offset-2"
                      >
                        {retryingId === job.jobId ? t("retrying") : t("retry")}
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
          <h2 className="text-base font-semibold text-stone-900">{t("summaryTitle")}</h2>
          <div className="mt-4 space-y-3 text-sm text-stone-600">
            <p>{t("ownFiles", { count: data.totalFiles })}</p>
            <p>{t("totalJobs", { count: data.totalJobs })}</p>
            <p>{t("activeJobs", { count: data.activeJobs })}</p>
            <p>{t("succeededJobs", { count: data.succeededJobs })}</p>
            <p>{t("failedJobs", { count: data.failedJobs })}</p>
          </div>
        </section>

        <section className="rounded-xl border border-stone-200 bg-white px-5 py-4">
          <h2 className="text-base font-semibold text-stone-900">{t("retentionTitle")}</h2>
          <p className="mt-3 text-sm leading-6 text-stone-600">
            {t("retentionDescription")}
          </p>
        </section>
      </aside>
    </div>
  );
}
