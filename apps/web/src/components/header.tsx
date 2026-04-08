import Link from "next/link";
import { ChevronDown, Moon } from "lucide-react";

export default function Header() {
  return (
    <header className="relative z-10 w-full">
      <div className="mx-auto flex w-full max-w-7xl items-center justify-between px-5 py-6 sm:px-8">
        <Link
          href="/"
          className="flex items-center gap-3"
          aria-label="Reform Lab — Inicio"
        >
          <span className="flex h-11 w-11 items-center justify-center rounded-2xl bg-coral-500 text-[12px] font-bold tracking-[-0.08em] text-white shadow-[0_10px_24px_-18px_rgba(232,111,80,0.7)]">
            RL
          </span>
          <span className="text-[18px] font-semibold tracking-tight text-stone-900">
            Reform Lab
          </span>
        </Link>

        <div className="flex items-center gap-2">
          <button
            type="button"
            aria-label="Cambiar apariencia"
            className="flex h-11 w-11 items-center justify-center rounded-full text-stone-600 transition-colors duration-150 hover:bg-white/70 hover:text-stone-800"
          >
            <Moon size={16} strokeWidth={1.8} />
          </button>

          <button
            type="button"
            className="hidden items-center gap-2 rounded-full px-4 py-2 text-[15px] font-medium text-stone-700 transition-colors duration-150 hover:bg-white/70 md:flex"
          >
            Historial
          </button>

          <button
            type="button"
            className="flex items-center gap-2 rounded-full bg-coral-500 px-5 py-3 text-[15px] font-semibold text-white transition-colors duration-150 hover:bg-coral-600"
          >
            Nuevo archivo
            <ChevronDown size={16} strokeWidth={1.8} className="text-white/80" />
          </button>
        </div>
      </div>
    </header>
  );
}
