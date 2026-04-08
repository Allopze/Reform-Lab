"use client";

import { useState, useCallback } from "react";
import { CheckCircle2 } from "lucide-react";
import type { CategoryConfig, FileState } from "@/types";
import { detectCategoryIdFromFile, getCategoryById } from "@/config/categories";
import Dropzone from "./dropzone";
import FormatSelector from "./format-selector";
import FilePreview from "./file-preview";

interface ConversionCardProps {
  category: CategoryConfig;
}

export default function ConversionCard({ category }: ConversionCardProps) {
  const isAutoCategory = category.id === "auto";
  const [detectedCategoryId, setDetectedCategoryId] = useState<Exclude<CategoryConfig["id"], "auto"> | null>(null);
  const [fileState, setFileState] = useState<FileState>({ status: "idle" });
  const [outputFormat, setOutputFormat] = useState(
    category.targetFormats[0]?.value ?? ""
  );
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
  const availableTargetFormats = effectiveCategory.targetFormats;
  const canChooseOutput = availableTargetFormats.length > 0;

  const handleFileSelected = useCallback(
    (file: File) => {
      if (isAutoCategory) {
        const nextCategoryId = detectCategoryIdFromFile(file);

        if (!nextCategoryId) {
          setDetectedCategoryId(null);
          setOutputFormat("");
          setFileState({
            status: "error",
            file,
            outputFormat: "",
            message:
              "No pudimos detectar un formato compatible. Prueba con PDF, imágenes, documentos, audio o video.",
          });
          return;
        }

        const nextCategory = getCategoryById(nextCategoryId);
        const nextOutputFormat = nextCategory.targetFormats[0]?.value ?? "";

        setDetectedCategoryId(nextCategoryId);
        setOutputFormat(nextOutputFormat);
        setFileState({ status: "selected", file, outputFormat: nextOutputFormat });
        return;
      }

      setFileState({ status: "selected", file, outputFormat });
    },
    [isAutoCategory, outputFormat]
  );

  const handleRemoveFile = useCallback(() => {
    setDetectedCategoryId(null);
    setOutputFormat(category.targetFormats[0]?.value ?? "");
    setFileState({ status: "idle" });
  }, [category.targetFormats]);

  const handleOutputFormatChange = useCallback(
    (value: string) => {
      setOutputFormat(value);
      if (fileState.status === "selected") {
        setFileState({ ...fileState, outputFormat: value });
      }
    },
    [fileState]
  );

  const handleConvert = useCallback(() => {
    // Mock: simulate conversion start. No real backend call.
    if (fileState.status === "selected") {
      setFileState({
        status: "converting",
        file: fileState.file,
        outputFormat: fileState.outputFormat,
        progress: 0,
      });

      // Simulate progress then completion
      let progress = 0;
      const interval = setInterval(() => {
        progress += 20;
        if (progress >= 100) {
          clearInterval(interval);
          setFileState({
            status: "done",
            file: fileState.file,
            outputFormat: fileState.outputFormat,
            resultUrl: "#",
          });
        } else {
          setFileState({
            status: "converting",
            file: fileState.file,
            outputFormat: fileState.outputFormat,
            progress,
          });
        }
      }, 400);
    }
  }, [fileState]);

  const isConverting = fileState.status === "converting";
  const isDone = fileState.status === "done";
  const hasFile =
    fileState.status === "selected" ||
    fileState.status === "converting" ||
    fileState.status === "done";

  return (
    <div
      role="tabpanel"
      id={`panel-${category.id}`}
      aria-labelledby={`tab-${category.id}`}
      className="mx-auto w-full max-w-215"
    >
      <div className="rounded-[34px] border border-white/80 bg-white px-7 py-7 shadow-[0_24px_60px_-42px_rgba(15,23,42,0.24)] sm:px-8 sm:py-8">
        {fileState.status === "idle" ? (
          <Dropzone
            text={effectiveCategory.dropzoneText}
            hint={effectiveCategory.dropzoneHint}
            supportLabel={effectiveCategory.supportLabel}
            detailLabel={detailLabel}
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
        ) : (
          <div className="mt-6 rounded-2xl border border-stone-200 bg-stone-50 px-4 py-4 text-sm text-stone-500">
            Sube un archivo y Reform Lab detectará el formato para mostrar las salidas compatibles.
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
            <p className="mt-3 text-sm text-stone-500">
              Convirtiendo… {fileState.status === "converting" ? fileState.progress : 0}%
            </p>
          </div>
        )}

        {isDone && (
          <div className="mt-5 flex flex-col gap-3 rounded-2xl border border-emerald-200 bg-emerald-50 p-4 sm:flex-row sm:items-center sm:justify-between">
            <p className="flex items-center gap-2 text-sm font-medium text-emerald-800">
              <CheckCircle2 size={16} strokeWidth={2} />
              Conversión completada
            </p>
            <button
              type="button"
              onClick={() => setFileState({ status: "idle" })}
              className="text-sm font-medium text-coral-700 underline underline-offset-2 hover:text-coral-800"
            >
              Convertir otro archivo
            </button>
          </div>
        )}

        <button
          type="button"
          disabled={!hasFile || isConverting || isDone}
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
