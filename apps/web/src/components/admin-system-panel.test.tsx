import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import AdminSystemPanel from "./admin-system-panel";
import { IntlWrapper } from "@/test/intl-wrapper";
import {
  drainQueuedJobs,
  getAdminEngines,
  getHealthInfo,
  pruneStaleWorkers,
  updateJobIntakeControl,
  type AdminEnginesInfo,
  type HealthInfo,
} from "@/lib/api";

const replaceMock = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: replaceMock }),
}));

let authState: { user: { name: string; role: string } | null; loading: boolean } = {
  user: { name: "Admin", role: "admin" },
  loading: false,
};

vi.mock("@/lib/auth-context", () => ({
  useAuth: () => authState,
}));

vi.mock("@/lib/api", () => ({
  getHealthInfo: vi.fn(),
  getAdminEngines: vi.fn(),
  updateJobIntakeControl: vi.fn(),
  drainQueuedJobs: vi.fn(),
  pruneStaleWorkers: vi.fn(),
}));

function buildHealthInfo(overrides?: Partial<HealthInfo>): HealthInfo {
  return {
    status: "ok",
    retention: {
      artifactTTLHours: 24,
      artifactTTLHoursByFamily: { document: 24 },
    },
    featureFlags: {
      disabledCapabilities: [],
      disabledEngines: [],
    },
    runtime: {
      queue: {
        mode: "in-process",
        workerConcurrency: 2,
        queuedJobs: 4,
        runningJobs: 1,
        stalledJobs: 0,
        stalledQueuedJobs: 0,
        stalledRunningJobs: 0,
        controls: {
          jobIntakePaused: false,
          pauseReason: "",
          updatedAt: "2026-04-16T10:00:00Z",
        },
        history: [
          {
            window: "5m",
            enqueuedJobs: 6,
            failedJobs: 1,
            completedJobs: 5,
            averageLatencySec: 1.8,
          },
          {
            window: "15m",
            enqueuedJobs: 18,
            failedJobs: 2,
            completedJobs: 16,
            averageLatencySec: 2.1,
          },
          {
            window: "1h",
            enqueuedJobs: 70,
            failedJobs: 7,
            completedJobs: 63,
            averageLatencySec: 2.9,
          },
        ],
      },
      storage: {
        status: "up",
        path: "/tmp/reform-lab",
        freeBytes: 2_000_000,
        totalBytes: 4_000_000,
        usedPercent: 50,
      },
      workers: {
        count: 1,
        apiEngineMode: "declared",
        apiEngineAvailability: {
          imagemagick: true,
          libreoffice: true,
        },
        workers: [
          {
            id: "worker-1",
            runtimeMode: "server",
            queueMode: "redis",
            lastHeartbeatAt: "2026-04-16T10:00:00Z",
            lastTaskStatus: "idle",
            lastTaskType: "conversion:image-to-webp",
            engines: {
              imagemagick: true,
              libreoffice: true,
            },
            recentFailures: [],
          },
        ],
      },
    },
    dependencies: {
      database: {
        status: "up",
        latencyMs: 3,
      },
      redis: {
        status: "up",
        latencyMs: 5,
      },
    },
    alerts: [],
    ...overrides,
  };
}

const enginesInfo: AdminEnginesInfo = {
  engines: {
    libreoffice: true,
    imagemagick: true,
  },
  capabilities: [
    {
      id: "image-to-webp",
      displayName: "Image to WebP",
      engine: "imagemagick",
      family: "image",
      operationType: "convert",
      targetFormat: "webp",
      available: true,
      reason: "available",
    },
  ],
  availableCapabilities: 1,
  totalCapabilities: 1,
};

function renderPanel() {
  render(
    <IntlWrapper>
      <AdminSystemPanel />
    </IntlWrapper>,
  );
}

describe("AdminSystemPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    replaceMock.mockClear();
    authState = { user: { name: "Admin", role: "admin" }, loading: false };
    vi.mocked(getHealthInfo).mockResolvedValue(buildHealthInfo());
    vi.mocked(getAdminEngines).mockResolvedValue(enginesInfo);
    vi.mocked(updateJobIntakeControl).mockResolvedValue({
      jobIntakePaused: true,
      pauseReason: "maintenance",
      updatedAt: "2026-04-16T10:05:00Z",
    });
    vi.mocked(drainQueuedJobs).mockResolvedValue({
      attempted: 20,
      cancelled: 7,
      skipped: 13,
      cancelledIds: ["j1"],
    });
    vi.mocked(pruneStaleWorkers).mockResolvedValue({
      deleted: 2,
      staleMinutes: 60,
      cutoff: "2026-04-16T09:00:00Z",
    });
  });

  it("renders system data and control sections", async () => {
    renderPanel();

    await waitFor(() => {
      expect(screen.getByText("Controles operativos")).toBeInTheDocument();
    });

    expect(screen.getByText("Cola y workers")).toBeInTheDocument();
    expect(screen.getByText("Modo de engines API: declared")).toBeInTheDocument();
    expect(screen.getByText("No se detectan divergencias entre API y workers reportados.")).toBeInTheDocument();
    expect(screen.getByText("Historico")).toBeInTheDocument();
    expect(screen.getByText("Disponibilidad por capability")).toBeInTheDocument();
    expect(screen.getByText("Image to WebP")).toBeInTheDocument();
  });

  it("surfaces API and worker engine divergences", async () => {
    vi.mocked(getHealthInfo).mockResolvedValue(
      buildHealthInfo({
        runtime: {
          queue: {
            mode: "redis",
            workerConcurrency: 2,
            queuedJobs: 4,
            runningJobs: 1,
            stalledJobs: 0,
            stalledQueuedJobs: 0,
            stalledRunningJobs: 0,
            controls: {
              jobIntakePaused: false,
              pauseReason: "",
              updatedAt: "2026-04-16T10:00:00Z",
            },
            history: [],
          },
          storage: {
            status: "up",
            path: "/tmp/reform-lab",
            freeBytes: 2_000_000,
            totalBytes: 4_000_000,
            usedPercent: 50,
          },
          workers: {
            count: 1,
            apiEngineMode: "declared",
            apiEngineAvailability: {
              ffmpeg: true,
              libreoffice: true,
              tesseract: false,
            },
            workers: [
              {
                id: "worker-1",
                runtimeMode: "standalone",
                queueMode: "redis",
                lastHeartbeatAt: "2026-04-16T10:00:00Z",
                lastTaskStatus: "idle",
                engines: {
                  ffmpeg: true,
                  libreoffice: false,
                  tesseract: true,
                },
                recentFailures: [],
              },
            ],
          },
        },
      }),
    );

    renderPanel();

    await waitFor(() => {
      expect(screen.getByText("worker-1: faltan en worker [libreoffice], solo worker [tesseract]")).toBeInTheDocument();
    });
  });

  it("validates reason before pausing intake", async () => {
    const user = userEvent.setup();
    renderPanel();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Pausar intake" })).toBeInTheDocument();
    });

    await user.clear(screen.getByPlaceholderText("Motivo de pausa"));
    await user.click(screen.getByRole("button", { name: "Pausar intake" }));

    expect(screen.getByText("El motivo de pausa es obligatorio.")).toBeInTheDocument();
    expect(updateJobIntakeControl).not.toHaveBeenCalled();
  });

  it("pauses intake and refreshes state", async () => {
    const user = userEvent.setup();
    renderPanel();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Pausar intake" })).toBeInTheDocument();
    });

    await user.clear(screen.getByPlaceholderText("Motivo de pausa"));
    await user.type(screen.getByPlaceholderText("Motivo de pausa"), "maintenance");
    await user.click(screen.getByRole("button", { name: "Pausar intake" }));

    await waitFor(() => {
      expect(updateJobIntakeControl).toHaveBeenCalledWith({ paused: true, reason: "maintenance" });
    });
    expect(screen.getByText("Intake pausado correctamente.")).toBeInTheDocument();
    expect(getHealthInfo).toHaveBeenCalled();
    expect(getAdminEngines).toHaveBeenCalled();
  });

  it("resumes intake when currently paused", async () => {
    const user = userEvent.setup();
    vi.mocked(getHealthInfo).mockResolvedValue(
      buildHealthInfo({
        runtime: {
          queue: {
            mode: "in-process",
            workerConcurrency: 2,
            queuedJobs: 4,
            runningJobs: 1,
            stalledJobs: 0,
            stalledQueuedJobs: 0,
            stalledRunningJobs: 0,
            controls: {
              jobIntakePaused: true,
              pauseReason: "maintenance",
              updatedAt: "2026-04-16T10:00:00Z",
            },
            history: [],
          },
          storage: {
            status: "up",
            path: "/tmp/reform-lab",
            freeBytes: 2_000_000,
            totalBytes: 4_000_000,
            usedPercent: 50,
          },
          workers: { count: 0, workers: [] },
        },
      }),
    );

    renderPanel();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Reanudar intake" })).toBeEnabled();
    });

    await user.click(screen.getByRole("button", { name: "Reanudar intake" }));

    await waitFor(() => {
      expect(updateJobIntakeControl).toHaveBeenCalledWith({ paused: false });
    });
    expect(screen.getByText("Intake reanudado correctamente.")).toBeInTheDocument();
  });

  it("drains queue and prunes workers", async () => {
    const user = userEvent.setup();
    renderPanel();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Drenar cola" })).toBeInTheDocument();
    });

    await user.clear(screen.getByPlaceholderText("100"));
    await user.type(screen.getByPlaceholderText("100"), "25");
    await user.click(screen.getByRole("button", { name: "Drenar cola" }));

    await waitFor(() => {
      expect(drainQueuedJobs).toHaveBeenCalledWith(25);
    });
    expect(screen.getByText("Drenado completado: 7 cancelados, 13 omitidos.")).toBeInTheDocument();

    await user.clear(screen.getByPlaceholderText("60"));
    await user.type(screen.getByPlaceholderText("60"), "120");
    await user.click(screen.getByRole("button", { name: "Limpiar workers" }));

    await waitFor(() => {
      expect(pruneStaleWorkers).toHaveBeenCalledWith(120);
    });
    expect(screen.getByText("Workers eliminados: 2.")).toBeInTheDocument();
  });

  it("shows prune validation error for out-of-range value", async () => {
    const user = userEvent.setup();
    renderPanel();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Limpiar workers" })).toBeInTheDocument();
    });

    await user.clear(screen.getByPlaceholderText("60"));
    await user.type(screen.getByPlaceholderText("60"), "4");
    await user.click(screen.getByRole("button", { name: "Limpiar workers" }));

    expect(screen.getByText("El valor de minutos debe estar entre 5 y 10080.")).toBeInTheDocument();
    expect(pruneStaleWorkers).not.toHaveBeenCalled();
  });
});
