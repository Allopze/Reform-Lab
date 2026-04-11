"use client";

import Image from "next/image";
import { useCallback, useRef, useState } from "react";
import { useTranslations } from "next-intl";

interface DropzoneProps {
  text: string;
  hint: string;
  supportLabel: string;
  detailLabel: string;
  accept: string;
  onFileSelected: (file: File) => void;
}

export default function Dropzone({
  text,
  hint,
  supportLabel,
  detailLabel,
  accept,
  onFileSelected,
}: DropzoneProps) {
  const t = useTranslations("common");
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
        group relative flex min-h-105 flex-col cursor-pointer
        items-center justify-center gap-6 rounded-[28px] border-[3px]
        border-dashed px-8 py-12 text-center
        transition-all duration-200
        sm:min-h-130
        ${
          isDragOver
            ? "border-coral-300 bg-coral-50/50"
            : "border-stone-300 bg-white hover:border-coral-200"
        }
      `}
    >
      <Image
        src="/favicon.svg"
        alt="Reform Lab"
        width={148}
        height={148}
        className="h-33 w-auto sm:h-37"
        priority
      />

      <div>
        <p className="text-[34px] font-semibold tracking-tight text-stone-900 sm:text-[38px]">
          {text}
        </p>
        <p className="mt-4 text-[18px] text-stone-500 sm:text-[19px]">
          {t("supportPrefix", { label: supportLabel })}
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
