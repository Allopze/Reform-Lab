"use client";

import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import AuthPanel, { type AuthMode } from "./auth-panel";

function resolveAuthMode(mode: string | null): AuthMode {
  return mode === "register" ? "register" : "login";
}

export default function AccessShell() {
  const searchParams = useSearchParams();
  const requestedMode = resolveAuthMode(searchParams.get("mode"));
  const [mode, setMode] = useState<AuthMode>(requestedMode);

  useEffect(() => {
    setMode(requestedMode);
  }, [requestedMode]);

  return (
    <main className="flex flex-1 items-center justify-center px-5 py-10 sm:px-8 sm:py-14">
      <AuthPanel mode={mode} onModeChange={setMode} />
    </main>
  );
}
