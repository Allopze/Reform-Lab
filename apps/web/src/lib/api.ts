import { API_URL } from "./config";
import { DEFAULT_FOOTER_MESSAGE } from "./footer-message";

function headers(): HeadersInit {
  return {};
}

const DEFAULT_TIMEOUT_MS = 15_000;
const TRANSFER_TIMEOUT_MS = 300_000; // 5 min for file uploads/downloads

function fetchWithTimeout(
  input: RequestInfo | URL,
  init?: RequestInit & { timeoutMs?: number },
): Promise<Response> {
  const { timeoutMs = DEFAULT_TIMEOUT_MS, ...fetchInit } = init ?? {};
  const controller = new AbortController();
  const id = setTimeout(() => controller.abort(), timeoutMs);

  return fetch(input, { ...fetchInit, signal: controller.signal }).finally(() =>
    clearTimeout(id),
  );
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

export interface UploadPolicy {
  guestMaxBytes: number;
  registeredMaxBytes: number;
  effectiveMaxBytes: number;
  viewerType: "guest" | "registered";
  absoluteMaxBytes: number;
}

export async function getUploadPolicy(): Promise<UploadPolicy> {
  const res = await fetchWithTimeout(`${API_URL}/api/upload-policy`, {
    headers: headers(),
    credentials: "include",
  });

  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(
      (data as { error?: string }).error || "Failed to load upload policy",
    );
  }
  return data as UploadPolicy;
}

export async function uploadFile(file: File): Promise<UploadedFile> {
  const form = new FormData();
  form.append("file", file);

  const res = await fetchWithTimeout(`${API_URL}/api/files`, {
    method: "POST",
    headers: headers(),
    credentials: "include",
    body: form,
    timeoutMs: TRANSFER_TIMEOUT_MS,
  });

  const data = await res.json();
  if (!res.ok) throw new Error(data.error || "Upload failed");
  return data;
}

export async function getBatchCapabilities(
  fileIds: string[],
): Promise<Capability[]> {
  const res = await fetchWithTimeout(`${API_URL}/api/files/capabilities/batch`, {
    method: "POST",
    headers: { ...headers(), "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify({ fileIds }),
  });

  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(
      (data as { error?: string }).error ||
        "Failed to load batch capabilities",
    );
  }

  return (data as { capabilities: Capability[] }).capabilities;
}

// ── Capabilities ──

export interface Capability {
  id: string;
  displayName: string;
  presentationOrder: number;
  targetFormat: string;
  operationType: string;
  timeoutSeconds: number;
}

export async function getCapabilities(fileId: string): Promise<Capability[]> {
  const res = await fetchWithTimeout(
    `${API_URL}/api/files/${fileId}/capabilities`,
    {
      headers: headers(),
      credentials: "include",
    },
  );

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
  status:
    | "queued"
    | "running"
    | "succeeded"
    | "failed"
    | "cancelled"
    | "expired";
  progress: number;
  error?: string;
  artifactId?: string;
  artifactFileName?: string;
  artifactMimeType?: string;
  artifactSize?: number;
  startedAt?: string;
  completedAt?: string;
  createdAt: string;
}

export async function createConversion(
  fileId: string,
  capabilityId: string,
): Promise<Job> {
  const res = await fetchWithTimeout(`${API_URL}/api/conversions`, {
    method: "POST",
    headers: { ...headers(), "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify({ fileId, capabilityId }),
  });

  const data = await res.json();
  if (!res.ok) throw new Error(data.error || "Conversion failed");
  return data;
}

export async function createBatchConversion(
  fileIds: string[],
  capabilityId: string,
): Promise<Job[]> {
  const res = await fetchWithTimeout(`${API_URL}/api/conversions/batch`, {
    method: "POST",
    headers: { ...headers(), "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify({ fileIds, capabilityId }),
  });

  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(
      (data as { error?: string }).error || "Batch conversion failed",
    );
  }
  return ((data as { jobs?: Job[] }).jobs ?? []) as Job[];
}

export async function getJob(jobId: string): Promise<Job> {
  const res = await fetchWithTimeout(`${API_URL}/api/jobs/${jobId}`, {
    headers: headers(),
    credentials: "include",
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
  const res = await fetchWithTimeout(`${API_URL}/api/dashboard/me`, {
    headers: headers(),
    credentials: "include",
  });

  const data = await res.json();
  if (!res.ok) throw new Error(data.error || "Failed to load dashboard");
  return data;
}

export async function getAdminOverview(): Promise<AdminDashboardData> {
  const res = await fetchWithTimeout(`${API_URL}/api/admin/overview`, {
    headers: headers(),
    credentials: "include",
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
  fallbackName?: string,
): Promise<void> {
  const res = await fetchWithTimeout(artifactDownloadUrl(artifactId), {
    headers: headers(),
    credentials: "include",
    timeoutMs: TRANSFER_TIMEOUT_MS,
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
  const res = await fetchWithTimeout(`${API_URL}/api/jobs/${jobId}/cancel`, {
    method: "POST",
    headers: headers(),
    credentials: "include",
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(
      (data as { error?: string }).error || "Failed to cancel job",
    );
  }
}

export async function cancelJobs(jobIds: string[]): Promise<string[]> {
  const res = await fetchWithTimeout(`${API_URL}/api/jobs/batch/cancel`, {
    method: "POST",
    headers: { ...headers(), "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify({ jobIds }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(
      (data as { error?: string }).error || "Failed to cancel jobs",
    );
  }
  return ((data as { cancelledJobIds?: string[] }).cancelledJobIds ?? []) as string[];
}

export async function retryJob(jobId: string): Promise<Job> {
  const res = await fetchWithTimeout(`${API_URL}/api/jobs/${jobId}/retry`, {
    method: "POST",
    headers: headers(),
    credentials: "include",
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(
      (data as { error?: string }).error || "Failed to retry job",
    );
  }
  return data as Job;
}

export async function retryJobs(jobIds: string[]): Promise<Job[]> {
  const res = await fetchWithTimeout(`${API_URL}/api/jobs/batch/retry`, {
    method: "POST",
    headers: { ...headers(), "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify({ jobIds }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(
      (data as { error?: string }).error || "Failed to retry jobs",
    );
  }
  return ((data as { jobs?: Job[] }).jobs ?? []) as Job[];
}

export interface WebhookSubscription {
  id: string;
  url: string;
  eventTypes: string[];
  enabled: boolean;
  hasSecret: boolean;
  lastDeliveredAt?: string;
  lastError?: string;
  deliveries?: WebhookDelivery[];
  createdAt: string;
  updatedAt: string;
}

export interface WebhookDelivery {
  id: string;
  eventId: string;
  eventType: string;
  attemptedAt: string;
  deliveredAt?: string;
  statusCode?: number;
  error?: string;
}

export interface WebhookDraft {
  url: string;
  secret?: string;
  eventTypes: string[];
  enabled?: boolean;
}

export async function getWebhooks(): Promise<WebhookSubscription[]> {
  const res = await fetchWithTimeout(`${API_URL}/api/admin/webhooks`, {
    headers: headers(),
    credentials: "include",
  });
  const data = await res.json().catch(() => []);
  if (!res.ok) {
    throw new Error("Failed to load webhooks");
  }
  return data as WebhookSubscription[];
}

export async function createWebhook(
  payload: WebhookDraft,
): Promise<WebhookSubscription> {
  const res = await fetchWithTimeout(`${API_URL}/api/admin/webhooks`, {
    method: "POST",
    headers: { ...headers(), "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify(payload),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error((data as { error?: string }).error || "Failed to create webhook");
  }
  return data as WebhookSubscription;
}

export async function updateWebhook(
  webhookId: string,
  payload: WebhookDraft,
): Promise<WebhookSubscription> {
  const res = await fetchWithTimeout(`${API_URL}/api/admin/webhooks/${webhookId}`, {
    method: "PUT",
    headers: { ...headers(), "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify(payload),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error((data as { error?: string }).error || "Failed to update webhook");
  }
  return data as WebhookSubscription;
}

export async function deleteWebhook(webhookId: string): Promise<void> {
  const res = await fetchWithTimeout(`${API_URL}/api/admin/webhooks/${webhookId}`, {
    method: "DELETE",
    headers: headers(),
    credentials: "include",
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error((data as { error?: string }).error || "Failed to delete webhook");
  }
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
  const res = await fetchWithTimeout(`${API_URL}/api/admin/health`, {
    credentials: "include",
  });
  const data = await res.json();
  if (!res.ok) throw new Error("Failed to fetch service info");
  return data;
}

function normalizeFooterMessage(message: unknown): string {
  if (typeof message !== "string") return DEFAULT_FOOTER_MESSAGE;
  const trimmed = message.trim();
  return trimmed || DEFAULT_FOOTER_MESSAGE;
}

export async function getFooterMessage(): Promise<string> {
  const res = await fetchWithTimeout(`${API_URL}/api/footer-message`, {
    headers: headers(),
    credentials: "include",
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok)
    throw new Error(
      (data as { error?: string }).error || "Failed to load footer message",
    );
  return normalizeFooterMessage((data as { message?: string }).message);
}

export async function updateFooterMessage(message: string): Promise<string> {
  const res = await fetchWithTimeout(`${API_URL}/api/admin/footer-message`, {
    method: "PUT",
    headers: { ...headers(), "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify({ message }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(
      (data as { error?: string }).error || "Failed to update footer message",
    );
  }
  return normalizeFooterMessage((data as { message?: string }).message);
}

export async function updateUploadPolicy(payload: {
  guestMaxBytes: number;
  registeredMaxBytes: number;
}): Promise<UploadPolicy> {
  const res = await fetchWithTimeout(`${API_URL}/api/admin/upload-policy`, {
    method: "PUT",
    headers: { ...headers(), "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify(payload),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(
      (data as { error?: string }).error || "Failed to update upload policy",
    );
  }
  return data as UploadPolicy;
}

// ── SMTP Settings ──

export interface SMTPSettings {
  host: string;
  port: number;
  user: string;
  password: string;
  from: string;
  use_tls: boolean;
  source: "env" | "admin" | "none";
}

export async function getSMTPSettings(): Promise<SMTPSettings> {
  const res = await fetchWithTimeout(`${API_URL}/api/admin/smtp-settings`, {
    headers: headers(),
    credentials: "include",
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok)
    throw new Error(
      (data as { error?: string }).error || "Failed to load SMTP settings",
    );
  return data as SMTPSettings;
}

export async function updateSMTPSettings(settings: {
  host: string;
  port: number;
  user: string;
  password: string;
  from: string;
  use_tls: boolean;
}): Promise<void> {
  const res = await fetchWithTimeout(`${API_URL}/api/admin/smtp-settings`, {
    method: "PUT",
    headers: { ...headers(), "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify(settings),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok)
    throw new Error(
      (data as { error?: string }).error || "Failed to save SMTP settings",
    );
}

export async function testSMTPConnection(): Promise<{
  success: boolean;
  message: string;
}> {
  const res = await fetchWithTimeout(`${API_URL}/api/admin/smtp-test`, {
    method: "POST",
    headers: headers(),
    credentials: "include",
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok)
    throw new Error(
      (data as { error?: string }).error || "Failed to test SMTP",
    );
  return data as { success: boolean; message: string };
}

// ── Email Templates ──

export interface EmailTemplate {
  key: string;
  subject: string;
  body_html: string;
  updated_at: string;
}

export async function getEmailTemplates(): Promise<EmailTemplate[]> {
  const res = await fetchWithTimeout(`${API_URL}/api/admin/email-templates`, {
    headers: headers(),
    credentials: "include",
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok)
    throw new Error(
      (data as { error?: string }).error || "Failed to load email templates",
    );
  return data as EmailTemplate[];
}

export async function getEmailTemplate(key: string): Promise<EmailTemplate> {
  const res = await fetchWithTimeout(
    `${API_URL}/api/admin/email-templates/${key}`,
    {
      headers: headers(),
      credentials: "include",
    },
  );
  const data = await res.json().catch(() => ({}));
  if (!res.ok)
    throw new Error(
      (data as { error?: string }).error || "Failed to load template",
    );
  return data as EmailTemplate;
}

export async function updateEmailTemplate(
  key: string,
  payload: { subject: string; body_html: string },
): Promise<EmailTemplate> {
  const res = await fetchWithTimeout(
    `${API_URL}/api/admin/email-templates/${key}`,
    {
      method: "PUT",
      headers: { ...headers(), "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify(payload),
    },
  );
  const data = await res.json().catch(() => ({}));
  if (!res.ok)
    throw new Error(
      (data as { error?: string }).error || "Failed to update template",
    );
  return data as EmailTemplate;
}

export async function previewEmailTemplate(
  key: string,
  draft?: { subject: string; body_html: string },
): Promise<{ subject: string; html: string }> {
  const res = await fetchWithTimeout(
    `${API_URL}/api/admin/email-templates/${key}/preview`,
    {
      method: "POST",
      headers: { ...headers(), "Content-Type": "application/json" },
      credentials: "include",
      body: draft ? JSON.stringify(draft) : "{}",
    },
  );
  const data = await res.json().catch(() => ({}));
  if (!res.ok)
    throw new Error(
      (data as { error?: string }).error || "Failed to preview template",
    );
  return data as { subject: string; html: string };
}

export async function createEmailTemplate(
  payload: { key: string; subject: string; body_html: string },
): Promise<EmailTemplate> {
  const res = await fetchWithTimeout(
    `${API_URL}/api/admin/email-templates`,
    {
      method: "POST",
      headers: { ...headers(), "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify(payload),
    },
  );
  const data = await res.json().catch(() => ({}));
  if (!res.ok)
    throw new Error(
      (data as { error?: string }).error || "Failed to create template",
    );
  return data as EmailTemplate;
}

export async function deleteEmailTemplate(key: string): Promise<void> {
  const res = await fetchWithTimeout(
    `${API_URL}/api/admin/email-templates/${encodeURIComponent(key)}`,
    {
      method: "DELETE",
      headers: headers(),
      credentials: "include",
    },
  );
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(
      (data as { error?: string }).error || "Failed to delete template",
    );
  }
}
