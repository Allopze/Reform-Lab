import { describe, expect, it } from "vitest";
import { applyCatalogHints, getCategoryById } from "./categories";
import type { CatalogFamily } from "@/lib/api";

const catalog: CatalogFamily[] = [
  {
    family: "document",
    capabilities: [
      {
        id: "doc-to-pdf",
        displayName: "Convertir a PDF",
        presentationOrder: 600,
        sourceFormats: ["application/msword"],
        targetFormat: "pdf",
        operationType: "convert",
        family: "document",
        maxInputBytes: 100,
        timeoutSeconds: 120,
        maxRetries: 1,
      },
      {
        id: "doc-to-docx",
        displayName: "Convertir a Word",
        presentationOrder: 610,
        sourceFormats: ["application/msword", "application/rtf"],
        targetFormat: "docx",
        operationType: "convert",
        family: "document",
        maxInputBytes: 100,
        timeoutSeconds: 120,
        maxRetries: 1,
      },
    ],
  },
];

describe("applyCatalogHints", () => {
  it("hydrates category hints from the backend catalog", () => {
    const category = applyCatalogHints(getCategoryById("documents"), catalog);

    expect(category.acceptedMimeTypes).toContain("application/msword");
    expect(category.acceptedFormats).toContainEqual({
      value: "doc",
      label: "DOC",
    });
    expect(category.targetFormats.map((format) => format.value)).toEqual([
      "pdf",
      "docx",
    ]);
  });

  it("keeps static hints when the catalog is unavailable", () => {
    const original = getCategoryById("documents");
    const category = applyCatalogHints(original, null);

    expect(category).toBe(original);
  });

  it("does not replace the auto category hints", () => {
    const original = getCategoryById("auto");
    const category = applyCatalogHints(original, catalog);

    expect(category).toBe(original);
  });
});
