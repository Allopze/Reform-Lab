"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  getEmailTemplates,
  updateEmailTemplate,
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
  { label: "Nombre", value: "{{.Name}}" },
  { label: "Email", value: "{{.Email}}" },
  { label: "App", value: "{{.AppName}}" },
  { label: "URL App", value: "{{.AppURL}}" },
  { label: "Año", value: "{{.Year}}" },
  { label: "Archivo", value: "{{.FileName}}" },
  { label: "Formato destino", value: "{{.OutputFormat}}" },
  { label: "Error", value: "{{.ErrorMessage}}" },
  { label: "URL reset", value: "{{.ResetURL}}" },
];

function formatDate(value: string): string {
  return new Intl.DateTimeFormat("es-ES", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

export default function EmailTemplatesSection() {
  const [templates, setTemplates] = useState<EmailTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<string | null>(null);

  useEffect(() => {
    getEmailTemplates()
      .then((data) => {
        setTemplates(data);
        if (data.length > 0 && !selected) setSelected(data[0].key);
      })
      .catch((err) => setError(err instanceof Error ? err.message : "No se pudieron cargar las plantillas."))
      .finally(() => setLoading(false));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  if (loading) {
    return (
      <section className="rounded-xl border border-stone-200 bg-white">
        <div className="px-5 py-8 text-sm text-stone-500">Cargando plantillas de correo...</div>
      </section>
    );
  }

  if (error) {
    return (
      <section className="rounded-xl border border-stone-200 bg-white">
        <div className="px-5 py-4">
          <p className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">{error}</p>
        </div>
      </section>
    );
  }

  if (templates.length === 0) {
    return (
      <section className="rounded-xl border border-stone-200 bg-white">
        <div className="px-5 py-8 text-sm text-stone-500">No hay plantillas de correo configuradas.</div>
      </section>
    );
  }

  const current = templates.find((t) => t.key === selected);

  return (
    <section className="rounded-xl border border-stone-200 bg-white">
      <div className="border-b border-stone-200 px-5 py-4">
        <h2 className="text-base font-semibold text-stone-900">Plantillas de correo</h2>
        <p className="mt-1 text-sm text-stone-500">
          Edita el HTML y el asunto de cada plantilla. Usa variables con doble llave: {"{{.Name}}"}, {"{{.Email}}"}, {"{{.AppName}}"}, {"{{.Year}}"}.
        </p>

        {templates.length > 1 && (
          <div className="mt-3 flex gap-2">
            {templates.map((t) => (
              <button
                key={t.key}
                type="button"
                onClick={() => setSelected(t.key)}
                className={
                  selected === t.key
                    ? "rounded-full border border-stone-900 bg-stone-900 px-3 py-1 text-xs font-medium text-white"
                    : "rounded-full border border-stone-300 bg-white px-3 py-1 text-xs font-medium text-stone-600"
                }
              >
                {t.key}
              </button>
            ))}
          </div>
        )}
      </div>

      {current && (
        <TemplateEditor
          key={current.key}
          template={current}
          onSave={(updated) => {
            setTemplates((prev) => prev.map((t) => (t.key === updated.key ? updated : t)));
          }}
        />
      )}
    </section>
  );
}

function TemplateEditor({
  template,
  onSave,
}: {
  template: EmailTemplate;
  onSave: (updated: EmailTemplate) => void;
}) {
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
      setStatus("Plantilla guardada correctamente.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "No se pudo guardar la plantilla.");
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
      setError(err instanceof Error ? err.message : "No se pudo generar la vista previa.");
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
        <span className="mb-1.5 block text-[13px] font-medium text-stone-600">Asunto</span>
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
          <span className="text-xs text-stone-500">Variables:</span>
          {TEMPLATE_VARS.map((v) => (
            <button
              key={v.value}
              type="button"
              onClick={() => insertVariable(v.value)}
              className="rounded border border-stone-200 bg-stone-50 px-2 py-0.5 text-xs font-medium text-stone-600 transition-colors hover:bg-stone-100"
            >
              {v.label}
            </button>
          ))}
        </div>
      )}

      <div className="mt-4">
        <div className="flex items-center justify-between">
          <span className="text-[13px] font-medium text-stone-600">
            Cuerpo HTML
          </span>
          <div className="flex gap-1">
            {(["visual", "code", "preview"] as const).map((mode) => {
              const labels: Record<ViewMode, string> = { visual: "Visual", code: "Codigo", preview: "Vista previa" };
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
                  {mode === "preview" && previewing ? "Cargando..." : labels[mode]}
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
                    <p className="text-xs text-stone-500">Asunto renderizado</p>
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
                  title="Vista previa del correo"
                  className={`h-125 border border-stone-200 bg-white transition-all ${previewWidth === "mobile" ? "w-93.75" : "w-full max-w-175"}`}
                  sandbox="allow-same-origin"
                />
              </div>
            </div>
          )}
        </div>
      </div>

      <div className="mt-4 flex items-center justify-between gap-3">
        <p className="text-xs text-stone-500">
          Ultima actualizacion: {formatDate(template.updated_at)}
        </p>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={handleDiscard}
            disabled={!dirty}
            className="inline-flex h-10 items-center rounded-lg border border-stone-200 bg-white px-4 text-sm font-medium text-stone-700 transition-colors hover:bg-stone-50 disabled:cursor-not-allowed disabled:text-stone-400"
          >
            Descartar
          </button>
          <button
            type="button"
            onClick={() => void handleSave()}
            disabled={saving || !dirty}
            className="inline-flex h-10 items-center rounded-lg bg-coral-500 px-4 text-sm font-medium text-white transition-colors hover:bg-coral-600 disabled:cursor-not-allowed disabled:bg-coral-200"
          >
            {saving ? "Guardando..." : "Guardar plantilla"}
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
          const url = window.prompt("URL del enlace");
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
        Izq
      </button>
      <button type="button" onClick={() => editor.chain().focus().setTextAlign("center").run()} className={btnClass(editor.isActive({ textAlign: "center" }))}>
        Centro
      </button>
      <button type="button" onClick={() => editor.chain().focus().setTextAlign("right").run()} className={btnClass(editor.isActive({ textAlign: "right" }))}>
        Der
      </button>

      <span className="mx-1 w-px self-stretch bg-stone-200" />

      <button type="button" onClick={() => editor.chain().focus().toggleBulletList().run()} className={btnClass(editor.isActive("bulletList"))}>
        Lista
      </button>
      <button type="button" onClick={() => editor.chain().focus().toggleOrderedList().run()} className={btnClass(editor.isActive("orderedList"))}>
        Num.
      </button>

      <span className="mx-1 w-px self-stretch bg-stone-200" />

      <label className="flex items-center gap-1 text-xs text-stone-600">
        Color
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
          const url = window.prompt("URL de la imagen");
          if (url) editor.chain().focus().setImage({ src: url }).run();
        }}
        className={btnClass(false)}
      >
        Imagen
      </button>
    </div>
  );
}
