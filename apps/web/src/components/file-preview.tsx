"use client";

import { X, FileIcon } from "lucide-react";
import { useTranslations } from "next-intl";

interface FilePreviewProps {
  file: File;
  outputFormat: string;
  onRemove: () => void;
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export default function FilePreview({
  file,
  outputFormat,
  onRemove,
}: FilePreviewProps) {
  const t = useTranslations("filePreview");

  return (
    <div className="flex items-center gap-4 rounded-2xl border border-stone-200 bg-stone-50 px-4 py-4">
      <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-white text-coral-600 shadow-[0_12px_22px_-20px_rgba(34,27,25,0.6)]">
        <FileIcon size={20} strokeWidth={1.5} aria-hidden="true" />
      </div>

      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium text-stone-800">
          {file.name}
        </p>
        <p className="mt-1 text-xs text-stone-500">
          {formatSize(file.size)}
          {outputFormat && (
            <span>
              {" "}
              a <span className="font-medium text-coral-700">.{outputFormat.toUpperCase()}</span>
            </span>
          )}
        </p>
      </div>

      <button
        type="button"
        onClick={onRemove}
        aria-label={t("removeAria")}
        className="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl text-stone-400 transition-colors duration-150 hover:bg-white hover:text-stone-700"
      >
        <X size={16} strokeWidth={2} />
      </button>
    </div>
  );
}
