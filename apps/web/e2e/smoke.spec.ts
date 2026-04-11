import { test, expect } from "@playwright/test";

test.describe("Smoke tests", () => {
  test("home page loads with main UI", async ({ page }) => {
    await page.goto("/");
    await expect(page).toHaveTitle(/Reform Lab/);
    await expect(page.getByRole("tab", { name: "Auto" })).toBeVisible();
  });

  test("category navigation works", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("tab", { name: "PDF" }).click();
    await expect(page.getByText("Convertir PDF")).toBeVisible();
  });

  test("login page loads", async ({ page }) => {
    await page.goto("/acceso");
    await expect(page.getByRole("heading", { name: "Iniciar sesión" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Crear cuenta" })).toBeVisible();
  });

  test("not-found page for invalid routes", async ({ page }) => {
    await page.goto("/ruta-inexistente");
    await expect(page.getByText("no encontrada")).toBeVisible();
  });
});
