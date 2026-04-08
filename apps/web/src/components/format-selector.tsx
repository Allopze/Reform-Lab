"use client";

import type { FormatOption } from "@/types";

interface FormatSelectorProps {
  label: string;
  options: FormatOption[];
  value: string;
  onChange: (value: string) => void;
  id: string;
}

export default function FormatSelector({
  label,
  options,
  value,
  onChange,
  id,
}: FormatSelectorProps) {
  return (
    <div className="flex flex-col gap-3">
      <label htmlFor={id} className="text-sm font-medium text-stone-600">
        {label}
      </label>
      <div className="flex flex-wrap gap-2.5">
        {options.map((opt) => {
          const isSelected = opt.value === value;
          return (
            <button
              key={opt.value}
              type="button"
              id={isSelected ? id : undefined}
              onClick={() => onChange(opt.value)}
              aria-pressed={isSelected}
              className={`
                rounded-2xl border px-4 py-2.5 text-[14px] font-medium
                transition-colors duration-150
                ${
                  isSelected
                    ? "border-coral-300 bg-coral-50 text-coral-700"
                    : "border-stone-200 bg-white text-stone-500 hover:border-stone-300 hover:text-stone-700"
                }
              `}
            >
              {opt.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}
