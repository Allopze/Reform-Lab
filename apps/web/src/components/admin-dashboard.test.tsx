import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, beforeEach, expect, it, vi } from "vitest";
import AdminDashboard from "./admin-dashboard";
import { IntlWrapper } from "@/test/intl-wrapper";
import {
  getAdminOverview,
  getFooterMessage,
  getUploadPolicy,
  updateFooterMessage,
  updateUploadPolicy,
  type AdminDashboardData,
  type UploadPolicy,
} from "@/lib/api";

const pushMock = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
}));

let authState: { user: { name: string; role: string } | null; loading: boolean } = {
  user: { name: "Admin", role: "admin" },
  loading: false,
};

vi.mock("@/lib/auth-context", () => ({
  useAuth: () => authState,
}));

vi.mock("@/lib/api", () => ({
  getAdminOverview: vi.fn(),
  getFooterMessage: vi.fn(),
  getUploadPolicy: vi.fn(),
  updateFooterMessage: vi.fn(),
  updateUploadPolicy: vi.fn(),
}));

vi.mock("@/components/smtp-settings", () => ({
  default: () => <div data-testid="smtp-settings" />,
}));

vi.mock("@/components/email-templates", () => ({
  default: () => <div data-testid="email-templates" />,
}));

const policy: UploadPolicy = {
  guestMaxBytes: 104857600,
  registeredMaxBytes: 524288000,
  effectiveMaxBytes: 524288000,
  absoluteMaxBytes: 524288000,
  viewerType: "registered",
};

const dashboardData: AdminDashboardData = {
  totalUsers: 10,
  totalFiles: 42,
  totalJobs: 30,
  queuedJobs: 2,
  runningJobs: 1,
  succeededJobs: 25,
  failedJobs: 2,
  cancelledJobs: 0,
  successRatePct: 92.6,
  averageDurationSec: 4.2,
  availableEngines: 8,
  totalEngines: 10,
  unavailableEngines: ["wkhtmltopdf", "ebook-convert"],
  engineUsage: [{ key: "libreoffice", count: 15 }],
  recentAudit: [
    {
      id: "au1",
      eventType: "upload",
      fileId: "f1",
      details: { originalName: "report.docx" },
      createdAt: "2026-04-11T09:00:00Z",
    },
    {
      id: "au2",
      eventType: "job_failed",
      jobId: "j1",
      details: { error: "timeout exceeded" },
      createdAt: "2026-04-11T08:30:00Z",
    },
  ],
  recentJobs: [
    {
      jobId: "abcdef12-0000-0000-0000-000000000000",
      userName: "Ada Lovelace",
      userEmail: "ada@example.com",
      fileName: "report.docx",
      capabilityId: "docx-pdf",
      outputFormat: "pdf",
      status: "succeeded",
      updatedAt: "2026-04-11T10:00:00Z",
    },
  ],
};

describe("AdminDashboard", () => {
  beforeEach(() => {
    pushMock.mockClear();
    vi.mocked(getAdminOverview).mockResolvedValue(dashboardData);
    vi.mocked(getFooterMessage).mockResolvedValue("Powered by Reform Lab");
    vi.mocked(getUploadPolicy).mockResolvedValue(policy);
    vi.mocked(updateFooterMessage).mockResolvedValue("New footer");
    vi.mocked(updateUploadPolicy).mockResolvedValue(policy);
    authState = { user: { name: "Admin", role: "admin" }, loading: false };
  });

  it("renders summary stats after loading", async () => {
    render(<IntlWrapper><AdminDashboard /></IntlWrapper>);
    expect(screen.getByText("Cargando panel admin...")).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText("Usuarios: 10")).toBeInTheDocument();
    });
    expect(screen.getByText("Archivos: 42")).toBeInTheDocument();
    expect(screen.getByText("Exitosos: 25")).toBeInTheDocument();
  });

  it("renders a recent job row with user info", async () => {
    render(<IntlWrapper><AdminDashboard /></IntlWrapper>);
    await waitFor(() => {
      expect(screen.getByText("Usuarios: 10")).toBeInTheDocument();
    });
    expect(screen.getAllByText("report.docx").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("Ada Lovelace")).toBeInTheDocument();
    expect(screen.getByText("ada@example.com")).toBeInTheDocument();
  });

  it("renders audit events", async () => {
    render(<IntlWrapper><AdminDashboard /></IntlWrapper>);
    await waitFor(() => {
      expect(screen.getByText("Archivo subido")).toBeInTheDocument();
    });
    expect(screen.getByText("Job fallido")).toBeInTheDocument();
  });

  it("shows error when API fails", async () => {
    vi.mocked(getAdminOverview).mockRejectedValue(new Error("forbidden"));
    render(<IntlWrapper><AdminDashboard /></IntlWrapper>);
    await waitFor(() => {
      expect(screen.getByText("forbidden")).toBeInTheDocument();
    });
  });

  it("redirects non-admin to /usuario", () => {
    authState = { user: { name: "User", role: "user" }, loading: false };
    render(<IntlWrapper><AdminDashboard /></IntlWrapper>);
    expect(pushMock).toHaveBeenCalledWith("/usuario");
  });

  it("redirects unauthenticated to /acceso", () => {
    authState = { user: null, loading: false };
    render(<IntlWrapper><AdminDashboard /></IntlWrapper>);
    expect(pushMock).toHaveBeenCalledWith("/acceso");
  });

  it("filters audit events by type", async () => {
    const user = userEvent.setup();
    render(<IntlWrapper><AdminDashboard /></IntlWrapper>);
    await waitFor(() => {
      expect(screen.getByText("Archivo subido")).toBeInTheDocument();
    });
    // Click the "Jobs fallidos" filter button
    await user.click(screen.getByRole("button", { name: "Jobs fallidos" }));
    expect(screen.getByText("Job fallido")).toBeInTheDocument();
    expect(screen.queryByText("Archivo subido")).not.toBeInTheDocument();
  });
});
