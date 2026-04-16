"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import {
  exportAdminAuditCSV,
  getAdminAudit,
  type AdminAuditEvent,
  type AdminAuditFilter,
  type AdminAuditPage,
} from "@/lib/api";
import { useAuth } from "@/lib/auth-context";

const PAGE_SIZE = 40;

function formatDate(iso: string): string {
  try {
    return new Intl.DateTimeFormat("es", {
      day: "2-digit",
      month: "short",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    }).format(new Date(iso));
  } catch {
    return iso;
  }
}

function detailsPreview(event: AdminAuditEvent): string {
  if (!event.details || Object.keys(event.details).length === 0) {
    return "-";
  }
  try {
    return JSON.stringify(event.details);
  } catch {
    return "-";
  }
}

export default function AdminAuditTable() {
  const { user, loading } = useAuth();
  const router = useRouter();
  const t = useTranslations("adminAudit");

  const [page, setPage] = useState<AdminAuditPage | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [eventType, setEventType] = useState("");
  const [group, setGroup] = useState<"" | "admin">("admin");
  const [offset, setOffset] = useState(0);
  const [exporting, setExporting] = useState(false);

  const fetchAudit = useCallback(async () => {
    const filter: AdminAuditFilter = {
      limit: PAGE_SIZE,
      offset,
    };
    if (eventType.trim()) {
      filter.eventType = eventType.trim();
    }
    if (group) {
      filter.group = group;
    }

    try {
      const data = await getAdminAudit(filter);
      setPage(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("loadError"));
    }
  }, [eventType, group, offset, t]);

  useEffect(() => {
    if (loading) return;
    if (!user || user.role !== "admin") {
      router.replace("/usuario");
      return;
    }
    fetchAudit();
  }, [fetchAudit, loading, router, user]);

  const totalPages = useMemo(() => {
    if (!page) return 1;
    return Math.max(1, Math.ceil(page.total / PAGE_SIZE));
  }, [page]);
  const currentPage = Math.floor(offset / PAGE_SIZE) + 1;

  async function handleExport() {
    setExporting(true);
    try {
      await exportAdminAuditCSV({
        eventType: eventType.trim() || undefined,
        group: group || undefined,
        limit: 5000,
      });
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("exportError"));
    } finally {
      setExporting(false);
    }
  }

  if (loading || !page) {
    return <p className="mt-6 text-sm text-stone-500">{t("loading")}</p>;
  }

  return (
    <div className="mt-6 space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <select
          value={group}
          onChange={(e) => {
            const value = e.target.value;
            setGroup(value === "admin" ? "admin" : "");
            setOffset(0);
          }}
          className="h-9 rounded-lg border border-stone-200 bg-white px-3 text-sm text-stone-700"
        >
          <option value="admin">{t("groupAdmin")}</option>
          <option value="">{t("groupAll")}</option>
        </select>

        <input
          type="text"
          value={eventType}
          onChange={(e) => {
            setEventType(e.target.value);
            setOffset(0);
          }}
          placeholder={t("eventTypePlaceholder")}
          className="h-9 w-64 rounded-lg border border-stone-200 bg-white px-3 text-sm text-stone-700 placeholder:text-stone-400"
        />

        <button
          type="button"
          onClick={() => {
            void handleExport();
          }}
          disabled={exporting}
          className="ml-auto rounded-lg border border-stone-200 px-3 py-1.5 text-sm font-medium text-stone-700 transition-colors hover:bg-stone-50 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {exporting ? t("exporting") : t("export")}
        </button>
      </div>

      {error && <p className="text-sm text-rose-600">{error}</p>}

      <div className="overflow-hidden rounded-2xl border border-stone-200 bg-white shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
        <div className="overflow-x-auto">
          <table className="w-full min-w-225 border-collapse text-left text-sm">
            <thead className="bg-stone-50 text-xs font-medium text-stone-500">
              <tr>
                <th className="px-4 py-3">{t("eventHeader")}</th>
                <th className="px-4 py-3">{t("resourceHeader")}</th>
                <th className="px-4 py-3">{t("detailsHeader")}</th>
                <th className="px-4 py-3">{t("createdHeader")}</th>
              </tr>
            </thead>
            <tbody>
              {page.events.length === 0 ? (
                <tr>
                  <td colSpan={4} className="px-4 py-8 text-sm text-stone-500">
                    {t("empty")}
                  </td>
                </tr>
              ) : (
                page.events.map((event) => (
                  <tr key={event.id} className="border-t border-stone-200 text-stone-700">
                    <td className="px-4 py-3 font-medium text-stone-900">{event.eventType}</td>
                    <td className="px-4 py-3 text-xs text-stone-500">
                      <div>{event.fileId ? `file:${event.fileId.slice(0, 8)}` : "-"}</div>
                      <div>{event.jobId ? `job:${event.jobId.slice(0, 8)}` : "-"}</div>
                    </td>
                    <td className="px-4 py-3 text-xs text-stone-500">
                      <p className="max-w-120 truncate" title={detailsPreview(event)}>
                        {detailsPreview(event)}
                      </p>
                    </td>
                    <td className="px-4 py-3 text-xs text-stone-500">{formatDate(event.createdAt)}</td>
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
