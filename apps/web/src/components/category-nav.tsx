"use client";

import { categories } from "@/config/categories";
import type { CategoryId } from "@/types";
import {
  FileText,
  Image,
  File,
  Music,
  Video,
  Archive,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useRef, useEffect, useCallback } from "react";

const iconMap: Record<string, LucideIcon> = {
  "file-text": FileText,
  image: Image,
  file: File,
  music: Music,
  video: Video,
  archive: Archive,
};

interface CategoryNavProps {
  activeCategory: CategoryId;
  onChange: (id: CategoryId) => void;
}

export default function CategoryNav({
  activeCategory,
  onChange,
}: CategoryNavProps) {
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
      aria-label="Categorías de conversión"
      onKeyDown={handleKeyDown}
      className="mx-auto flex max-w-full items-center gap-1 overflow-x-auto rounded-full border border-stone-200 bg-white px-1.5 py-1.5 shadow-[0_10px_22px_-20px_rgba(15,23,42,0.18)]"
      style={{ scrollbarWidth: "none" }}
    >
      {categories.map((cat) => {
        const Icon = iconMap[cat.icon] ?? File;
        const isActive = cat.id === activeCategory;

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
              flex min-h-10 shrink-0 items-center gap-2 rounded-full px-4 py-2
              text-[13px] font-medium whitespace-nowrap
              transition-colors duration-150
              ${
                isActive
                  ? "bg-coral-500 text-white"
                  : "text-stone-500 hover:bg-stone-100 hover:text-stone-700"
              }
            `}
          >
            <Icon size={16} strokeWidth={2} aria-hidden="true" />
            {cat.label}
          </button>
        );
      })}
    </div>
  );
}
