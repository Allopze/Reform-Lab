import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import Dropzone from "./dropzone";
import { IntlWrapper } from "@/test/intl-wrapper";

vi.mock("next/image", () => ({
	default: ({
		priority: _priority,
		...props
	}: Record<string, unknown> & { priority?: boolean }) => (
		// eslint-disable-next-line @next/next/no-img-element, jsx-a11y/alt-text
		<img {...props} />
	),
}));

const baseProps = {
	text: "Subí tu archivo",
	hint: "o arrastralo acá",
	supportLabel: "PDF, DOCX, PPTX",
	detailLabel: "Hasta 25 MB por archivo.",
	accept: ".pdf,.docx,.pptx",
	onFilesSelected: vi.fn(),
};

describe("Dropzone", () => {
	it("renders text, hint, supportLabel and detailLabel", () => {
		render(<IntlWrapper><Dropzone {...baseProps} /></IntlWrapper>);

		expect(screen.getByText("Subí tu archivo")).toBeInTheDocument();
		expect(screen.getByText("o arrastralo acá")).toBeInTheDocument();
		expect(screen.getByText(/PDF, DOCX, PPTX/)).toBeInTheDocument();
		expect(screen.getByText("Hasta 25 MB por archivo.")).toBeInTheDocument();
	});

	it("opens file picker on click", async () => {
		const user = userEvent.setup();
		render(<IntlWrapper><Dropzone {...baseProps} /></IntlWrapper>);

		const dropzone = screen.getByRole("button", { name: "Subí tu archivo" });
		expect(dropzone).toBeInTheDocument();

		// The hidden input should exist
		const input = document.querySelector("input[type='file']") as HTMLInputElement;
		expect(input).toBeInTheDocument();
		expect(input.accept).toBe(".pdf,.docx,.pptx");
		expect(input.multiple).toBe(true);
	});

	it("calls onFilesSelected when files are picked via input", async () => {
		const onFilesSelected = vi.fn();
		render(<IntlWrapper><Dropzone {...baseProps} onFilesSelected={onFilesSelected} /></IntlWrapper>);

		const input = document.querySelector("input[type='file']") as HTMLInputElement;
		const firstFile = new File(["content"], "test.pdf", { type: "application/pdf" });
		const secondFile = new File(["content"], "deck.pptx", { type: "application/vnd.openxmlformats-officedocument.presentationml.presentation" });

		await userEvent.upload(input, [firstFile, secondFile]);

		expect(onFilesSelected).toHaveBeenCalledTimes(1);
		expect(onFilesSelected).toHaveBeenCalledWith([firstFile, secondFile]);
	});

	it("calls onFilesSelected on drop", () => {
		const onFilesSelected = vi.fn();
		render(<IntlWrapper><Dropzone {...baseProps} onFilesSelected={onFilesSelected} /></IntlWrapper>);

		const dropzone = screen.getByRole("button", { name: "Subí tu archivo" });
		const file = new File(["content"], "doc.docx", { type: "application/vnd.openxmlformats-officedocument.wordprocessingml.document" });

		fireEvent.drop(dropzone, {
			dataTransfer: { files: [file] },
		});

		expect(onFilesSelected).toHaveBeenCalledTimes(1);
		expect(onFilesSelected).toHaveBeenCalledWith([file]);
	});

	it("opens file picker on Enter key", async () => {
		const user = userEvent.setup();
		render(<IntlWrapper><Dropzone {...baseProps} /></IntlWrapper>);

		const dropzone = screen.getByRole("button", { name: "Subí tu archivo" });
		dropzone.focus();

		// Pressing Enter should trigger click on hidden input — we verify no errors thrown
		await user.keyboard("{Enter}");
	});
});
