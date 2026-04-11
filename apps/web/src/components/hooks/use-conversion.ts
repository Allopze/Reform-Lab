"use client";

import { useState, useCallback, useRef, useEffect } from "react";
import { useTranslations } from "next-intl";
import {
  createConversion,
  getJob,
  downloadArtifact,
  cancelJob,
  type Capability,
} from "@/lib/api";
import type { FileState } from "@/types";

export interface UseConversionReturn {
  activeJobId: string | null;
  downloadError: string | null;
  handleConvert: () => Promise<void>;
  handleDownload: () => Promise<void>;
  handleCancel: () => Promise<void>;
  clearDownloadError: () => void;
  stopPolling: () => void;
}

export function useConversion(
  fileState: FileState,
  setFileState: (state: FileState) => void,
  uploadedFileId: string | null,
  capabilities: Capability[],
  outputFormat: string,
): UseConversionReturn {
  const t = useTranslations("conversion");
  const [activeJobId, setActiveJobId] = useState<string | null>(null);
  const [downloadError, setDownloadError] = useState<string | null>(null);
  const pollingRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (pollingRef.current) clearTimeout(pollingRef.current);
    };
  }, []);

  const stopPolling = useCallback(() => {
    if (pollingRef.current) {
      clearTimeout(pollingRef.current);
      pollingRef.current = null;
    }
  }, []);

  const handleConvert = useCallback(async () => {
    if (fileState.status !== "selected" || !uploadedFileId) return;

    const cap = capabilities.find((c) => c.targetFormat === outputFormat);
    if (!cap) {
      setFileState({
        status: "error",
        file: fileState.file,
        outputFormat,
        message: t("noCapability"),
      });
      return;
    }

    setFileState({
      status: "converting",
      file: fileState.file,
      outputFormat,
      progress: 0,
    });

    try {
      const job = await createConversion(uploadedFileId, cap.id);
      setActiveJobId(job.id);
      const timeoutMs = ((cap.timeoutSeconds ?? 90) + 30) * 1000;

      const INITIAL_DELAY = 1000;
      const MAX_DELAY = 15000;
      let elapsed = 0;
      let delay = INITIAL_DELAY;

      const pollOnce = async () => {
        elapsed += delay;
        if (elapsed > timeoutMs) {
          pollingRef.current = null;
          setActiveJobId(null);
          setFileState({
            status: "error",
            file: fileState.file,
            outputFormat,
            message: t("timeout"),
          });
          return;
        }

        try {
          const updated = await getJob(job.id);

          if (updated.status === "succeeded" && updated.artifactId) {
            pollingRef.current = null;
            setActiveJobId(null);
            setFileState({
              status: "done",
              file: fileState.file,
              outputFormat,
              artifactId: updated.artifactId,
              artifactFileName: updated.artifactFileName,
              artifactMimeType: updated.artifactMimeType,
              artifactSize: updated.artifactSize,
            });
          } else if (updated.status === "failed") {
            pollingRef.current = null;
            setActiveJobId(null);
            setFileState({
              status: "error",
              file: fileState.file,
              outputFormat,
              message: updated.error || t("failed"),
            });
          } else if (updated.status === "cancelled") {
            pollingRef.current = null;
            setActiveJobId(null);
            setFileState({ status: "idle" });
          } else {
            setFileState({
              status: "converting",
              file: fileState.file,
              outputFormat,
              progress: updated.progress,
            });
            delay = Math.min(delay * 2, MAX_DELAY);
            pollingRef.current = setTimeout(pollOnce, delay);
          }
        } catch {
          pollingRef.current = null;
          setActiveJobId(null);
          setFileState({
            status: "error",
            file: fileState.file,
            outputFormat,
            message: t("pollingError"),
          });
        }
      };

      pollingRef.current = setTimeout(pollOnce, delay);
    } catch (err: unknown) {
      setFileState({
        status: "error",
        file: fileState.file,
        outputFormat,
        message: err instanceof Error ? err.message : t("startError"),
      });
    }
  }, [fileState, uploadedFileId, capabilities, outputFormat, setFileState, t]);

  const handleDownload = useCallback(async () => {
    if (fileState.status !== "done") return;

    try {
      setDownloadError(null);
      await downloadArtifact(
        fileState.artifactId,
        fileState.artifactFileName || `${fileState.file.name}.${fileState.outputFormat}`,
      );
    } catch (err) {
      setDownloadError(
        err instanceof Error ? err.message : t("downloadError"),
      );
    }
  }, [fileState, t]);

  const handleCancel = useCallback(async () => {
    if (!activeJobId) return;
    try {
      await cancelJob(activeJobId);
      stopPolling();
      setActiveJobId(null);
      setFileState({ status: "idle" });
    } catch {
      // If cancel fails (e.g. already completed), ignore — polling will handle it.
    }
  }, [activeJobId, stopPolling, setFileState]);

  const clearDownloadError = useCallback(() => setDownloadError(null), []);

  return {
    activeJobId,
    downloadError,
    handleConvert,
    handleDownload,
    handleCancel,
    clearDownloadError,
    stopPolling,
  };
}
