import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import SMTPSettingsSection from "./smtp-settings";
import { IntlWrapper } from "@/test/intl-wrapper";

import * as api from "@/lib/api";
vi.mock("@/lib/api");

const mockSettings: api.SMTPSettings = {
  host: "mail.example.com",
  port: 587,
  user: "admin",
  password: "****",
  from: "noreply@example.com",
  use_tls: true,
  source: "admin",
};

describe("SMTPSettingsSection", () => {
  beforeEach(() => {
    vi.mocked(api.getSMTPSettings).mockResolvedValue(mockSettings);
    vi.mocked(api.updateSMTPSettings).mockResolvedValue(undefined as never);
    vi.mocked(api.testSMTPConnection).mockResolvedValue({ success: true, message: "OK" });
  });
  it("renders the form with loaded settings", async () => {
    render(<IntlWrapper><SMTPSettingsSection /></IntlWrapper>);

    await waitFor(() => {
      expect(screen.getByDisplayValue("mail.example.com")).toBeInTheDocument();
    });

    expect(screen.getByDisplayValue("587")).toBeInTheDocument();
    expect(screen.getByDisplayValue("admin")).toBeInTheDocument();
    expect(screen.getByDisplayValue("noreply@example.com")).toBeInTheDocument();
  });

  it("shows source badge", async () => {
    render(<IntlWrapper><SMTPSettingsSection /></IntlWrapper>);

    await waitFor(() => {
      expect(screen.getByText(/personalizados por admin/i)).toBeInTheDocument();
    });
  });

  it("disables save when form is clean", async () => {
    render(<IntlWrapper><SMTPSettingsSection /></IntlWrapper>);

    await waitFor(() => {
      expect(screen.getByDisplayValue("mail.example.com")).toBeInTheDocument();
    });

    const saveButton = screen.getByRole("button", { name: /guardar/i });
    expect(saveButton).toBeDisabled();
  });

  it("enables save when form is dirty", async () => {
    const user = userEvent.setup();
    render(<IntlWrapper><SMTPSettingsSection /></IntlWrapper>);

    await waitFor(() => {
      expect(screen.getByDisplayValue("mail.example.com")).toBeInTheDocument();
    });

    const hostInput = screen.getByDisplayValue("mail.example.com");
    await user.clear(hostInput);
    await user.type(hostInput, "new.smtp.com");

    const saveButton = screen.getByRole("button", { name: /guardar/i });
    expect(saveButton).toBeEnabled();
  });

  it("calls testSMTPConnection when test button is clicked", async () => {
    const user = userEvent.setup();
    render(<IntlWrapper><SMTPSettingsSection /></IntlWrapper>);

    await waitFor(() => {
      expect(screen.getByDisplayValue("mail.example.com")).toBeInTheDocument();
    });

    const testButton = screen.getByRole("button", { name: /enviar correo de prueba/i });
    await user.click(testButton);

    await waitFor(() => {
      expect(api.testSMTPConnection).toHaveBeenCalled();
    });
  });
});
