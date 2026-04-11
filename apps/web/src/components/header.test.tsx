/* eslint-disable @next/next/no-img-element */
import type { ReactNode } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import Header from "./header";
import { ThemeProvider } from "@/lib/theme-context";
import { IntlWrapper } from "@/test/intl-wrapper";

const authState: {
  user: { name: string; role: "admin" | "user" } | null;
} = {
  user: {
    name: "Ada",
    role: "admin",
  },
};

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    ...props
  }: {
    children: ReactNode;
    href: string;
  }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

vi.mock("next/image", () => ({
  default: ({
    alt,
    src,
    priority: _priority,
    ...props
  }: {
    alt: string;
    src: string;
    priority?: boolean;
  }) => <img alt={alt} src={src} {...props} />,
}));

const logoutMock = vi.fn();

vi.mock("next/navigation", () => ({
  usePathname: () => "/",
}));

vi.mock("@/lib/auth-context", () => ({
  useAuth: () => ({
    user: authState.user,
    logout: logoutMock,
  }),
}));

describe("Header", () => {
  beforeEach(() => {
    window.localStorage.clear();
    document.documentElement.removeAttribute("data-theme");
    document.documentElement.style.colorScheme = "";
    logoutMock.mockClear();
    authState.user = {
      name: "Ada",
      role: "admin",
    };
  });

  it("toggles the theme, updates the label and persists the preference", async () => {
    const user = userEvent.setup();

    render(
      <IntlWrapper>
        <ThemeProvider>
          <Header />
        </ThemeProvider>
      </IntlWrapper>
    );

    await waitFor(() => {
      expect(document.documentElement).toHaveAttribute("data-theme", "light");
    });

    const toggle = screen.getByRole("button", { name: "Activar tema oscuro" });
    expect(screen.getByRole("img", { name: "Reform Lab" })).toHaveAttribute("src", "/logo-light.svg");

    await user.click(toggle);

    await waitFor(() => {
      expect(document.documentElement).toHaveAttribute("data-theme", "dark");
    });

    expect(window.localStorage.getItem("reform-lab-theme")).toBe("dark");
    expect(screen.getByRole("button", { name: "Activar tema claro" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("img", { name: "Reform Lab" })).toHaveAttribute("src", "/logo-dark.svg");
  });

  it("restores the saved theme on mount", async () => {
    window.localStorage.setItem("reform-lab-theme", "dark");

    render(
      <IntlWrapper>
        <ThemeProvider>
          <Header />
        </ThemeProvider>
      </IntlWrapper>
    );

    await waitFor(() => {
      expect(document.documentElement).toHaveAttribute("data-theme", "dark");
    });

    expect(screen.getByRole("button", { name: "Activar tema claro" })).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Reform Lab" })).toHaveAttribute("src", "/logo-dark.svg");
  });

  it("shows login and register links for guests in the dropdown", async () => {
    authState.user = null;
    const user = userEvent.setup();

    render(
      <IntlWrapper>
        <ThemeProvider>
          <Header />
        </ThemeProvider>
      </IntlWrapper>
    );

    await user.click(screen.getByLabelText("Abrir menu de usuario"));

    expect(screen.getByText("Accede o crea tu cuenta")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Iniciar sesion" })).toHaveAttribute("href", "/acceso");
    expect(screen.getByRole("link", { name: "Crear cuenta" })).toHaveAttribute("href", "/acceso?mode=register");
    expect(screen.getByRole("link", { name: "Crear cuenta" })).toHaveClass("bg-coral-50", "text-coral-700");
  });

  it("updates the guest register link styles when the theme changes", async () => {
    authState.user = null;
    const user = userEvent.setup();

    render(
      <IntlWrapper>
        <ThemeProvider>
          <Header />
        </ThemeProvider>
      </IntlWrapper>
    );

    await waitFor(() => {
      expect(document.documentElement).toHaveAttribute("data-theme", "light");
    });

    await user.click(screen.getByLabelText("Abrir menu de usuario"));

    const registerLink = screen.getByRole("link", { name: "Crear cuenta" });
    expect(registerLink).toHaveClass("bg-coral-50", "text-coral-700");

    await user.click(screen.getByRole("button", { name: "Activar tema oscuro" }));

    await waitFor(() => {
      expect(document.documentElement).toHaveAttribute("data-theme", "dark");
    });

    expect(registerLink).toHaveClass("bg-coral-900", "text-coral-100", "border", "border-coral-800");
  });
});