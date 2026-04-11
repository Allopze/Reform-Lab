"use client";

import { useState, useCallback, useRef, useEffect, type RefObject } from "react";
import { CheckCircle2 } from "lucide-react";
import type { CategoryConfig, FileState } from "@/types";
import { categoryIdFromDetectedFamily, getCategoryById } from "@/config/categories";
import {
  uploadFile,
  getCapabilities,
  createConversion,
  getJob,
  downloadArtifact,
  cancelJob,
  getUploadPolicy,
  type Capability,
  type UploadPolicy,
} from "@/lib/api";
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

function uploadSupportLabel(policy: UploadPolicy | null, fallback: string) {
  if (!policy) return fallback;
  return policy.viewerType === "registered"
    ? `hasta ${formatMegabytes(policy.effectiveMaxBytes)} con tu cuenta`
    : `hasta ${formatMegabytes(policy.effectiveMaxBytes)} como invitado`;
}

function uploadPolicyDetail(policy: UploadPolicy | null) {
  if (!policy) return null;
  if (policy.viewerType === "registered") {
    return `Tu limite actual es ${formatMegabytes(policy.effectiveMaxBytes)} por archivo.`;
  }
  return `Como invitado puedes subir hasta ${formatMegabytes(policy.effectiveMaxBytes)} por archivo; con cuenta registrada, hasta ${formatMegabytes(policy.registeredMaxBytes)}.`;
}

export default function ConversionCard({ category }: ConversionCardProps) {
  const isAutoCategory = category.id === "auto";
  const [detectedCategoryId, setDetectedCategoryId] = useState<Exclude<CategoryConfig["id"], "auto"> | null>(null);
  const [fileState, setFileState] = useState<FileState>({ status: "idle" });
  const [outputFormat, setOutputFormat] = useState("");
  const detectedCategory = detectedCategoryId
    ? getCategoryById(detectedCategoryId)
    : null;
  const effectiveCategory = isAutoCategory && detectedCategory
    ? detectedCategory
    : category;
  const detailLabel = isAutoCategory
    ? detectedCategory
      ? `Detectamos ${detectedCategory.label.toLowerCase()} y habilitamos ${detectedCategory.targetFormats.map((format) => format.label).join(", ")}.`
      : "Detecta el formato real del archivo y habilita solo conversiones compatibles."
    : `Convierte a ${effectiveCategory.targetFormats
        .map((format) => format.label)
        .join(", ")}.`;

  // Track uploaded file ID and capabilities from backend
  const [uploadedFileId, setUploadedFileId] = useState<string | null>(null);
  const [capabilities, setCapabilities] = useState<Capability[]>([]);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [downloadError, setDownloadError] = useState<string | null>(null);
  const [activeJobId, setActiveJobId] = useState<string | null>(null);
  const [uploadPolicy, setUploadPolicy] = useState<UploadPolicy | null>(null);
  const availableTargetFormats = capabilities.map((capability) => ({
    value: capability.targetFormat,
    label: capability.displayName,
  }));
  const canChooseOutput = availableTargetFormats.length > 0;
  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const outputFormatRef = useRef(outputFormat);
  outputFormatRef.current = outputFormat;

  // Fade transition on category change
  const [faded, setFaded] = useState(false);
  const prevCategoryIdRef = useRef(category.id);

  useEffect(() => {
    getUploadPolicy()
      .then(setUploadPolicy)
      .catch(() => {
        setUploadPolicy(null);
      });
  }, []);

  useEffect(() => {
    if (prevCategoryIdRef.current !== category.id) {
      setFaded(true);
      const timer = setTimeout(() => {
        if (pollingRef.current) {
          clearInterval(pollingRef.current);
          pollingRef.current = null;
        }
        setDetectedCategoryId(null);
        setUploadedFileId(null);
        setCapabilities([]);
        setUploadError(null);
        setDownloadError(null);
        setActiveJobId(null);
        setOutputFormat("");
        setFileState({ status: "idle" });
        prevCategoryIdRef.current = category.id;
        setFaded(false);
      }, 150);
      return () => clearTimeout(timer);
    }
  }, [category.id]);

  // Cleanup polling interval on unmount
  useEffect(() => {
    return () => {
      if (pollingRef.current) clearInterval(pollingRef.current);
    };
  }, []);

  const handleFileSelected = useCallback(
    async (file: File) => {
      setDownloadError(null);
      setUploadError(null);
      setCapabilities([]);
      setUploadedFileId(null);
      setFileState({ status: "selected", file, outputFormat: outputFormatRef.current });

      // Upload file to backend and fetch capabilities
      try {
        const uploaded = await uploadFile(file);
        setUploadedFileId(uploaded.id);
        const nextCategoryId = categoryIdFromDetectedFamily(
          uploaded.detectedFormat.family
        );
        setDetectedCategoryId(nextCategoryId);

        const caps = await getCapabilities(uploaded.id);
        setCapabilities(caps);

        const nextOutputFormat = caps[0]?.targetFormat ?? "";
        setOutputFormat(nextOutputFormat);
        setFileState({ status: "selected", file, outputFormat: nextOutputFormat });
      } catch (err) {
        setUploadedFileId(null);
        setUploadError(
          err instanceof Error ? err.message : "Error al subir el archivo."
        );
        setCapabilities([]);
      }
    },
    []
  );

  const handleRemoveFile = useCallback(() => {
    if (pollingRef.current) clearInterval(pollingRef.current);
    setDetectedCategoryId(null);
    setUploadedFileId(null);
    setCapabilities([]);
    setUploadError(null);
    setDownloadError(null);
    setActiveJobId(null);
    setOutputFormat("");
    setFileState({ status: "idle" });
  }, []);

  const handleOutputFormatChange = useCallback(
    (value: string) => {
      setOutputFormat(value);
      if (fileState.status === "selected") {
        setFileState({ ...fileState, outputFormat: value });
      }
    },
    [fileState]
  );

  const handleConvert = useCallback(async () => {
    if (fileState.status !== "selected" || !uploadedFileId) return;

    // Find the capability matching the chosen output format
    const cap = capabilities.find((c) => c.targetFormat === outputFormat);
    if (!cap) {
      setFileState({
        status: "error",
        file: fileState.file,
        outputFormat,
        message: "No hay capacidad disponible para este formato de salida.",
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
      const maxPolls = Math.max(
        40,
        Math.ceil(((cap.timeoutSeconds ?? 90) + 30) * 1000 / 1500)
      );

      // Poll job status until terminal using backend-provided timeout as the baseline.
      let pollCount = 0;
      pollingRef.current = setInterval(async () => {
        pollCount++;
        if (pollCount > maxPolls) {
          if (pollingRef.current) {
            clearInterval(pollingRef.current);
            pollingRef.current = null;
          }
          setActiveJobId(null);
          setFileState({
            status: "error",
            file: fileState.file,
            outputFormat,
            message: "La conversión tardó demasiado. Intenta de nuevo.",
          });
          return;
        }

        try {
          const updated = await getJob(job.id);

          if (updated.status === "succeeded" && updated.artifactId) {
            if (pollingRef.current) {
              clearInterval(pollingRef.current);
              pollingRef.current = null;
            }
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
            if (pollingRef.current) {
              clearInterval(pollingRef.current);
              pollingRef.current = null;
            }
            setActiveJobId(null);
            setFileState({
              status: "error",
              file: fileState.file,
              outputFormat,
              message: updated.error || "La conversión falló.",
            });
          } else if (updated.status === "cancelled") {
            if (pollingRef.current) {
              clearInterval(pollingRef.current);
              pollingRef.current = null;
            }
            setActiveJobId(null);
            setFileState({ status: "idle" });
          } else {
            setFileState({
              status: "converting",
              file: fileState.file,
              outputFormat,
              progress: updated.progress,
            });
          }
        } catch {
          if (pollingRef.current) {
            clearInterval(pollingRef.current);
            pollingRef.current = null;
          }
          setActiveJobId(null);
          setFileState({
            status: "error",
            file: fileState.file,
            outputFormat,
            message: "Error al consultar el estado de la conversión.",
          });
        }
      }, 1500);
    } catch (err: unknown) {
      setFileState({
        status: "error",
        file: fileState.file,
        outputFormat,
        message: err instanceof Error ? err.message : "Error al iniciar la conversión.",
      });
    }
  }, [fileState, uploadedFileId, capabilities, outputFormat]);

  const handleDownload = useCallback(async () => {
    if (fileState.status !== "done") return;

    try {
      setDownloadError(null);
      await downloadArtifact(
        fileState.artifactId,
        fileState.artifactFileName || `${fileState.file.name}.${fileState.outputFormat}`
      );
    } catch (err) {
      setDownloadError(
        err instanceof Error ? err.message : "No se pudo descargar el artefacto."
      );
    }
  }, [fileState]);

  const handleCancel = useCallback(async () => {
    if (!activeJobId) return;
    try {
      await cancelJob(activeJobId);
      if (pollingRef.current) clearInterval(pollingRef.current);
      setActiveJobId(null);
      setFileState({ status: "idle" });
    } catch {
      // If cancel fails (e.g. already completed), ignore — polling will handle it.
    }
  }, [activeJobId]);

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
            text={effectiveCategory.dropzoneText}
            hint={effectiveCategory.dropzoneHint}
            supportLabel={uploadSupportLabel(uploadPolicy, effectiveCategory.supportLabel)}
            detailLabel={effectiveDetailLabel}
            accept={effectiveCategory.acceptedMimeTypes}
            onFileSelected={handleFileSelected}
          />
        ) : fileState.status === "error" ? (
          <div className="rounded-[28px] border border-rose-200 bg-rose-50 px-6 py-6 text-left">
            <p className="text-base font-semibold text-rose-800">
              Archivo no compatible con detección automática
            </p>
            <p className="mt-2 text-sm leading-6 text-rose-700">
              {fileState.message}
            </p>
            <button
              type="button"
              onClick={handleRemoveFile}
              className="mt-4 text-sm font-medium text-rose-800 underline underline-offset-2"
            >
              Probar otro archivo
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
            Formato detectado: <span className="text-stone-800">{detectedCategory.label}</span>
          </p>
        ) : null}

        {canChooseOutput ? (
          <div className="mt-6">
            <FormatSelector
              label="Salida disponible"
              options={availableTargetFormats}
              value={outputFormat}
              onChange={handleOutputFormatChange}
              id={`format-${category.id}`}
            />
          </div>
        ) : uploadedFileId ? (
          <div className="mt-6 rounded-2xl border border-stone-200 bg-stone-50 px-4 py-4 text-sm text-stone-500">
            Este archivo no tiene capacidades disponibles con la politica actual.
          </div>
        ) : null}

        {uploadError && (
          <div className="mt-4 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
            {uploadError} — la conversión no estará disponible hasta que se suba correctamente.
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
                Convirtiendo… {fileState.status === "converting" ? fileState.progress : 0}%
              </p>
              <button
                type="button"
                onClick={() => void handleCancel()}
                className="text-sm font-medium text-stone-400 underline underline-offset-2 hover:text-stone-600"
              >
                Cancelar
              </button>
            </div>
          </div>
        )}

        {isDone && fileState.status === "done" && (
          <div className="mt-5 flex flex-col gap-3 rounded-2xl border border-emerald-200 bg-emerald-50 p-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <p className="flex items-center gap-2 text-sm font-medium text-emerald-800">
                <CheckCircle2 size={16} strokeWidth={2} />
                Conversión completada
              </p>
              <p className="mt-1 text-sm text-emerald-900/80">
                {doneIsArchive
                  ? `La salida incluye varios archivos y se agrupó como ${doneArtifactName}.`
                  : `Artefacto listo: ${doneArtifactName}.`}
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
                {doneIsArchive ? "Descargar ZIP" : "Descargar archivo"}
              </a>
              <button
                type="button"
                onClick={handleRemoveFile}
                className="text-sm font-medium text-coral-700 underline underline-offset-2 hover:text-coral-800"
              >
                Convertir otro archivo
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
            ? "Convirtiendo…"
            : isDone
              ? "Completado"
                : effectiveCategory.cta}
        </button>
      </div>
    </div>
  );
}
