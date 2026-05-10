import { API_URL } from "./config";
import { csrfHeaders } from "./csrf";
import { DEFAULT_FOOTER_MESSAGE } from "./footer-message";

function headers(): HeadersInit {
  return csrfHeaders();
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
  cumulativeQuotaBytes: number;
  cumulativeUsedBytes: number;
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

export interface CatalogCapability extends Capability {
	sourceFormats: string[];
	family: string;
	maxInputBytes: number;
	maxRetries: number;
	expectedQuality?: string;
	knownLimitations?: string[];
}

export interface CatalogFamily {
	family: string;
	capabilities: CatalogCapability[];
}

export async function getCatalog(): Promise<CatalogFamily[]> {
	const res = await fetchWithTimeout(`${API_URL}/api/catalog`, {
		headers: headers(),
		credentials: "include",
	});

	const data = await res.json().catch(() => ({}));
	if (!res.ok) {
		throw new Error(
			(data as { error?: string }).error || "Failed to load catalog",
		);
	}
	return (data as { families: CatalogFamily[] }).families;
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

// ── Admin Jobs ──

export interface AdminJobFilter {
  status?: string;
  capability?: string;
  q?: string;
  stalled?: boolean;
  limit?: number;
  offset?: number;
}

export interface AdminJobRow {
  jobId: string;
  userName: string;
  userEmail: string;
  fileName: string;
  capabilityId: string;
  outputFormat: string;
  status: Job["status"];
  error?: string;
  createdAt: string;
  updatedAt: string;
  stalled: boolean;
  stalledReason?: "queued_too_long" | "running_too_long";
  backlogAgeSec?: number;
}

export interface AdminJobPage {
  jobs: AdminJobRow[];
  total: number;
  stalledJobs: number;
  stalledQueuedJobs: number;
  stalledRunningJobs: number;
}

interface AdminBatchJobFilterPayload {
  status?: string;
  capabilityId?: string;
  search?: string;
  stalledOnly?: boolean;
}

function toAdminBatchJobFilter(
  filter: AdminJobFilter,
): AdminBatchJobFilterPayload {
  return {
    ...(filter.status ? { status: filter.status } : {}),
    ...(filter.capability ? { capabilityId: filter.capability } : {}),
    ...(filter.q ? { search: filter.q } : {}),
    ...(filter.stalled ? { stalledOnly: true } : {}),
  };
}

export async function getAdminJobs(
  filter: AdminJobFilter = {},
): Promise<AdminJobPage> {
  const params = new URLSearchParams();
  if (filter.status) params.set("status", filter.status);
  if (filter.capability) params.set("capability", filter.capability);
  if (filter.q) params.set("q", filter.q);
  if (filter.stalled) params.set("stalled", "true");
  if (filter.limit) params.set("limit", String(filter.limit));
  if (filter.offset) params.set("offset", String(filter.offset));

  const qs = params.toString();
  const url = `${API_URL}/api/admin/jobs${qs ? `?${qs}` : ""}`;
  const res = await fetchWithTimeout(url, {
    headers: headers(),
    credentials: "include",
  });

  const data = await res.json();
  if (!res.ok) throw new Error(data.error || "Failed to load admin jobs");
  return data;
}

// ── Admin Users ──

export interface AdminUser {
  id: string;
  name: string;
  email: string;
  team?: string;
  role: "admin" | "user";
  isSuspended: boolean;
  suspendedReason?: string;
  sessionVersion: number;
  createdAt: string;
}

export interface AdminUserFilter {
  q?: string;
  role?: "admin" | "user";
  limit?: number;
  offset?: number;
}

export interface AdminUserPage {
  users: AdminUser[];
  total: number;
}

export async function getAdminUsers(
  filter: AdminUserFilter = {},
): Promise<AdminUserPage> {
  const params = new URLSearchParams();
  if (filter.q) params.set("q", filter.q);
  if (filter.role) params.set("role", filter.role);
  if (filter.limit) params.set("limit", String(filter.limit));
  if (filter.offset) params.set("offset", String(filter.offset));

  const qs = params.toString();
  const url = `${API_URL}/api/admin/users${qs ? `?${qs}` : ""}`;

  const res = await fetchWithTimeout(url, {
    headers: headers(),
    credentials: "include",
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || "Failed to load users");
  return data as AdminUserPage;
}

export async function updateUserRole(
  userId: string,
  role: "admin" | "user",
): Promise<void> {
  const res = await fetchWithTimeout(
    `${API_URL}/api/admin/users/${userId}/role`,
    {
      method: "PATCH",
      headers: { ...headers(), "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify({ role }),
    },
  );
  if (!res.ok) {
    const data = await res.json();
    throw new Error(data.error || "Failed to update role");
  }
}

export async function updateUserSuspension(
  userId: string,
  payload: { suspended: boolean; reason?: string },
): Promise<void> {
  const res = await fetchWithTimeout(
    `${API_URL}/api/admin/users/${userId}/suspension`,
    {
      method: "PATCH",
      headers: { ...headers(), "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify(payload),
    },
  );
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(
      (data as { error?: string }).error || "Failed to update suspension",
    );
  }
}

export async function revokeUserSessions(userId: string): Promise<number> {
  const res = await fetchWithTimeout(
    `${API_URL}/api/admin/users/${userId}/revoke-sessions`,
    {
      method: "POST",
      headers: headers(),
      credentials: "include",
    },
  );
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(
      (data as { error?: string }).error || "Failed to revoke sessions",
    );
  }
  return (data as { sessionVersion?: number }).sessionVersion ?? 0;
}

export async function cancelAdminJobs(input: {
  jobIds?: string[];
  filter?: AdminJobFilter;
}): Promise<string[]> {
  const body: { jobIds?: string[]; filter?: AdminBatchJobFilterPayload } = {};
  if (input.jobIds?.length) body.jobIds = input.jobIds;
  if (input.filter) body.filter = toAdminBatchJobFilter(input.filter);
  const res = await fetchWithTimeout(`${API_URL}/api/admin/jobs/batch/cancel`, {
    method: "POST",
    headers: { ...headers(), "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify(body),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(
      (data as { error?: string }).error || "Failed to cancel admin jobs",
    );
  }
  return (data as { cancelledJobIds?: string[] }).cancelledJobIds ?? [];
}

export async function retryAdminJobs(input: {
  jobIds?: string[];
  filter?: AdminJobFilter;
}): Promise<Job[]> {
  const body: { jobIds?: string[]; filter?: AdminBatchJobFilterPayload } = {};
  if (input.jobIds?.length) body.jobIds = input.jobIds;
  if (input.filter) body.filter = toAdminBatchJobFilter(input.filter);
  const res = await fetchWithTimeout(`${API_URL}/api/admin/jobs/batch/retry`, {
    method: "POST",
    headers: { ...headers(), "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify(body),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(
      (data as { error?: string }).error || "Failed to retry admin jobs",
    );
  }
  return (data as { jobs?: Job[] }).jobs ?? [];
}

// ── Admin Audit ──

export interface AdminAuditFilter {
  eventType?: string;
  group?: "admin";
  limit?: number;
  offset?: number;
}

export interface AdminAuditPage {
  events: AdminAuditEvent[];
  total: number;
}

export async function getAdminAudit(
  filter: AdminAuditFilter = {},
): Promise<AdminAuditPage> {
  const params = new URLSearchParams();
  if (filter.eventType) params.set("eventType", filter.eventType);
  if (filter.group) params.set("group", filter.group);
  if (filter.limit) params.set("limit", String(filter.limit));
  if (filter.offset) params.set("offset", String(filter.offset));

  const qs = params.toString();
  const url = `${API_URL}/api/admin/audit${qs ? `?${qs}` : ""}`;

  const res = await fetchWithTimeout(url, {
    headers: headers(),
    credentials: "include",
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(
      (data as { error?: string }).error || "Failed to load audit events",
    );
  }
  return data as AdminAuditPage;
}

export async function exportAdminAuditCSV(
  filter: Omit<AdminAuditFilter, "offset"> = {},
): Promise<void> {
  const params = new URLSearchParams();
  if (filter.eventType) params.set("eventType", filter.eventType);
  if (filter.group) params.set("group", filter.group);
  if (filter.limit) params.set("limit", String(filter.limit));

  const qs = params.toString();
  const url = `${API_URL}/api/admin/audit/export${qs ? `?${qs}` : ""}`;

  const res = await fetchWithTimeout(url, {
    headers: headers(),
    credentials: "include",
    timeoutMs: TRANSFER_TIMEOUT_MS,
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(
      (data as { error?: string }).error || "Failed to export audit events",
    );
  }

  const blob = await res.blob();
  const objectUrl = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = objectUrl;
  anchor.download =
    fileNameFromDisposition(res.headers.get("Content-Disposition")) ||
    "admin-audit.csv";
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(objectUrl);
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
  runtime: {
    queue: {
      mode: string;
      workerConcurrency: number;
      queuedJobs: number;
      runningJobs: number;
      stalledJobs: number;
      stalledQueuedJobs: number;
      stalledRunningJobs: number;
      controls: {
        jobIntakePaused?: boolean;
        pauseReason?: string;
        updatedAt?: string;
      };
      history: Array<{
        window: string;
        enqueuedJobs: number;
        failedJobs: number;
        completedJobs: number;
        averageLatencySec: number;
      }>;
    };
    storage: {
      status: "up" | "down" | "unknown";
      path?: string;
      totalBytes?: number;
      freeBytes?: number;
      usedPercent?: number;
      error?: string;
    };
    workers: {
      count: number;
      apiEngineMode?: "probed" | "declared" | string;
      apiEngineAvailability?: Record<string, boolean>;
      workers: Array<{
        id: string;
        runtimeMode: string;
        queueMode: string;
        lastHeartbeatAt: string;
        lastTaskType?: string;
        lastJobId?: string;
        lastTaskStatus: string;
        lastTaskStartedAt?: string;
        lastTaskFinishedAt?: string;
        lastError?: string;
        engines?: Record<string, boolean>;
        recentFailures: Array<{
          id: string;
          workerId: string;
          taskType?: string;
          jobId?: string;
          error: string;
          failedAt: string;
        }>;
      }>;
    };
  };
  dependencies: {
    database: {
      status: "up" | "down" | "unknown";
      latencyMs?: number;
      error?: string;
    };
    redis: {
      status: "up" | "down" | "not_configured";
      address?: string;
      latencyMs?: number;
      error?: string;
    };
  };
  alerts: {
    code: string;
    severity: "info" | "warning" | "critical";
    summary: string;
    description: string;
  }[];
}

export interface AdminEnginesInfo {
  engines: Record<string, boolean>;
  capabilities: Array<{
    id: string;
    displayName: string;
    engine: string;
    family: string;
    operationType: string;
    targetFormat: string;
    available: boolean;
    reason: "available" | "capability_disabled" | "engine_disabled" | "engine_unavailable";
  }>;
  availableCapabilities: number;
  totalCapabilities: number;
}

export async function getHealthInfo(): Promise<HealthInfo> {
  const res = await fetchWithTimeout(`${API_URL}/api/admin/health`, {
    credentials: "include",
  });
  const data = await res.json();
  if (!res.ok) throw new Error("Failed to fetch service info");
  return data;
}

export async function getAdminEngines(): Promise<AdminEnginesInfo> {
  const res = await fetchWithTimeout(`${API_URL}/api/admin/engines`, {
    credentials: "include",
  });
  const data = await res.json();
  if (!res.ok) throw new Error("Failed to fetch engines info");
  return data;
}

export interface JobIntakeControlState {
  jobIntakePaused: boolean;
  pauseReason?: string;
  updatedBy?: string;
  updatedAt: string;
}

export async function updateJobIntakeControl(input: {
  paused: boolean;
  reason?: string;
}): Promise<JobIntakeControlState> {
  const res = await fetchWithTimeout(`${API_URL}/api/admin/support/queue/intake`, {
    method: "PATCH",
    headers: { ...headers(), "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify(input),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(
      (data as { error?: string }).error || "Failed to update intake state",
    );
  }
  return data as JobIntakeControlState;
}

export interface DrainQueuedJobsResult {
  attempted: number;
  cancelled: number;
  skipped: number;
  cancelledIds: string[];
}

export async function drainQueuedJobs(limit?: number): Promise<DrainQueuedJobsResult> {
  const body = typeof limit === "number" ? { limit } : {};
  const res = await fetchWithTimeout(`${API_URL}/api/admin/support/queue/drain`, {
    method: "POST",
    headers: { ...headers(), "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify(body),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(
      (data as { error?: string }).error || "Failed to drain queued jobs",
    );
  }
  return {
    attempted: (data as { attempted?: number }).attempted ?? 0,
    cancelled: (data as { cancelled?: number }).cancelled ?? 0,
    skipped: (data as { skipped?: number }).skipped ?? 0,
    cancelledIds: (data as { cancelledIds?: string[] }).cancelledIds ?? [],
  };
}

export interface PruneWorkersResult {
  deleted: number;
  staleMinutes: number;
  cutoff: string;
}

export async function pruneStaleWorkers(staleMinutes: number): Promise<PruneWorkersResult> {
  const res = await fetchWithTimeout(`${API_URL}/api/admin/support/workers/prune-stale`, {
    method: "POST",
    headers: { ...headers(), "Content-Type": "application/json" },
    credentials: "include",
    body: JSON.stringify({ staleMinutes }),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(
      (data as { error?: string }).error || "Failed to prune stale workers",
    );
  }
  return {
    deleted: (data as { deleted?: number }).deleted ?? 0,
    staleMinutes: (data as { staleMinutes?: number }).staleMinutes ?? staleMinutes,
    cutoff: (data as { cutoff?: string }).cutoff ?? "",
  };
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
