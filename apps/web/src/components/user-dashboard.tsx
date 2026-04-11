"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { downloadArtifact, getMyDashboard, retryJob, type UserDashboardData } from "@/lib/api";
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

  return (
    <div className="mt-6 grid gap-6 lg:grid-cols-[minmax(0,1.45fr)_minmax(280px,0.55fr)]">
      <section className="overflow-hidden rounded-xl border border-stone-200 bg-white">
        <table className="w-full border-collapse text-left">
          <thead className="bg-stone-50 text-xs font-medium text-stone-500">
            <tr>
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
                <td colSpan={6} className="px-5 py-8 text-sm text-stone-500">
                  {t("emptyState")}
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