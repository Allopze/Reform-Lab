"use client";

import { useState } from "react";
import Image from "next/image";
import Link from "next/link";
import AuthPanel, { type AuthMode } from "./auth-panel";

export default function AccessShell() {
  const [mode, setMode] = useState<AuthMode>("login");

  return (
    <>
      <div className="grid w-full grid-cols-[1fr_minmax(0,940px)_1fr] items-center gap-4 px-2 py-6 sm:px-3 lg:grid-cols-[1fr_minmax(0,980px)_1fr]">
        <Link
          href="/"
          aria-label="Reform Lab — Inicio"
          className="min-w-0 justify-self-start self-center ml-2 sm:ml-3"
        >
          <Image
            src="/logo-light.svg"
            alt="Reform Lab"
            width={216}
            height={54}
            className="h-8 w-auto sm:h-10"
            priority
          />
        </Link>

        <div className="min-w-0 self-center" />

        <div className="mr-2 flex items-center justify-self-end self-center gap-1.5 sm:mr-3">
          <button
            type="button"
            onClick={() => setMode("login")}
            className={
              mode === "login"
                ? "inline-flex h-9 items-center rounded-xl bg-coral-500 px-4 text-[13px] font-medium whitespace-nowrap text-white transition-colors duration-150 hover:bg-coral-600"
                : "inline-flex h-9 items-center rounded-xl border border-stone-200 bg-white/90 px-4 text-[13px] font-medium whitespace-nowrap text-stone-600 transition-colors duration-150 hover:border-stone-300 hover:text-stone-900"
            }
          >
            Iniciar sesion
          </button>

          <button
            type="button"
            onClick={() => setMode("register")}
            className={
              mode === "register"
                ? "inline-flex h-9 items-center rounded-xl bg-coral-500 px-4 text-[13px] font-medium whitespace-nowrap text-white transition-colors duration-150 hover:bg-coral-600"
                : "inline-flex h-9 items-center rounded-xl border border-stone-200 bg-white/90 px-4 text-[13px] font-medium whitespace-nowrap text-stone-600 transition-colors duration-150 hover:border-stone-300 hover:text-stone-900"
            }
          >
            Registrarse
          </button>
        </div>
      </div>

      <main className="flex flex-1 items-center justify-center px-5 pb-12 sm:px-8">
        <AuthPanel mode={mode} onModeChange={setMode} />
      </main>
    </>
  );
}