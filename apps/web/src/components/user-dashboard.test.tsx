import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, beforeEach, expect, it, vi } from "vitest";
import UserDashboard from "./user-dashboard";
import { IntlWrapper } from "@/test/intl-wrapper";
import {
  cancelJob,
  cancelJobs,
  downloadArtifact,
  getMyDashboard,
  retryJob,
  retryJobs,
  type UserDashboardData,
} from "@/lib/api";

const pushMock = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
}));

let authState: { user: { name: string; role: string } | null; loading: boolean } = {
  user: { name: "Test", role: "user" },
  loading: false,
};

vi.mock("@/lib/auth-context", () => ({
  useAuth: () => authState,
}));

vi.mock("@/lib/api", () => ({
  cancelJob: vi.fn(),
  cancelJobs: vi.fn(),
  getMyDashboard: vi.fn(),
  downloadArtifact: vi.fn(),
  retryJob: vi.fn(),
  retryJobs: vi.fn(),
}));

const succeededJob = {
  jobId: "j1",
  fileId: "f1",
  fileName: "photo.heic",
  detectedFamily: "image",
  capabilityId: "heic-png",
  outputFormat: "png",
  status: "succeeded" as const,
  progress: 100,
  artifactId: "a1",
  artifactFileName: "photo.png",
  expiresAt: "2026-04-12T00:00:00Z",
  updatedAt: "2026-04-11T10:00:00Z",
};

const failedJob = {
  jobId: "j2",
  fileId: "f2",
  fileName: "broken.svg",
  detectedFamily: "image",
  capabilityId: "svg-pdf",
  outputFormat: "pdf",
  status: "failed" as const,
  progress: 0,
  error: "conversion timeout",
  updatedAt: "2026-04-11T09:00:00Z",
};

const dashboardData: UserDashboardData = {
  totalFiles: 5,
  totalJobs: 3,
  activeJobs: 1,
  succeededJobs: 2,
  failedJobs: 1,
  recentJobs: [succeededJob, failedJob],
};

describe("UserDashboard", () => {
  beforeEach(() => {
    pushMock.mockClear();
    vi.mocked(getMyDashboard).mockResolvedValue(dashboardData);
    vi.mocked(cancelJob).mockResolvedValue(undefined);
    vi.mocked(cancelJobs).mockResolvedValue(["j3"]);
    vi.mocked(downloadArtifact).mockResolvedValue(undefined);
    vi.mocked(retryJob).mockResolvedValue({
      id: "j3",
      fileId: "f2",
      capabilityId: "svg-pdf",
      outputFormat: "pdf",
      status: "queued",
      progress: 0,
      createdAt: "2026-04-11T10:00:00Z",
    });
    vi.mocked(retryJobs).mockResolvedValue([]);
    authState = { user: { name: "Test", role: "user" }, loading: false };
  });

  it("renders loading state then summary", async () => {
    render(<IntlWrapper><UserDashboard /></IntlWrapper>);
    await waitFor(() => {
      expect(screen.getByText("Archivos propios: 5")).toBeInTheDocument();
    });
    expect(screen.getByText("Jobs registrados: 3")).toBeInTheDocument();
    expect(screen.getByText("Exitosos: 2")).toBeInTheDocument();
    expect(screen.getByText("Fallidos: 1")).toBeInTheDocument();
  });

  it("renders job rows with file names", async () => {
    render(<IntlWrapper><UserDashboard /></IntlWrapper>);
    await waitFor(() => {
      expect(screen.getByText("photo.heic")).toBeInTheDocument();
    });
    expect(screen.getByText("broken.svg")).toBeInTheDocument();
  });

  it("shows download button for succeeded jobs", async () => {
    const user = userEvent.setup();
    render(<IntlWrapper><UserDashboard /></IntlWrapper>);
    await waitFor(() => {
      expect(screen.getByText("Descargar")).toBeInTheDocument();
    });
    await user.click(screen.getByText("Descargar"));
    expect(downloadArtifact).toHaveBeenCalledWith("a1", "photo.png");
  });

  it("shows retry button for failed jobs", async () => {
    const user = userEvent.setup();
    render(<IntlWrapper><UserDashboard /></IntlWrapper>);
    await waitFor(() => {
      expect(screen.getByText("Reintentar")).toBeInTheDocument();
    });
    await user.click(screen.getByText("Reintentar"));
    expect(retryJob).toHaveBeenCalledWith("j2");
  });

  it("shows cancel button for queued jobs", async () => {
    const user = userEvent.setup();
    vi.mocked(getMyDashboard).mockResolvedValue({
      ...dashboardData,
      recentJobs: [
        ...dashboardData.recentJobs,
        {
          jobId: "j3",
          fileId: "f3",
          fileName: "video.mov",
          detectedFamily: "video",
          capabilityId: "video-mp4",
          outputFormat: "mp4",
          status: "queued" as const,
          progress: 0,
          updatedAt: "2026-04-11T09:30:00Z",
        },
      ],
    });

    render(<IntlWrapper><UserDashboard /></IntlWrapper>);
    await waitFor(() => {
      expect(screen.getByText("video.mov")).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "Cancelar" }));
    expect(cancelJob).toHaveBeenCalledWith("j3");
  });

  it("runs batch retry for failed selections", async () => {
    const user = userEvent.setup();
    render(<IntlWrapper><UserDashboard /></IntlWrapper>);
    await waitFor(() => {
      expect(screen.getByText("broken.svg")).toBeInTheDocument();
    });

    await user.click(
      screen.getByRole("checkbox", {
        name: "Seleccionar job para broken.svg",
      }),
    );
    await user.click(screen.getByRole("button", { name: "Reintentar seleccionados" }));

    expect(retryJobs).toHaveBeenCalledWith(["j2"]);
  });

  it("shows empty state when no jobs exist", async () => {
    vi.mocked(getMyDashboard).mockResolvedValue({
      ...dashboardData,
      recentJobs: [],
    });
    render(<IntlWrapper><UserDashboard /></IntlWrapper>);
    await waitFor(() => {
      expect(
        screen.getByText("Todavia no tienes conversiones registradas."),
      ).toBeInTheDocument();
    });
  });

  it("shows error when API fails", async () => {
    vi.mocked(getMyDashboard).mockRejectedValue(new Error("network error"));
    render(<IntlWrapper><UserDashboard /></IntlWrapper>);
    await waitFor(() => {
      expect(screen.getByText("network error")).toBeInTheDocument();
    });
  });

  it("redirects to /acceso when not authenticated", () => {
    authState = { user: null, loading: false };
    render(<IntlWrapper><UserDashboard /></IntlWrapper>);
    expect(pushMock).toHaveBeenCalledWith("/acceso");
  });
});
