import { test, expect } from "@playwright/test";

// Verify all admin UI pages load without crashing.
// Each test navigates to a page and checks for the absence of the
// "Something went wrong" error boundary, then asserts page-specific content.

test.describe("Admin UI pages", () => {
  test("Overview page loads", async ({ page }) => {
    await page.goto("/#/overview/list");
    await expect(page.locator("text=Node Status")).toBeVisible();
    await expect(page.locator("text=Something went wrong")).not.toBeVisible();
  });

  test("Metrics page loads", async ({ page }) => {
    await page.goto("/#/metrics/overview/cluster");
    // Metrics page renders graph panels, not a heading
    await expect(page.locator(".linegraph").first()).toBeVisible();
    await expect(page.locator("text=Something went wrong")).not.toBeVisible();
  });

  test("Databases page loads", async ({ page }) => {
    await page.goto("/#/databases");
    await expect(
      page.getByRole("heading", { name: "Databases", exact: true }),
    ).toBeVisible();
    await expect(page.locator("text=Something went wrong")).not.toBeVisible();
  });

  test("SQL Activity page loads", async ({ page }) => {
    await page.goto("/#/sql-activity");
    await expect(
      page.getByRole("heading", { name: "SQL Activity" }),
    ).toBeVisible();
    await expect(page.locator("text=Something went wrong")).not.toBeVisible();
  });

  test("SQL Query page loads", async ({ page }) => {
    await page.goto("/#/sql-query");
    await expect(
      page.getByRole("heading", { name: "SQL Query" }),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: "Run Query" })).toBeVisible();
    await expect(page.locator("text=Something went wrong")).not.toBeVisible();
  });

  test("SQL Query page executes a query", async ({ page }) => {
    await page.goto("/#/sql-query");
    const textarea = page.locator("textarea");
    await textarea.fill("SELECT 42 AS answer;");
    await page.getByRole("button", { name: "Run Query" }).click();
    await expect(page.locator("th:has-text('answer')")).toBeVisible();
    await expect(page.locator("td:has-text('42')")).toBeVisible();
  });

  test("Jobs page loads", async ({ page }) => {
    await page.goto("/#/jobs");
    await expect(page.getByRole("heading", { name: "Jobs", exact: true })).toBeVisible();
    await expect(page.locator("text=Something went wrong")).not.toBeVisible();
  });

  test("Hot Ranges page loads", async ({ page }) => {
    await page.goto("/#/hotranges");
    await expect(
      page.getByRole("heading", { name: /Hot Ranges/ }),
    ).toBeVisible();
    await expect(page.locator("text=Something went wrong")).not.toBeVisible();
  });

  test("Advanced Debug page loads", async ({ page }) => {
    await page.goto("/#/debug");
    await expect(
      page.getByRole("heading", { name: "Advanced Debugging", exact: true }),
    ).toBeVisible();
    await expect(page.locator("text=Something went wrong")).not.toBeVisible();
  });

  test("Network Latency page loads", async ({ page }) => {
    await page.goto("/#/reports/network");
    // Single-node clusters may show "No results" instead of a latency table
    await expect(page.locator("text=Something went wrong")).not.toBeVisible();
  });

  test("Events page loads", async ({ page }) => {
    await page.goto("/#/events");
    await expect(page.getByRole("heading", { name: "Events" })).toBeVisible();
    await expect(page.locator("text=Something went wrong")).not.toBeVisible();
  });

  test("Settings report loads", async ({ page }) => {
    await page.goto("/#/reports/settings");
    await expect(
      page.getByRole("heading", { name: "Cluster Settings" }).first(),
    ).toBeVisible();
    await expect(page.locator("text=Something went wrong")).not.toBeVisible();
  });
});
