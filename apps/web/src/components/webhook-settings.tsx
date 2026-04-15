"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import {
  createWebhook,
  deleteWebhook,
  getWebhooks,
  updateWebhook,
  type WebhookDraft,
  type WebhookSubscription,
} from "@/lib/api";

const SUPPORTED_EVENTS = ["job.completed", "job.failed"] as const;

function formatDate(value?: string): string | null {
  if (!value) {
    return null;
  }

  return new Intl.DateTimeFormat("es-ES", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

function buildDraft(subscription?: WebhookSubscription): WebhookDraft {
  return {
    url: subscription?.url ?? "",
    secret: "",
    eventTypes: subscription?.eventTypes ?? ["job.completed"],
    enabled: subscription?.enabled ?? true,
  };
}

function isDeliverySuccessful(
  delivery: NonNullable<WebhookSubscription["deliveries"]>[number],
) {
  return Boolean(delivery.deliveredAt) && (delivery.statusCode ?? 0) < 400;
}

export default function WebhookSettings() {
  const t = useTranslations("webhookSettings");
  const tCommon = useTranslations("common");
  const [items, setItems] = useState<WebhookSubscription[]>([]);
  const [draft, setDraft] = useState<WebhookDraft>(() => buildDraft());
  const [editingId, setEditingId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [status, setStatus] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    const nextItems = await getWebhooks();
    setItems(nextItems);
  }, []);

  useEffect(() => {
    refresh()
      .catch((err) => {
        setError(err instanceof Error ? err.message : t("loadError"));
      })
      .finally(() => setLoading(false));
  }, [refresh, t]);

  const selectedEvents = useMemo(
    () => new Set(draft.eventTypes),
    [draft.eventTypes],
  );

  async function handleSubmit() {
    setSaving(true);
    setError(null);
    setStatus(null);

    try {
      if (editingId) {
        await updateWebhook(editingId, draft);
      } else {
        await createWebhook(draft);
      }
      await refresh();
      setDraft(buildDraft());
      setEditingId(null);
      setStatus(editingId ? t("updated") : t("created"));
    } catch (err) {
      setError(err instanceof Error ? err.message : t("saveError"));
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete(webhookId: string) {
    setDeletingId(webhookId);
    setError(null);
    setStatus(null);

    try {
      await deleteWebhook(webhookId);
      await refresh();
      if (editingId === webhookId) {
        setEditingId(null);
        setDraft(buildDraft());
      }
      setStatus(t("deleted"));
    } catch (err) {
      setError(err instanceof Error ? err.message : t("deleteError"));
    } finally {
      setDeletingId(null);
    }
  }

  return (
    <section className="rounded-2xl border border-stone-200 bg-white px-5 py-4 shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-base font-semibold text-stone-900">
            {t("title")}
          </h2>
          <p className="mt-1 text-sm text-stone-500">{t("description")}</p>
        </div>
        {editingId ? (
          <button
            type="button"
            onClick={() => {
              setEditingId(null);
              setDraft(buildDraft());
              setError(null);
              setStatus(null);
            }}
            className="text-sm font-medium text-stone-600 underline underline-offset-2"
          >
            {tCommon("discard")}
          </button>
        ) : null}
      </div>

      <div className="mt-4 grid gap-4">
        <div className="space-y-3">
          {loading ? (
            <p className="text-sm text-stone-500">{t("loading")}</p>
          ) : items.length === 0 ? (
            <p className="rounded-lg border border-stone-200 bg-stone-50 px-4 py-3 text-sm text-stone-500">
              {t("empty")}
            </p>
          ) : (
            items.map((item) => {
              const lastDelivered = formatDate(item.lastDeliveredAt);
              return (
                <div
                  key={item.id}
                  className="rounded-lg border border-stone-200 px-4 py-3"
                >
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <p className="font-medium text-stone-900">{item.url}</p>
                      <p className="mt-1 text-sm text-stone-500">
                        {item.enabled ? t("enabled") : t("disabled")}
                        {item.hasSecret ? ` · ${t("hasSecret")}` : ` · ${t("noSecret")}`}
                      </p>
                    </div>
                    <div className="flex gap-3 text-sm">
                      <button
                        type="button"
                        onClick={() => {
                          setEditingId(item.id);
                          setDraft(buildDraft(item));
                          setError(null);
                          setStatus(null);
                        }}
                        className="font-medium text-stone-700 underline underline-offset-2"
                      >
                        {t("edit")}
                      </button>
                      <button
                        type="button"
                        onClick={() => {
                          void handleDelete(item.id);
                        }}
                        className="font-medium text-rose-700 underline underline-offset-2"
                      >
                        {deletingId === item.id ? tCommon("loading") : t("delete")}
                      </button>
                    </div>
                  </div>

                  <div className="mt-3 flex flex-wrap gap-2">
                    {item.eventTypes.map((eventType) => (
                      <span
                        key={eventType}
                        className="rounded-md border border-stone-300 px-2 py-1 text-xs text-stone-600"
                      >
                        {t(`eventLabel.${eventType}`)}
                      </span>
                    ))}
                  </div>

                  <div className="mt-3 space-y-1 text-sm text-stone-500">
                    <p>
                      {lastDelivered
                        ? t("lastDelivered", { date: lastDelivered })
                        : t("neverDelivered")}
                    </p>
                    {item.lastError ? (
                      <p className="text-rose-700">
                        {t("lastError", { error: item.lastError })}
                      </p>
                    ) : null}
                  </div>

                  {item.deliveries && item.deliveries.length > 0 ? (
                    <div className="mt-4 rounded-lg border border-stone-200 bg-stone-50 px-3 py-3">
                      <p className="text-sm font-medium text-stone-800">
                        {t("historyTitle")}
                      </p>
                      <div className="mt-3 space-y-2">
                        {item.deliveries.map((delivery) => {
                          const attemptedAt = formatDate(delivery.attemptedAt);
                          const delivered = isDeliverySuccessful(delivery);

                          return (
                            <div
                              key={delivery.id}
                              className="rounded-md border border-stone-200 bg-white px-3 py-2"
                            >
                              <div className="flex flex-wrap items-center justify-between gap-2">
                                <span className="text-sm font-medium text-stone-800">
                                  {t(`eventLabel.${delivery.eventType}`)}
                                </span>
                                <span
                                  className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                                    delivered
                                      ? "bg-emerald-100 text-emerald-700"
                                      : "bg-rose-100 text-rose-700"
                                  }`}
                                >
                                  {delivered
                                    ? t("deliverySuccess")
                                    : t("deliveryFailure")}
                                </span>
                              </div>
                              <div className="mt-1 space-y-1 text-xs text-stone-500">
                                {attemptedAt ? (
                                  <p>
                                    {t("deliveryAttempted", {
                                      date: attemptedAt,
                                    })}
                                  </p>
                                ) : null}
                                {typeof delivery.statusCode === "number" ? (
                                  <p>
                                    {t("deliveryStatusCode", {
                                      status: delivery.statusCode,
                                    })}
                                  </p>
                                ) : null}
                                {delivery.error ? (
                                  <p className="text-rose-700">{delivery.error}</p>
                                ) : null}
                              </div>
                            </div>
                          );
                        })}
                      </div>
                    </div>
                  ) : null}
                </div>
              );
            })
          )}
        </div>

        <div className="rounded-lg border border-stone-200 bg-stone-50 px-4 py-4">
          <div className="space-y-4">
            <label className="block">
              <span className="mb-1.5 block text-sm font-medium text-stone-700">
                {t("urlLabel")}
              </span>
              <input
                type="url"
                value={draft.url}
                onChange={(event) => {
                  setDraft((current) => ({ ...current, url: event.target.value }));
                }}
                placeholder={t("urlPlaceholder")}
                className="h-11 w-full rounded-lg border border-stone-300 bg-white px-3 text-sm text-stone-900"
              />
            </label>

            <label className="block">
              <span className="mb-1.5 block text-sm font-medium text-stone-700">
                {t("secretLabel")}
              </span>
              <input
                type="text"
                value={draft.secret ?? ""}
                onChange={(event) => {
                  setDraft((current) => ({
                    ...current,
                    secret: event.target.value,
                  }));
                }}
                placeholder={t("secretPlaceholder")}
                className="h-11 w-full rounded-lg border border-stone-300 bg-white px-3 text-sm text-stone-900"
              />
            </label>

            <div>
              <p className="mb-2 text-sm font-medium text-stone-700">
                {t("eventsLabel")}
              </p>
              <div className="space-y-2">
                {SUPPORTED_EVENTS.map((eventType) => (
                  <label
                    key={eventType}
                    className="flex items-center gap-3 text-sm text-stone-700"
                  >
                    <input
                      type="checkbox"
                      checked={selectedEvents.has(eventType)}
                      onChange={(event) => {
                        setDraft((current) => ({
                          ...current,
                          eventTypes: event.target.checked
                            ? [...current.eventTypes, eventType]
                            : current.eventTypes.filter(
                                (currentEvent) => currentEvent !== eventType,
                              ),
                        }));
                      }}
                    />
                    <span>{t(`eventLabel.${eventType}`)}</span>
                  </label>
                ))}
              </div>
            </div>

            <label className="flex items-center gap-3 text-sm text-stone-700">
              <input
                type="checkbox"
                checked={draft.enabled ?? true}
                onChange={(event) => {
                  setDraft((current) => ({
                    ...current,
                    enabled: event.target.checked,
                  }));
                }}
              />
              <span>{t("enabledLabel")}</span>
            </label>

            <button
              type="button"
              onClick={() => {
                void handleSubmit();
              }}
              disabled={saving || !draft.url || draft.eventTypes.length === 0}
              className="inline-flex h-10 items-center rounded-lg bg-coral-500 px-4 text-sm font-medium text-white transition-colors duration-150 hover:bg-coral-600 disabled:cursor-not-allowed disabled:bg-coral-200"
            >
              {saving
                ? tCommon("saving")
                : editingId
                  ? t("saveChanges")
                  : t("create")}
            </button>

            {status ? (
              <p className="rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                {status}
              </p>
            ) : null}

            {error ? (
              <p className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
                {error}
              </p>
            ) : null}
          </div>
        </div>
      </div>
    </section>
  );
}