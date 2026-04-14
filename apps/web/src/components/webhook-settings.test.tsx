import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import WebhookSettings from "./webhook-settings";
import { IntlWrapper } from "@/test/intl-wrapper";
import * as api from "@/lib/api";

vi.mock("@/lib/api");

const existingWebhook: api.WebhookSubscription = {
  id: "wh-1",
  url: "https://example.com/webhooks/reform-lab",
  eventTypes: ["job.completed", "job.failed"],
  enabled: true,
  hasSecret: true,
  lastDeliveredAt: "2026-04-14T10:00:00Z",
  createdAt: "2026-04-14T09:00:00Z",
  updatedAt: "2026-04-14T10:00:00Z",
  deliveries: [
    {
      id: "wd-1",
      eventId: "evt-1",
      eventType: "job.completed",
      attemptedAt: "2026-04-14T10:00:00Z",
      deliveredAt: "2026-04-14T10:00:01Z",
      statusCode: 202,
    },
    {
      id: "wd-2",
      eventId: "evt-2",
      eventType: "job.failed",
      attemptedAt: "2026-04-14T11:00:00Z",
      statusCode: 500,
      error: "unexpected status 500",
    },
  ],
};

describe("WebhookSettings", () => {
  beforeEach(() => {
    vi.mocked(api.getWebhooks).mockResolvedValue([existingWebhook]);
    vi.mocked(api.createWebhook).mockResolvedValue(existingWebhook);
    vi.mocked(api.updateWebhook).mockResolvedValue(existingWebhook);
    vi.mocked(api.deleteWebhook).mockResolvedValue(undefined);
  });

  it("renders webhook history entries", async () => {
    render(
      <IntlWrapper>
        <WebhookSettings />
      </IntlWrapper>,
    );

    await waitFor(() => {
      expect(
        screen.getByText("https://example.com/webhooks/reform-lab"),
      ).toBeInTheDocument();
    });

    expect(screen.getByText("Historial reciente")).toBeInTheDocument();
    expect(screen.getByText("Entregado")).toBeInTheDocument();
    expect(screen.getByText("Fallido")).toBeInTheDocument();
    expect(screen.getByText("Respuesta HTTP: 202")).toBeInTheDocument();
    expect(screen.getByText("unexpected status 500")).toBeInTheDocument();
  });

  it("creates a webhook from the form", async () => {
    const user = userEvent.setup();
    vi.mocked(api.getWebhooks).mockResolvedValue([]);

    render(
      <IntlWrapper>
        <WebhookSettings />
      </IntlWrapper>,
    );

    await waitFor(() => {
      expect(
        screen.getByText("Todavia no hay suscripciones configuradas."),
      ).toBeInTheDocument();
    });

    await user.type(
      screen.getByLabelText("URL destino"),
      "https://receiver.example/hooks",
    );
    await user.click(screen.getByRole("button", { name: "Crear webhook" }));

    await waitFor(() => {
      expect(api.createWebhook).toHaveBeenCalledWith({
        url: "https://receiver.example/hooks",
        secret: "",
        eventTypes: ["job.completed"],
        enabled: true,
      });
    });
  });

  it("deletes an existing webhook", async () => {
    const user = userEvent.setup();

    render(
      <IntlWrapper>
        <WebhookSettings />
      </IntlWrapper>,
    );

    await waitFor(() => {
      expect(
        screen.getByText("https://example.com/webhooks/reform-lab"),
      ).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Eliminar" }));

    await waitFor(() => {
      expect(api.deleteWebhook).toHaveBeenCalledWith("wh-1");
    });
  });
});