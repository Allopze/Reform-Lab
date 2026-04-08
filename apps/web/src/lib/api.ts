import { API_URL } from "./config";
import { getToken } from "./auth";

function headers(): HeadersInit {
  const h: HeadersInit = {};
  const token = getToken();
  if (token) h["Authorization"] = `Bearer ${token}`;
  return h;
}

// ── Upload ──

export interface UploadedFile {
  id: string;
  originalName: string;
  size: number;
  detectedFormat: {
    mimeType: string;
    family: string;
    extension: string;
  };
  uploadedAt: string;
}

export async function uploadFile(file: File): Promise<UploadedFile> {
  const form = new FormData();
  form.append("file", file);

  const res = await fetch(`${API_URL}/api/files`, {
    method: "POST",
    headers: headers(),
    body: form,
  });

  const data = await res.json();
  if (!res.ok) throw new Error(data.error || "Upload failed");
  return data;
}

// ── Capabilities ──

export interface Capability {
  id: string;
  displayName: string;
  targetFormat: string;
  operationType: string;
  timeoutSeconds: number;
}

export async function getCapabilities(fileId: string): Promise<Capability[]> {
  const res = await fetch(`${API_URL}/api/files/${fileId}/capabilities`, {
    headers: headers(),
  });

  const data = await res.json();
  if (!res.ok) throw new Error(data.error || "Failed to load capabilities");
  return data.capabilities;
}

// ── Conversion ──

export interface Job {
  id: string;
  userId?: string;
  fileId: string;
  capabilityId: string;
  outputFormat: string;
  status: "queued" | "running" | "succeeded" | "failed" | "cancelled" | "expired";
  progress: number;
  error?: string;
  artifactId?: string;
  startedAt?: string;
  completedAt?: string;
  createdAt: string;
}

export async function createConversion(fileId: string, capabilityId: string): Promise<Job> {
  const res = await fetch(`${API_URL}/api/conversions`, {
    method: "POST",
    headers: { ...headers(), "Content-Type": "application/json" },
    body: JSON.stringify({ fileId, capabilityId }),
  });

  const data = await res.json();
  if (!res.ok) throw new Error(data.error || "Conversion failed");
  return data;
}

export async function getJob(jobId: string): Promise<Job> {
  const res = await fetch(`${API_URL}/api/jobs/${jobId}`, {
    headers: headers(),
  });

  const data = await res.json();
  if (!res.ok) throw new Error(data.error || "Failed to get job status");
  return data;
}

export interface UserDashboardJob {
  jobId: string;
  fileId: string;
  fileName: string;
  detectedFamily: string;
  capabilityId: string;
  outputFormat: string;
  status: Job["status"];
  progress: number;
  error?: string;
  artifactId?: string;
  artifactFileName?: string;
  expiresAt?: string;
  updatedAt: string;
}

export interface UserDashboardData {
  totalFiles: number;
  totalJobs: number;
  activeJobs: number;
  succeededJobs: number;
  failedJobs: number;
  recentJobs: UserDashboardJob[];
}

export interface AdminDashboardJob {
  jobId: string;
  userName: string;
  userEmail: string;
  fileName: string;
  capabilityId: string;
  outputFormat: string;
  status: Job["status"];
  error?: string;
  updatedAt: string;
}

export interface AdminUsageStat {
  key: string;
  count: number;
}

export interface AdminAuditEvent {
  id: string;
  eventType: string;
  fileId?: string;
  jobId?: string;
  details?: Record<string, unknown>;
  createdAt: string;
}

export interface AdminDashboardData {
  totalUsers: number;
  totalFiles: number;
  totalJobs: number;
  queuedJobs: number;
  runningJobs: number;
  succeededJobs: number;
  failedJobs: number;
  cancelledJobs: number;
  successRatePct: number;
  averageDurationSec: number;
  availableEngines: number;
  totalEngines: number;
  unavailableEngines: string[];
  engineUsage: AdminUsageStat[];
  recentAudit: AdminAuditEvent[];
  recentJobs: AdminDashboardJob[];
}

export async function getMyDashboard(): Promise<UserDashboardData> {
  const res = await fetch(`${API_URL}/api/dashboard/me`, {
    headers: headers(),
  });

  const data = await res.json();
  if (!res.ok) throw new Error(data.error || "Failed to load dashboard");
  return data;
}

export async function getAdminOverview(): Promise<AdminDashboardData> {
  const res = await fetch(`${API_URL}/api/admin/overview`, {
    headers: headers(),
  });

  const data = await res.json();
  if (!res.ok) throw new Error(data.error || "Failed to load admin overview");
  return data;
}

// ── Download ──

export function artifactDownloadUrl(artifactId: string): string {
  return `${API_URL}/api/artifacts/${artifactId}/download`;
}

function fileNameFromDisposition(value: string | null): string | null {
  if (!value) return null;
  const match = value.match(/filename="?([^";]+)"?/i);
  return match?.[1] ?? null;
}

export async function downloadArtifact(
  artifactId: string,
  fallbackName?: string
): Promise<void> {
  const res = await fetch(artifactDownloadUrl(artifactId), {
    headers: headers(),
  });

  if (!res.ok) {
    let message = "Download failed";
    try {
      const data = await res.json();
      message = data.error || message;
    } catch {
      // ignore JSON parse failures for non-JSON errors
    }
    throw new Error(message);
  }

  const blob = await res.blob();
  const objectUrl = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = objectUrl;
  anchor.download =
    fileNameFromDisposition(res.headers.get("Content-Disposition")) ||
    fallbackName ||
    `artifact-${artifactId}`;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(objectUrl);
}

// ── Job management ──

export async function cancelJob(jobId: string): Promise<void> {
  const res = await fetch(`${API_URL}/api/jobs/${jobId}/cancel`, {
    method: "POST",
    headers: headers(),
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error((data as { error?: string }).error || "Failed to cancel job");
  }
}

export async function retryJob(jobId: string): Promise<Job> {
  const res = await fetch(`${API_URL}/api/jobs/${jobId}/retry`, {
    method: "POST",
    headers: headers(),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error((data as { error?: string }).error || "Failed to retry job");
  }
  return data as Job;
}

// ── Health / Service info ──

export interface HealthInfo {
  status: string;
  retention: {
    artifactTTLHours: number;
    artifactTTLHoursByFamily: Record<string, number>;
  };
  featureFlags: {
    disabledCapabilities: string[];
    disabledEngines: string[];
  };
}

export async function getHealthInfo(): Promise<HealthInfo> {
  const res = await fetch(`${API_URL}/api/health`);
  const data = await res.json();
  if (!res.ok) throw new Error("Failed to fetch service info");
  return data;
}
