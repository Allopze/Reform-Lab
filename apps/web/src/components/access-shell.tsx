"use client";

import { useState } from "react";
import AuthPanel, { type AuthMode } from "./auth-panel";

export default function AccessShell() {
  const [mode, setMode] = useState<AuthMode>("login");

  return (
    <main className="flex flex-1 items-center justify-center px-5 py-10 sm:px-8 sm:py-14">
      <AuthPanel mode={mode} onModeChange={setMode} />
    </main>
  );
}
