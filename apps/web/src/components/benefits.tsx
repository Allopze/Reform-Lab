"use client";

import { Zap, Shield, Globe } from "lucide-react";
import { useTranslations } from "next-intl";

const benefitKeys = [
  { key: "fast", icon: Zap },
  { key: "secure", icon: Shield },
  { key: "noInstall", icon: Globe },
] as const;

export default function Benefits() {
  const t = useTranslations("benefits");

  return (
    <section className="mx-auto mt-12 grid max-w-xl gap-4 sm:grid-cols-3">
      {benefitKeys.map((b) => {
        const Icon = b.icon;
        return (
          <div
            key={b.key}
            className="flex flex-col items-center gap-2 rounded-xl border border-stone-200 bg-white p-4 text-center"
          >
            <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-stone-100 text-stone-600">
              <Icon size={18} strokeWidth={2} aria-hidden="true" />
            </div>
            <p className="text-sm font-medium text-gray-800">{t(`${b.key}.title`)}</p>
            <p className="text-xs leading-relaxed text-gray-400">
              {t(`${b.key}.description`)}
            </p>
          </div>
        );
      })}
    </section>
  );
}
