import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import EmailTemplatesSection from "./email-templates";
import { IntlWrapper } from "@/test/intl-wrapper";

import * as api from "@/lib/api";
vi.mock("@/lib/api");

const mockTemplate: api.EmailTemplate = {
  key: "welcome",
  subject: "Bienvenido a {{.AppName}}",
  body_html: "<h1>Hello {{.Name}}</h1>",
  updated_at: "2025-01-01T12:00:00Z",
};

// Mock TipTap — useEditor returns null in jsdom (no DOM range support)
vi.mock("@tiptap/react", () => ({
  useEditor: () => null,
  EditorContent: () => <div data-testid="tiptap-editor" />,
}));

// Mock CodeMirror — heavy DOM dependency
vi.mock("@uiw/react-codemirror", () => ({
  default: ({ value, onChange }: { value: string; onChange?: (v: string) => void }) => (
    <textarea
      data-testid="codemirror"
      value={value}
      onChange={(e) => onChange?.(e.target.value)}
    />
  ),
}));

vi.mock("@codemirror/lang-html", () => ({
  html: () => [],
}));

describe("EmailTemplatesSection", () => {
  beforeEach(() => {
    vi.mocked(api.getEmailTemplates).mockResolvedValue([mockTemplate]);
    vi.mocked(api.updateEmailTemplate).mockResolvedValue({
      ...mockTemplate,
      subject: "Updated",
      body_html: "<p>Updated</p>",
      updated_at: "2025-01-01T13:00:00Z",
    });
    vi.mocked(api.previewEmailTemplate).mockResolvedValue({
      subject: "Bienvenido a Reform Lab",
      html: "<h1>Hello User</h1>",
    });
  });
  it("renders template editor after loading", async () => {
    render(<IntlWrapper><EmailTemplatesSection /></IntlWrapper>);

    // With a single template, the editor is shown immediately.
    await waitFor(() => {
      expect(screen.getByDisplayValue(mockTemplate.subject)).toBeInTheDocument();
    });
  });

  it("shows template editor when a template is selected", async () => {
    render(<IntlWrapper><EmailTemplatesSection /></IntlWrapper>);

    await waitFor(() => {
      expect(screen.getByDisplayValue("Bienvenido a {{.AppName}}")).toBeInTheDocument();
    });
  });

  it("shows mode toggle buttons", async () => {
    render(<IntlWrapper><EmailTemplatesSection /></IntlWrapper>);

    await waitFor(() => {
      expect(screen.getByText("Visual")).toBeInTheDocument();
    });

    expect(screen.getByText("Codigo")).toBeInTheDocument();
    expect(screen.getByText("Vista previa")).toBeInTheDocument();
  });

  it("switches to code mode and shows CodeMirror", async () => {
    const user = userEvent.setup();
    render(<IntlWrapper><EmailTemplatesSection /></IntlWrapper>);

    await waitFor(() => {
      expect(screen.getByText("Codigo")).toBeInTheDocument();
    });

    await user.click(screen.getByText("Codigo"));

    await waitFor(() => {
      expect(screen.getByTestId("codemirror")).toBeInTheDocument();
    });
  });

  it("shows variable chips in visual mode", async () => {
    render(<IntlWrapper><EmailTemplatesSection /></IntlWrapper>);

    await waitFor(() => {
      expect(screen.getByText("Variables:")).toBeInTheDocument();
    });

    expect(screen.getByText("Nombre")).toBeInTheDocument();
    expect(screen.getByText("Email")).toBeInTheDocument();
    expect(screen.getByText("App")).toBeInTheDocument();
  });

  it("shows discard button disabled when form is clean", async () => {
    render(<IntlWrapper><EmailTemplatesSection /></IntlWrapper>);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /descartar/i })).toBeDisabled();
    });
  });

  it("shows save button disabled when form is clean", async () => {
    render(<IntlWrapper><EmailTemplatesSection /></IntlWrapper>);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /guardar plantilla/i })).toBeDisabled();
    });
  });
});
