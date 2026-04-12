import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import FilePreview from "./file-preview";
import { IntlWrapper } from "@/test/intl-wrapper";

describe("FilePreview", () => {
  const file = new File(["x".repeat(2048)], "report.pdf", {
    type: "application/pdf",
  });

  it("renders file name and size", () => {
    render(
      <IntlWrapper>
        <FilePreview
          file={file}
          selectionLabel="Convertir a DOCX"
          outputFormat="docx"
          onRemove={vi.fn()}
        />
      </IntlWrapper>,
    );

    expect(screen.getByText("report.pdf")).toBeInTheDocument();
    expect(
      screen.getByText("2.0 KB / Convertir a DOCX / .DOCX"),
    ).toBeInTheDocument();
  });

  it("calls onRemove when the remove button is clicked", async () => {
    const onRemove = vi.fn();
    const user = userEvent.setup();
    render(
      <IntlWrapper>
        <FilePreview
          file={file}
          selectionLabel="Convertir a DOCX"
          outputFormat="docx"
          onRemove={onRemove}
        />
      </IntlWrapper>,
    );

    await user.click(
      screen.getByRole("button", { name: "Eliminar archivo seleccionado" }),
    );

    expect(onRemove).toHaveBeenCalledTimes(1);
  });

  it("does not show output format badge when outputFormat is empty", () => {
    render(
      <IntlWrapper>
        <FilePreview
          file={file}
          selectionLabel=""
          outputFormat=""
          onRemove={vi.fn()}
        />
      </IntlWrapper>,
    );

    expect(screen.getByText("2.0 KB")).toBeInTheDocument();
  });
});
