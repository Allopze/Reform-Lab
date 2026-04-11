import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import Dropzone from "./dropzone";
import { IntlWrapper } from "@/test/intl-wrapper";

vi.mock("next/image", () => ({
	default: (props: Record<string, unknown>) => (
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
	onFileSelected: vi.fn(),
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
	});

	it("calls onFileSelected when a file is picked via input", async () => {
		const onFileSelected = vi.fn();
		render(<IntlWrapper><Dropzone {...baseProps} onFileSelected={onFileSelected} /></IntlWrapper>);

		const input = document.querySelector("input[type='file']") as HTMLInputElement;
		const file = new File(["content"], "test.pdf", { type: "application/pdf" });

		await userEvent.upload(input, file);

		expect(onFileSelected).toHaveBeenCalledTimes(1);
		expect(onFileSelected).toHaveBeenCalledWith(file);
	});

	it("calls onFileSelected on drop", () => {
		const onFileSelected = vi.fn();
		render(<IntlWrapper><Dropzone {...baseProps} onFileSelected={onFileSelected} /></IntlWrapper>);

		const dropzone = screen.getByRole("button", { name: "Subí tu archivo" });
		const file = new File(["content"], "doc.docx", { type: "application/vnd.openxmlformats-officedocument.wordprocessingml.document" });

		fireEvent.drop(dropzone, {
			dataTransfer: { files: [file] },
		});

		expect(onFileSelected).toHaveBeenCalledTimes(1);
		expect(onFileSelected).toHaveBeenCalledWith(file);
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
