"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import {
  cancelJobs,
  createBatchConversion,
  downloadArtifact,
  getJob,
  type Capability,
} from "@/lib/api";
import type { BatchFileItem } from "@/types";

function findCapabilityByID(
  capabilities: Capability[],
  capabilityID: string,
): Capability | undefined {
  return capabilities.find((capability) => capability.id === capabilityID);
}

function stripExtension(fileName: string): string {
  const lastDot = fileName.lastIndexOf(".");
  if (lastDot <= 0) return fileName;
  return fileName.slice(0, lastDot);
}

function extensionFromArtifactName(fileName?: string): string | null {
  if (!fileName) return null;
  const lastDot = fileName.lastIndexOf(".");
  if (lastDot <= 0 || lastDot === fileName.length - 1) return null;
  return fileName.slice(lastDot + 1);
}

export function getConvertedArtifactName(
  inputFileName: string,
  outputFormat: string,
  artifactFileName?: string,
): string {
  const extension = extensionFromArtifactName(artifactFileName) || outputFormat;
  return `${stripExtension(inputFileName)}.${extension}`;
}

const INITIAL_POLL_DELAY_MS = 1000;
const POLL_INTERVAL_MS = 1500;
const MAX_STATUS_POLL_FAILURES = 3;

export interface UseConversionReturn {
  activeJobIds: string[];
  downloadError: string | null;
  isConverting: boolean;
  handleConvert: () => Promise<void>;
  handleDownload: (item: BatchFileItem) => Promise<void>;
  handleCancel: () => Promise<void>;
  clearDownloadError: () => void;
  stopPolling: () => void;
}

export function useConversion(
  items: BatchFileItem[],
  setItems: React.Dispatch<React.SetStateAction<BatchFileItem[]>>,
  capabilities: Capability[],
  selectedCapabilityId: string,
): UseConversionReturn {
  const t = useTranslations("conversion");
  const [activeJobIds, setActiveJobIds] = useState<string[]>([]);
  const [downloadError, setDownloadError] = useState<string | null>(null);
  const pollingRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pollingFailuresRef = useRef<Map<string, number>>(new Map());
  const terminalJobIdsRef = useRef<Set<string>>(new Set());

  useEffect(() => {
    return () => {
      if (pollingRef.current) {
        clearTimeout(pollingRef.current);
      }
    };
  }, []);

  const stopPolling = useCallback(() => {
    if (pollingRef.current) {
      clearTimeout(pollingRef.current);
      pollingRef.current = null;
    }
    pollingFailuresRef.current.clear();
    terminalJobIdsRef.current.clear();
  }, []);

  const isConverting = useMemo(
    () => items.some((item) => item.status === "converting"),
    [items],
  );

  const handleConvert = useCallback(async () => {
    const selectedItems = items.filter(
      (item) => item.uploadedFileId && item.status === "selected",
    );
    if (selectedItems.length === 0) {
      return;
    }

    const cap = findCapabilityByID(capabilities, selectedCapabilityId);
    if (!cap) {
      setItems((current) =>
        current.map((item) =>
          item.status === "selected"
            ? { ...item, status: "error", message: t("noCapability") }
            : item,
        ),
      );
      return;
    }

    const fileIds = selectedItems
      .map((item) => item.uploadedFileId)
      .filter(Boolean) as string[];

    setItems((current) =>
      current.map((item) =>
        item.uploadedFileId && fileIds.includes(item.uploadedFileId)
          ? {
              ...item,
              status: "converting",
              progress: 0,
              message: undefined,
              selectedCapabilityId: cap.id,
              outputFormat: cap.targetFormat,
            }
          : item,
      ),
    );

    try {
      const jobs = await createBatchConversion(fileIds, cap.id);
      const nextJobIds = jobs.map((job) => job.id);
      setActiveJobIds(nextJobIds);
      pollingFailuresRef.current = new Map(
        nextJobIds.map((jobId) => [jobId, 0]),
      );
      terminalJobIdsRef.current = new Set();

      setItems((current) =>
        current.map((item) => {
          if (!item.uploadedFileId) {
            return item;
          }

          const job = jobs.find((candidate) => candidate.fileId === item.uploadedFileId);
          if (!job) {
            return item;
          }

          return {
            ...item,
            jobId: job.id,
            progress: job.progress,
            status: job.status === "failed" ? "error" : "converting",
            message: job.error,
          };
        }),
      );

      const poll = async () => {
        try {
          const pollJobIds = nextJobIds.filter(
            (jobId) => !terminalJobIdsRef.current.has(jobId),
          );
          if (pollJobIds.length === 0) {
            pollingRef.current = null;
            setActiveJobIds([]);
            pollingFailuresRef.current.clear();
            terminalJobIdsRef.current.clear();
            return;
          }

          const statusResults = await Promise.allSettled(
            pollJobIds.map(async (jobId) => ({
              jobId,
              job: await getJob(jobId),
            })),
          );

          const updatedJobs = statusResults.flatMap((result) =>
            result.status === "fulfilled" ? [result.value.job] : [],
          );
          const failedPollJobIds = statusResults.flatMap((result, index) =>
            result.status === "rejected" ? [pollJobIds[index]] : [],
          );
          const permanentlyUnavailableJobIds: string[] = [];

          for (const job of updatedJobs) {
            pollingFailuresRef.current.set(job.id, 0);
            if (
              job.status === "succeeded" ||
              job.status === "failed" ||
              job.status === "cancelled" ||
              job.status === "expired"
            ) {
              terminalJobIdsRef.current.add(job.id);
            }
          }

          for (const jobId of failedPollJobIds) {
            const nextFailures =
              (pollingFailuresRef.current.get(jobId) ?? 0) + 1;
            pollingFailuresRef.current.set(jobId, nextFailures);
            if (nextFailures >= MAX_STATUS_POLL_FAILURES) {
              permanentlyUnavailableJobIds.push(jobId);
            }
          }

          const pendingJobs = updatedJobs.filter(
            (job) => job.status === "queued" || job.status === "running",
          );
          const retryablePollJobIds = failedPollJobIds.filter(
            (jobId) => !permanentlyUnavailableJobIds.includes(jobId),
          );
          const unresolvedJobIds = [
            ...pendingJobs.map((job) => job.id),
            ...retryablePollJobIds,
          ];

          setItems((current) =>
            current.map((item) => {
              if (!item.jobId) {
                return item;
              }

              if (
                item.status === "converting" &&
                permanentlyUnavailableJobIds.includes(item.jobId)
              ) {
                return {
                  ...item,
                  status: "error",
                  message: t("pollingError"),
                };
              }

              const job = updatedJobs.find(
                (candidate) => candidate.id === item.jobId,
              );
              if (!job) {
                return item;
              }

              if (job.status === "succeeded" && job.artifactId) {
                return {
                  ...item,
                  status: "done",
                  progress: 100,
                  artifactId: job.artifactId,
                  artifactFileName: job.artifactFileName,
                  artifactMimeType: job.artifactMimeType,
                  artifactSize: job.artifactSize,
                  message: undefined,
                };
              }

              if (job.status === "failed") {
                return {
                  ...item,
                  status: "error",
                  message: job.error || t("failed"),
                };
              }

              if (job.status === "cancelled") {
                return {
                  ...item,
                  status: "selected",
                  progress: 0,
                  jobId: undefined,
                  message: undefined,
                };
              }

              if (job.status === "expired") {
                return {
                  ...item,
                  status: "error",
                  message: t("expired"),
                };
              }

              return {
                ...item,
                status: "converting",
                progress: job.progress,
              };
            }),
          );

          if (unresolvedJobIds.length > 0) {
            setActiveJobIds(unresolvedJobIds);
            pollingRef.current = setTimeout(poll, POLL_INTERVAL_MS);
            return;
          }

          pollingRef.current = null;
          setActiveJobIds([]);
          pollingFailuresRef.current.clear();
          terminalJobIdsRef.current.clear();
        } catch {
          pollingRef.current = null;
          setActiveJobIds([]);
          pollingFailuresRef.current.clear();
          terminalJobIdsRef.current.clear();
          setItems((current) =>
            current.map((item) =>
              item.status === "converting"
                ? { ...item, status: "error", message: t("pollingError") }
                : item,
            ),
          );
        }
      };

      pollingRef.current = setTimeout(poll, INITIAL_POLL_DELAY_MS);
    } catch (err: unknown) {
      setItems((current) =>
        current.map((item) =>
          item.status === "converting"
            ? {
                ...item,
                status: "error",
                message: err instanceof Error ? err.message : t("startError"),
              }
            : item,
        ),
      );
    }
  }, [capabilities, items, selectedCapabilityId, setItems, t]);

  const handleDownload = useCallback(
    async (item: BatchFileItem) => {
      if (!item.artifactId) {
        return;
      }

      try {
        setDownloadError(null);
        await downloadArtifact(
          item.artifactId,
          getConvertedArtifactName(
            item.file.name,
            item.outputFormat,
            item.artifactFileName,
          ),
        );
      } catch (err) {
        setDownloadError(err instanceof Error ? err.message : t("downloadError"));
      }
    },
    [t],
  );

  const handleCancel = useCallback(async () => {
    if (activeJobIds.length === 0) {
      return;
    }

    try {
      await cancelJobs(activeJobIds);
      stopPolling();
      setActiveJobIds([]);
      setItems((current) =>
        current.map((item) =>
          item.jobId && activeJobIds.includes(item.jobId)
            ? {
                ...item,
                status: "selected",
                progress: 0,
                jobId: undefined,
                message: undefined,
              }
            : item,
        ),
      );
    } catch {
      // If cancellation races with completion, polling or the next refresh will settle state.
    }
  }, [activeJobIds, setItems, stopPolling]);

  const clearDownloadError = useCallback(() => setDownloadError(null), []);

  return {
    activeJobIds,
    downloadError,
    isConverting,
    handleConvert,
    handleDownload,
    handleCancel,
    clearDownloadError,
    stopPolling,
  };
}
