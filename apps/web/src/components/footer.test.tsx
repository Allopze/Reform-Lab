import { act, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import Footer from "./footer";
import { getFooterMessage } from "@/lib/api";
import {
  DEFAULT_FOOTER_MESSAGE,
  emitFooterMessageUpdated,
} from "@/lib/footer-message";

vi.mock("@/lib/api", () => ({
  getFooterMessage: vi.fn(),
}));

const getFooterMessageMock = vi.mocked(getFooterMessage);

describe("Footer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows the default message while loading", () => {
    getFooterMessageMock.mockReturnValue(new Promise<string>(() => {}));

    render(<Footer />);

    expect(screen.getByText(DEFAULT_FOOTER_MESSAGE)).toBeInTheDocument();
  });

  it("renders the configured footer message from the API", async () => {
    getFooterMessageMock.mockResolvedValue("Operado por Reform Lab · Sincronizado desde admin");

    render(<Footer />);

    expect(
      await screen.findByText("Operado por Reform Lab · Sincronizado desde admin")
    ).toBeInTheDocument();
  });

  it("updates immediately when admin broadcasts a footer change", async () => {
    getFooterMessageMock.mockResolvedValue(DEFAULT_FOOTER_MESSAGE);

    render(<Footer />);

    await waitFor(() => {
      expect(getFooterMessageMock).toHaveBeenCalledTimes(1);
    });

    await act(async () => {
      emitFooterMessageUpdated("Nuevo mensaje visible sin recargar la pagina");
    });

    expect(
      screen.getByText("Nuevo mensaje visible sin recargar la pagina")
    ).toBeInTheDocument();
  });
});