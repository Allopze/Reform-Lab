import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import CategoryNav from "./category-nav";
import { IntlWrapper } from "@/test/intl-wrapper";

beforeEach(() => {
	Element.prototype.scrollTo = vi.fn();
});

describe("CategoryNav", () => {
	it("renders all category tabs", () => {
		render(<IntlWrapper><CategoryNav activeCategory="auto" onChange={vi.fn()} /></IntlWrapper>);

		expect(screen.getByRole("tablist", { name: "Categorías de conversión" })).toBeInTheDocument();
		expect(screen.getByRole("tab", { name: /Auto/i })).toBeInTheDocument();
		expect(screen.getByRole("tab", { name: /Documentos/i })).toBeInTheDocument();
	});

	it("marks the active tab with aria-selected", () => {
		render(<IntlWrapper><CategoryNav activeCategory="pdf" onChange={vi.fn()} /></IntlWrapper>);

		const pdfTab = screen.getByRole("tab", { name: /PDF/i });
		expect(pdfTab).toHaveAttribute("aria-selected", "true");
	});

	it("calls onChange when a tab is clicked", async () => {
		const onChange = vi.fn();
		const user = userEvent.setup();
		render(<IntlWrapper><CategoryNav activeCategory="auto" onChange={onChange} /></IntlWrapper>);

		await user.click(screen.getByRole("tab", { name: /Imágenes/i }));

		expect(onChange).toHaveBeenCalledWith("images");
	});

	it("navigates tabs with arrow keys", async () => {
		const onChange = vi.fn();
		const user = userEvent.setup();
		render(<IntlWrapper><CategoryNav activeCategory="auto" onChange={onChange} /></IntlWrapper>);

		const autoTab = screen.getByRole("tab", { name: /Auto/i });
		autoTab.focus();

		await user.keyboard("{ArrowRight}");
		expect(onChange).toHaveBeenCalled();
	});
});
