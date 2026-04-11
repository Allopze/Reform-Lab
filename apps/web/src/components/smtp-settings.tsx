"use client";

import { useEffect, useState } from "react";
import {
  getSMTPSettings,
  updateSMTPSettings,
  testSMTPConnection,
  type SMTPSettings,
} from "@/lib/api";

export default function SMTPSettingsSection() {
  const [settings, setSettings] = useState<SMTPSettings | null>(null);
  const [loading, setLoading] = useState(true);

  const [host, setHost] = useState("");
  const [port, setPort] = useState("587");
  const [user, setUser] = useState("");
  const [password, setPassword] = useState("");
  const [from, setFrom] = useState("");
  const [useTLS, setUseTLS] = useState(true);

  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [status, setStatus] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getSMTPSettings()
      .then((data) => {
        setSettings(data);
        setHost(data.host);
        setPort(String(data.port || 587));
        setUser(data.user);
        setPassword(data.password);
        setFrom(data.from);
        setUseTLS(data.use_tls);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "No se pudo cargar la configuracion SMTP.");
      })
      .finally(() => setLoading(false));
  }, []);

  const dirty =
    settings !== null &&
    (host !== settings.host ||
      port !== String(settings.port || 587) ||
      user !== settings.user ||
      (password !== settings.password && password !== "") ||
      from !== settings.from ||
      useTLS !== settings.use_tls);

  async function handleSave() {
    setSaving(true);
    setError(null);
    setStatus(null);

    try {
      await updateSMTPSettings({
        host,
        port: Number(port) || 587,
        user,
        password,
        from,
        use_tls: useTLS,
      });
      const updated = await getSMTPSettings();
      setSettings(updated);
      setHost(updated.host);
      setPort(String(updated.port || 587));
      setUser(updated.user);
      setPassword(updated.password);
      setFrom(updated.from);
      setUseTLS(updated.use_tls);
      setStatus("Configuracion SMTP guardada.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "No se pudo guardar la configuracion.");
    } finally {
      setSaving(false);
    }
  }

  async function handleTest() {
    setTesting(true);
    setError(null);
    setStatus(null);

    try {
      const result = await testSMTPConnection();
      if (result.success) {
        setStatus(result.message);
      } else {
        setError(result.message);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "No se pudo enviar el correo de prueba.");
    } finally {
      setTesting(false);
    }
  }

  if (loading) {
    return (
      <section className="rounded-xl border border-stone-200 bg-white px-5 py-4">
        <p className="text-sm text-stone-500">Cargando configuracion SMTP...</p>
      </section>
    );
  }

  return (
    <section className="rounded-xl border border-stone-200 bg-white px-5 py-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-base font-semibold text-stone-900">Correo SMTP</h2>
          <p className="mt-1 text-sm text-stone-500">
            Configura el servidor de correo saliente.
            {settings?.source === "env" && " Valores base cargados desde .env."}
            {settings?.source === "admin" && " Valores personalizados por admin."}
            {settings?.source === "none" && " Sin configurar — los correos no se enviaran."}
          </p>
        </div>
      </div>

      <div className="mt-4 space-y-3">
        <div className="grid gap-3 sm:grid-cols-2">
          <label className="block">
            <span className="mb-1.5 block text-[13px] font-medium text-stone-600">Host</span>
            <input
              type="text"
              value={host}
              onChange={(e) => { setHost(e.target.value); setError(null); setStatus(null); }}
              placeholder="smtp.ejemplo.com"
              className="h-10 w-full rounded-lg border border-stone-200 bg-stone-50/60 px-3 text-sm text-stone-900 transition-colors focus:border-coral-400 focus:bg-white"
            />
          </label>
          <label className="block">
            <span className="mb-1.5 block text-[13px] font-medium text-stone-600">Puerto</span>
            <input
              type="number"
              value={port}
              onChange={(e) => { setPort(e.target.value); setError(null); setStatus(null); }}
              min={1}
              max={65535}
              className="h-10 w-full rounded-lg border border-stone-200 bg-stone-50/60 px-3 text-sm text-stone-900 transition-colors focus:border-coral-400 focus:bg-white"
            />
          </label>
        </div>

        <div className="grid gap-3 sm:grid-cols-2">
          <label className="block">
            <span className="mb-1.5 block text-[13px] font-medium text-stone-600">Usuario</span>
            <input
              type="text"
              value={user}
              onChange={(e) => { setUser(e.target.value); setError(null); setStatus(null); }}
              placeholder="usuario@ejemplo.com"
              autoComplete="off"
              className="h-10 w-full rounded-lg border border-stone-200 bg-stone-50/60 px-3 text-sm text-stone-900 transition-colors focus:border-coral-400 focus:bg-white"
            />
          </label>
          <label className="block">
            <span className="mb-1.5 block text-[13px] font-medium text-stone-600">Contraseña</span>
            <input
              type="password"
              value={password}
              onChange={(e) => { setPassword(e.target.value); setError(null); setStatus(null); }}
              placeholder={settings?.password === "****" ? "••••••••" : ""}
              autoComplete="new-password"
              className="h-10 w-full rounded-lg border border-stone-200 bg-stone-50/60 px-3 text-sm text-stone-900 transition-colors focus:border-coral-400 focus:bg-white"
            />
          </label>
        </div>

        <label className="block">
          <span className="mb-1.5 block text-[13px] font-medium text-stone-600">Remitente (From)</span>
          <input
            type="email"
            value={from}
            onChange={(e) => { setFrom(e.target.value); setError(null); setStatus(null); }}
            placeholder="noreply@ejemplo.com"
            className="h-10 w-full rounded-lg border border-stone-200 bg-stone-50/60 px-3 text-sm text-stone-900 transition-colors focus:border-coral-400 focus:bg-white"
          />
        </label>

        <label className="flex items-center gap-2.5 py-1">
          <input
            type="checkbox"
            checked={useTLS}
            onChange={(e) => { setUseTLS(e.target.checked); setError(null); setStatus(null); }}
            className="h-4 w-4 rounded border-stone-300 text-coral-500 focus:ring-coral-400"
          />
          <span className="text-sm text-stone-700">Usar TLS</span>
        </label>
      </div>

      <div className="mt-4 flex items-center gap-3">
        <button
          type="button"
          onClick={() => void handleSave()}
          disabled={saving || !dirty}
          className="inline-flex h-10 items-center rounded-lg bg-coral-500 px-4 text-sm font-medium text-white transition-colors hover:bg-coral-600 disabled:cursor-not-allowed disabled:bg-coral-200"
        >
          {saving ? "Guardando..." : "Guardar SMTP"}
        </button>
        <button
          type="button"
          onClick={() => void handleTest()}
          disabled={testing || !host}
          className="inline-flex h-10 items-center rounded-lg border border-stone-200 bg-white px-4 text-sm font-medium text-stone-700 transition-colors hover:bg-stone-50 disabled:cursor-not-allowed disabled:text-stone-400"
        >
          {testing ? "Enviando..." : "Enviar correo de prueba"}
        </button>
      </div>

      {status && (
        <p className="mt-3 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
          {status}
        </p>
      )}

      {error && (
        <p className="mt-3 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
          {error}
        </p>
      )}
    </section>
  );
}
