"use client";

import { useEffect, useRef, useState } from "react";
import Image from "next/image";
import Link from "next/link";
import { useAuth } from "@/lib/auth-context";
import { confirmEmailVerification, confirmPasswordReset, requestPasswordReset } from "@/lib/auth";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { ChevronLeft } from "lucide-react";

export type AuthMode = "login" | "register" | "recover" | "reset" | "verify";

interface AuthPanelProps {
  mode: AuthMode;
  onModeChange: (mode: AuthMode) => void;
  resetToken?: string | null;
  verifyToken?: string | null;
}

export default function AuthPanel({ mode, onModeChange, resetToken, verifyToken }: AuthPanelProps) {
  const isLogin = mode === "login";
  const isRegister = mode === "register";
  const isRecover = mode === "recover";
  const isReset = mode === "reset";
  const isVerify = mode === "verify";
  const router = useRouter();
  const { login: authLogin, register: authRegister } = useAuth();
  const t = useTranslations("auth");
  const tc = useTranslations("common");

  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const verificationStarted = useRef(false);

  useEffect(() => {
    if (!isVerify || verificationStarted.current) {
      return;
    }
    verificationStarted.current = true;
    setError(null);
    setNotice(null);
    if (!verifyToken) {
      setError(t("verifyTokenMissing"));
      return;
    }

    setLoading(true);
    confirmEmailVerification({ token: verifyToken })
      .then(() => setNotice(t("verifySuccess")))
      .catch((err: unknown) => setError(err instanceof Error ? err.message : t("unexpectedError")))
      .finally(() => setLoading(false));
  }, [isVerify, t, verifyToken]);

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);
    setNotice(null);

    if (isVerify) {
      onModeChange("login");
      router.push("/acceso");
      return;
    }

    const form = new FormData(e.currentTarget);
    const email = (form.get("email") as string)?.trim().toLowerCase();
    const password = form.get("password") as string;

    if (isRecover) {
      if (!email) {
        setError(t("emailOnlyRequired"));
        return;
      }
      setLoading(true);
      try {
        await requestPasswordReset({ email });
        setNotice(t("recoverSent"));
      } catch (err: unknown) {
        setError(err instanceof Error ? err.message : t("unexpectedError"));
      } finally {
        setLoading(false);
      }
      return;
    }

    if (isReset) {
      const passwordConfirmation = form.get("passwordConfirmation") as string;
      if (!resetToken) {
        setError(t("resetTokenMissing"));
        return;
      }
      if (!password || !passwordConfirmation) {
        setError(t("passwordRequired"));
        return;
      }
      if (password.length < 8) {
        setError(t("passwordMinLength"));
        return;
      }
      if (password !== passwordConfirmation) {
        setError(t("passwordMismatch"));
        return;
      }
      setLoading(true);
      try {
        await confirmPasswordReset({ token: resetToken, password });
        setNotice(t("resetSuccess"));
        onModeChange("login");
        router.push("/acceso");
      } catch (err: unknown) {
        setError(err instanceof Error ? err.message : t("unexpectedError"));
      } finally {
        setLoading(false);
      }
      return;
    }

    if (!email || !password) {
      setError(t("emailRequired"));
      return;
    }

    if (password.length < 8) {
      setError(t("passwordMinLength"));
      return;
    }

    if (isRegister) {
      const name = (form.get("name") as string)?.trim();
      if (!name) {
        setError(t("nameRequired"));
        return;
      }
    }

    setLoading(true);

    try {
      if (isLogin) {
        await authLogin({ email, password });
      } else {
        const name = (form.get("name") as string)?.trim();
        const passwordConfirmation = form.get("passwordConfirmation") as string;

        if (password !== passwordConfirmation) {
          setError(t("passwordMismatch"));
          setLoading(false);
          return;
        }

        await authRegister({ name, email, password });
      }
      router.push("/");
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t("unexpectedError"));
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="relative w-full max-w-135">
      <Link
        href="/"
        className="absolute -left-16 top-0 hidden lg:inline-flex h-12 w-12 items-center justify-center rounded-full border border-stone-200 bg-white text-stone-600 shadow-sm transition-colors duration-150 hover:border-stone-300 hover:text-stone-900 outline-none focus-visible:ring-2 focus-visible:ring-coral-400/40 focus-visible:ring-offset-1"
      >
        <ChevronLeft size={22} strokeWidth={2} />
      </Link>

    <section className="flex w-full min-h-175 flex-col rounded-[34px] border border-white/80 bg-white px-7 py-7 shadow-[0_24px_60px_-42px_rgba(15,23,42,0.24)] sm:min-h-170 sm:px-9 sm:py-8">
      <div className="flex justify-center">
        <Image
          src="/favicon.svg"
          alt="Reform Lab"
          width={128}
          height={128}
          className="h-32 w-32"
          priority
        />
      </div>

      {!isRecover && !isReset && !isVerify ? (
			<div className="mt-5 flex gap-1 rounded-[22px] bg-stone-100 p-1">
				<button
					type="button"
					onClick={() => onModeChange("login")}
					className={`flex-1 rounded-[18px] py-2.5 text-sm font-medium outline-none transition-colors duration-150 focus-visible:ring-2 focus-visible:ring-coral-400/40 focus-visible:ring-offset-1 ${
						isLogin ? "bg-white text-stone-900 shadow-sm" : "text-stone-500 hover:text-stone-700"
					}`}
				>
					{t("loginTab")}
				</button>
				<button
					type="button"
					onClick={() => onModeChange("register")}
					className={`flex-1 rounded-[18px] py-2.5 text-sm font-medium outline-none transition-colors duration-150 focus-visible:ring-2 focus-visible:ring-coral-400/40 focus-visible:ring-offset-1 ${
						!isLogin ? "bg-white text-stone-900 shadow-sm" : "text-stone-500 hover:text-stone-700"
					}`}
				>
					{t("registerTab")}
				</button>
			</div>
		) : null}

    <h1 className="mt-6 text-[22px] font-semibold tracking-[-0.02em] text-stone-900">
      {isRecover
        ? t("recoverTitle")
        : isReset
          ? t("resetTitle")
          : isVerify
            ? t("verifyTitle")
          : isLogin
            ? t("loginTitle")
            : t("registerTitle")}
    </h1>
    <p className="mt-1.5 text-sm leading-5 text-stone-500">
      {isRecover
        ? t("recoverSubtitle")
        : isReset
          ? t("resetSubtitle")
          : isVerify
            ? t("verifySubtitle")
          : isLogin
            ? t("loginSubtitle")
            : t("registerSubtitle")}
    </p>

      <form className="mt-5 flex flex-1 flex-col space-y-3.5" onSubmit={handleSubmit}>
        {error && (
          <p className="rounded-[18px] bg-red-50 px-3.5 py-2.5 text-[13px] font-medium text-red-600">{error}</p>
        )}
			{notice && (
				<p className="rounded-[18px] bg-stone-100 px-3.5 py-2.5 text-[13px] font-medium text-stone-700">{notice}</p>
			)}
			{isRegister ? (
          <label className="block">
            <span className="mb-1.5 block text-[13px] font-medium text-stone-600">{t("nameLabel")}</span>
            <input
              type="text"
              name="name"
              placeholder={t("namePlaceholder")}
              className="h-11 w-full rounded-[18px] border border-stone-200 bg-stone-50/60 px-3.5 text-sm text-stone-900 placeholder:text-stone-400 outline-none transition-colors duration-150 focus:border-coral-400 focus:bg-white focus-visible:ring-2 focus-visible:ring-coral-400/40 focus-visible:ring-offset-1"
            />
          </label>
        ) : null}

      {!isReset && !isVerify ? (
        <label className="block">
          <span className="mb-1.5 block text-[13px] font-medium text-stone-600">{t("emailLabel")}</span>
          <input
            type="email"
            name="email"
            placeholder={t("emailPlaceholder")}
            className="h-11 w-full rounded-[18px] border border-stone-200 bg-stone-50/60 px-3.5 text-sm text-stone-900 placeholder:text-stone-400 outline-none transition-colors duration-150 focus:border-coral-400 focus:bg-white focus-visible:ring-2 focus-visible:ring-coral-400/40 focus-visible:ring-offset-1"
          />
        </label>
      ) : null}

      {isRecover ? (
        <div className="mt-1! flex items-center justify-end">
          <button
            type="button"
            onClick={() => onModeChange("login")}
            className="text-[13px] font-medium text-stone-600 hover:text-stone-900"
          >
            {t("backToLogin")}
          </button>
        </div>
      ) : isVerify ? (
        <div className="mt-1! flex items-center justify-end">
          <button
            type="button"
            onClick={() => {
              onModeChange("login");
              router.push("/acceso");
            }}
            className="text-[13px] font-medium text-stone-600 hover:text-stone-900"
          >
            {t("backToLogin")}
          </button>
        </div>
      ) : isLogin ? (
          <>
            <label className="block">
              <span className="mb-1.5 block text-[13px] font-medium text-stone-600">{t("passwordLabel")}</span>
              <input
                type="password"
                name="password"
                placeholder={t("passwordPlaceholder")}
                className="h-11 w-full rounded-[18px] border border-stone-200 bg-stone-50/60 px-3.5 text-sm text-stone-900 placeholder:text-stone-400 outline-none transition-colors duration-150 focus:border-coral-400 focus:bg-white focus-visible:ring-2 focus-visible:ring-coral-400/40 focus-visible:ring-offset-1"
              />
            </label>
            <div className="mt-2! flex items-center justify-end">
					<Link href="/acceso?mode=recover" className="text-[13px] font-medium text-stone-600 hover:text-stone-900">
						{t("recoverAccess")}
					</Link>
            </div>
          </>
        ) : isRegister || isReset ? (
          <div className="grid gap-3.5 sm:grid-cols-2">
            <label className="block">
              <span className="mb-1.5 block text-[13px] font-medium text-stone-600">{t("passwordLabel")}</span>
              <input
                type="password"
                name="password"
                placeholder={t("passwordPlaceholder")}
                className="h-11 w-full rounded-[18px] border border-stone-200 bg-stone-50/60 px-3.5 text-sm text-stone-900 placeholder:text-stone-400 outline-none transition-colors duration-150 focus:border-coral-400 focus:bg-white focus-visible:ring-2 focus-visible:ring-coral-400/40 focus-visible:ring-offset-1"
              />
            </label>
            <label className="block">
              <span className="mb-1.5 block text-[13px] font-medium text-stone-600">{t("confirmPasswordLabel")}</span>
              <input
                type="password"
                name="passwordConfirmation"
                placeholder={t("passwordPlaceholder")}
                className="h-11 w-full rounded-[18px] border border-stone-200 bg-stone-50/60 px-3.5 text-sm text-stone-900 placeholder:text-stone-400 outline-none transition-colors duration-150 focus:border-coral-400 focus:bg-white focus-visible:ring-2 focus-visible:ring-coral-400/40 focus-visible:ring-offset-1"
              />
            </label>
          </div>
		) : null}

			{isRegister ? (
          <label className="flex items-start gap-2.5 pt-1 text-[13px] leading-5 text-stone-500">
            <input type="checkbox" className="mt-0.5 h-4 w-4 rounded-md border-stone-300 accent-coral-500" />
            <span>
              {t("terms")}
            </span>
          </label>
        ) : null}

        <button
          type="submit"
          disabled={loading}
          className="mt-auto h-11 w-full rounded-[20px] bg-coral-500 text-sm font-medium text-white outline-none transition-colors duration-150 hover:bg-coral-600 disabled:opacity-50 focus-visible:ring-2 focus-visible:ring-coral-400/40 focus-visible:ring-offset-2"
        >
			{loading
				? tc("loading")
				: isRecover
					? t("submitRecover")
					: isReset
						? t("submitReset")
						: isVerify
							? t("backToLogin")
						: isLogin
							? t("submitLogin")
							: t("submitRegister")}
        </button>
      </form>
    </section>
    </div>
  );
}
