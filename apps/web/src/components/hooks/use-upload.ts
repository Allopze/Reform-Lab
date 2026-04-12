"use client";

import { useState, useCallback, useEffect } from "react";
import { useTranslations } from "next-intl";
import {
  uploadFile,
  getCapabilities,
  getUploadPolicy,
  type Capability,
  type UploadPolicy,
} from "@/lib/api";
import { categoryIdFromDetectedFamily } from "@/config/categories";
import type { CategoryId, FileState } from "@/types";

const BYTES_PER_MB = 1024 * 1024;

function formatMegabytes(bytes: number) {
  return `${Math.round(bytes / BYTES_PER_MB)} MB`;
}

export interface UseUploadReturn {
  uploadPolicy: UploadPolicy | null;
  uploadedFileId: string | null;
  capabilities: Capability[];
  uploadError: string | null;
  detectedCategoryId: Exclude<CategoryId, "auto"> | null;
  handleFileSelected: (file: File) => Promise<void>;
  resetUpload: () => void;
}

export function useUpload(
  setFileState: (state: FileState) => void,
  setSelectedCapabilityId: (id: string) => void,
): UseUploadReturn {
  const t = useTranslations("upload");
  const [uploadPolicy, setUploadPolicy] = useState<UploadPolicy | null>(null);
  const [uploadedFileId, setUploadedFileId] = useState<string | null>(null);
  const [capabilities, setCapabilities] = useState<Capability[]>([]);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [detectedCategoryId, setDetectedCategoryId] = useState<Exclude<
    CategoryId,
    "auto"
  > | null>(null);

  useEffect(() => {
    getUploadPolicy()
      .then(setUploadPolicy)
      .catch(() => setUploadPolicy(null));
  }, []);

  const handleFileSelected = useCallback(
    async (file: File) => {
      setUploadError(null);
      setCapabilities([]);
      setUploadedFileId(null);
      setSelectedCapabilityId("");

      if (uploadPolicy && file.size > uploadPolicy.effectiveMaxBytes) {
        setUploadError(
          t("exceedsLimit", {
            limit: formatMegabytes(uploadPolicy.effectiveMaxBytes),
          }),
        );
        return;
      }

      setFileState({
        status: "selected",
        file,
        selectedCapabilityId: "",
        outputFormat: "",
      });

      try {
        const uploaded = await uploadFile(file);
        setUploadedFileId(uploaded.id);
        const nextCategoryId = categoryIdFromDetectedFamily(
          uploaded.detectedFormat.family,
        );
        setDetectedCategoryId(nextCategoryId);

        const caps = await getCapabilities(uploaded.id);
        setCapabilities(caps);

        const nextCapabilityId = caps[0]?.id ?? "";
        const nextOutputFormat = caps[0]?.targetFormat ?? "";
        setSelectedCapabilityId(nextCapabilityId);
        setFileState({
          status: "selected",
          file,
          selectedCapabilityId: nextCapabilityId,
          outputFormat: nextOutputFormat,
        });
      } catch (err) {
        setUploadedFileId(null);
        setUploadError(err instanceof Error ? err.message : t("genericError"));
        setCapabilities([]);
        setSelectedCapabilityId("");
      }
    },
    [uploadPolicy, setFileState, setSelectedCapabilityId, t],
  );

  const resetUpload = useCallback(() => {
    setDetectedCategoryId(null);
    setUploadedFileId(null);
    setCapabilities([]);
    setUploadError(null);
    setSelectedCapabilityId("");
  }, [setSelectedCapabilityId]);

  return {
    uploadPolicy,
    uploadedFileId,
    capabilities,
    uploadError,
    detectedCategoryId,
    handleFileSelected,
    resetUpload,
  };
}
