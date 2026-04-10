"use client";

import { useState } from "react";
import Image from "next/image";
import Link from "next/link";
import AuthPanel, { type AuthMode } from "./auth-panel";

export default function AccessShell() {
  const [mode, setMode] = useState<AuthMode>("login");

  return (
    <>
      <div className="w-full px-4 py-6 sm:px-6">
        <Link
          href="/"
          aria-label="Reform Lab — Inicio"
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
      </div>

      <main className="flex flex-1 items-center justify-center px-5 pb-12 sm:px-8">
        <AuthPanel mode={mode} onModeChange={setMode} />
      </main>
    </>
  );
}