"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import {
  cancelAdminJobs,
  cancelJob,
  getAdminJobs,
  retryAdminJobs,
  retryJob,
  type AdminJobPage,
  type AdminJobFilter,
  type AdminJobRow,
} from "@/lib/api";
import { useAuth } from "@/lib/auth-context";

const STATUS_BADGE_CLASS: Record<string, string> = {
  queued: "border-amber-200 bg-amber-50 text-amber-700",
  running: "border-amber-200 bg-amber-50 text-amber-700",
  succeeded: "border-emerald-200 bg-emerald-50 text-emerald-700",
  failed: "border-rose-200 bg-rose-50 text-rose-700",
  cancelled: "border-stone-200 bg-stone-100 text-stone-500",
  expired: "border-stone-200 bg-stone-100 text-stone-500",
};
const STATUS_BADGE_FALLBACK = "border-stone-200 bg-stone-100 text-stone-600";

const STATUSES = ["", "queued", "running", "succeeded", "failed", "cancelled", "expired"] as const;
const PAGE_SIZE = 30;

function formatDate(iso: string): string {
  try {
    return new Intl.DateTimeFormat("es", {
      day: "2-digit",
      month: "short",
      hour: "2-digit",
      minute: "2-digit",
    }).format(new Date(iso));
  } catch {
    return iso;
  }
}

function formatBacklogMinutes(seconds: number): number {
  if (!Number.isFinite(seconds) || seconds < 0) {
    return 0;
  }
  return Math.max(1, Math.round(seconds / 60));
}

export default function AdminJobsTable() {
  const { user, loading } = useAuth();
  const router = useRouter();
  const t = useTranslations("adminJobs");

  const [page, setPage] = useState<AdminJobPage | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState("");
  const [search, setSearch] = useState("");
  const [stalledOnly, setStalledOnly] = useState(false);
  const [offset, setOffset] = useState(0);
  const [actingJobId, setActingJobId] = useState<string | null>(null);
  const [actingType, setActingType] = useState<"cancel" | "retry" | null>(null);
  const [selectedJobIds, setSelectedJobIds] = useState<string[]>([]);
  const [bulkAction, setBulkAction] = useState<"cancel" | "retry" | "filter-cancel" | "filter-retry" | null>(null);

  const fetchJobs = useCallback(async () => {
    try {
      const filter: AdminJobFilter = {
        limit: PAGE_SIZE,
        offset,
      };
      if (statusFilter) filter.status = statusFilter;
      if (search.trim()) filter.q = search.trim();
      if (stalledOnly) filter.stalled = true;
      const result = await getAdminJobs(filter);
      setPage(result);
      setSelectedJobIds((current) =>
        current.filter((jobId) => result.jobs.some((job) => job.jobId === jobId)),
      );
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("loadError"));
    }
  }, [statusFilter, search, stalledOnly, offset, t]);

  useEffect(() => {
    if (loading) return;
    if (!user || user.role !== "admin") {
      router.replace("/usuario");
      return;
    }
    fetchJobs();
  }, [loading, user, router, fetchJobs]);

  if (loading || !page) {
    return <p className="mt-6 text-sm text-stone-500">{t("loading")}</p>;
  }

  if (error) {
    return <p className="mt-6 text-sm text-rose-600">{error}</p>;
  }

  const isCancelable = (job: AdminJobRow) =>
    job.status === "queued" || job.status === "running";
  const isRetryable = (job: AdminJobRow) => job.status === "failed";

  const pageTotal = page.total;
  const totalPages = Math.ceil(pageTotal / PAGE_SIZE);
  const currentPage = Math.floor(offset / PAGE_SIZE) + 1;
  const selectedJobs = page.jobs.filter((job) => selectedJobIds.includes(job.jobId));
  const selectableJobs = page.jobs.filter((job) => isCancelable(job) || isRetryable(job));
  const currentFilter: AdminJobFilter = {
    ...(statusFilter ? { status: statusFilter } : {}),
    ...(search.trim() ? { q: search.trim() } : {}),
    ...(stalledOnly ? { stalled: true } : {}),
  };

  function toggleSelection(jobId: string) {
    setSelectedJobIds((current) =>
      current.includes(jobId)
        ? current.filter((value) => value !== jobId)
        : [...current, jobId],
    );
  }

  function toggleVisibleSelection(checked: boolean) {
    const ids = selectableJobs.map((job) => job.jobId);
    setSelectedJobIds((current) =>
      checked
        ? Array.from(new Set([...current, ...ids]))
        : current.filter((id) => !ids.includes(id)),
    );
  }

  async function handleCancel(jobId: string) {
    try {
      setError(null);
      setActingJobId(jobId);
      setActingType("cancel");
      await cancelJob(jobId);
      await fetchJobs();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("loadError"));
    } finally {
      setActingJobId(null);
      setActingType(null);
    }
  }

  async function handleRetry(jobId: string) {
    try {
      setError(null);
      setActingJobId(jobId);
      setActingType("retry");
      await retryJob(jobId);
      await fetchJobs();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("loadError"));
    } finally {
      setActingJobId(null);
      setActingType(null);
    }
  }

  async function handleBulkCancelSelected() {
    if (selectedJobs.length === 0) return;
    if (!window.confirm(t("confirmCancelSelected", { count: selectedJobs.length }))) {
      return;
    }
    try {
      setBulkAction("cancel");
      setError(null);
      await cancelAdminJobs({ jobIds: selectedJobs.map((job) => job.jobId) });
      setSelectedJobIds([]);
      await fetchJobs();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("loadError"));
    } finally {
      setBulkAction(null);
    }
  }

  async function handleBulkRetrySelected() {
    const retryableIds = selectedJobs.filter(isRetryable).map((job) => job.jobId);
    if (retryableIds.length === 0) return;
    if (!window.confirm(t("confirmRetrySelected", { count: retryableIds.length }))) {
      return;
    }
    try {
      setBulkAction("retry");
      setError(null);
      await retryAdminJobs({ jobIds: retryableIds });
      setSelectedJobIds([]);
      await fetchJobs();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("loadError"));
    } finally {
      setBulkAction(null);
    }
  }

  async function handleFilterAction(action: "cancel" | "retry") {
    if (!window.confirm(t(action === "cancel" ? "confirmCancelFilter" : "confirmRetryFilter", { count: pageTotal }))) {
      return;
    }
    try {
      setBulkAction(action === "cancel" ? "filter-cancel" : "filter-retry");
      setError(null);
      if (action === "cancel") {
        await cancelAdminJobs({ filter: currentFilter });
      } else {
        await retryAdminJobs({ filter: currentFilter });
      }
      setSelectedJobIds([]);
      await fetchJobs();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("loadError"));
    } finally {
      setBulkAction(null);
    }
  }

  return (
    <div className="mt-6 space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <select
          value={statusFilter}
          onChange={(e) => { setStatusFilter(e.target.value); setOffset(0); }}
          className="h-9 rounded-lg border border-stone-200 bg-white px-3 text-sm text-stone-700"
        >
          {STATUSES.map((s) => (
            <option key={s} value={s}>
              {s === "" ? t("allStatuses") : s}
            </option>
          ))}
        </select>

        <input
          type="text"
          value={search}
          onChange={(e) => { setSearch(e.target.value); setOffset(0); }}
          placeholder={t("searchPlaceholder")}
          className="h-9 w-64 rounded-lg border border-stone-200 bg-white px-3 text-sm text-stone-700 placeholder:text-stone-400"
        />

        <label className="inline-flex items-center gap-2 text-sm text-stone-600">
          <input
            type="checkbox"
            checked={stalledOnly}
            onChange={(e) => {
              setStalledOnly(e.target.checked);
              setOffset(0);
            }}
            className="h-4 w-4 rounded border-stone-300 text-coral-600"
          />
          {t("stalledOnly")}
        </label>

        <span className="ml-auto text-xs text-stone-500">
          {t("totalJobs", { count: pageTotal })}
        </span>
      </div>

      <p className="text-xs text-stone-500">
        {t("stalledSummary", {
          count: page.stalledJobs,
          queued: page.stalledQueuedJobs,
          running: page.stalledRunningJobs,
        })}
      </p>

      <div className="flex flex-wrap items-center gap-2 rounded-xl border border-stone-200 bg-stone-50 px-3 py-3 text-sm">
        <span className="text-stone-600">{t("selectedCount", { count: selectedJobIds.length })}</span>
        <button
          type="button"
          disabled={selectedJobs.filter(isCancelable).length === 0 || bulkAction !== null}
          onClick={() => {
            void handleBulkCancelSelected();
          }}
          className="rounded-lg border border-stone-200 bg-white px-3 py-1.5 text-sm text-stone-700 disabled:cursor-not-allowed disabled:opacity-40"
        >
          {bulkAction === "cancel" ? t("cancelling") : t("cancelSelected")}
        </button>
        <button
          type="button"
          disabled={selectedJobs.filter(isRetryable).length === 0 || bulkAction !== null}
          onClick={() => {
            void handleBulkRetrySelected();
          }}
          className="rounded-lg border border-stone-200 bg-white px-3 py-1.5 text-sm text-stone-700 disabled:cursor-not-allowed disabled:opacity-40"
        >
          {bulkAction === "retry" ? t("retrying") : t("retrySelected")}
        </button>
        <button
          type="button"
          disabled={pageTotal === 0 || bulkAction !== null}
          onClick={() => {
            void handleFilterAction("cancel");
          }}
          className="rounded-lg border border-stone-200 bg-white px-3 py-1.5 text-sm text-stone-700 disabled:cursor-not-allowed disabled:opacity-40"
        >
          {bulkAction === "filter-cancel" ? t("cancelling") : t("cancelFiltered")}
        </button>
        <button
          type="button"
          disabled={pageTotal === 0 || bulkAction !== null}
          onClick={() => {
            void handleFilterAction("retry");
          }}
          className="rounded-lg border border-stone-200 bg-white px-3 py-1.5 text-sm text-stone-700 disabled:cursor-not-allowed disabled:opacity-40"
        >
          {bulkAction === "filter-retry" ? t("retrying") : t("retryFiltered")}
        </button>
      </div>

      <div className="overflow-hidden rounded-2xl border border-stone-200 bg-white shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
        <div className="overflow-x-auto">
          <table className="w-full min-w-225 border-collapse text-left">
            <thead className="bg-stone-50 text-xs font-medium text-stone-500">
              <tr>
                <th className="px-4 py-3">
                  <input
                    type="checkbox"
                    checked={selectableJobs.length > 0 && selectableJobs.every((job) => selectedJobIds.includes(job.jobId))}
                    onChange={(event) => toggleVisibleSelection(event.target.checked)}
                    className="h-4 w-4 rounded border-stone-300"
                  />
                </th>
                <th className="px-4 py-3">{t("jobHeader")}</th>
                <th className="px-4 py-3">{t("fileHeader")}</th>
                <th className="px-4 py-3">{t("userHeader")}</th>
                <th className="px-4 py-3">{t("capabilityHeader")}</th>
                <th className="px-4 py-3">{t("outputHeader")}</th>
                <th className="px-4 py-3">{t("statusHeader")}</th>
                <th className="px-4 py-3">{t("backlogHeader")}</th>
                <th className="px-4 py-3">{t("createdHeader")}</th>
                <th className="px-4 py-3">{t("actionHeader")}</th>
              </tr>
            </thead>
            <tbody>
              {page.jobs.length === 0 ? (
                <tr>
                  <td colSpan={10} className="px-4 py-8 text-sm text-stone-500">
                    {t("emptyJobs")}
                  </td>
                </tr>
              ) : (
                page.jobs.map((job) => (
                  <tr key={job.jobId} className="border-t border-stone-200 text-sm text-stone-700">
                    <td className="px-4 py-3">
                      <input
                        type="checkbox"
                        checked={selectedJobIds.includes(job.jobId)}
                        disabled={!isCancelable(job) && !isRetryable(job)}
                        onChange={() => toggleSelection(job.jobId)}
                        className="h-4 w-4 rounded border-stone-300"
                      />
                    </td>
                    <td className="px-4 py-3 font-medium text-stone-900">{job.jobId.slice(0, 8)}</td>
                    <td className="px-4 py-3">{job.fileName}</td>
                    <td className="px-4 py-3">
                      <div className="font-medium text-stone-900">{job.userName}</div>
                      <div className="text-xs text-stone-500">{job.userEmail}</div>
                    </td>
                    <td className="px-4 py-3 text-xs text-stone-500">{job.capabilityId}</td>
                    <td className="px-4 py-3">{job.outputFormat.toUpperCase()}</td>
                    <td className="px-4 py-3">
                      <span className={`inline-block rounded-full border px-2.5 py-0.5 text-xs font-medium ${STATUS_BADGE_CLASS[job.status] ?? STATUS_BADGE_FALLBACK}`}>
                        {job.status}
                      </span>
                      {job.error && (
                        <p className="mt-1 max-w-48 truncate text-xs text-rose-600" title={job.error}>
                          {job.error}
                        </p>
                      )}
                    </td>
                    <td className="px-4 py-3">
                      {job.backlogAgeSec == null ? (
                        <span className="text-xs text-stone-400">-</span>
                      ) : (
                        <div className="space-y-1">
                          <div className="text-xs text-stone-600">
                            {t("backlogAge", {
                              minutes: formatBacklogMinutes(job.backlogAgeSec),
                            })}
                          </div>
                          <span
                            className={`inline-block rounded-full border px-2 py-0.5 text-[11px] font-medium ${
                              job.stalled
                                ? "border-rose-200 bg-rose-50 text-rose-700"
                                : "border-emerald-200 bg-emerald-50 text-emerald-700"
                            }`}
                          >
                            {job.stalled ? t("stalled") : t("healthy")}
                          </span>
                          {job.stalledReason && (
                            <p className="text-xs text-rose-600">
                              {t(`stalledReason.${job.stalledReason}`)}
                            </p>
                          )}
                        </div>
                      )}
                    </td>
                    <td className="px-4 py-3 text-xs text-stone-500">{formatDate(job.createdAt)}</td>
                    <td className="px-4 py-3">
                      {isCancelable(job) ? (
                        <button
                          type="button"
                          disabled={actingJobId === job.jobId}
                          onClick={() => {
                            void handleCancel(job.jobId);
                          }}
                          className="font-medium text-stone-700 underline underline-offset-2 disabled:cursor-not-allowed disabled:text-stone-400"
                        >
                          {actingJobId === job.jobId && actingType === "cancel"
                            ? t("cancelling")
                            : t("cancel")}
                        </button>
                      ) : isRetryable(job) ? (
                        <button
                          type="button"
                          disabled={actingJobId === job.jobId}
                          onClick={() => {
                            void handleRetry(job.jobId);
                          }}
                          className="font-medium text-coral-700 underline underline-offset-2 disabled:cursor-not-allowed disabled:text-coral-300"
                        >
                          {actingJobId === job.jobId && actingType === "retry"
                            ? t("retrying")
                            : t("retry")}
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
        </div>
      </div>

      {totalPages > 1 && (
        <div className="flex items-center justify-between text-sm text-stone-600">
          <button
            type="button"
            disabled={offset === 0}
            onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
            className="rounded-lg border border-stone-200 px-3 py-1.5 text-sm transition-colors hover:bg-stone-50 disabled:cursor-not-allowed disabled:opacity-40"
          >
            {t("prev")}
          </button>
          <span>{t("pageOf", { current: currentPage, total: totalPages })}</span>
          <button
            type="button"
            disabled={currentPage >= totalPages}
            onClick={() => setOffset(offset + PAGE_SIZE)}
            className="rounded-lg border border-stone-200 px-3 py-1.5 text-sm transition-colors hover:bg-stone-50 disabled:cursor-not-allowed disabled:opacity-40"
          >
            {t("next")}
          </button>
        </div>
      )}
    </div>
  );
}
