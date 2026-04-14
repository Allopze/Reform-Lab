import { expect, test, type Page, type Route } from "@playwright/test";

const ISO_NOW = "2026-04-14T10:00:00Z";
const DOCX_MIME =
  "application/vnd.openxmlformats-officedocument.wordprocessingml.document";

function jsonResponse(route: Route, body: unknown, status = 200) {
  return route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(body),
  });
}

function uploadPolicy() {
  return {
    guestMaxBytes: 50 * 1024 * 1024,
    registeredMaxBytes: 100 * 1024 * 1024,
    effectiveMaxBytes: 50 * 1024 * 1024,
    viewerType: "guest",
    absoluteMaxBytes: 500 * 1024 * 1024,
  };
}

function adminOverview() {
  return {
    totalUsers: 4,
    totalFiles: 8,
    totalJobs: 12,
    queuedJobs: 1,
    runningJobs: 1,
    succeededJobs: 9,
    failedJobs: 1,
    cancelledJobs: 0,
    successRatePct: 75,
    averageDurationSec: 12.3,
    availableEngines: 2,
    totalEngines: 2,
    unavailableEngines: [],
    engineUsage: [],
    recentAudit: [],
    recentJobs: [],
  };
}

function smtpSettings() {
  return {
    host: "smtp.example.com",
    port: 587,
    user: "bot@example.com",
    password: "",
    from: "bot@example.com",
    use_tls: true,
    source: "admin",
  };
}

async function mockBatchConversionApi(page: Page) {
  let uploadCount = 0;

  await page.route("**/api/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const method = request.method();

    if (path === "/api/auth/me") {
      return jsonResponse(route, { error: "invalid token" }, 401);
    }

    if (path === "/api/footer-message") {
      return jsonResponse(route, { message: "Footer E2E" });
    }

    if (path === "/api/upload-policy") {
      return jsonResponse(route, uploadPolicy());
    }

    if (path === "/api/files" && method === "POST") {
      uploadCount += 1;
      return jsonResponse(
        route,
        {
          id: `file-${uploadCount}`,
          originalName: uploadCount === 1 ? "contrato-a.pdf" : "contrato-b.pdf",
          size: 2048,
          detectedFormat: {
            mimeType: "application/pdf",
            family: "pdf",
            extension: "pdf",
          },
          uploadedAt: ISO_NOW,
        },
        201,
      );
    }

    if (path === "/api/files/capabilities/batch" && method === "POST") {
      return jsonResponse(route, {
        capabilities: [
          {
            id: "pdf-to-docx",
            displayName: "PDF a Word",
            presentationOrder: 1,
            targetFormat: "docx",
            operationType: "convert",
            timeoutSeconds: 60,
          },
        ],
      });
    }

    if (path === "/api/conversions/batch" && method === "POST") {
      return jsonResponse(
        route,
        {
          jobs: [
            {
              id: "job-1",
              fileId: "file-1",
              capabilityId: "pdf-to-docx",
              outputFormat: "docx",
              status: "queued",
              progress: 0,
              createdAt: ISO_NOW,
            },
            {
              id: "job-2",
              fileId: "file-2",
              capabilityId: "pdf-to-docx",
              outputFormat: "docx",
              status: "queued",
              progress: 0,
              createdAt: ISO_NOW,
            },
          ],
        },
        201,
      );
    }

    if (path === "/api/jobs/job-1") {
      return jsonResponse(route, {
        id: "job-1",
        fileId: "file-1",
        capabilityId: "pdf-to-docx",
        outputFormat: "docx",
        status: "succeeded",
        progress: 100,
        artifactId: "artifact-1",
        artifactFileName: "contrato-a.docx",
        artifactMimeType: DOCX_MIME,
        artifactSize: 4096,
        createdAt: ISO_NOW,
        completedAt: ISO_NOW,
      });
    }

    if (path === "/api/jobs/job-2") {
      return jsonResponse(route, {
        id: "job-2",
        fileId: "file-2",
        capabilityId: "pdf-to-docx",
        outputFormat: "docx",
        status: "succeeded",
        progress: 100,
        artifactId: "artifact-2",
        artifactFileName: "contrato-b.docx",
        artifactMimeType: DOCX_MIME,
        artifactSize: 4096,
        createdAt: ISO_NOW,
        completedAt: ISO_NOW,
      });
    }

    return jsonResponse(
      route,
      { error: `Unhandled API route: ${method} ${path}` },
      404,
    );
  });
}

async function mockAdminWebhookApi(page: Page) {
  let webhooks = [] as Array<{
    id: string;
    url: string;
    eventTypes: string[];
    enabled: boolean;
    hasSecret: boolean;
    lastDeliveredAt?: string;
    lastError?: string;
    deliveries: Array<{
      id: string;
      eventId: string;
      eventType: string;
      attemptedAt: string;
      deliveredAt?: string;
      statusCode?: number;
      error?: string;
    }>;
    createdAt: string;
    updatedAt: string;
  }>;

  await page.route("**/api/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const method = request.method();

    if (path === "/api/auth/me") {
      return jsonResponse(route, {
        id: "admin-1",
        name: "Admin",
        email: "admin@example.com",
        team: "ops",
        role: "admin",
        createdAt: ISO_NOW,
      });
    }

    if (path === "/api/footer-message") {
      return jsonResponse(route, { message: "Footer admin E2E" });
    }

    if (path === "/api/upload-policy") {
      return jsonResponse(route, uploadPolicy());
    }

    if (path === "/api/admin/overview") {
      return jsonResponse(route, adminOverview());
    }

    if (path === "/api/admin/smtp-settings") {
      return jsonResponse(route, smtpSettings());
    }

    if (path === "/api/admin/email-templates") {
      return jsonResponse(route, []);
    }

    if (path === "/api/admin/webhooks" && method === "GET") {
      return jsonResponse(route, webhooks);
    }

    if (path === "/api/admin/webhooks" && method === "POST") {
      const payload = request.postDataJSON() as {
        url: string;
        secret?: string;
        eventTypes: string[];
        enabled?: boolean;
      };
      const nextWebhook = {
        id: `wh-${webhooks.length + 1}`,
        url: payload.url,
        eventTypes: payload.eventTypes,
        enabled: payload.enabled ?? true,
        hasSecret: Boolean(payload.secret),
        deliveries: [],
        createdAt: ISO_NOW,
        updatedAt: ISO_NOW,
      };
      webhooks = [nextWebhook, ...webhooks];
      return jsonResponse(route, nextWebhook, 201);
    }

    if (path.startsWith("/api/admin/webhooks/") && method === "PUT") {
      const webhookId = path.split("/").pop()!;
      const payload = request.postDataJSON() as {
        url: string;
        secret?: string;
        eventTypes: string[];
        enabled?: boolean;
      };

      webhooks = webhooks.map((webhook) =>
        webhook.id === webhookId
          ? {
              ...webhook,
              url: payload.url,
              eventTypes: payload.eventTypes,
              enabled: payload.enabled ?? webhook.enabled,
              hasSecret: webhook.hasSecret || Boolean(payload.secret),
              updatedAt: ISO_NOW,
            }
          : webhook,
      );

      const updated = webhooks.find((webhook) => webhook.id === webhookId);
      return jsonResponse(route, updated ?? { error: "not found" }, updated ? 200 : 404);
    }

    if (path.startsWith("/api/admin/webhooks/") && method === "DELETE") {
      const webhookId = path.split("/").pop()!;
      webhooks = webhooks.filter((webhook) => webhook.id !== webhookId);
      return route.fulfill({ status: 204, body: "" });
    }

    return jsonResponse(
      route,
      { error: `Unhandled API route: ${method} ${path}` },
      404,
    );
  });
}

test.describe("Suggested flows", () => {
  test("multi-file conversion works with mocked batch APIs", async ({ page }) => {
    await mockBatchConversionApi(page);

    await page.goto("/");

    await page.locator('input[type="file"][multiple]').setInputFiles([
      {
        name: "contrato-a.pdf",
        mimeType: "application/pdf",
        buffer: Buffer.from("%PDF-1.4\narchivo-a"),
      },
      {
        name: "contrato-b.pdf",
        mimeType: "application/pdf",
        buffer: Buffer.from("%PDF-1.4\narchivo-b"),
      },
    ]);

    await expect(screenText(page, "contrato-a.pdf")).toBeVisible();
    await expect(screenText(page, "contrato-b.pdf")).toBeVisible();
    await expect(
      screenText(page, "2 archivos en el lote · 2 listos · 0 completados · 0 con error."),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Convertir PDF" }),
    ).toBeEnabled();

    await page.getByRole("button", { name: "Convertir PDF" }).click();

    await expect(
      screenText(page, "Procesando 2 jobs del lote."),
    ).toBeVisible();
    await expect(
      screenText(page, "2 artefactos del lote quedaron listos para descarga individual."),
    ).toBeVisible({ timeout: 10_000 });
    await expect(
      page.getByRole("button", { name: "Descargar contrato-a.docx" }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Descargar contrato-b.docx" }),
    ).toBeVisible();
  });

  test("admin can create, edit and delete webhook subscriptions", async ({ page }) => {
    await mockAdminWebhookApi(page);

    await page.goto("/admin");

    await expect(
      page.getByRole("heading", { name: "Webhooks" }),
    ).toBeVisible();
    await expect(
      screenText(page, "Todavia no hay suscripciones configuradas."),
    ).toBeVisible();

    await page.getByLabel("URL destino").fill("https://hooks.example.com/reform-lab");
    await page.getByLabel("Secreto").fill("super-secret");
    await page.getByLabel("Job fallido").check();
    await page.getByRole("button", { name: "Crear webhook" }).click();

    await expect(
      screenText(page, "Webhook creado correctamente."),
    ).toBeVisible();
    await expect(
      screenText(page, "https://hooks.example.com/reform-lab"),
    ).toBeVisible();

    await page.getByRole("button", { name: "Editar" }).click();
    await page.getByLabel("URL destino").fill("https://hooks.example.com/actualizado");
    await page.getByLabel("Webhook habilitado").uncheck();
    await page.getByRole("button", { name: "Guardar cambios" }).click();

    await expect(
      screenText(page, "Webhook actualizado correctamente."),
    ).toBeVisible();
    await expect(
      screenText(page, "https://hooks.example.com/actualizado"),
    ).toBeVisible();
    await expect(screenText(page, "Pausado · con firma secreta")).toBeVisible();

    await page.getByRole("button", { name: "Eliminar" }).click();

    await expect(screenText(page, "Webhook eliminado.")).toBeVisible();
    await expect(
      screenText(page, "Todavia no hay suscripciones configuradas."),
    ).toBeVisible();
  });
});

function screenText(page: Page, value: string) {
  return page.getByText(value, { exact: true });
}