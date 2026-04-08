"use client";

import { useState, useCallback } from "react";
import { CheckCircle2 } from "lucide-react";
import type { CategoryConfig, FileState } from "@/types";
import Dropzone from "./dropzone";
import FormatSelector from "./format-selector";
import FilePreview from "./file-preview";

interface ConversionCardProps {
  category: CategoryConfig;
}

export default function ConversionCard({ category }: ConversionCardProps) {
  const [fileState, setFileState] = useState<FileState>({ status: "idle" });
  const [outputFormat, setOutputFormat] = useState(
    category.targetFormats[0]?.value ?? ""
  );
  const detailLabel = `Convierte a ${category.targetFormats
    .map((format) => format.label)
    .join(", ")}.`;

  const handleFileSelected = useCallback(
    (file: File) => {
      setFileState({ status: "selected", file, outputFormat });
    },
    [outputFormat]
  );

  const handleRemoveFile = useCallback(() => {
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
  const hasFile = fileState.status !== "idle";

  return (
    <div
      role="tabpanel"
      id={`panel-${category.id}`}
      aria-labelledby={`tab-${category.id}`}
      className="w-full mx-auto max-w-[860px]"
    >
      <div className="rounded-[34px] border border-white/80 bg-white px-7 py-7 shadow-[0_24px_60px_-42px_rgba(15,23,42,0.24)] sm:px-8 sm:py-8">
        {fileState.status === "idle" ? (
          <Dropzone
            text={category.dropzoneText}
            hint={category.dropzoneHint}
            supportLabel={category.supportLabel}
            detailLabel={detailLabel}
            accept={category.acceptedMimeTypes}
            icon={category.icon}
            onFileSelected={handleFileSelected}
          />
        ) : (
          <FilePreview
            file={fileState.file}
            outputFormat={outputFormat}
            onRemove={handleRemoveFile}
          />
        )}

        <div className="mt-6">
          <FormatSelector
            label="Salida disponible"
            options={category.targetFormats}
            value={outputFormat}
            onChange={handleOutputFormatChange}
            id={`format-${category.id}`}
          />
        </div>

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
              : category.cta}
        </button>
      </div>
    </div>
  );
}
