import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import FormatSelector from "./format-selector";

const options = [
	{ value: "jpg", label: "JPG" },
	{ value: "png", label: "PNG" },
	{ value: "webp", label: "WebP" },
];

describe("FormatSelector", () => {
	it("renders all options", () => {
		render(
			<FormatSelector
				id="fmt"
				label="Formato de salida"
				options={options}
				value="jpg"
				onChange={vi.fn()}
			/>
		);

		expect(screen.getByText("JPG")).toBeInTheDocument();
		expect(screen.getByText("PNG")).toBeInTheDocument();
		expect(screen.getByText("WebP")).toBeInTheDocument();
	});

	it("marks the selected option with aria-pressed", () => {
		render(
			<FormatSelector
				id="fmt"
				label="Formato de salida"
				options={options}
				value="png"
				onChange={vi.fn()}
			/>
		);

		expect(screen.getByText("PNG").closest("button")).toHaveAttribute("aria-pressed", "true");
		expect(screen.getByText("JPG").closest("button")).toHaveAttribute("aria-pressed", "false");
	});

	it("calls onChange when an option is clicked", async () => {
		const onChange = vi.fn();
		const user = userEvent.setup();
		render(
			<FormatSelector
				id="fmt"
				label="Formato de salida"
				options={options}
				value="jpg"
				onChange={onChange}
			/>
		);

		await user.click(screen.getByText("WebP"));

		expect(onChange).toHaveBeenCalledWith("webp");
	});

	it("renders the label", () => {
		render(
			<FormatSelector
				id="fmt"
				label="Formato de salida"
				options={options}
				value="jpg"
				onChange={vi.fn()}
			/>
		);

		expect(screen.getByText("Formato de salida")).toBeInTheDocument();
	});
});
