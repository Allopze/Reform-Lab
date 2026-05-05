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
          const updatedJobs = await Promise.all(
            nextJobIds.map((jobId) => getJob(jobId)),
          );
          const pendingJobs = updatedJobs.filter(
            (job) => job.status === "queued" || job.status === "running",
          );

          setItems((current) =>
            current.map((item) => {
              if (!item.jobId) {
                return item;
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

          if (pendingJobs.length > 0) {
            pollingRef.current = setTimeout(poll, 1500);
            return;
          }

          pollingRef.current = null;
          setActiveJobIds([]);
        } catch {
          pollingRef.current = null;
          setActiveJobIds([]);
          setItems((current) =>
            current.map((item) =>
              item.status === "converting"
                ? { ...item, status: "error", message: t("pollingError") }
                : item,
            ),
          );
        }
      };

      pollingRef.current = setTimeout(poll, 1000);
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
