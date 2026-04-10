import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, beforeEach, afterEach, expect, it, vi } from "vitest";
import ConversionCard from "./conversion-card";
import { getCategoryById } from "@/config/categories";
import {
	cancelJob,
	createConversion,
	downloadArtifact,
	getCapabilities,
	getJob,
	getUploadPolicy,
	uploadFile,
} from "@/lib/api";

vi.mock("next/link", () => ({
	default: ({ children, href, ...props }: { children: React.ReactNode; href: string }) => (
		<a href={href} {...props}>
			{children}
		</a>
	),
}));

vi.mock("@/lib/api", () => ({
	uploadFile: vi.fn(),
	getCapabilities: vi.fn(),
	createConversion: vi.fn(),
	getJob: vi.fn(),
	downloadArtifact: vi.fn(),
	cancelJob: vi.fn(),
	getUploadPolicy: vi.fn(),
}));

vi.mock("./dropzone", () => ({
	default: ({
		onFileSelected,
		supportLabel,
		detailLabel,
	}: {
		onFileSelected: (file: File) => Promise<void> | void;
		supportLabel: string;
		detailLabel: string;
	}) => (
		<div>
			<p>{supportLabel}</p>
			<p>{detailLabel}</p>
			<button
				type="button"
				onClick={() =>
					void onFileSelected(
						new File(["deck"], "slides.pptx", {
							type: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
						})
					)
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
			<select id={id} aria-label={label} value={value} onChange={(event) => onChange(event.target.value)}>
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
		outputFormat,
		onRemove,
	}: {
		file: File;
		outputFormat: string;
		onRemove: () => void;
	}) => (
		<div>
			<p>{file.name}</p>
			<p>{outputFormat}</p>
			<button type="button" onClick={onRemove}>
				mock-remove
			</button>
		</div>
	),
}));

const uploadFileMock = vi.mocked(uploadFile);
const getCapabilitiesMock = vi.mocked(getCapabilities);
const createConversionMock = vi.mocked(createConversion);
const getJobMock = vi.mocked(getJob);
const downloadArtifactMock = vi.mocked(downloadArtifact);
const cancelJobMock = vi.mocked(cancelJob);
const getUploadPolicyMock = vi.mocked(getUploadPolicy);

function mockUploadAndCapabilities() {
	uploadFileMock.mockResolvedValue({
		id: "file-1",
		originalName: "slides.pptx",
		size: 1024,
		detectedFormat: {
			mimeType: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
			family: "document",
			extension: "pptx",
		},
		uploadedAt: "2026-04-09T10:00:00Z",
	});
	getCapabilitiesMock.mockResolvedValue([
		{
			id: "presentation-to-jpg",
			displayName: "JPG",
			targetFormat: "jpg",
			operationType: "convert",
			timeoutSeconds: 30,
		},
	]);
	createConversionMock.mockResolvedValue({
		id: "job-1",
		fileId: "file-1",
		capabilityId: "presentation-to-jpg",
		outputFormat: "jpg",
		status: "queued",
		progress: 0,
		createdAt: "2026-04-09T10:00:01Z",
	});
}

async function uploadAndStartConversion(user: ReturnType<typeof userEvent.setup>) {
	await user.click(screen.getByRole("button", { name: "mock-dropzone" }));
	await waitFor(() => expect(uploadFileMock).toHaveBeenCalledTimes(1));
	await user.click(screen.getByRole("button", { name: "Convertir documento" }));
	await waitFor(() =>
		expect(createConversionMock).toHaveBeenCalledWith("file-1", "presentation-to-jpg")
	);
}

describe("ConversionCard", () => {
	beforeEach(() => {
		mockUploadAndCapabilities();
		cancelJobMock.mockResolvedValue(undefined);
		getUploadPolicyMock.mockResolvedValue({
			guestMaxBytes: 5 * 1024 * 1024,
			registeredMaxBytes: 25 * 1024 * 1024,
			effectiveMaxBytes: 5 * 1024 * 1024,
			viewerType: "guest",
			absoluteMaxBytes: 500 * 1024 * 1024,
		});
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	it("allows anonymous users to upload and start a conversion", async () => {
		createConversionMock.mockResolvedValueOnce({
			id: "job-1",
			fileId: "file-1",
			capabilityId: "presentation-to-jpg",
			outputFormat: "jpg",
			status: "queued",
			progress: 0,
			createdAt: "2026-04-09T10:00:01Z",
		});

		const user = userEvent.setup();
		render(<ConversionCard category={getCategoryById("auto")} />);

		await uploadAndStartConversion(user);

		expect(screen.queryByText("Acceso requerido para convertir archivos")).not.toBeInTheDocument();
		expect(createConversionMock).toHaveBeenCalledWith("file-1", "presentation-to-jpg");
	});

	it("shows the active guest upload limit from backend policy", async () => {
		const user = userEvent.setup();
		render(<ConversionCard category={getCategoryById("auto")} />);

		expect(await screen.findByText("hasta 5 MB como invitado")).toBeInTheDocument();
		expect(
			screen.getByText(/Como invitado puedes subir hasta 5 MB por archivo; con cuenta registrada, hasta 25 MB\./)
		).toBeInTheDocument();

		await user.click(screen.getByRole("button", { name: "mock-dropzone" }));
		await waitFor(() => expect(uploadFileMock).toHaveBeenCalledTimes(1));
	});

	it("renders ZIP success UX from backend artifact metadata", async () => {
		getJobMock.mockResolvedValue({
			id: "job-1",
			fileId: "file-1",
			capabilityId: "presentation-to-jpg",
			outputFormat: "jpg",
			status: "succeeded",
			progress: 100,
			artifactId: "artifact-1",
			artifactFileName: "slides.zip",
			artifactMimeType: "application/zip",
			artifactSize: 4096,
			createdAt: "2026-04-09T10:00:01Z",
		});
		downloadArtifactMock.mockResolvedValue(undefined);

		const user = userEvent.setup();
		render(<ConversionCard category={getCategoryById("auto")} />);

		await uploadAndStartConversion(user);
		expect(
			await screen.findByText(
				"La salida incluye varios archivos y se agrupó como slides.zip.",
				{},
				{ timeout: 3000 }
			)
		).toBeInTheDocument();
		expect(screen.getByRole("link", { name: "Descargar ZIP" })).toBeInTheDocument();

		await user.click(screen.getByRole("link", { name: "Descargar ZIP" }));

		await waitFor(() =>
			expect(downloadArtifactMock).toHaveBeenCalledWith("artifact-1", "slides.zip")
		);
	});

	it("shows the backend job failure message", async () => {
		getJobMock.mockResolvedValue({
			id: "job-1",
			fileId: "file-1",
			capabilityId: "presentation-to-jpg",
			outputFormat: "jpg",
			status: "failed",
			progress: 42,
			error: "El deck está corrupto y no pudo renderizarse.",
			createdAt: "2026-04-09T10:00:01Z",
		});

		const user = userEvent.setup();
		render(<ConversionCard category={getCategoryById("auto")} />);

		await uploadAndStartConversion(user);
		expect(
			await screen.findByText(
				"El deck está corrupto y no pudo renderizarse.",
				{},
				{ timeout: 3000 }
			)
		).toBeInTheDocument();
		expect(screen.getByRole("button", { name: "Probar otro archivo" })).toBeInTheDocument();
	});

	it("keeps the success state visible when download fails", async () => {
		getJobMock.mockResolvedValue({
			id: "job-1",
			fileId: "file-1",
			capabilityId: "presentation-to-jpg",
			outputFormat: "jpg",
			status: "succeeded",
			progress: 100,
			artifactId: "artifact-1",
			artifactFileName: "slides.zip",
			artifactMimeType: "application/zip",
			artifactSize: 4096,
			createdAt: "2026-04-09T10:00:01Z",
		});
		downloadArtifactMock.mockRejectedValue(new Error("El ZIP ya expiró."));

		const user = userEvent.setup();
		render(<ConversionCard category={getCategoryById("auto")} />);

		await uploadAndStartConversion(user);
		await user.click(
			await screen.findByRole("link", { name: "Descargar ZIP" }, { timeout: 3000 })
		);

		expect(await screen.findByText("El ZIP ya expiró.")).toBeInTheDocument();
		expect(screen.getByRole("link", { name: "Descargar ZIP" })).toBeInTheDocument();
	});
});
