"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { CheckCircle2 } from "lucide-react";
import { useTranslations } from "next-intl";
import type { BatchFileItem, CategoryConfig } from "@/types";
import { getCategoryById } from "@/config/categories";
import type { Capability, UploadPolicy } from "@/lib/api";
import { useUpload } from "./hooks/use-upload";
import {
  getConvertedArtifactName,
  useConversion,
} from "./hooks/use-conversion";
import Dropzone from "./dropzone";
import FormatSelector from "./format-selector";
import FilePreview from "./file-preview";

interface ConversionCardProps {
  category: CategoryConfig;
}

const BYTES_PER_MB = 1024 * 1024;

function isArchiveArtifact(
  artifactFileName?: string,
  artifactMimeType?: string,
) {
  if (artifactMimeType === "application/zip") return true;
  return artifactFileName?.toLowerCase().endsWith(".zip") ?? false;
}

function formatMegabytes(bytes: number) {
  return `${Math.round(bytes / BYTES_PER_MB)} MB`;
}

function getCapabilityByID(
  capabilities: Capability[],
  capabilityID: string,
): Capability | null {
  return (
    capabilities.find((capability) => capability.id === capabilityID) ?? null
  );
}

function itemStatusLabel(
  item: BatchFileItem,
  t: ReturnType<typeof useTranslations>,
) {
  switch (item.status) {
    case "uploading":
      return t("uploadingItem");
    case "selected":
      return t("selectedItem");
    case "converting":
      return t("converting", { progress: item.progress });
    case "done":
      return t("conversionDone");
    case "error":
      return item.message || t("errorTitle");
    default:
      return "";
  }
}

export default function ConversionCard({ category }: ConversionCardProps) {
  const t = useTranslations("conversionCard");
  const tc = useTranslations("categories");
  const tCommon = useTranslations("common");
  const isAutoCategory = category.id === "auto";
  const [selectedCapabilityId, setSelectedCapabilityId] = useState("");

  const {
    uploadPolicy,
    items,
    setItems,
    capabilities,
    uploadError,
    detectedCategoryId,
    handleFilesSelected,
    removeFile,
    resetUpload,
  } = useUpload(setSelectedCapabilityId);

  const {
    downloadError,
    activeJobIds,
    isConverting,
    handleConvert,
    handleDownload,
    handleCancel,
    clearDownloadError,
    stopPolling,
  } = useConversion(items, setItems, capabilities, selectedCapabilityId);

  const detectedCategory = detectedCategoryId
    ? getCategoryById(detectedCategoryId)
    : null;
  const effectiveCategory =
    isAutoCategory && detectedCategory ? detectedCategory : category;
  const effectiveCategoryId = effectiveCategory.id;

  function uploadSupportLabel(policy: UploadPolicy | null) {
    if (!policy) return tc(`${effectiveCategoryId}.supportLabel`);
    const cumulativeQuota = policy.cumulativeQuotaBytes ?? policy.effectiveMaxBytes;
    const cumulativeUsed = policy.cumulativeUsedBytes ?? 0;
    const remaining = Math.max(
      0,
      cumulativeQuota - cumulativeUsed,
    );
    return policy.viewerType === "registered"
      ? t("registeredSupport", {
          limit: formatMegabytes(policy.effectiveMaxBytes),
          remaining: formatMegabytes(remaining),
        })
      : t("guestSupport", {
          limit: formatMegabytes(policy.effectiveMaxBytes),
          remaining: formatMegabytes(remaining),
        });
  }

  function uploadPolicyDetail(policy: UploadPolicy | null) {
    if (!policy) return null;
    const quota = formatMegabytes(policy.cumulativeQuotaBytes ?? policy.effectiveMaxBytes);
    const used = formatMegabytes(policy.cumulativeUsedBytes ?? 0);
    if (policy.viewerType === "registered") {
      return t("registeredLimit", {
        limit: formatMegabytes(policy.effectiveMaxBytes),
        quota,
        used,
      });
    }
    return t("guestLimit", {
      guestLimit: formatMegabytes(policy.effectiveMaxBytes),
      registeredLimit: formatMegabytes(policy.registeredMaxBytes),
      quota,
      used,
    });
  }

  const detailLabel = isAutoCategory
    ? detectedCategory
      ? t("detectedDetail", {
          category: tc(`${detectedCategory.id}.label`).toLowerCase(),
          formats: detectedCategory.targetFormats
            .map((f) => f.label)
            .join(", "),
        })
      : items.length > 0
        ? t("mixedDetectedDetail")
        : t("defaultDetail")
    : t("categoryFormats", {
        formats: effectiveCategory.targetFormats.map((f) => f.label).join(", "),
      });

  const availableCapabilities = capabilities.map((capability) => ({
    value: capability.id,
    label: capability.displayName,
  }));
  const selectedCapability = getCapabilityByID(capabilities, selectedCapabilityId);
  const selectedOutputFormat = selectedCapability?.targetFormat ?? "";
  const selectedCapabilityLabel = selectedCapability?.displayName ?? "";

  const [faded, setFaded] = useState(false);
  const prevCategoryIdRef = useRef(category.id);

  useEffect(() => {
    if (prevCategoryIdRef.current !== category.id) {
      setFaded(true);
      const timer = setTimeout(() => {
        stopPolling();
        resetUpload();
        clearDownloadError();
        prevCategoryIdRef.current = category.id;
        setFaded(false);
      }, 150);
      return () => clearTimeout(timer);
    }
  }, [category.id, clearDownloadError, resetUpload, stopPolling]);

  const handleRemoveFile = useCallback(
    async (localId: string) => {
      if (isConverting) {
        return;
      }
      await removeFile(localId);
      clearDownloadError();
    },
    [clearDownloadError, isConverting, removeFile],
  );

  const handleCapabilityChange = useCallback(
    (value: string) => {
      const nextCapability = getCapabilityByID(capabilities, value);
      if (!nextCapability) {
        return;
      }

      setSelectedCapabilityId(value);
      setItems((current) =>
        current.map((item) =>
          item.status === "error" || !item.uploadedFileId
            ? item
            : {
                ...item,
                selectedCapabilityId: value,
                outputFormat: nextCapability.targetFormat,
              },
        ),
      );
    },
    [capabilities, setItems],
  );

  const selectedItems = useMemo(
    () => items.filter((item) => item.status === "selected"),
    [items],
  );
  const doneItems = useMemo(
    () => items.filter((item) => item.status === "done"),
    [items],
  );
  const errorItems = useMemo(
    () => items.filter((item) => item.status === "error"),
    [items],
  );
  const uploadedDetectedFamilies = useMemo(
    () =>
      Array.from(
        new Set(
          items
            .filter((item) => item.uploadedFileId && item.status !== "error")
            .map((item) => item.detectedFamily)
            .filter(Boolean),
        ),
      ),
    [items],
  );
  const hasMixedDetectedFamilies = uploadedDetectedFamilies.length > 1;
  const canStartConversion =
    selectedItems.length > 0 &&
    !isConverting &&
    !!selectedCapabilityId &&
    capabilities.length > 0;
  const effectiveDetailLabel = [detailLabel, uploadPolicyDetail(uploadPolicy)]
    .filter(Boolean)
    .join(" ");
  const trustDetailLabel = t("privacyRetentionDetail");

  return (
    <div
      role="tabpanel"
      id={`panel-${category.id}`}
      aria-labelledby={`tab-${category.id}`}
      className={`mx-auto w-full max-w-215 transition-opacity duration-150 ease-in-out ${faded ? "opacity-0" : "opacity-100"}`}
    >
      <div className="rounded-[34px] border border-white/80 bg-white px-7 py-7 shadow-[0_24px_60px_-42px_rgba(15,23,42,0.24)] sm:px-8 sm:py-8">
        {items.length === 0 ? (
          <Dropzone
            text={tc(`${effectiveCategoryId}.dropzoneText`)}
            hint={tc(`${effectiveCategoryId}.dropzoneHint`)}
            supportLabel={uploadSupportLabel(uploadPolicy)}
            detailLabel={`${effectiveDetailLabel} ${trustDetailLabel}`}
            accept={effectiveCategory.acceptedMimeTypes}
            onFilesSelected={handleFilesSelected}
          />
        ) : (
          <div className="space-y-3">
            {items.map((item) => {
              const capability = getCapabilityByID(
                capabilities,
                item.selectedCapabilityId,
              );
              const itemOutputFormat = item.outputFormat || selectedOutputFormat;
              const doneArtifactName = item.artifactId
                ? getConvertedArtifactName(
                    item.file.name,
                    itemOutputFormat,
                    item.artifactFileName,
                  )
                : null;
              const doneIsArchive = isArchiveArtifact(
                item.artifactFileName,
                item.artifactMimeType,
              );

              return (
                <div
                  key={item.localId}
                  className="rounded-[28px] border border-stone-200 bg-stone-50/65 p-3"
                >
                  <FilePreview
                    file={item.file}
                    selectionLabel={
                      capability?.displayName || selectedCapabilityLabel
                    }
                    outputFormat={itemOutputFormat}
                    onRemove={() => {
                      void handleRemoveFile(item.localId);
                    }}
                  />

                  <div className="mt-3 flex flex-col gap-2 px-2 text-sm text-stone-600 sm:flex-row sm:items-center sm:justify-between">
                    <p>{itemStatusLabel(item, t)}</p>
                    {item.status === "done" && item.artifactId ? (
                      <button
                        type="button"
                        onClick={() => {
                          void handleDownload(item);
                        }}
                        className="text-left font-medium text-emerald-700 underline underline-offset-2 hover:text-emerald-800"
                      >
                        {doneIsArchive
                          ? t("downloadArchive", { fileName: doneArtifactName ?? "" })
                          : t("downloadArtifact", { fileName: doneArtifactName ?? "" })}
                      </button>
                    ) : null}
                  </div>

                  {item.status === "converting" ? (
                    <div className="mt-3 px-2">
                      <div className="h-2 w-full overflow-hidden rounded-full bg-stone-100">
                        <progress
                          aria-label={t("converting", { progress: item.progress })}
                          className="conversion-progress"
                          max={100}
                          value={item.progress}
                        />
                      </div>
                    </div>
                  ) : null}

                  {item.status === "error" && item.message ? (
                    <p className="mt-3 rounded-xl border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
                      {item.message}
                    </p>
                  ) : null}
                </div>
              );
            })}
          </div>
        )}

        {items.length > 0 ? (
          <p className="mt-5 text-sm text-stone-500">
            {t("batchSummary", {
              total: items.length,
              ready: selectedItems.length,
              done: doneItems.length,
              failed: errorItems.length,
            })}
          </p>
        ) : null}

        {isAutoCategory && detectedCategory ? (
          <p className="mt-4 text-sm font-medium text-stone-500">
            {t("detectedFormat")} {" "}
            <span className="text-stone-800">
              {tc(`${detectedCategory.id}.label`)}
            </span>
          </p>
        ) : null}

        {availableCapabilities.length > 0 ? (
          <div className="mt-6">
            <FormatSelector
              label={t("outputLabel")}
              options={availableCapabilities}
              value={selectedCapabilityId}
              onChange={handleCapabilityChange}
              id={`format-${category.id}`}
            />
          </div>
        ) : items.some((item) => item.uploadedFileId) ? (
          <div className="mt-6 rounded-2xl border border-stone-200 bg-stone-50 px-4 py-4 text-sm text-stone-500">
            {hasMixedDetectedFamilies
              ? t("mixedBatchNoCapabilities")
              : t("noCapabilities")}
          </div>
        ) : null}

        {uploadError ? (
          <div className="mt-4 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
            {uploadError} {t("uploadErrorSuffix")}
          </div>
        ) : null}

        {downloadError ? (
          <div className="mt-4 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
            {downloadError}
          </div>
        ) : null}

        {isConverting ? (
          <div className="mt-5 flex items-center justify-between rounded-2xl border border-stone-200 bg-white p-4 text-sm text-stone-500">
            <p>
              {t("convertingBatch", {
                count: activeJobIds.length,
              })}
            </p>
            <button
              type="button"
              onClick={() => {
                void handleCancel();
              }}
              className="font-medium text-stone-400 underline underline-offset-2 hover:text-stone-600"
            >
              {tCommon("cancel")}
            </button>
          </div>
        ) : null}

        {doneItems.length > 0 && !isConverting ? (
          <div className="mt-5 flex flex-col gap-3 rounded-2xl border border-emerald-200 bg-emerald-50 p-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <p className="flex items-center gap-2 text-sm font-medium text-emerald-800">
                <CheckCircle2 size={16} strokeWidth={2} />
                {t("conversionDone")}
              </p>
              <p className="mt-1 text-sm text-emerald-900/80">
                {t("batchDoneMessage", { count: doneItems.length })}
              </p>
            </div>
            <button
              type="button"
              onClick={resetUpload}
              className="text-sm font-medium text-coral-700 underline underline-offset-2 hover:text-coral-800"
            >
              {t("convertAnother")}
            </button>
          </div>
        ) : null}

        <button
          type="button"
          disabled={!canStartConversion}
          onClick={() => {
            void handleConvert();
          }}
          className={`
            mt-6 w-full rounded-[22px] px-5 py-5 text-[16px] font-semibold
            transition-all duration-200
            ${
              canStartConversion
                ? "bg-coral-400 text-white hover:bg-coral-500 active:bg-coral-600"
                : "cursor-not-allowed bg-coral-200/75 text-white"
            }
          `}
        >
          {isConverting ? t("convertingButton") : tc(`${effectiveCategoryId}.cta`)}
        </button>
      </div>
    </div>
  );
}
