"use client";

import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import AuthPanel, { type AuthMode } from "./auth-panel";

function resolveAuthMode(mode: string | null): AuthMode {
  switch (mode) {
    case "register":
      return "register";
    case "recover":
      return "recover";
    case "reset":
      return "reset";
    case "verify":
      return "verify";
    default:
      return "login";
  }
}

export default function AccessShell() {
  const searchParams = useSearchParams();
  const requestedMode = resolveAuthMode(searchParams.get("mode"));
  const token = searchParams.get("token");
  const [mode, setMode] = useState<AuthMode>(requestedMode);

  useEffect(() => {
    setMode(requestedMode);
  }, [requestedMode]);

  return (
    <main className="flex flex-1 items-center justify-center px-5 py-10 sm:px-8 sm:py-14">
      <AuthPanel mode={mode} onModeChange={setMode} resetToken={token} verifyToken={token} />
    </main>
  );
}
