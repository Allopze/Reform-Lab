"use client";

import { categories } from "@/config/categories";
import type { CategoryId } from "@/types";
import { useTranslations } from "next-intl";
import {
  Search,
  FileText,
  Files,
  Image,
  File,
  Music,
  Video,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useRef, useEffect, useCallback } from "react";

const iconMap: Record<string, LucideIcon> = {
  search: Search,
  "file-text": FileText,
  files: Files,
  image: Image,
  file: File,
  music: Music,
  video: Video,
};

interface CategoryNavProps {
  activeCategory: CategoryId;
  onChange: (id: CategoryId) => void;
}

export default function CategoryNav({
  activeCategory,
  onChange,
}: CategoryNavProps) {
  const tc = useTranslations("categories");
  const scrollRef = useRef<HTMLDivElement>(null);
  const activeRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (activeRef.current && scrollRef.current) {
      const container = scrollRef.current;
      const button = activeRef.current;
      const scrollLeft =
        button.offsetLeft - container.offsetWidth / 2 + button.offsetWidth / 2;
      container.scrollTo({ left: scrollLeft, behavior: "smooth" });
    }
  }, [activeCategory]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      const currentIndex = categories.findIndex((c) => c.id === activeCategory);
      let nextIndex = currentIndex;

      if (e.key === "ArrowRight") {
        e.preventDefault();
        nextIndex = (currentIndex + 1) % categories.length;
      } else if (e.key === "ArrowLeft") {
        e.preventDefault();
        nextIndex = (currentIndex - 1 + categories.length) % categories.length;
      } else if (e.key === "Home") {
        e.preventDefault();
        nextIndex = 0;
      } else if (e.key === "End") {
        e.preventDefault();
        nextIndex = categories.length - 1;
      } else {
        return;
      }

      onChange(categories[nextIndex].id);
    },
    [activeCategory, onChange]
  );

  return (
    <div
      ref={scrollRef}
      role="tablist"
      aria-label={tc("navAriaLabel")}
      onKeyDown={handleKeyDown}
      className="relative flex w-full min-w-0 items-center gap-1 overflow-x-auto rounded-full border border-stone-200 bg-white px-1.5 py-1.5 shadow-[0_10px_22px_-20px_rgba(15,23,42,0.18)] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden md:justify-between"
    >
      {categories.map((cat) => {
        const Icon = iconMap[cat.icon] ?? File;
        const isActive = cat.id === activeCategory;
        const iconSize = cat.id === "documents" ? 19 : 17;
        const iconStrokeWidth = cat.id === "documents" ? 2.15 : 2;
        const widthClass = cat.id === "documents" ? "md:flex-[1.16]" : "md:flex-1";

        return (
          <button
            key={cat.id}
            ref={isActive ? activeRef : undefined}
            role="tab"
            id={`tab-${cat.id}`}
            aria-selected={isActive}
            aria-controls={`panel-${cat.id}`}
            tabIndex={isActive ? 0 : -1}
            onClick={() => onChange(cat.id)}
            className={`
              relative z-10 flex min-h-11 shrink-0 items-center justify-center gap-2 rounded-full px-3.5 py-2 md:min-w-0 ${widthClass}
              text-[12px] font-medium leading-none whitespace-nowrap sm:text-[13px]
              transition-colors duration-150
              ${
                isActive
                  ? "bg-coral-500 text-white shadow-[0_8px_18px_-14px_rgba(222,36,22,0.9)]"
                  : "text-stone-500 hover:bg-stone-100 hover:text-stone-700"
              }
            `}
          >
            <Icon
              size={iconSize}
              strokeWidth={iconStrokeWidth}
              aria-hidden="true"
              className="shrink-0"
            />
            {tc(`${cat.id}.label`)}
          </button>
        );
      })}
    </div>
  );
}
