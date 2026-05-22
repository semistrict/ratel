const assert = require("node:assert/strict");
const { chromium } = require("playwright");

const baseURL = process.env.RATEL_CHAT_URL || "http://localhost:26257/workers/chat/";

async function main() {
  const browser = await chromium.launch();
  try {
    const page = await browser.newPage();
    const errors = [];
    const failedRequests = [];

    page.on("pageerror", error => errors.push(error.message));
    page.on("requestfailed", request => {
      failedRequests.push(`${request.method()} ${request.url()} ${request.failure()?.errorText || ""}`);
    });

    const response = await page.goto(baseURL, { waitUntil: "domcontentloaded" });
    assert.equal(response.status(), 200, "chat page should load");
    assert.equal(await page.locator("h1").textContent(), "Ratel Chat");
    await assertVisible(page, "#log");
    await assertVisible(page, "#name");
    await assertVisible(page, "#message");

    const cssResponse = await page.request.get(new URL("styles.css", baseURL).toString());
    assert.equal(cssResponse.status(), 200, "styles.css should load");
    assert.match(await cssResponse.text(), /#log/);

    const jsResponse = await page.request.get(new URL("client.js", baseURL).toString());
    assert.equal(jsResponse.status(), 200, "client.js should load");
    assert.match(await jsResponse.text(), /new WebSocket/);

    assert.deepEqual(errors, [], "page should not throw JavaScript errors");
    assert.deepEqual(failedRequests, [], "page should not have failed network requests");

    const logText = await page.locator("#log").innerText({ timeout: 5000 });
    assert.match(
      logText,
      /connected to room lobby/,
      "chat should establish its WebSocket and report a ready room connection",
    );

    await page.locator("#message").fill("hello from playwright");
    await page.locator("form").evaluate(form => form.requestSubmit());
    await page.waitForFunction(
      () => document.querySelector("#log")?.innerText.includes("guest: hello from playwright"),
      null,
      { timeout: 5000 },
    );
    assert.match(await page.locator("#log").innerText(), /guest: hello from playwright/);

    const secondPage = await browser.newPage();
    await secondPage.goto(baseURL, { waitUntil: "domcontentloaded" });
    await secondPage.locator("#log").innerText({ timeout: 5000 });
    await secondPage.locator("#name").fill("guest2");
    await secondPage.locator("#message").fill("hello from guest2");
    await secondPage.locator("form").evaluate(form => form.requestSubmit());

    await page.waitForFunction(
      () => document.querySelector("#log")?.innerText.includes("guest2: hello from guest2"),
      null,
      { timeout: 5000 },
    );
    await secondPage.waitForFunction(
      () => document.querySelector("#log")?.innerText.includes("guest2: hello from guest2"),
      null,
      { timeout: 5000 },
    );
    assert.match(await page.locator("#log").innerText(), /guest2: hello from guest2/);
    assert.match(await secondPage.locator("#log").innerText(), /guest2: hello from guest2/);
  } finally {
    await browser.close();
  }
}

async function assertVisible(page, selector) {
  await page.locator(selector).waitFor({ state: "visible", timeout: 5000 });
}

main().catch(error => {
  console.error(error);
  process.exit(1);
});
