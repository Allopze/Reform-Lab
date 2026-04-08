"use client";

import { useCallback, useRef, useState } from "react";
import {
  FileText,
  Image,
  File,
  Music,
  Video,
  Archive,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

const categoryIconMap: Record<string, LucideIcon> = {
  "file-text": FileText,
  image: Image,
  file: File,
  music: Music,
  video: Video,
  archive: Archive,
};

interface DropzoneProps {
  text: string;
  hint: string;
  supportLabel: string;
  detailLabel: string;
  accept: string;
  icon?: string;
  onFileSelected: (file: File) => void;
}

export default function Dropzone({
  text,
  hint,
  supportLabel,
  detailLabel,
  accept,
  icon,
  onFileSelected,
}: DropzoneProps) {
  const [isDragOver, setIsDragOver] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragOver(true);
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragOver(false);
  }, []);

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      e.stopPropagation();
      setIsDragOver(false);
      const file = e.dataTransfer.files[0];
      if (file) onFileSelected(file);
    },
    [onFileSelected]
  );

  const handleInputChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (file) onFileSelected(file);
      // Reset so the same file can be re-selected
      e.target.value = "";
    },
    [onFileSelected]
  );

  const handleClick = useCallback(() => {
    inputRef.current?.click();
  }, []);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        handleClick();
      }
    },
    [handleClick]
  );

  const CategoryIcon = icon ? (categoryIconMap[icon] ?? File) : File;

  return (
    <div
      role="button"
      tabIndex={0}
      aria-label={text}
      onClick={handleClick}
      onKeyDown={handleKeyDown}
      onDragOver={handleDragOver}
      onDragEnter={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
      className={`
        group relative flex min-h-[420px] flex-col cursor-pointer
        items-center justify-center gap-6 rounded-[28px] border-[3px]
        border-dashed px-8 py-12 text-center
        transition-all duration-200
        sm:min-h-[520px]
        ${
          isDragOver
            ? "border-coral-300 bg-coral-50/50"
            : "border-stone-300 bg-white hover:border-coral-200"
        }
      `}
    >
      <div
        className={`
          flex h-32 w-32 items-center justify-center rounded-[30px]
          bg-coral-500 text-white shadow-[0_16px_36px_-28px_rgba(232,111,80,0.65)]
          transition-colors duration-200
          ${
            isDragOver
              ? "bg-coral-400"
              : "group-hover:bg-coral-500"
          }
        `}
      >
        <div className="flex flex-col items-center gap-1">
          <CategoryIcon size={32} strokeWidth={1.7} aria-hidden="true" />
          <span className="text-[22px] font-bold tracking-[-0.08em]">RL</span>
        </div>
      </div>

      <div>
        <p className="text-[34px] font-semibold tracking-tight text-stone-900 sm:text-[38px]">
          {text}
        </p>
        <p className="mt-4 text-[18px] text-stone-500 sm:text-[19px]">
          Soporte {supportLabel}.
        </p>
        <p className="mt-2 text-[15px] leading-7 text-stone-500 sm:text-[16px]">
          {detailLabel}
        </p>
      </div>

      <p className="text-sm text-stone-400">{hint}</p>

      <input
        ref={inputRef}
        type="file"
        accept={accept}
        onChange={handleInputChange}
        className="sr-only"
        tabIndex={-1}
        aria-hidden="true"
      />
    </div>
  );
}
