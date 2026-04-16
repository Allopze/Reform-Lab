"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import {
  getAdminEngines,
  getHealthInfo,
  type AdminEnginesInfo,
  type HealthInfo,
} from "@/lib/api";
import { useAuth } from "@/lib/auth-context";

const ALERT_STYLES: Record<string, string> = {
  critical: "border-rose-200 bg-rose-50 text-rose-800",
  warning: "border-amber-200 bg-amber-50 text-amber-800",
  info: "border-sky-200 bg-sky-50 text-sky-800",
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

export default function AdminSystemPanel() {
  const { user, loading } = useAuth();
  const router = useRouter();
  const t = useTranslations("adminSystem");

  const [health, setHealth] = useState<HealthInfo | null>(null);
  const [engines, setEngines] = useState<AdminEnginesInfo | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (loading) return;
    if (!user || user.role !== "admin") {
      router.replace("/usuario");
      return;
    }

    Promise.all([getHealthInfo(), getAdminEngines()])
      .then(([healthData, enginesData]) => {
        setHealth(healthData);
        setEngines(enginesData);
        setError(null);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : t("loadError"));
      });
  }, [loading, router, t, user]);

  const engineEntries = useMemo(
    () => Object.entries(engines?.engines ?? {}).sort(([a], [b]) => a.localeCompare(b)),
    [engines],
  );

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
            <p>{t("workerConcurrency", { count: queue.workerConcurrency })}</p>
            <p>{t("queuedJobs", { count: queue.queuedJobs })}</p>
            <p>{t("runningJobs", { count: queue.runningJobs })}</p>
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
    </div>
  );
}
