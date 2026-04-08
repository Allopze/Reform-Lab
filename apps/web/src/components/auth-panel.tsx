"use client";

import Link from "next/link";

export type AuthMode = "login" | "register";

interface AuthPanelProps {
  mode: AuthMode;
  onModeChange: (mode: AuthMode) => void;
}

export default function AuthPanel({ mode, onModeChange }: AuthPanelProps) {
  const isLogin = mode === "login";

  return (
    <section className="w-full max-w-135 rounded-[34px] border border-white/80 bg-white px-7 py-7 shadow-[0_24px_60px_-42px_rgba(15,23,42,0.24)] sm:px-9 sm:py-8">
      <h1 className="text-xl font-semibold tracking-[-0.01em] text-stone-900">
        {isLogin ? "Iniciar sesion" : "Crear cuenta"}
      </h1>
      <p className="mt-1 text-[13px] leading-5 text-stone-400">
        {isLogin
          ? "Accede a tu historial y descargas."
          : "Conserva historial, artefactos y accesos."}
      </p>

      <form className="mt-5 space-y-3.5" onSubmit={(event) => event.preventDefault()}>
        {!isLogin ? (
          <div className="grid gap-3.5 sm:grid-cols-2">
            <label className="block">
              <span className="mb-1.5 block text-[13px] font-medium text-stone-600">Nombre</span>
              <input
                type="text"
                name="name"
                placeholder="Allopze"
                className="h-10 w-full rounded-xl border border-stone-200 bg-stone-50/60 px-3.5 text-sm text-stone-900 placeholder:text-stone-400 transition-colors duration-150 focus:border-coral-400 focus:bg-white"
              />
            </label>

            <label className="block">
              <span className="mb-1.5 block text-[13px] font-medium text-stone-600">Equipo</span>
              <input
                type="text"
                name="team"
                placeholder="Reform Lab"
                className="h-10 w-full rounded-xl border border-stone-200 bg-stone-50/60 px-3.5 text-sm text-stone-900 placeholder:text-stone-400 transition-colors duration-150 focus:border-coral-400 focus:bg-white"
              />
            </label>
          </div>
        ) : null}

        <label className="block">
          <span className="mb-1.5 block text-[13px] font-medium text-stone-600">Correo</span>
          <input
            type="email"
            name="email"
            placeholder="nombre@empresa.com"
            className="h-10 w-full rounded-xl border border-stone-200 bg-stone-50/60 px-3.5 text-sm text-stone-900 placeholder:text-stone-400 transition-colors duration-150 focus:border-coral-400 focus:bg-white"
          />
        </label>

        <div className={isLogin ? "space-y-3.5" : "grid gap-3.5 sm:grid-cols-2"}>
          <label className="block">
            <span className="mb-1.5 block text-[13px] font-medium text-stone-600">Contrasena</span>
            <input
              type="password"
              name="password"
              placeholder="••••••••"
              className="h-10 w-full rounded-xl border border-stone-200 bg-stone-50/60 px-3.5 text-sm text-stone-900 placeholder:text-stone-400 transition-colors duration-150 focus:border-coral-400 focus:bg-white"
            />
          </label>

          {!isLogin ? (
            <label className="block">
              <span className="mb-1.5 block text-[13px] font-medium text-stone-600">Repetir contrasena</span>
              <input
                type="password"
                name="passwordConfirmation"
                placeholder="••••••••"
                className="h-10 w-full rounded-xl border border-stone-200 bg-stone-50/60 px-3.5 text-sm text-stone-900 placeholder:text-stone-400 transition-colors duration-150 focus:border-coral-400 focus:bg-white"
              />
            </label>
          ) : (
            <div className="flex items-center justify-end">
              <Link
                href="/acceso"
                className="text-[13px] font-medium text-stone-400 transition-colors duration-150 hover:text-stone-700"
              >
                Recuperar acceso
              </Link>
            </div>
          )}
        </div>

        {!isLogin ? (
          <label className="flex items-start gap-2.5 pt-1 text-[13px] leading-5 text-stone-500">
            <input type="checkbox" className="mt-0.5 h-4 w-4 rounded border-stone-300 accent-coral-500" />
            <span>
              Acepto la retencion temporal de artefactos y la separacion entre cuenta de usuario y panel operativo.
            </span>
          </label>
        ) : null}

        <div className="flex items-center justify-between pt-1.5">
          <button
            type="submit"
            className="inline-flex h-10 items-center justify-center rounded-xl bg-coral-500 px-5 text-sm font-medium text-white transition-colors duration-150 hover:bg-coral-600"
          >
            {isLogin ? "Entrar" : "Crear cuenta"}
          </button>

          <p className="text-[13px] text-stone-400">
            {isLogin ? "¿Sin cuenta?" : "¿Ya tienes cuenta?"}{" "}
            <button
              type="button"
              onClick={() => onModeChange(isLogin ? "register" : "login")}
              className="font-medium text-stone-700 underline underline-offset-2 transition-colors duration-150 hover:text-stone-900"
            >
              {isLogin ? "Crear cuenta" : "Iniciar sesion"}
            </button>
          </p>
        </div>
      </form>
    </section>
  );
}