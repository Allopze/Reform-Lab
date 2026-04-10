export const DEFAULT_FOOTER_MESSAGE =
  "© 2026 Reform Lab — Detección real de formato y conversión segura";

export const FOOTER_MESSAGE_UPDATED_EVENT = "reform:footer-message-updated";

export function emitFooterMessageUpdated(message: string): void {
  if (typeof window === "undefined") return;
  window.dispatchEvent(
    new CustomEvent<string>(FOOTER_MESSAGE_UPDATED_EVENT, {
      detail: message,
    })
  );
}