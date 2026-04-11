"use client";

import { useState, useCallback, useRef, useEffect } from "react";
import { CheckCircle2 } from "lucide-react";
import { useTranslations } from "next-intl";
import type { CategoryConfig, FileState } from "@/types";
import { getCategoryById } from "@/config/categories";
import type { UploadPolicy } from "@/lib/api";
import { useUpload } from "./hooks/use-upload";
import { useConversion } from "./hooks/use-conversion";
import Dropzone from "./dropzone";
import FormatSelector from "./format-selector";
import FilePreview from "./file-preview";

interface ConversionCardProps {
  category: CategoryConfig;
}

const BYTES_PER_MB = 1024 * 1024;

function isArchiveArtifact(artifactFileName?: string, artifactMimeType?: string) {
  if (artifactMimeType === "application/zip") return true;
  return artifactFileName?.toLowerCase().endsWith(".zip") ?? false;
}

function artifactLabel(fileState: Extract<FileState, { status: "done" }>) {
  return (
    fileState.artifactFileName ||
    `${fileState.file.name}.${fileState.outputFormat}`
  );
}

function formatMegabytes(bytes: number) {
  return `${Math.round(bytes / BYTES_PER_MB)} MB`;
}

export default function ConversionCard({ category }: ConversionCardProps) {
  const t = useTranslations("conversionCard");
  const tc = useTranslations("categories");
  const tCommon = useTranslations("common");
  const isAutoCategory = category.id === "auto";
  const [fileState, setFileState] = useState<FileState>({ status: "idle" });
  const [outputFormat, setOutputFormat] = useState("");
  const outputFormatRef = useRef(outputFormat);
  outputFormatRef.current = outputFormat;

  const {
    uploadPolicy,
    uploadedFileId,
    capabilities,
    uploadError,
    detectedCategoryId,
    handleFileSelected,
    resetUpload,
  } = useUpload(setFileState, outputFormatRef, setOutputFormat);

  const {
    downloadError,
    handleConvert,
    handleDownload,
    handleCancel,
    clearDownloadError,
    stopPolling,
  } = useConversion(fileState, setFileState, uploadedFileId, capabilities, outputFormat);

  const detectedCategory = detectedCategoryId
    ? getCategoryById(detectedCategoryId)
    : null;
  const effectiveCategory = isAutoCategory && detectedCategory
    ? detectedCategory
    : category;
  const effectiveCategoryId = effectiveCategory.id;

  function uploadSupportLabel(policy: UploadPolicy | null) {
    if (!policy) return tc(`${effectiveCategoryId}.supportLabel`);
    return policy.viewerType === "registered"
      ? t("registeredSupport", { limit: formatMegabytes(policy.effectiveMaxBytes) })
      : t("guestSupport", { limit: formatMegabytes(policy.effectiveMaxBytes) });
  }

  function uploadPolicyDetail(policy: UploadPolicy | null) {
    if (!policy) return null;
    if (policy.viewerType === "registered") {
      return t("registeredLimit", { limit: formatMegabytes(policy.effectiveMaxBytes) });
    }
    return t("guestLimit", {
      guestLimit: formatMegabytes(policy.effectiveMaxBytes),
      registeredLimit: formatMegabytes(policy.registeredMaxBytes),
    });
  }

  const detailLabel = isAutoCategory
    ? detectedCategory
      ? t("detectedDetail", {
          category: tc(`${detectedCategory.id}.label`).toLowerCase(),
          formats: detectedCategory.targetFormats.map((f) => f.label).join(", "),
        })
      : t("defaultDetail")
    : t("categoryFormats", {
        formats: effectiveCategory.targetFormats.map((f) => f.label).join(", "),
      });

  const availableTargetFormats = capabilities.map((capability) => ({
    value: capability.targetFormat,
    label: capability.displayName,
  }));
  const canChooseOutput = availableTargetFormats.length > 0;

  // Fade transition on category change
  const [faded, setFaded] = useState(false);
  const prevCategoryIdRef = useRef(category.id);

  useEffect(() => {
    if (prevCategoryIdRef.current !== category.id) {
      setFaded(true);
      const timer = setTimeout(() => {
        stopPolling();
        resetUpload();
        clearDownloadError();
        setOutputFormat("");
        setFileState({ status: "idle" });
        prevCategoryIdRef.current = category.id;
        setFaded(false);
      }, 150);
      return () => clearTimeout(timer);
    }
  }, [category.id, stopPolling, resetUpload, clearDownloadError]);

  const handleRemoveFile = useCallback(() => {
    stopPolling();
    resetUpload();
    clearDownloadError();
    setOutputFormat("");
    setFileState({ status: "idle" });
  }, [stopPolling, resetUpload, clearDownloadError]);

  const handleOutputFormatChange = useCallback(
    (value: string) => {
      setOutputFormat(value);
      if (fileState.status === "selected") {
        setFileState({ ...fileState, outputFormat: value });
      }
    },
    [fileState]
  );

  const isConverting = fileState.status === "converting";
  const isDone = fileState.status === "done";
  const doneArtifactName =
    fileState.status === "done" ? artifactLabel(fileState) : null;
  const doneIsArchive =
    fileState.status === "done"
      ? isArchiveArtifact(fileState.artifactFileName, fileState.artifactMimeType)
      : false;
  const hasFile =
    fileState.status === "selected" ||
    fileState.status === "converting" ||
    fileState.status === "done";
  const effectiveDetailLabel = [detailLabel, uploadPolicyDetail(uploadPolicy)]
    .filter(Boolean)
    .join(" ");

  return (
    <div
      role="tabpanel"
      id={`panel-${category.id}`}
      aria-labelledby={`tab-${category.id}`}
      className={`mx-auto w-full max-w-215 transition-opacity duration-150 ease-in-out ${faded ? "opacity-0" : "opacity-100"}`}
    >
      <div className="rounded-[34px] border border-white/80 bg-white px-7 py-7 shadow-[0_24px_60px_-42px_rgba(15,23,42,0.24)] sm:px-8 sm:py-8">
        {fileState.status === "idle" ? (
          <Dropzone
            text={tc(`${effectiveCategoryId}.dropzoneText`)}
            hint={tc(`${effectiveCategoryId}.dropzoneHint`)}
            supportLabel={uploadSupportLabel(uploadPolicy)}
            detailLabel={effectiveDetailLabel}
            accept={effectiveCategory.acceptedMimeTypes}
            onFileSelected={handleFileSelected}
          />
        ) : fileState.status === "error" ? (
          <div className="rounded-[28px] border border-rose-200 bg-rose-50 px-6 py-6 text-left">
            <p className="text-base font-semibold text-rose-800">
              {t("errorTitle")}
            </p>
            <p className="mt-2 text-sm leading-6 text-rose-700">
              {fileState.message}
            </p>
            <button
              type="button"
              onClick={handleRemoveFile}
              className="mt-4 text-sm font-medium text-rose-800 underline underline-offset-2"
            >
              {t("tryAnother")}
            </button>
          </div>
        ) : (
          <FilePreview
            file={fileState.file}
            outputFormat={outputFormat}
            onRemove={handleRemoveFile}
          />
        )}

        {isAutoCategory && detectedCategory ? (
          <p className="mt-6 text-sm font-medium text-stone-500">
            {t("detectedFormat")} <span className="text-stone-800">{tc(`${detectedCategory.id}.label`)}</span>
          </p>
        ) : null}

        {canChooseOutput ? (
          <div className="mt-6">
            <FormatSelector
              label={t("outputLabel")}
              options={availableTargetFormats}
              value={outputFormat}
              onChange={handleOutputFormatChange}
              id={`format-${category.id}`}
            />
          </div>
        ) : uploadedFileId ? (
          <div className="mt-6 rounded-2xl border border-stone-200 bg-stone-50 px-4 py-4 text-sm text-stone-500">
            {t("noCapabilities")}
          </div>
        ) : null}

        {uploadError && (
          <div className="mt-4 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
            {uploadError} {t("uploadErrorSuffix")}
          </div>
        )}

        {downloadError && (
          <div className="mt-4 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
            {downloadError}
          </div>
        )}

        {isConverting && (
          <div className="mt-5 rounded-2xl border border-stone-200 bg-white p-4">
            <div className="h-2 w-full overflow-hidden rounded-full bg-stone-100">
              <div
                className="h-full rounded-full bg-coral-500 transition-all duration-300"
                style={{
                  width: `${fileState.status === "converting" ? fileState.progress : 0}%`,
                }}
              />
            </div>
            <div className="mt-3 flex items-center justify-between">
              <p className="text-sm text-stone-500">
                {t("converting", { progress: fileState.status === "converting" ? fileState.progress : 0 })}
              </p>
              <button
                type="button"
                onClick={() => void handleCancel()}
                className="text-sm font-medium text-stone-400 underline underline-offset-2 hover:text-stone-600"
              >
                {tCommon("cancel")}
              </button>
            </div>
          </div>
        )}

        {isDone && fileState.status === "done" && (
          <div className="mt-5 flex flex-col gap-3 rounded-2xl border border-emerald-200 bg-emerald-50 p-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <p className="flex items-center gap-2 text-sm font-medium text-emerald-800">
                <CheckCircle2 size={16} strokeWidth={2} />
                {t("conversionDone")}
              </p>
              <p className="mt-1 text-sm text-emerald-900/80">
                {doneIsArchive
                  ? t("archiveMessage", { fileName: doneArtifactName ?? "" })
                  : t("artifactMessage", { fileName: doneArtifactName ?? "" })}
              </p>
            </div>
            <div className="flex items-center gap-3">
              <a
                href="#"
                onClick={(event) => {
                  event.preventDefault();
                  void handleDownload();
                }}
                className="text-sm font-medium text-emerald-700 underline underline-offset-2 hover:text-emerald-900"
              >
                {doneIsArchive ? t("downloadZip") : t("downloadFile")}
              </a>
              <button
                type="button"
                onClick={handleRemoveFile}
                className="text-sm font-medium text-coral-700 underline underline-offset-2 hover:text-coral-800"
              >
                {t("convertAnother")}
              </button>
            </div>
          </div>
        )}

        <button
          type="button"
          disabled={!hasFile || isConverting || isDone || !uploadedFileId || capabilities.length === 0}
          onClick={handleConvert}
          className={`
            mt-6 w-full rounded-[22px] px-5 py-5 text-[16px] font-semibold
            transition-all duration-200
            ${
              hasFile && !isConverting && !isDone
                ? "bg-coral-400 text-white hover:bg-coral-500 active:bg-coral-600"
                : "cursor-not-allowed bg-coral-200/75 text-white"
            }
          `}
        >
          {isConverting
            ? t("convertingButton")
            : isDone
              ? t("completed")
                : tc(`${effectiveCategoryId}.cta`)}
        </button>
      </div>
    </div>
  );
}
