"use client";

import type { ReactNode } from "react";
import Image from "next/image";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  ChevronDown,
  FolderOpen,
  LogIn,
  LogOut,
  Moon,
  Shield,
} from "lucide-react";

interface HeaderProps {
  toolbar?: ReactNode;
}

export default function Header({ toolbar }: HeaderProps) {
  const pathname = usePathname();
  const userName = "Allopze";
  const userInitial = userName.slice(0, 1).toUpperCase();
  const menuItems = [
    {
      href: "/usuario",
      label: "Mis Archivos",
      icon: FolderOpen,
    },
    {
      href: "/admin",
      label: "Panel Admin",
      icon: Shield,
    },
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
            src="/logo-light.svg"
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
            aria-label="Cambiar apariencia"
            className="flex h-10 w-10 items-center justify-center rounded-full text-stone-600 transition-colors duration-150 hover:bg-white/80 hover:text-stone-900"
          >
            <Moon size={20} strokeWidth={1.9} />
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

            <div className="absolute right-0 top-[calc(100%+8px)] w-52 overflow-hidden rounded-xl border border-stone-200 bg-white shadow-[0_8px_22px_rgba(15,23,42,0.1)]">
              {menuItems.map(({ href, label, icon: Icon }) => {
                const isActive = pathname === href;

                return (
                  <Link
                    key={href}
                    href={href}
                    className={
                      isActive
                        ? "flex w-full items-center gap-3 bg-stone-50 px-4 py-3 text-left text-[14px] font-medium text-stone-900"
                        : "flex w-full items-center gap-3 px-4 py-3 text-left text-[14px] font-medium text-stone-800 transition-colors duration-150 hover:bg-stone-50"
                    }
                  >
                    <Icon size={18} strokeWidth={1.9} className="text-stone-700" />
                    {label}
                  </Link>
                );
              })}
              <div className="border-t border-stone-200" />
              <Link
                href="/acceso"
                className="flex w-full items-center gap-3 px-4 py-3 text-left text-[14px] font-medium text-[#ef4339] transition-colors duration-150 hover:bg-stone-50"
              >
                {pathname === "/acceso" ? (
                  <LogIn size={18} strokeWidth={1.9} className="text-[#ef4339]" />
                ) : (
                  <LogOut size={18} strokeWidth={1.9} className="text-[#ef4339]" />
                )}
                {pathname === "/acceso" ? "Volver al acceso" : "Cerrar sesion"}
              </Link>
            </div>
          </details>
        </div>
      </div>
    </header>
  );
}
