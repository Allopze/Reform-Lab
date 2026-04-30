"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import {
  getBatchCapabilities,
  getUploadPolicy,
  uploadFile,
  type Capability,
  type UploadPolicy,
} from "@/lib/api";
import { categoryIdFromDetectedFamily } from "@/config/categories";
import type { BatchFileItem, CategoryId } from "@/types";

const BYTES_PER_MB = 1024 * 1024;

function formatMegabytes(bytes: number) {
  return `${Math.round(bytes / BYTES_PER_MB)} MB`;
}

function cumulativeRemaining(policy: UploadPolicy) {
  const quota = policy.cumulativeQuotaBytes ?? policy.effectiveMaxBytes;
  const used = policy.cumulativeUsedBytes ?? 0;
  return Math.max(0, quota - used);
}

function makeLocalId(file: File, index: number) {
  return `${file.name}-${file.size}-${index}-${crypto.randomUUID()}`;
}

function applySharedCapability(
  items: BatchFileItem[],
  capabilityId: string,
  outputFormat: string,
) {
  return items.map((item) => {
    if (!item.uploadedFileId || item.status === "error") {
      return item;
    }

    return {
      ...item,
      selectedCapabilityId: capabilityId,
      outputFormat,
    };
  });
}

function resolveDetectedCategory(
  items: BatchFileItem[],
): Exclude<CategoryId, "auto"> | null {
  const families = Array.from(
    new Set(items.map((item) => item.detectedFamily).filter(Boolean)),
  );

  if (families.length !== 1) {
    return null;
  }

  return categoryIdFromDetectedFamily(families[0]!);
}

export interface UseUploadReturn {
  uploadPolicy: UploadPolicy | null;
  items: BatchFileItem[];
  capabilities: Capability[];
  uploadError: string | null;
  detectedCategoryId: Exclude<CategoryId, "auto"> | null;
  handleFilesSelected: (files: File[]) => Promise<void>;
  removeFile: (localId: string) => Promise<void>;
  resetUpload: () => void;
  setItems: React.Dispatch<React.SetStateAction<BatchFileItem[]>>;
}

export function useUpload(
  setSelectedCapabilityId: (id: string) => void,
): UseUploadReturn {
  const t = useTranslations("upload");
  const [uploadPolicy, setUploadPolicy] = useState<UploadPolicy | null>(null);
  const [items, setItems] = useState<BatchFileItem[]>([]);
  const [capabilities, setCapabilities] = useState<Capability[]>([]);
  const [uploadError, setUploadError] = useState<string | null>(null);

  useEffect(() => {
    getUploadPolicy()
      .then(setUploadPolicy)
      .catch(() => setUploadPolicy(null));
  }, []);

  const syncCapabilities = useCallback(
    async (nextItems: BatchFileItem[]) => {
      const uploadedIds = nextItems
        .filter((item) => item.uploadedFileId && item.status !== "error")
        .map((item) => item.uploadedFileId!)
        .filter(Boolean);

      if (uploadedIds.length === 0) {
        setCapabilities([]);
        setSelectedCapabilityId("");
        return nextItems;
      }

      const nextCapabilities = await getBatchCapabilities(uploadedIds);
      const nextCapabilityId = nextCapabilities[0]?.id ?? "";
      const nextOutputFormat = nextCapabilities[0]?.targetFormat ?? "";

      setCapabilities(nextCapabilities);
      setSelectedCapabilityId(nextCapabilityId);

      return applySharedCapability(
        nextItems,
        nextCapabilityId,
        nextOutputFormat,
      );
    },
    [setSelectedCapabilityId],
  );

  const handleFilesSelected = useCallback(
    async (files: File[]) => {
      setUploadError(null);
      setCapabilities([]);
      setSelectedCapabilityId("");

      const nextItems: BatchFileItem[] = files.map((file, index) => ({
        localId: makeLocalId(file, index),
        file,
        selectedCapabilityId: "",
        outputFormat: "",
        status: "uploading" as const,
        progress: 0,
      }));

      setItems(nextItems);

      let reservedBytes = 0;
      for (let index = 0; index < nextItems.length; index += 1) {
        const fileSize = nextItems[index].file.size;
        if (
          uploadPolicy &&
          fileSize > uploadPolicy.effectiveMaxBytes
        ) {
          nextItems[index] = {
            ...nextItems[index],
            status: "error",
            message: t("exceedsLimit", {
              limit: formatMegabytes(uploadPolicy.effectiveMaxBytes),
            }),
          };
          setItems([...nextItems]);
          continue;
        }
        if (
          uploadPolicy &&
          fileSize > cumulativeRemaining(uploadPolicy) - reservedBytes
        ) {
          const remaining = Math.max(0, cumulativeRemaining(uploadPolicy) - reservedBytes);
          nextItems[index] = {
            ...nextItems[index],
            status: "error",
            message: t("exceedsQuota", {
              remaining: formatMegabytes(remaining),
            }),
          };
          setItems([...nextItems]);
          continue;
        }

        try {
          const uploaded = await uploadFile(nextItems[index].file);
          reservedBytes += fileSize;
          setUploadPolicy((current) =>
            current
              ? {
                  ...current,
                  cumulativeUsedBytes: current.cumulativeUsedBytes + fileSize,
                }
              : current,
          );
          nextItems[index] = {
            ...nextItems[index],
            uploadedFileId: uploaded.id,
            detectedFamily: uploaded.detectedFormat.family,
            status: "selected",
            message: undefined,
          };
        } catch (err) {
          nextItems[index] = {
            ...nextItems[index],
            status: "error",
            message:
              err instanceof Error ? err.message : t("genericError"),
          };
        }

        setItems([...nextItems]);
      }

      try {
        const resolvedItems = await syncCapabilities(nextItems);
        setItems(resolvedItems);
      } catch (err) {
        setUploadError(err instanceof Error ? err.message : t("genericError"));
        setItems(nextItems);
      }
    },
    [setSelectedCapabilityId, syncCapabilities, t, uploadPolicy],
  );

  const removeFile = useCallback(
    async (localId: string) => {
      setUploadError(null);
      const filteredItems = items.filter((item) => item.localId !== localId);

      try {
        const resolvedItems = await syncCapabilities(filteredItems);
        setItems(resolvedItems);
      } catch (err) {
        setUploadError(err instanceof Error ? err.message : t("genericError"));
        setItems(filteredItems);
      }
    },
    [items, syncCapabilities, t],
  );

  const resetUpload = useCallback(() => {
    setItems([]);
    setCapabilities([]);
    setUploadError(null);
    setSelectedCapabilityId("");
  }, [setSelectedCapabilityId]);

  const detectedCategoryId = useMemo(
    () => resolveDetectedCategory(items),
    [items],
  );

  return {
    uploadPolicy,
    items,
    capabilities,
    uploadError,
    detectedCategoryId,
    handleFilesSelected,
    removeFile,
    resetUpload,
    setItems,
  };
}
