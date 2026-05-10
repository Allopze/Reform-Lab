"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import {
  drainQueuedJobs,
  getAdminEngines,
  getHealthInfo,
  pruneStaleWorkers,
  updateJobIntakeControl,
  type AdminEnginesInfo,
  type HealthInfo,
} from "@/lib/api";
import { useAuth } from "@/lib/auth-context";

const ALERT_STYLES: Record<string, string> = {
  critical: "border-rose-200 bg-rose-50 text-rose-800",
  warning: "border-amber-200 bg-amber-50 text-amber-800",
  info: "border-sky-200 bg-sky-50 text-sky-800",
};

type WorkerEngineDivergence = {
  workerId: string;
  missingOnWorker: string[];
  workerOnly: string[];
};

function formatBytes(value?: number): string {
  if (typeof value !== "number" || Number.isNaN(value) || value < 0) {
    return "-";
  }
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  if (value < 1024 * 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(1)} MB`;
  return `${(value / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

function formatDate(iso?: string): string {
  if (!iso) return "-";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "-";
  return date.toLocaleString("es-ES", {
    year: "numeric",
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export default function AdminSystemPanel() {
  const { user, loading } = useAuth();
  const router = useRouter();
  const t = useTranslations("adminSystem");

  const [health, setHealth] = useState<HealthInfo | null>(null);
  const [engines, setEngines] = useState<AdminEnginesInfo | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionStatus, setActionStatus] = useState<string | null>(null);
  const [actionBusy, setActionBusy] = useState<null | "pause" | "resume" | "drain" | "prune">(null);
  const [pauseReasonDraft, setPauseReasonDraft] = useState("maintenance window");
  const [drainLimitDraft, setDrainLimitDraft] = useState("100");
  const [staleMinutesDraft, setStaleMinutesDraft] = useState("60");

  const loadSystemData = useCallback(async () => {
    const [healthData, enginesData] = await Promise.all([getHealthInfo(), getAdminEngines()]);
    setHealth(healthData);
    setEngines(enginesData);
    setError(null);
    const pauseReason = healthData.runtime.queue.controls?.pauseReason;
    if (typeof pauseReason === "string" && pauseReason.trim().length > 0) {
      setPauseReasonDraft((prev) => (prev.trim().length === 0 ? pauseReason : prev));
    }
  }, []);

  useEffect(() => {
    if (loading) return;
    if (!user || user.role !== "admin") {
      router.replace("/usuario");
      return;
    }

    loadSystemData().catch((err) => {
      setError(err instanceof Error ? err.message : t("loadError"));
    });
  }, [loading, loadSystemData, router, t, user]);

  async function handlePauseIntake() {
    const reason = pauseReasonDraft.trim();
    if (!reason) {
      setActionError(t("intakeReasonRequired"));
      setActionStatus(null);
      return;
    }

    setActionBusy("pause");
    setActionError(null);
    setActionStatus(null);
    try {
      await updateJobIntakeControl({ paused: true, reason });
      await loadSystemData();
      setActionStatus(t("intakePausedStatus"));
    } catch (err) {
      setActionError(err instanceof Error ? err.message : t("supportActionError"));
    } finally {
      setActionBusy(null);
    }
  }

  async function handleResumeIntake() {
    setActionBusy("resume");
    setActionError(null);
    setActionStatus(null);
    try {
      await updateJobIntakeControl({ paused: false });
      await loadSystemData();
      setActionStatus(t("intakeResumedStatus"));
    } catch (err) {
      setActionError(err instanceof Error ? err.message : t("supportActionError"));
    } finally {
      setActionBusy(null);
    }
  }

  async function handleDrainQueue() {
    const parsed = Number.parseInt(drainLimitDraft, 10);
    const limit = Number.isInteger(parsed) && parsed > 0 ? parsed : 100;

    setActionBusy("drain");
    setActionError(null);
    setActionStatus(null);
    try {
      const result = await drainQueuedJobs(limit);
      await loadSystemData();
      setActionStatus(t("drainStatus", { cancelled: result.cancelled, skipped: result.skipped }));
    } catch (err) {
      setActionError(err instanceof Error ? err.message : t("supportActionError"));
    } finally {
      setActionBusy(null);
    }
  }

  async function handlePruneWorkers() {
    const parsed = Number.parseInt(staleMinutesDraft, 10);
    if (!Number.isInteger(parsed) || parsed < 5 || parsed > 10080) {
      setActionError(t("pruneValidation"));
      setActionStatus(null);
      return;
    }

    setActionBusy("prune");
    setActionError(null);
    setActionStatus(null);
    try {
      const result = await pruneStaleWorkers(parsed);
      await loadSystemData();
      setActionStatus(t("pruneStatus", { deleted: result.deleted }));
    } catch (err) {
      setActionError(err instanceof Error ? err.message : t("supportActionError"));
    } finally {
      setActionBusy(null);
    }
  }

  const engineEntries = useMemo(
    () => Object.entries(engines?.engines ?? {}).sort(([a], [b]) => a.localeCompare(b)),
    [engines],
  );
  const capabilityEntries = useMemo(
    () => engines?.capabilities ?? [],
    [engines],
  );
  const workerEngineDivergences = useMemo<WorkerEngineDivergence[]>(() => {
    const apiEngines = health?.runtime.workers.apiEngineAvailability ?? {};
    const apiEngineNames = Object.keys(apiEngines);
    if (!health || apiEngineNames.length === 0) {
      return [];
    }
    return health.runtime.workers.workers
      .map((worker) => {
        const workerEngines = worker.engines ?? {};
        return {
          workerId: worker.id,
          missingOnWorker: apiEngineNames
            .filter((engine) => apiEngines[engine] && workerEngines[engine] === false)
            .sort((a, b) => a.localeCompare(b)),
          workerOnly: Object.entries(workerEngines)
            .filter(([engine, available]) => available && apiEngines[engine] === false)
            .map(([engine]) => engine)
            .sort((a, b) => a.localeCompare(b)),
        };
      })
      .filter((item) => item.missingOnWorker.length > 0 || item.workerOnly.length > 0);
  }, [health]);

  if (loading || (!health && !engines && !error)) {
    return <p className="mt-6 text-sm text-stone-500">{t("loading")}</p>;
  }

  if (error) {
    return <p className="mt-6 rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{error}</p>;
  }

  if (!health || !engines) {
    return null;
  }

  const availableEngines = engineEntries.filter(([, available]) => available).length;
  const totalEngines = engineEntries.length;
  const hasDisabledFlags =
    health.featureFlags.disabledCapabilities.length > 0 ||
    health.featureFlags.disabledEngines.length > 0;
  const storage = health.runtime.storage;
  const queue = health.runtime.queue;
  const database = health.dependencies.database;
  const redis = health.dependencies.redis;
  const alerts = health.alerts ?? [];
  const stalledJobs = typeof queue.stalledJobs === "number" ? queue.stalledJobs : 0;
  const workers = health.runtime.workers;
  const apiEngineMode = workers.apiEngineMode ?? "unknown";
  const queueHistory = queue.history ?? [];
  const queueControls = queue.controls ?? {};
  const intakePaused = queueControls.jobIntakePaused === true;
  const availableCapabilities = engines.availableCapabilities ?? capabilityEntries.filter((item) => item.available).length;
  const totalCapabilities = engines.totalCapabilities ?? capabilityEntries.length;

  return (
    <div className="mt-6 space-y-6">
      <section className="grid gap-4 md:grid-cols-2">
        <div className="rounded-2xl border border-stone-200 bg-white px-5 py-4 shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
          <p className="text-xs font-medium uppercase tracking-[0.12em] text-stone-500">{t("statusLabel")}</p>
          <p className="mt-2 text-2xl font-semibold text-stone-900">
            {health.status === "ok" ? t("statusOk") : t("statusUnknown")}
          </p>
          <p className="mt-2 text-sm text-stone-600">
            {t("enginesSummary", { available: availableEngines, total: totalEngines })}
          </p>
        </div>

        <div className="rounded-2xl border border-stone-200 bg-white px-5 py-4 shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
          <p className="text-xs font-medium uppercase tracking-[0.12em] text-stone-500">{t("retentionTitle")}</p>
          <p className="mt-2 text-sm text-stone-700">
            {t("defaultTTL", { hours: health.retention.artifactTTLHours })}
          </p>
          <div className="mt-3">
            <p className="text-xs text-stone-500">{t("familyTTLTitle")}</p>
            <div className="mt-1 space-y-1">
              {Object.entries(health.retention.artifactTTLHoursByFamily).length === 0 ? (
                <p className="text-xs text-stone-400">-</p>
              ) : (
                Object.entries(health.retention.artifactTTLHoursByFamily).map(([family, hours]) => (
                  <p key={family} className="text-xs text-stone-600">{family}: {hours}h</p>
                ))
              )}
            </div>
          </div>
        </div>
      </section>

      <section className="grid gap-4 md:grid-cols-2">
        <div className="rounded-2xl border border-stone-200 bg-white px-5 py-4 shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
          <h2 className="text-base font-semibold text-stone-900">{t("queueTitle")}</h2>
          <div className="mt-3 space-y-1 text-sm text-stone-600">
            <p>{t("queueMode", { mode: queue.mode })}</p>
            <p>{t("intakeStatus", { status: intakePaused ? t("intakePaused") : t("intakeRunning") })}</p>
            {intakePaused && (
              <p className="text-xs text-amber-700">{t("intakePauseReason", { reason: queueControls.pauseReason ?? "-" })}</p>
            )}
            <p>{t("workerConcurrency", { count: queue.workerConcurrency })}</p>
            <p>{t("queuedJobs", { count: queue.queuedJobs })}</p>
            <p>{t("runningJobs", { count: queue.runningJobs })}</p>
            <p>{t("stalledJobs", { count: queue.stalledJobs })}</p>
            <p className="text-xs text-stone-500">
              {t("stalledBreakdown", {
                queued: queue.stalledQueuedJobs,
                running: queue.stalledRunningJobs,
              })}
            </p>
            {stalledJobs > 0 && (
              <p className="text-xs text-amber-700">{t("stalledHint")}</p>
            )}
          </div>
        </div>

        <div className="rounded-2xl border border-stone-200 bg-white px-5 py-4 shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
          <h2 className="text-base font-semibold text-stone-900">{t("storageTitle")}</h2>
          <div className="mt-3 space-y-1 text-sm text-stone-600">
            <p>{t("storageStatus", { status: storage.status })}</p>
            <p>{t("storageFree", { value: formatBytes(storage.freeBytes) })}</p>
            <p>{t("storageTotal", { value: formatBytes(storage.totalBytes) })}</p>
            <p>{t("storageUsed", { value: typeof storage.usedPercent === "number" ? storage.usedPercent.toFixed(1) : "-" })}</p>
          </div>
        </div>
      </section>

      <section className="rounded-2xl border border-stone-200 bg-white px-5 py-4 shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
        <h2 className="text-base font-semibold text-stone-900">{t("supportTitle")}</h2>
        <p className="mt-2 text-sm text-stone-500">{t("supportDescription")}</p>

        <div className="mt-4 grid gap-4 lg:grid-cols-3">
          <div className="rounded-xl border border-stone-200 p-3">
            <p className="text-sm font-medium text-stone-900">{t("intakeControlTitle")}</p>
            <input
              value={pauseReasonDraft}
              onChange={(event) => setPauseReasonDraft(event.target.value)}
              placeholder={t("intakeReasonPlaceholder")}
              className="mt-2 w-full rounded-md border border-stone-300 px-2 py-1.5 text-sm text-stone-800 focus:border-stone-500 focus:outline-none"
            />
            <div className="mt-2 flex gap-2">
              <button
                type="button"
                onClick={handlePauseIntake}
                disabled={actionBusy !== null || intakePaused}
                className="rounded-md border border-stone-300 px-3 py-1.5 text-xs font-medium text-stone-700 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {t("pauseIntake")}
              </button>
              <button
                type="button"
                onClick={handleResumeIntake}
                disabled={actionBusy !== null || !intakePaused}
                className="rounded-md border border-stone-300 px-3 py-1.5 text-xs font-medium text-stone-700 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {t("resumeIntake")}
              </button>
            </div>
          </div>

          <div className="rounded-xl border border-stone-200 p-3">
            <p className="text-sm font-medium text-stone-900">{t("drainTitle")}</p>
            <input
              value={drainLimitDraft}
              onChange={(event) => setDrainLimitDraft(event.target.value)}
              placeholder="100"
              inputMode="numeric"
              className="mt-2 w-full rounded-md border border-stone-300 px-2 py-1.5 text-sm text-stone-800 focus:border-stone-500 focus:outline-none"
            />
            <button
              type="button"
              onClick={handleDrainQueue}
              disabled={actionBusy !== null}
              className="mt-2 rounded-md border border-stone-300 px-3 py-1.5 text-xs font-medium text-stone-700 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {t("drainQueued")}
            </button>
          </div>

          <div className="rounded-xl border border-stone-200 p-3">
            <p className="text-sm font-medium text-stone-900">{t("pruneTitle")}</p>
            <input
              value={staleMinutesDraft}
              onChange={(event) => setStaleMinutesDraft(event.target.value)}
              placeholder="60"
              inputMode="numeric"
              className="mt-2 w-full rounded-md border border-stone-300 px-2 py-1.5 text-sm text-stone-800 focus:border-stone-500 focus:outline-none"
            />
            <button
              type="button"
              onClick={handlePruneWorkers}
              disabled={actionBusy !== null}
              className="mt-2 rounded-md border border-stone-300 px-3 py-1.5 text-xs font-medium text-stone-700 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {t("pruneWorkers")}
            </button>
          </div>
        </div>

        {actionStatus && (
          <p className="mt-3 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-xs text-emerald-700">{actionStatus}</p>
        )}
        {actionError && (
          <p className="mt-3 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-xs text-rose-700">{actionError}</p>
        )}
      </section>

      <section className="grid gap-4 lg:grid-cols-2">
        <div className="rounded-2xl border border-stone-200 bg-white px-5 py-4 shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
          <h2 className="text-base font-semibold text-stone-900">{t("historyTitle")}</h2>
          <div className="mt-3 overflow-x-auto">
            <table className="w-full border-collapse text-left text-sm">
              <thead className="bg-stone-50 text-xs font-medium text-stone-500">
                <tr>
                  <th className="px-3 py-2">{t("windowHeader")}</th>
                  <th className="px-3 py-2">{t("enqueuedHeader")}</th>
                  <th className="px-3 py-2">{t("failedHeader")}</th>
                  <th className="px-3 py-2">{t("latencyHeader")}</th>
                </tr>
              </thead>
              <tbody>
                {queueHistory.map((point) => (
                  <tr key={point.window} className="border-t border-stone-200 text-stone-700">
                    <td className="px-3 py-2 font-medium text-stone-900">{point.window}</td>
                    <td className="px-3 py-2">{point.enqueuedJobs}</td>
                    <td className="px-3 py-2">{point.failedJobs}</td>
                    <td className="px-3 py-2">{point.averageLatencySec.toFixed(1)}s</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>

        <div className="rounded-2xl border border-stone-200 bg-white px-5 py-4 shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
          <h2 className="text-base font-semibold text-stone-900">{t("workersTitle")}</h2>
          <p className="mt-2 text-sm text-stone-500">{t("workersSummary", { count: workers.count })}</p>
          <div className="mt-3 rounded-lg border border-stone-200 bg-stone-50 px-3 py-2 text-xs text-stone-600">
            <p className="font-medium text-stone-800">{t("apiEngineMode", { mode: apiEngineMode })}</p>
            {workerEngineDivergences.length === 0 ? (
              <p className="mt-1">{t("engineParityOk")}</p>
            ) : (
              <div className="mt-2 space-y-1">
                {workerEngineDivergences.map((item) => (
                  <p key={item.workerId} className="text-amber-700">
                    {t("engineDivergence", {
                      worker: item.workerId,
                      missing: item.missingOnWorker.length > 0 ? item.missingOnWorker.join(", ") : "-",
                      extra: item.workerOnly.length > 0 ? item.workerOnly.join(", ") : "-",
                    })}
                  </p>
                ))}
              </div>
            )}
          </div>
          <div className="mt-3 space-y-3">
            {workers.workers.length === 0 ? (
              <p className="text-sm text-stone-500">{t("noWorkers")}</p>
            ) : (
              workers.workers.map((worker) => (
                <article key={worker.id} className="rounded-xl border border-stone-200 px-4 py-3">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <p className="font-medium text-stone-900">{worker.id}</p>
                    <span className="text-xs text-stone-500">
                      {worker.runtimeMode} · {worker.queueMode}
                    </span>
                  </div>
                  <div className="mt-2 space-y-1 text-sm text-stone-600">
                    <p>{t("heartbeatAt", { value: formatDate(worker.lastHeartbeatAt) })}</p>
                    <p>{t("workerLastTask", { value: worker.lastTaskType ?? "-" })}</p>
                    <p>{t("workerStatus", { value: worker.lastTaskStatus })}</p>
                    {worker.lastJobId && <p>{t("workerJob", { value: worker.lastJobId.slice(0, 8) })}</p>}
                    {worker.lastError && <p className="text-rose-600">{worker.lastError}</p>}
                  </div>
                  {worker.engines && Object.keys(worker.engines).length > 0 && (
                    <div className="mt-3 flex flex-wrap gap-1.5 border-t border-stone-200 pt-3">
                      {Object.entries(worker.engines)
                        .sort(([a], [b]) => a.localeCompare(b))
                        .map(([engine, available]) => (
                          <span
                            key={engine}
                            className={`rounded-full border px-2 py-0.5 text-[11px] font-medium ${
                              available
                                ? "border-emerald-200 bg-emerald-50 text-emerald-700"
                                : "border-rose-200 bg-rose-50 text-rose-700"
                            }`}
                          >
                            {engine}
                          </span>
                        ))}
                    </div>
                  )}
                  {worker.recentFailures.length > 0 && (
                    <div className="mt-3 space-y-1 border-t border-stone-200 pt-3 text-xs text-stone-600">
                      {worker.recentFailures.map((failure) => (
                        <p key={failure.id}>
                          {failure.taskType ?? "task"}: {failure.error}
                        </p>
                      ))}
                    </div>
                  )}
                </article>
              ))
            )}
          </div>
        </div>
      </section>

      <section className="rounded-2xl border border-stone-200 bg-white px-5 py-4 shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
        <h2 className="text-base font-semibold text-stone-900">{t("alertsTitle")}</h2>
        {alerts.length === 0 ? (
          <p className="mt-3 text-sm text-stone-500">{t("noActiveAlerts")}</p>
        ) : (
          <div className="mt-3 space-y-2">
            {alerts.map((alert) => (
              <article
                key={alert.code}
                className={`rounded-lg border px-3 py-2 ${ALERT_STYLES[alert.severity] ?? ALERT_STYLES.info}`}
              >
                <div className="flex items-center justify-between gap-2">
                  <p className="text-sm font-medium">{alert.summary}</p>
                  <span className="text-[11px] font-semibold uppercase tracking-[0.08em]">
                    {t(`severity.${alert.severity}`)}
                  </span>
                </div>
                <p className="mt-1 text-xs">{alert.description}</p>
              </article>
            ))}
          </div>
        )}
      </section>

      <section className="overflow-hidden rounded-2xl border border-stone-200 bg-white shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
        <div className="border-b border-stone-200 px-5 py-4">
          <h2 className="text-base font-semibold text-stone-900">{t("dependenciesTitle")}</h2>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full border-collapse text-left text-sm">
            <thead className="bg-stone-50 text-xs font-medium text-stone-500">
              <tr>
                <th className="px-5 py-3">{t("dependencyHeader")}</th>
                <th className="px-5 py-3">{t("statusHeader")}</th>
                <th className="px-5 py-3">{t("latencyHeader")}</th>
              </tr>
            </thead>
            <tbody>
              <tr className="border-t border-stone-200 text-stone-700">
                <td className="px-5 py-3 font-medium text-stone-900">database</td>
                <td className="px-5 py-3">{database.status}</td>
                <td className="px-5 py-3">
                  {typeof database.latencyMs === "number" ? `${database.latencyMs.toFixed(1)}ms` : "-"}
                </td>
              </tr>
              <tr className="border-t border-stone-200 text-stone-700">
                <td className="px-5 py-3 font-medium text-stone-900">redis</td>
                <td className="px-5 py-3">{redis.status}</td>
                <td className="px-5 py-3">
                  {typeof redis.latencyMs === "number" ? `${redis.latencyMs.toFixed(1)}ms` : "-"}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section className="rounded-2xl border border-stone-200 bg-white px-5 py-4 shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
        <h2 className="text-base font-semibold text-stone-900">{t("featureFlagsTitle")}</h2>
        {!hasDisabledFlags ? (
          <p className="mt-3 text-sm text-stone-500">{t("noFeatureFlags")}</p>
        ) : (
          <div className="mt-3 space-y-2">
            {health.featureFlags.disabledEngines.length > 0 && (
              <p className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">
                {t("disabledEngines")}: {health.featureFlags.disabledEngines.join(", ")}
              </p>
            )}
            {health.featureFlags.disabledCapabilities.length > 0 && (
              <p className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">
                {t("disabledCapabilities")}: {health.featureFlags.disabledCapabilities.join(", ")}
              </p>
            )}
          </div>
        )}
      </section>

      <section className="overflow-hidden rounded-2xl border border-stone-200 bg-white shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
        <div className="border-b border-stone-200 px-5 py-4">
          <h2 className="text-base font-semibold text-stone-900">{t("enginesTitle")}</h2>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full border-collapse text-left text-sm">
            <thead className="bg-stone-50 text-xs font-medium text-stone-500">
              <tr>
                <th className="px-5 py-3">{t("engineHeader")}</th>
                <th className="px-5 py-3">{t("availabilityHeader")}</th>
              </tr>
            </thead>
            <tbody>
              {engineEntries.length === 0 ? (
                <tr>
                  <td colSpan={2} className="px-5 py-8 text-sm text-stone-500">-</td>
                </tr>
              ) : (
                engineEntries.map(([name, available]) => (
                  <tr key={name} className="border-t border-stone-200 text-stone-700">
                    <td className="px-5 py-3 font-medium text-stone-900">{name}</td>
                    <td className="px-5 py-3">
                      <span
                        className={`inline-block rounded-full border px-2.5 py-0.5 text-xs font-medium ${
                          available
                            ? "border-emerald-200 bg-emerald-50 text-emerald-700"
                            : "border-rose-200 bg-rose-50 text-rose-700"
                        }`}
                      >
                        {available ? t("available") : t("unavailable")}
                      </span>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>

      <section className="overflow-hidden rounded-2xl border border-stone-200 bg-white shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
        <div className="border-b border-stone-200 px-5 py-4">
          <h2 className="text-base font-semibold text-stone-900">{t("capabilitiesTitle")}</h2>
          <p className="mt-1 text-sm text-stone-500">
            {t("capabilitiesSummary", { available: availableCapabilities, total: totalCapabilities })}
          </p>
        </div>

        <div className="max-h-96 overflow-auto">
          <table className="w-full border-collapse text-left text-sm">
            <thead className="sticky top-0 bg-stone-50 text-xs font-medium text-stone-500">
              <tr>
                <th className="px-5 py-3">{t("capabilityHeader")}</th>
                <th className="px-5 py-3">{t("engineHeader")}</th>
                <th className="px-5 py-3">{t("familyHeader")}</th>
                <th className="px-5 py-3">{t("availabilityHeader")}</th>
                <th className="px-5 py-3">{t("reasonHeader")}</th>
              </tr>
            </thead>
            <tbody>
              {capabilityEntries.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-5 py-8 text-sm text-stone-500">-</td>
                </tr>
              ) : (
                capabilityEntries.map((capability) => (
                  <tr key={capability.id} className="border-t border-stone-200 text-stone-700">
                    <td className="px-5 py-3">
                      <p className="font-medium text-stone-900">{capability.displayName}</p>
                      <p className="text-xs text-stone-500">{capability.id}</p>
                    </td>
                    <td className="px-5 py-3 font-medium text-stone-900">{capability.engine}</td>
                    <td className="px-5 py-3 text-xs text-stone-600">
                      {capability.family} · {capability.operationType} · {capability.targetFormat}
                    </td>
                    <td className="px-5 py-3">
                      <span
                        className={`inline-block rounded-full border px-2.5 py-0.5 text-xs font-medium ${
                          capability.available
                            ? "border-emerald-200 bg-emerald-50 text-emerald-700"
                            : "border-rose-200 bg-rose-50 text-rose-700"
                        }`}
                      >
                        {capability.available ? t("available") : t("unavailable")}
                      </span>
                    </td>
                    <td className="px-5 py-3 text-xs text-stone-600">{t(`reason.${capability.reason}`)}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
