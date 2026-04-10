"use client";

import { useState } from "react";
import Image from "next/image";
import Link from "next/link";
import { useAuth } from "@/lib/auth-context";
import { useRouter } from "next/navigation";
import { ChevronLeft } from "lucide-react";

export type AuthMode = "login" | "register";

interface AuthPanelProps {
  mode: AuthMode;
  onModeChange: (mode: AuthMode) => void;
}

export default function AuthPanel({ mode, onModeChange }: AuthPanelProps) {
  const isLogin = mode === "login";
  const router = useRouter();
  const { login: authLogin, register: authRegister } = useAuth();

  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);

    const form = new FormData(e.currentTarget);
    const email = (form.get("email") as string)?.trim().toLowerCase();
    const password = form.get("password") as string;

    if (!email || !password) {
      setError("Correo y contraseña son obligatorios");
      return;
    }

    if (password.length < 8) {
      setError("La contraseña debe tener al menos 8 caracteres");
      return;
    }

    if (!isLogin) {
      const name = (form.get("name") as string)?.trim();
      if (!name) {
        setError("El nombre es obligatorio");
        return;
      }
    }

    setLoading(true);

    try {
      if (isLogin) {
        await authLogin({ email, password });
      } else {
        const name = (form.get("name") as string)?.trim();
        const team = (form.get("team") as string)?.trim() ?? "";
        const passwordConfirmation = form.get("passwordConfirmation") as string;

        if (password !== passwordConfirmation) {
          setError("Las contraseñas no coinciden");
          setLoading(false);
          return;
        }

        await authRegister({ name, email, password, team });
      }
      router.push("/");
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Error inesperado");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="relative flex w-full max-w-135 flex-col items-center">
      <Link
        href="/"
        className="absolute -left-[4.5rem] top-0 hidden h-13 w-13 items-center justify-center rounded-full border border-stone-200 bg-white text-stone-500 shadow-sm transition-colors duration-150 hover:border-stone-300 hover:bg-stone-50 hover:text-stone-900 outline-none focus-visible:ring-2 focus-visible:ring-coral-400/40 focus-visible:ring-offset-1 lg:flex"
        aria-label="Volver a conversiones"
      >
        <ChevronLeft size={22} strokeWidth={2} />
      </Link>

      <section className="w-full rounded-[34px] border border-white/80 bg-white px-7 py-7 shadow-[0_24px_60px_-42px_rgba(15,23,42,0.24)] sm:px-9 sm:py-8">
        <div className="-mt-3 flex justify-center">
          <Image
            src="/favicon.svg"
            alt="Reform Lab"
            width={112}
            height={112}
            className="h-28 w-auto"
            priority
          />
        </div>
        <div className="mt-4 flex gap-1 rounded-[22px] bg-stone-100 p-1">
        <button
          type="button"
          onClick={() => onModeChange("login")}
          className={`flex-1 rounded-[18px] py-2.5 text-sm font-medium outline-none transition-colors duration-150 focus-visible:ring-2 focus-visible:ring-coral-400/40 focus-visible:ring-offset-1 ${
            isLogin ? "bg-white text-stone-900 shadow-sm" : "text-stone-500 hover:text-stone-700"
          }`}
        >
          Iniciar sesión
        </button>
        <button
          type="button"
          onClick={() => onModeChange("register")}
          className={`flex-1 rounded-[18px] py-2.5 text-sm font-medium outline-none transition-colors duration-150 focus-visible:ring-2 focus-visible:ring-coral-400/40 focus-visible:ring-offset-1 ${
            !isLogin ? "bg-white text-stone-900 shadow-sm" : "text-stone-500 hover:text-stone-700"
          }`}
        >
          Crear cuenta
        </button>
      </div>

      <h1 className="mt-6 text-[22px] font-semibold tracking-[-0.02em] text-stone-900">
        {isLogin ? "Iniciar sesión" : "Crear cuenta"}
      </h1>
      <p className="mt-1.5 text-sm leading-5 text-stone-500">
        {isLogin
          ? "Accede a tu historial y descargas."
          : "Conserva historial, artefactos y accesos."}
      </p>

      <form className="mt-5 flex flex-col space-y-3.5" onSubmit={handleSubmit}>
        {error && (
          <p className="rounded-[18px] bg-red-50 px-3.5 py-2.5 text-[13px] font-medium text-red-600">{error}</p>
        )}
        <div className={`grid gap-3.5 sm:grid-cols-2 ${isLogin ? "invisible" : ""}`} aria-hidden={isLogin}>
            <label className="block">
              <span className="mb-1.5 block text-[13px] font-medium text-stone-600">Nombre</span>
              <input
                type="text"
                name="name"
                tabIndex={isLogin ? -1 : undefined}
                placeholder="Allopze"
                className="h-11 w-full rounded-[18px] border border-stone-200 bg-stone-50/60 px-3.5 text-sm text-stone-900 placeholder:text-stone-400 outline-none transition-colors duration-150 focus:border-coral-400 focus:bg-white focus-visible:ring-2 focus-visible:ring-coral-400/40 focus-visible:ring-offset-1"
              />
            </label>

            <label className="block">
              <span className="mb-1.5 block text-[13px] font-medium text-stone-600">Equipo</span>
              <input
                type="text"
                name="team"
                tabIndex={isLogin ? -1 : undefined}
                placeholder="Reform Lab"
                className="h-11 w-full rounded-[18px] border border-stone-200 bg-stone-50/60 px-3.5 text-sm text-stone-900 placeholder:text-stone-400 outline-none transition-colors duration-150 focus:border-coral-400 focus:bg-white focus-visible:ring-2 focus-visible:ring-coral-400/40 focus-visible:ring-offset-1"
              />
            </label>
          </div>

        <label className="block">
          <span className="mb-1.5 block text-[13px] font-medium text-stone-600">Correo</span>
          <input
            type="email"
            name="email"
            placeholder="nombre@empresa.com"
            className="h-11 w-full rounded-[18px] border border-stone-200 bg-stone-50/60 px-3.5 text-sm text-stone-900 placeholder:text-stone-400 outline-none transition-colors duration-150 focus:border-coral-400 focus:bg-white focus-visible:ring-2 focus-visible:ring-coral-400/40 focus-visible:ring-offset-1"
          />
        </label>

        <div className="grid gap-3.5 sm:grid-cols-2">
          <label className="block">
            <span className="mb-1.5 block text-[13px] font-medium text-stone-600">Contraseña</span>
            <input
              type="password"
              name="password"
              placeholder="••••••••"
              className="h-11 w-full rounded-[18px] border border-stone-200 bg-stone-50/60 px-3.5 text-sm text-stone-900 placeholder:text-stone-400 outline-none transition-colors duration-150 focus:border-coral-400 focus:bg-white focus-visible:ring-2 focus-visible:ring-coral-400/40 focus-visible:ring-offset-1"
            />
          </label>

          {isLogin ? (
            <div className="flex items-center justify-end">
              <button
                type="button"
                disabled
                className="text-[13px] font-medium text-stone-400 cursor-not-allowed"
              >
                Recuperar acceso
              </button>
            </div>
          ) : (
            <label className="block">
              <span className="mb-1.5 block text-[13px] font-medium text-stone-600">Repetir contraseña</span>
              <input
                type="password"
                name="passwordConfirmation"
                placeholder="••••••••"
                className="h-11 w-full rounded-[18px] border border-stone-200 bg-stone-50/60 px-3.5 text-sm text-stone-900 placeholder:text-stone-400 outline-none transition-colors duration-150 focus:border-coral-400 focus:bg-white focus-visible:ring-2 focus-visible:ring-coral-400/40 focus-visible:ring-offset-1"
              />
            </label>
          )}
        </div>

        <label className={`flex items-start gap-2.5 pt-1 text-[13px] leading-5 text-stone-500 ${isLogin ? "invisible" : ""}`} aria-hidden={isLogin}>
            <input type="checkbox" tabIndex={isLogin ? -1 : undefined} className="mt-0.5 h-4 w-4 rounded-md border-stone-300 accent-coral-500" />
            <span>
              Acepto la retención temporal de artefactos y la separación entre cuenta de usuario y panel operativo.
            </span>
          </label>

        <button
          type="submit"
          disabled={loading}
          className="mt-auto h-11 w-full rounded-[20px] bg-coral-500 text-sm font-medium text-white outline-none transition-colors duration-150 hover:bg-coral-600 disabled:opacity-50 focus-visible:ring-2 focus-visible:ring-coral-400/40 focus-visible:ring-offset-2"
        >
          {loading ? "Cargando..." : isLogin ? "Entrar" : "Crear cuenta"}
        </button>
      </form>
      </section>
    </div>
  );
}
