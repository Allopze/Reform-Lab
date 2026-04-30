import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, afterEach, beforeEach, expect, it, vi } from "vitest";
import ConversionCard from "./conversion-card";
import { getCategoryById } from "@/config/categories";
import { IntlWrapper } from "@/test/intl-wrapper";
import {
  cancelJobs,
  createBatchConversion,
  downloadArtifact,
  getBatchCapabilities,
  getJob,
  getUploadPolicy,
  uploadFile,
} from "@/lib/api";

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    ...props
  }: {
    children: React.ReactNode;
    href: string;
  }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

vi.mock("@/lib/api", () => ({
  uploadFile: vi.fn(),
  getBatchCapabilities: vi.fn(),
  createBatchConversion: vi.fn(),
  getJob: vi.fn(),
  downloadArtifact: vi.fn(),
  cancelJobs: vi.fn(),
  getUploadPolicy: vi.fn(),
}));

vi.mock("./dropzone", () => ({
  default: ({
    onFilesSelected,
    supportLabel,
    detailLabel,
  }: {
    onFilesSelected: (files: File[]) => Promise<void> | void;
    supportLabel: string;
    detailLabel: string;
  }) => (
    <div>
      <p>{supportLabel}</p>
      <p>{detailLabel}</p>
      <button
        type="button"
        onClick={() =>
          void onFilesSelected([
            new File(["deck"], "slides-1.pptx", {
              type: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
            }),
            new File(["deck"], "slides-2.pptx", {
              type: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
            }),
          ])
        }
      >
        mock-dropzone
      </button>
    </div>
  ),
}));

vi.mock("./format-selector", () => ({
  default: ({
    id,
    label,
    options,
    value,
    onChange,
  }: {
    id: string;
    label: string;
    options: Array<{ value: string; label: string }>;
    value: string;
    onChange: (value: string) => void;
  }) => (
    <label htmlFor={id}>
      {label}
      <select
        id={id}
        aria-label={label}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </label>
  ),
}));

vi.mock("./file-preview", () => ({
  default: ({
    file,
    selectionLabel,
    outputFormat,
    onRemove,
  }: {
    file: File;
    selectionLabel?: string;
    outputFormat: string;
    onRemove: () => void;
  }) => (
    <div>
      <p>{file.name}</p>
      <p>{selectionLabel}</p>
      <p>{outputFormat}</p>
      <button type="button" onClick={onRemove}>
        mock-remove
      </button>
    </div>
  ),
}));

const uploadFileMock = vi.mocked(uploadFile);
const getBatchCapabilitiesMock = vi.mocked(getBatchCapabilities);
const createBatchConversionMock = vi.mocked(createBatchConversion);
const getJobMock = vi.mocked(getJob);
const downloadArtifactMock = vi.mocked(downloadArtifact);
const cancelJobsMock = vi.mocked(cancelJobs);
const getUploadPolicyMock = vi.mocked(getUploadPolicy);

function mockUploadAndCapabilities() {
  uploadFileMock
    .mockResolvedValueOnce({
      id: "file-1",
      originalName: "slides-1.pptx",
      size: 1024,
      detectedFormat: {
        mimeType:
          "application/vnd.openxmlformats-officedocument.presentationml.presentation",
        family: "document",
        extension: "pptx",
      },
      uploadedAt: "2026-04-09T10:00:00Z",
    })
    .mockResolvedValueOnce({
      id: "file-2",
      originalName: "slides-2.pptx",
      size: 1024,
      detectedFormat: {
        mimeType:
          "application/vnd.openxmlformats-officedocument.presentationml.presentation",
        family: "document",
        extension: "pptx",
      },
      uploadedAt: "2026-04-09T10:00:01Z",
    });
  getBatchCapabilitiesMock.mockResolvedValue([
    {
      id: "presentation-to-jpg",
      displayName: "Convertir presentación a JPG",
      presentationOrder: 710,
      targetFormat: "jpg",
      operationType: "convert",
      timeoutSeconds: 30,
    },
  ]);
  createBatchConversionMock.mockResolvedValue([
    {
      id: "job-1",
      fileId: "file-1",
      capabilityId: "presentation-to-jpg",
      outputFormat: "jpg",
      status: "queued",
      progress: 0,
      createdAt: "2026-04-09T10:00:01Z",
    },
    {
      id: "job-2",
      fileId: "file-2",
      capabilityId: "presentation-to-jpg",
      outputFormat: "jpg",
      status: "queued",
      progress: 0,
      createdAt: "2026-04-09T10:00:01Z",
    },
  ]);
}

async function uploadAndStartConversion(
  user: ReturnType<typeof userEvent.setup>,
) {
  await user.click(screen.getByRole("button", { name: "mock-dropzone" }));
  await waitFor(() => expect(uploadFileMock).toHaveBeenCalledTimes(2));
  await user.click(screen.getByRole("button", { name: "Convertir documento" }));
  await waitFor(() =>
    expect(createBatchConversionMock).toHaveBeenCalledWith(
      ["file-1", "file-2"],
      "presentation-to-jpg",
    ),
  );
}

describe("ConversionCard", () => {
  beforeEach(() => {
    mockUploadAndCapabilities();
    cancelJobsMock.mockResolvedValue(["job-1", "job-2"]);
    getUploadPolicyMock.mockResolvedValue({
      guestMaxBytes: 5 * 1024 * 1024,
      registeredMaxBytes: 25 * 1024 * 1024,
      effectiveMaxBytes: 5 * 1024 * 1024,
      viewerType: "guest",
      absoluteMaxBytes: 500 * 1024 * 1024,
      cumulativeQuotaBytes: 15 * 1024 * 1024,
      cumulativeUsedBytes: 5 * 1024 * 1024,
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("allows anonymous users to upload and start a conversion", async () => {
    const user = userEvent.setup();
    render(
      <IntlWrapper>
        <ConversionCard category={getCategoryById("auto")} />
      </IntlWrapper>,
    );

    await uploadAndStartConversion(user);

    expect(
      screen.queryByText("Acceso requerido para convertir archivos"),
    ).not.toBeInTheDocument();
    expect(createBatchConversionMock).toHaveBeenCalledWith(
      ["file-1", "file-2"],
      "presentation-to-jpg",
    );
  });

  it("keeps same-format capabilities visible and submits the selected capability id", async () => {
    getBatchCapabilitiesMock.mockResolvedValue([
      {
        id: "video-to-webm",
        displayName: "Convertir a WebM",
        presentationOrder: 100,
        targetFormat: "webm",
        operationType: "convert",
        timeoutSeconds: 30,
      },
      {
        id: "video-preview-webm",
        displayName: "Generar preview corto WebM",
        presentationOrder: 110,
        targetFormat: "webm",
        operationType: "preview",
        timeoutSeconds: 30,
      },
    ]);
    createBatchConversionMock.mockResolvedValueOnce([
      {
        id: "job-1",
        fileId: "file-1",
        capabilityId: "video-preview-webm",
        outputFormat: "webm",
        status: "queued",
        progress: 0,
        createdAt: "2026-04-09T10:00:01Z",
      },
      {
        id: "job-2",
        fileId: "file-2",
        capabilityId: "video-preview-webm",
        outputFormat: "webm",
        status: "queued",
        progress: 0,
        createdAt: "2026-04-09T10:00:01Z",
      },
    ]);

    const user = userEvent.setup();
    render(
      <IntlWrapper>
        <ConversionCard category={getCategoryById("auto")} />
      </IntlWrapper>,
    );

    await user.click(screen.getByRole("button", { name: "mock-dropzone" }));
    await waitFor(() => expect(uploadFileMock).toHaveBeenCalledTimes(2));

    expect(
      screen.getAllByRole("option", { name: "Convertir a WebM" }),
    ).toHaveLength(1);
    expect(
      screen.getAllByRole("option", { name: "Generar preview corto WebM" }),
    ).toHaveLength(1);
    expect(screen.getAllByText("Convertir a WebM").length).toBeGreaterThan(0);

    await user.selectOptions(
      screen.getByRole("combobox", { name: "Opciones disponibles" }),
      "video-preview-webm",
    );
    await user.click(
      screen.getByRole("button", { name: "Convertir documento" }),
    );

    await waitFor(() =>
      expect(createBatchConversionMock).toHaveBeenCalledWith(
        ["file-1", "file-2"],
        "video-preview-webm",
      ),
    );
  });

  it("autoselects the first capability returned by the backend", async () => {
    getBatchCapabilitiesMock.mockResolvedValue([
      {
        id: "video-preview-webm",
        displayName: "Generar preview corto WebM",
        presentationOrder: 100,
        targetFormat: "webm",
        operationType: "preview",
        timeoutSeconds: 30,
      },
      {
        id: "video-to-webm",
        displayName: "Convertir a WebM",
        presentationOrder: 110,
        targetFormat: "webm",
        operationType: "convert",
        timeoutSeconds: 30,
      },
    ]);

    const user = userEvent.setup();
    render(
      <IntlWrapper>
        <ConversionCard category={getCategoryById("auto")} />
      </IntlWrapper>,
    );

    await user.click(screen.getByRole("button", { name: "mock-dropzone" }));
    await waitFor(() => expect(uploadFileMock).toHaveBeenCalledTimes(2));

    expect(
      screen.getByRole("combobox", { name: "Opciones disponibles" }),
    ).toHaveValue("video-preview-webm");
    expect(
      screen.getAllByText("Generar preview corto WebM").length,
    ).toBeGreaterThan(0);
    expect(screen.getAllByText("webm")).toHaveLength(2);

    await user.click(
      screen.getByRole("button", { name: "Convertir documento" }),
    );

    await waitFor(() =>
      expect(createBatchConversionMock).toHaveBeenCalledWith(
        ["file-1", "file-2"],
        "video-preview-webm",
      ),
    );
  });

  it("shows the active guest upload limit from backend policy", async () => {
    const user = userEvent.setup();
    render(
      <IntlWrapper>
        <ConversionCard category={getCategoryById("auto")} />
      </IntlWrapper>,
    );

    expect(
      await screen.findByText("hasta 5 MB por archivo como invitado; quedan 10 MB"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        /Como invitado puedes subir hasta 5 MB por archivo; con cuenta registrada, hasta 25 MB\. Cuota usada: 5 MB de 15 MB\./,
      ),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "mock-dropzone" }));
    await waitFor(() => expect(uploadFileMock).toHaveBeenCalledTimes(2));
  });

  it("renders per-file downloads after batch completion", async () => {
    getJobMock
      .mockResolvedValueOnce({
        id: "job-1",
        fileId: "file-1",
        capabilityId: "presentation-to-jpg",
        outputFormat: "jpg",
        status: "succeeded",
        progress: 100,
        artifactId: "artifact-1",
        artifactFileName: "slides-1.zip",
        artifactMimeType: "application/zip",
        artifactSize: 4096,
        createdAt: "2026-04-09T10:00:01Z",
      })
      .mockResolvedValueOnce({
        id: "job-2",
        fileId: "file-2",
        capabilityId: "presentation-to-jpg",
        outputFormat: "jpg",
        status: "succeeded",
        progress: 100,
        artifactId: "artifact-2",
        artifactFileName: "slides-2.jpg",
        artifactMimeType: "image/jpeg",
        artifactSize: 4096,
        createdAt: "2026-04-09T10:00:01Z",
      });
    downloadArtifactMock.mockResolvedValue(undefined);

    const user = userEvent.setup();
    render(
      <IntlWrapper>
        <ConversionCard category={getCategoryById("auto")} />
      </IntlWrapper>,
    );

    await uploadAndStartConversion(user);
    expect(
      await screen.findByText(
        "2 artefactos del lote quedaron listos para descarga individual.",
        {},
        { timeout: 3000 },
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("slides-1.pptx")).toBeInTheDocument();
    expect(screen.getByText("slides-2.pptx")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Descargar slides-1.zip" }));

    await waitFor(() =>
      expect(downloadArtifactMock).toHaveBeenCalledWith(
        "artifact-1",
        "slides-1.zip",
      ),
    );
  });

  it("cancels active batch jobs", async () => {
    getJobMock
      .mockResolvedValueOnce({
        id: "job-1",
        fileId: "file-1",
        capabilityId: "presentation-to-jpg",
        outputFormat: "jpg",
        status: "running",
        progress: 25,
        createdAt: "2026-04-09T10:00:01Z",
      })
      .mockResolvedValueOnce({
        id: "job-2",
        fileId: "file-2",
        capabilityId: "presentation-to-jpg",
        outputFormat: "jpg",
        status: "running",
        progress: 40,
        createdAt: "2026-04-09T10:00:01Z",
      });

    const user = userEvent.setup();
    render(
      <IntlWrapper>
        <ConversionCard category={getCategoryById("auto")} />
      </IntlWrapper>,
    );

    await uploadAndStartConversion(user);
    await user.click(
      await screen.findByRole("button", { name: "Cancelar" }, { timeout: 3000 }),
    );

    await waitFor(() =>
      expect(cancelJobsMock).toHaveBeenCalledWith(["job-1", "job-2"]),
    );
  });
});
