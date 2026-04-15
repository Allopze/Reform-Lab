/* eslint-disable @next/next/no-img-element */
import type { ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi, beforeEach } from "vitest";
import AccessShell from "./access-shell";
import { IntlWrapper } from "@/test/intl-wrapper";

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

const pushMock = vi.fn();
const loginMock = vi.fn();
const registerMock = vi.fn();
let searchParamsValue = "";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
  useSearchParams: () => new URLSearchParams(searchParamsValue),
}));

vi.mock("@/lib/auth-context", () => ({
  useAuth: () => ({
    login: loginMock,
    register: registerMock,
  }),
}));

describe("AccessShell", () => {
  beforeEach(() => {
    searchParamsValue = "";
  });

  it("renders the favicon, chevron and auth card with correct styling", () => {
    const { container } = render(<IntlWrapper><AccessShell /></IntlWrapper>);

    expect(screen.getByRole("link", { name: "" })).toHaveAttribute("href", "/");
    expect(screen.getByRole("img", { name: "Reform Lab" })).toHaveAttribute("src", "/favicon.svg");
    expect(container.querySelector("section")).toHaveClass("rounded-[34px]");
    expect(screen.getByRole("heading", { name: "Iniciar sesión" })).toBeInTheDocument();
  });

  it("keeps the same card shell while switching to register", async () => {
    const user = userEvent.setup();
    const { container } = render(<IntlWrapper><AccessShell /></IntlWrapper>);

    const card = container.querySelector("section");
    expect(screen.getByRole("heading", { name: "Iniciar sesión" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Crear cuenta" }));

    expect(card).toHaveClass("rounded-[34px]");
    expect(screen.getByRole("heading", { name: "Crear cuenta" })).toBeInTheDocument();
    expect(screen.getByLabelText("Nombre o apodo")).toBeInTheDocument();
    expect(screen.getByLabelText("Repetir contraseña")).toBeInTheDocument();
  });

  it("opens the register mode directly when requested in the URL", () => {
    searchParamsValue = "mode=register";

    render(<IntlWrapper><AccessShell /></IntlWrapper>);

    expect(screen.getByRole("heading", { name: "Crear cuenta" })).toBeInTheDocument();
    expect(screen.getByLabelText("Nombre o apodo")).toBeInTheDocument();
  });
});
