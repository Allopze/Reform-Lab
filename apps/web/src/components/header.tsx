"use client";

import type { ReactNode } from "react";
import Image from "next/image";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { useTheme } from "@/lib/theme-context";
import {
  ChevronDown,
  FolderOpen,
  LogIn,
  LogOut,
  Moon,
  Shield,
  Sun,
  UserPlus,
} from "lucide-react";

interface HeaderProps {
  toolbar?: ReactNode;
}

export default function Header({ toolbar }: HeaderProps) {
  const pathname = usePathname();
  const { user, logout } = useAuth();
  const { resolvedTheme, toggleTheme, isReady } = useTheme();
  const userName = user?.name ?? "Invitado";
  const userInitial = userName.slice(0, 1).toUpperCase();
  const isDarkTheme = resolvedTheme === "dark";
  const themeButtonLabel = isDarkTheme ? "Activar tema claro" : "Activar tema oscuro";
  const logoSrc = isReady && isDarkTheme ? "/logo-dark.svg" : "/logo-light.svg";
  const guestRegisterLinkClassName = isDarkTheme
    ? "flex items-center gap-3 rounded-xl border border-coral-800 bg-coral-900 px-3 py-2.5 text-left text-[14px] font-medium text-coral-100 transition-colors duration-150 hover:bg-coral-800"
    : "flex items-center gap-3 rounded-xl bg-coral-50 px-3 py-2.5 text-left text-[14px] font-medium text-coral-700 transition-colors duration-150 hover:bg-coral-100";
  const userRoleLabel = user
    ? user.role === "admin"
      ? "Administrador"
      : "Cuenta registrada"
    : "Guarda historial, artefactos y límites más amplios.";
  const menuItems = [
    ...(user
      ? [
          {
            href: "/usuario",
            label: "Mis Archivos",
            icon: FolderOpen,
          },
        ]
      : []),
    ...(user?.role === "admin"
      ? [
          {
            href: "/admin",
            label: "Panel Admin",
            icon: Shield,
          },
        ]
      : []),
  ];

  return (
    <header className="relative z-10 w-full">
      <div className="grid w-full grid-cols-[1fr_minmax(0,940px)_1fr] items-center gap-4 px-2 py-6 sm:px-3 lg:grid-cols-[1fr_minmax(0,980px)_1fr]">
        <Link
          href="/"
          aria-label="Reform Lab — Inicio"
          className="min-w-0 justify-self-start self-center ml-2 sm:ml-3"
        >
          <Image
            src={logoSrc}
            alt="Reform Lab"
            width={216}
            height={54}
            className="h-8 w-auto sm:h-10"
            priority
          />
        </Link>

        {toolbar ? (
          <div className="min-w-0 self-center">
            {toolbar}
          </div>
        ) : null}

        <div className="mr-2 flex items-center justify-self-end self-center gap-2 sm:mr-3">
          <button
            type="button"
            aria-label={themeButtonLabel}
            aria-pressed={isDarkTheme}
            onClick={toggleTheme}
            className="flex h-10 w-10 items-center justify-center rounded-full text-stone-600 transition-colors duration-150 hover:bg-white/80 hover:text-stone-900"
          >
            {isDarkTheme ? <Sun size={20} strokeWidth={1.9} /> : <Moon size={20} strokeWidth={1.9} />}
          </button>

          <details className="group relative">
            <summary
              aria-label="Abrir menu de usuario"
              className="flex h-10 list-none items-center gap-1.5 rounded-full bg-white/95 pl-1.5 pr-2.5 text-stone-700 shadow-[0_4px_12px_rgba(15,23,42,0.06)] ring-1 ring-black/5 transition-colors duration-150 hover:bg-white [&::-webkit-details-marker]:hidden"
            >
              <span className="flex h-7 w-7 items-center justify-center rounded-full bg-[#e52b25] text-[15px] font-semibold text-white">
                {userInitial}
              </span>
              <span className="max-w-20 truncate text-[13px] font-semibold text-stone-700 sm:max-w-none sm:text-[14px]">
                {userName}
              </span>
              <ChevronDown
                size={13}
                strokeWidth={1.8}
                className="text-stone-500 transition-transform duration-150 group-open:rotate-180"
              />
            </summary>

            <div className="absolute right-0 top-[calc(100%+8px)] w-64 overflow-hidden rounded-2xl border border-stone-200 bg-white shadow-[0_12px_28px_rgba(15,23,42,0.14)]">
              <div className="border-b border-stone-200 px-4 py-3.5">
                <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-stone-400">
                  {user ? "Sesion activa" : "Cuenta"}
                </p>
                <p className="mt-1 text-sm font-semibold text-stone-900">
                  {user ? userName : "Accede o crea tu cuenta"}
                </p>
                <p className="mt-0.5 text-xs leading-5 text-stone-500">
                  {userRoleLabel}
                </p>
              </div>

              <div className="py-1.5">
                {menuItems.map(({ href, label, icon: Icon }) => {
                  const isActive = pathname === href;

                  return (
                    <Link
                      key={href}
                      href={href}
                      className={
                        isActive
                          ? "mx-1.5 flex items-center gap-3 rounded-xl bg-stone-50 px-3 py-2.5 text-left text-[14px] font-medium text-stone-900"
                          : "mx-1.5 flex items-center gap-3 rounded-xl px-3 py-2.5 text-left text-[14px] font-medium text-stone-800 transition-colors duration-150 hover:bg-stone-50"
                      }
                    >
                      <Icon size={18} strokeWidth={1.9} className="text-stone-700" />
                      {label}
                    </Link>
                  );
                })}

                {user ? (
                  <>
                    {menuItems.length > 0 ? <div className="my-1.5 border-t border-stone-200" /> : null}
                    <button
                      type="button"
                      onClick={logout}
                      className="mx-1.5 flex w-[calc(100%-12px)] items-center gap-3 rounded-xl px-3 py-2.5 text-left text-[14px] font-medium text-[#ef4339] transition-colors duration-150 hover:bg-stone-50"
                    >
                      <LogOut size={18} strokeWidth={1.9} className="text-[#ef4339]" />
                      Cerrar sesion
                    </button>
                  </>
                ) : (
                  <div className="space-y-1 px-1.5">
                    <Link
                      href="/acceso"
                      className="flex items-center gap-3 rounded-xl px-3 py-2.5 text-left text-[14px] font-medium text-stone-800 transition-colors duration-150 hover:bg-stone-50"
                    >
                      <LogIn size={18} strokeWidth={1.9} className="text-stone-700" />
                      Iniciar sesion
                    </Link>
                    <Link
                      href="/acceso?mode=register"
                      className={guestRegisterLinkClassName}
                    >
                      <UserPlus size={18} strokeWidth={1.9} />
                      Crear cuenta
                    </Link>
                  </div>
                )}
              </div>
            </div>
          </details>
        </div>
      </div>
    </header>
  );
}
