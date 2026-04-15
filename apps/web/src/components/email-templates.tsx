"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import {
  getEmailTemplates,
  updateEmailTemplate,
  createEmailTemplate,
  deleteEmailTemplate,
  previewEmailTemplate,
  type EmailTemplate,
} from "@/lib/api";
import CodeMirror from "@uiw/react-codemirror";
import { html } from "@codemirror/lang-html";
import { useEditor, EditorContent } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import Link from "@tiptap/extension-link";
import TextAlign from "@tiptap/extension-text-align";
import { TextStyle } from "@tiptap/extension-text-style";
import { Color } from "@tiptap/extension-color";
import Image from "@tiptap/extension-image";

type ViewMode = "visual" | "code" | "preview";
type PreviewWidth = "desktop" | "mobile";

const TEMPLATE_VARS = [
  { labelKey: "varName", value: "{{.Name}}" },
  { labelKey: "varEmail", value: "{{.Email}}" },
  { labelKey: "varApp", value: "{{.AppName}}" },
  { labelKey: "varAppUrl", value: "{{.AppURL}}" },
  { labelKey: "varYear", value: "{{.Year}}" },
  { labelKey: "varFile", value: "{{.FileName}}" },
  { labelKey: "varOutputFormat", value: "{{.OutputFormat}}" },
  { labelKey: "varError", value: "{{.ErrorMessage}}" },
  { labelKey: "varResetUrl", value: "{{.ResetURL}}" },
] as const;

function formatDate(value: string): string {
  const date = new Date(value);
  if (date.getFullYear() < 2000) return "";
  return new Intl.DateTimeFormat("es-ES", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}

const TEMPLATE_FRIENDLY_KEYS: Record<string, string> = {
  "conversion-complete": "templateName.conversionComplete",
  "conversion-failed": "templateName.conversionFailed",
  "welcome": "templateName.welcome",
  "password-reset": "templateName.passwordReset",
};

export default function EmailTemplatesSection() {
  const t = useTranslations("emailTemplates");
  const [templates, setTemplates] = useState<EmailTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  useEffect(() => {
    getEmailTemplates()
      .then((data) => {
        setTemplates(data);
        if (data.length > 0 && !selected) setSelected(data[0].key);
      })
      .catch((err) => setError(err instanceof Error ? err.message : t("loadError")))
      .finally(() => setLoading(false));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  function friendlyName(key: string): string {
    const i18nKey = TEMPLATE_FRIENDLY_KEYS[key];
    return i18nKey ? t(i18nKey) : key;
  }

  async function handleDelete(key: string) {
    setDeleteError(null);
    try {
      await deleteEmailTemplate(key);
      setTemplates((prev) => prev.filter((tpl) => tpl.key !== key));
      setConfirmDelete(null);
      if (selected === key) {
        const remaining = templates.filter((tpl) => tpl.key !== key);
        setSelected(remaining.length > 0 ? remaining[0].key : null);
      }
    } catch (err) {
      setDeleteError(err instanceof Error ? err.message : t("deleteError"));
    }
  }

  function handleCreated(newTemplate: EmailTemplate) {
    setTemplates((prev) => [...prev, newTemplate]);
    setSelected(newTemplate.key);
    setCreating(false);
  }

  if (loading) {
    return (
      <section className="rounded-2xl border border-stone-200 bg-white shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
        <div className="px-5 py-8 text-sm text-stone-500">{t("loading")}</div>
      </section>
    );
  }

  if (error) {
    return (
      <section className="rounded-2xl border border-stone-200 bg-white shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
        <div className="px-5 py-4">
          <p className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">{error}</p>
        </div>
      </section>
    );
  }

  const current = templates.find((tpl) => tpl.key === selected);

  return (
    <section className="rounded-2xl border border-stone-200 bg-white shadow-[0_1px_3px_rgba(15,23,42,0.04)]">
      <div className="border-b border-stone-200 px-5 py-4">
        <div className="flex items-start justify-between gap-3">
          <div>
            <h2 className="text-base font-semibold text-stone-900">{t("title")}</h2>
            <p className="mt-1 text-sm text-stone-500">{t("description")}</p>
          </div>
          {!creating && (
            <button
              type="button"
              onClick={() => setCreating(true)}
              className="inline-flex h-9 shrink-0 items-center gap-1.5 rounded-lg border border-stone-200 bg-white px-3 text-sm font-medium text-stone-700 transition-colors hover:bg-stone-50"
            >
              <span className="text-base leading-none">+</span>
              {t("createNew")}
            </button>
          )}
        </div>
      </div>

      {creating ? (
        <CreateTemplateForm
          existingKeys={templates.map((tpl) => tpl.key)}
          onCreated={handleCreated}
          onCancel={() => setCreating(false)}
        />
      ) : (
        <div className="grid lg:grid-cols-[240px_minmax(0,1fr)]">
          {/* Template list sidebar */}
          <div className="border-b border-stone-200 lg:border-b-0 lg:border-r">
            <nav className="flex gap-1 overflow-x-auto p-2 lg:flex-col lg:overflow-x-visible">
              {templates.map((tpl) => {
                const isSelected = tpl.key === selected;
                const dateStr = formatDate(tpl.updated_at);
                return (
                  <button
                    key={tpl.key}
                    type="button"
                    onClick={() => setSelected(tpl.key)}
                    className={`group flex w-full min-w-36 flex-col rounded-lg px-3 py-2.5 text-left transition-colors lg:min-w-0 ${
                      isSelected
                        ? "bg-stone-100 text-stone-900"
                        : "text-stone-600 hover:bg-stone-50"
                    }`}
                  >
                    <span className="text-sm font-medium leading-tight">{friendlyName(tpl.key)}</span>
                    <span className="mt-0.5 truncate text-xs text-stone-500">{tpl.subject}</span>
                    {dateStr && <span className="mt-1 text-[11px] text-stone-400">{dateStr}</span>}
                  </button>
                );
              })}
            </nav>
          </div>

          {/* Editor panel */}
          <div className="min-w-0">
            {current ? (
              <TemplateEditor
                key={current.key}
                template={current}
                onSave={(updated) => {
                  setTemplates((prev) => prev.map((tpl) => (tpl.key === updated.key ? updated : tpl)));
                }}
                onDeleteRequest={() => setConfirmDelete(current.key)}
              />
            ) : (
              <div className="px-5 py-12 text-center text-sm text-stone-500">
                {templates.length === 0 ? t("noTemplates") : t("selectTemplate")}
              </div>
            )}
          </div>
        </div>
      )}

      {/* Delete confirmation */}
      {confirmDelete && (
        <div className="border-t border-stone-200 px-5 py-4">
          <div className="flex items-center justify-between gap-3 rounded-lg border border-rose-200 bg-rose-50 px-4 py-3">
            <p className="text-sm text-rose-700">
              {t("deleteConfirm", { name: friendlyName(confirmDelete) })}
            </p>
            <div className="flex gap-2">
              <button
                type="button"
                onClick={() => { setConfirmDelete(null); setDeleteError(null); }}
                className="inline-flex h-8 items-center rounded-md border border-stone-200 bg-white px-3 text-xs font-medium text-stone-700 hover:bg-stone-50"
              >
                {t("cancelDelete")}
              </button>
              <button
                type="button"
                onClick={() => void handleDelete(confirmDelete)}
                className="inline-flex h-8 items-center rounded-md bg-rose-600 px-3 text-xs font-medium text-white hover:bg-rose-700"
              >
                {t("confirmDeleteBtn")}
              </button>
            </div>
          </div>
          {deleteError && (
            <p className="mt-2 text-sm text-rose-600">{deleteError}</p>
          )}
        </div>
      )}
    </section>
  );
}

// ── Create Template Form ──

function CreateTemplateForm({
  existingKeys,
  onCreated,
  onCancel,
}: {
  existingKeys: string[];
  onCreated: (template: EmailTemplate) => void;
  onCancel: () => void;
}) {
  const t = useTranslations("emailTemplates");
  const tCommon = useTranslations("common");
  const [key, setKey] = useState("");
  const [subject, setSubject] = useState("");
  const [bodyHtml, setBodyHtml] = useState(DEFAULT_NEW_BODY);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const keySlug = key
    .toLowerCase()
    .replace(/[^a-z0-9-]/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "");

  const keyConflict = existingKeys.includes(keySlug);

  async function handleCreate() {
    if (!keySlug || !subject.trim()) return;
    setSaving(true);
    setError(null);
    try {
      const created = await createEmailTemplate({
        key: keySlug,
        subject: subject.trim(),
        body_html: bodyHtml,
      });
      onCreated(created);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("createError"));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="px-5 py-5">
      <div className="grid gap-4 sm:grid-cols-2">
        <label className="block">
          <span className="mb-1.5 block text-[13px] font-medium text-stone-600">{t("keyLabel")}</span>
          <input
            type="text"
            value={key}
            onChange={(e) => { setKey(e.target.value); setError(null); }}
            placeholder={t("keyPlaceholder")}
            className="h-10 w-full rounded-lg border border-stone-200 bg-stone-50/60 px-3 text-sm text-stone-900 transition-colors focus:border-coral-400 focus:bg-white"
          />
          {keySlug && (
            <p className={`mt-1 text-xs ${keyConflict ? "text-rose-500" : "text-stone-400"}`}>
              {keyConflict ? t("keyConflict") : keySlug}
            </p>
          )}
        </label>

        <label className="block">
          <span className="mb-1.5 block text-[13px] font-medium text-stone-600">{t("subjectLabel")}</span>
          <input
            type="text"
            value={subject}
            onChange={(e) => { setSubject(e.target.value); setError(null); }}
            placeholder={t("subjectPlaceholder")}
            className="h-10 w-full rounded-lg border border-stone-200 bg-stone-50/60 px-3 text-sm text-stone-900 transition-colors focus:border-coral-400 focus:bg-white"
          />
        </label>
      </div>

      <label className="mt-4 block">
        <span className="mb-1.5 block text-[13px] font-medium text-stone-600">{t("bodyLabel")}</span>
        <div className="overflow-hidden rounded-lg border border-stone-200">
          <CodeMirror
            value={bodyHtml}
            onChange={(v) => setBodyHtml(v)}
            extensions={[html()]}
            height="220px"
            basicSetup={{ lineNumbers: true, foldGutter: true, highlightActiveLine: true, autocompletion: true }}
          />
        </div>
      </label>

      <div className="mt-4 flex items-center justify-between gap-3">
        <p className="text-xs text-stone-500">{t("createHint")}</p>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={onCancel}
            className="inline-flex h-10 items-center rounded-lg border border-stone-200 bg-white px-4 text-sm font-medium text-stone-700 transition-colors hover:bg-stone-50"
          >
            {tCommon("cancel")}
          </button>
          <button
            type="button"
            onClick={() => void handleCreate()}
            disabled={saving || !keySlug || !subject.trim() || keyConflict}
            className="inline-flex h-10 items-center rounded-lg bg-coral-500 px-4 text-sm font-medium text-white transition-colors hover:bg-coral-600 disabled:cursor-not-allowed disabled:bg-coral-200"
          >
            {saving ? tCommon("saving") : t("createTemplate")}
          </button>
        </div>
      </div>

      {error && (
        <p className="mt-3 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">{error}</p>
      )}
    </div>
  );
}

const DEFAULT_NEW_BODY = `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: sans-serif; padding: 24px;">
  <h2>{{.AppName}}</h2>
  <p>Hola {{.Name}},</p>
  <p></p>
  <p style="color: #999; font-size: 12px;">&copy; {{.Year}} {{.AppName}}</p>
</body>
</html>`;

function TemplateEditor({
  template,
  onSave,
  onDeleteRequest,
}: {
  template: EmailTemplate;
  onSave: (updated: EmailTemplate) => void;
  onDeleteRequest: () => void;
}) {
  const t = useTranslations("emailTemplates");
  const tCommon = useTranslations("common");
  const [subject, setSubject] = useState(template.subject);
  const [bodyHtml, setBodyHtml] = useState(template.body_html);
  const [viewMode, setViewMode] = useState<ViewMode>("visual");
  const [previewHtml, setPreviewHtml] = useState<string | null>(null);
  const [previewSubject, setPreviewSubject] = useState<string | null>(null);
  const [previewWidth, setPreviewWidth] = useState<PreviewWidth>("desktop");

  const [saving, setSaving] = useState(false);
  const [previewing, setPreviewing] = useState(false);
  const [status, setStatus] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const iframeRef = useRef<HTMLIFrameElement>(null);
  const bodyHtmlRef = useRef(bodyHtml);
  bodyHtmlRef.current = bodyHtml;

  const dirty = subject !== template.subject || bodyHtml !== template.body_html;

  // TipTap editor instance
  const editor = useEditor({
    extensions: [
      StarterKit,
      Link.configure({ openOnClick: false }),
      TextAlign.configure({ types: ["heading", "paragraph"] }),
      TextStyle,
      Color.configure({ types: [TextStyle.name] }),
      Image.configure({ inline: true, allowBase64: true }),
    ],
    content: template.body_html,
    immediatelyRender: false,
    onUpdate: ({ editor: e }) => {
      const html = e.getHTML();
      setBodyHtml(html);
      setError(null);
      setStatus(null);
    },
  });

  const handleCodeChange = useCallback((value: string) => {
    setBodyHtml(value);
    setError(null);
    setStatus(null);
  }, []);

  // Sync TipTap content when switching from code → visual
  function switchMode(mode: ViewMode) {
    if (mode === "visual" && viewMode === "code" && editor) {
      editor.commands.setContent(bodyHtmlRef.current);
    }
    if (mode === "preview") {
      void handlePreview();
      return;
    }
    setViewMode(mode);
  }

  function handleDiscard() {
    setSubject(template.subject);
    setBodyHtml(template.body_html);
    if (editor) editor.commands.setContent(template.body_html);
    setError(null);
    setStatus(null);
  }

  function insertVariable(v: string) {
    if (viewMode === "visual" && editor) {
      editor.chain().focus().insertContent(v).run();
    }
  }

  async function handleSave() {
    setSaving(true);
    setError(null);
    setStatus(null);

    try {
      const updated = await updateEmailTemplate(template.key, {
        subject,
        body_html: bodyHtml,
      });
      onSave(updated);
      setStatus(t("saved"));
    } catch (err) {
      setError(err instanceof Error ? err.message : t("saveError"));
    } finally {
      setSaving(false);
    }
  }

  async function handlePreview() {
    setPreviewing(true);
    setError(null);

    try {
      const result = await previewEmailTemplate(template.key, {
        subject,
        body_html: bodyHtml,
      });
      setPreviewHtml(result.html);
      setPreviewSubject(result.subject);
      setViewMode("preview");
    } catch (err) {
      setError(err instanceof Error ? err.message : t("previewError"));
    } finally {
      setPreviewing(false);
    }
  }

  useEffect(() => {
    if (viewMode === "preview" && previewHtml && iframeRef.current) {
      const doc = iframeRef.current.contentDocument;
      if (doc) {
        doc.open();
        doc.write(previewHtml);
        doc.close();
      }
    }
  }, [viewMode, previewHtml]);

  return (
    <div className="px-5 py-4">
      <label className="block">
        <span className="mb-1.5 block text-[13px] font-medium text-stone-600">{t("subjectLabel")}</span>
        <input
          type="text"
          value={subject}
          onChange={(e) => { setSubject(e.target.value); setError(null); setStatus(null); }}
          className="h-10 w-full rounded-lg border border-stone-200 bg-stone-50/60 px-3 text-sm text-stone-900 transition-colors focus:border-coral-400 focus:bg-white"
        />
      </label>

      {/* Variable chips — only shown in visual mode */}
      {viewMode === "visual" && (
        <div className="mt-3 flex flex-wrap items-center gap-1.5">
          <span className="text-xs text-stone-500">{t("variablesLabel")}:</span>
          {TEMPLATE_VARS.map((v) => (
            <button
              key={v.value}
              type="button"
              onClick={() => insertVariable(v.value)}
              className="rounded border border-stone-200 bg-stone-50 px-2 py-0.5 text-xs font-medium text-stone-600 transition-colors hover:bg-stone-100"
            >
              {t(v.labelKey)}
            </button>
          ))}
        </div>
      )}

      <div className="mt-4">
        <div className="flex items-center justify-between">
          <span className="text-[13px] font-medium text-stone-600">
            {t("bodyLabel")}
          </span>
          <div className="flex gap-1">
            {(["visual", "code", "preview"] as const).map((mode) => {
              const labels: Record<ViewMode, string> = { visual: t("modeVisual"), code: t("modeCode"), preview: t("modePreview") };
              return (
                <button
                  key={mode}
                  type="button"
                  onClick={() => switchMode(mode)}
                  disabled={mode === "preview" && previewing}
                  className={
                    viewMode === mode
                      ? "rounded-md border border-stone-900 bg-stone-900 px-2.5 py-1 text-xs font-medium text-white"
                      : "rounded-md border border-stone-200 bg-white px-2.5 py-1 text-xs font-medium text-stone-600 hover:bg-stone-50"
                  }
                >
                  {mode === "preview" && previewing ? tCommon("loading") : labels[mode]}
                </button>
              );
            })}
          </div>
        </div>

        {/* TipTap toolbar — only in visual mode */}
        {viewMode === "visual" && editor && <TiptapToolbar editor={editor} />}

        <div className="mt-2 overflow-hidden rounded-lg border border-stone-200">
          {viewMode === "visual" ? (
            <div className="prose prose-sm max-w-none px-4 py-3 [&_.ProseMirror]:min-h-95 [&_.ProseMirror]:outline-none">
              <EditorContent editor={editor} />
            </div>
          ) : viewMode === "code" ? (
            <CodeMirror
              value={bodyHtml}
              onChange={handleCodeChange}
              extensions={[html()]}
              height="420px"
              basicSetup={{
                lineNumbers: true,
                foldGutter: true,
                highlightActiveLine: true,
                autocompletion: true,
              }}
            />
          ) : (
            <div className="bg-white">
              {previewSubject && (
                <div className="flex items-center justify-between border-b border-stone-200 bg-stone-50 px-4 py-2.5">
                  <div>
                    <p className="text-xs text-stone-500">{t("renderedSubject")}</p>
                    <p className="mt-0.5 text-sm font-medium text-stone-900">{previewSubject}</p>
                  </div>
                  <div className="flex gap-1">
                    <button
                      type="button"
                      onClick={() => setPreviewWidth("desktop")}
                      className={
                        previewWidth === "desktop"
                          ? "rounded-md border border-stone-900 bg-stone-900 px-2 py-0.5 text-xs font-medium text-white"
                          : "rounded-md border border-stone-200 bg-white px-2 py-0.5 text-xs font-medium text-stone-600 hover:bg-stone-50"
                      }
                    >
                      Desktop
                    </button>
                    <button
                      type="button"
                      onClick={() => setPreviewWidth("mobile")}
                      className={
                        previewWidth === "mobile"
                          ? "rounded-md border border-stone-900 bg-stone-900 px-2 py-0.5 text-xs font-medium text-white"
                          : "rounded-md border border-stone-200 bg-white px-2 py-0.5 text-xs font-medium text-stone-600 hover:bg-stone-50"
                      }
                    >
                      Mobile
                    </button>
                  </div>
                </div>
              )}
              <div className="flex justify-center bg-stone-100 py-4">
                <iframe
                  ref={iframeRef}
                  title={t("previewIframeTitle")}
                  className={`h-125 border border-stone-200 bg-white transition-all ${previewWidth === "mobile" ? "w-93.75" : "w-full max-w-175"}`}
                  sandbox="allow-same-origin"
                />
              </div>
            </div>
          )}
        </div>
      </div>

      <div className="mt-4 flex items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <p className="text-xs text-stone-500">
            {t("lastUpdate")}: {formatDate(template.updated_at) || t("neverModified")}
          </p>
          <button
            type="button"
            onClick={onDeleteRequest}
            className="text-xs text-stone-400 transition-colors hover:text-rose-500"
          >
            {t("deleteTemplate")}
          </button>
        </div>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={handleDiscard}
            disabled={!dirty}
            className="inline-flex h-10 items-center rounded-lg border border-stone-200 bg-white px-4 text-sm font-medium text-stone-700 transition-colors hover:bg-stone-50 disabled:cursor-not-allowed disabled:text-stone-400"
          >
            {t("discard")}
          </button>
          <button
            type="button"
            onClick={() => void handleSave()}
            disabled={saving || !dirty}
            className="inline-flex h-10 items-center rounded-lg bg-coral-500 px-4 text-sm font-medium text-white transition-colors hover:bg-coral-600 disabled:cursor-not-allowed disabled:bg-coral-200"
          >
            {saving ? tCommon("saving") : t("saveTemplate")}
          </button>
        </div>
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
    </div>
  );
}

// ── TipTap Toolbar ──

function TiptapToolbar({ editor }: { editor: ReturnType<typeof useEditor> }) {
  const t = useTranslations("emailTemplates");
  if (!editor) return null;

  const btnClass = (active: boolean) =>
    active
      ? "rounded-md border border-stone-400 bg-stone-200 px-2 py-1 text-xs font-medium text-stone-900"
      : "rounded-md border border-stone-200 bg-white px-2 py-1 text-xs font-medium text-stone-600 hover:bg-stone-50";

  return (
    <div className="mt-2 flex flex-wrap gap-1 rounded-t-lg border border-b-0 border-stone-200 bg-stone-50 px-3 py-2">
      <button type="button" onClick={() => editor.chain().focus().toggleBold().run()} className={btnClass(editor.isActive("bold"))}>
        B
      </button>
      <button type="button" onClick={() => editor.chain().focus().toggleItalic().run()} className={btnClass(editor.isActive("italic"))}>
        I
      </button>
      <button
        type="button"
        onClick={() => {
          const url = window.prompt(t("linkPrompt"));
          if (url) editor.chain().focus().extendMarkRange("link").setLink({ href: url }).run();
        }}
        className={btnClass(editor.isActive("link"))}
      >
        Link
      </button>
      <button type="button" onClick={() => editor.chain().focus().unsetLink().run()} className={btnClass(false)}>
        Unlink
      </button>

      <span className="mx-1 w-px self-stretch bg-stone-200" />

      <button type="button" onClick={() => editor.chain().focus().setTextAlign("left").run()} className={btnClass(editor.isActive({ textAlign: "left" }))}>
        {t("alignLeft")}
      </button>
      <button type="button" onClick={() => editor.chain().focus().setTextAlign("center").run()} className={btnClass(editor.isActive({ textAlign: "center" }))}>
        {t("alignCenter")}
      </button>
      <button type="button" onClick={() => editor.chain().focus().setTextAlign("right").run()} className={btnClass(editor.isActive({ textAlign: "right" }))}>
        {t("alignRight")}
      </button>

      <span className="mx-1 w-px self-stretch bg-stone-200" />

      <button type="button" onClick={() => editor.chain().focus().toggleBulletList().run()} className={btnClass(editor.isActive("bulletList"))}>
        {t("list")}
      </button>
      <button type="button" onClick={() => editor.chain().focus().toggleOrderedList().run()} className={btnClass(editor.isActive("orderedList"))}>
        {t("numberedList")}
      </button>

      <span className="mx-1 w-px self-stretch bg-stone-200" />

      <label className="flex items-center gap-1 text-xs text-stone-600">
        {t("colorLabel")}
        <input
          type="color"
          onChange={(e) => editor.chain().focus().setColor(e.target.value).run()}
          defaultValue="#000000"
          className="h-5 w-5 cursor-pointer border-0 bg-transparent p-0"
        />
      </label>

      <button
        type="button"
        onClick={() => {
          const url = window.prompt(t("imagePrompt"));
          if (url) editor.chain().focus().setImage({ src: url }).run();
        }}
        className={btnClass(false)}
      >
        {t("imageBtn")}
      </button>
    </div>
  );
}
