"use strict";

const TOKEN_URL = "http://localhost:38479/token";
const FETCH_TIMEOUT_MS = 3000;

async function sendToken(value) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);

  try {
    await fetch(TOKEN_URL, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token: value }),
      signal: controller.signal,
    });
  } catch {
    // Desktop app may not be running — silently ignore.
  } finally {
    clearTimeout(timer);
  }
}

chrome.cookies.onChanged.addListener((changeInfo) => {
  if (
    !changeInfo.removed &&
    changeInfo.cookie.name === "li_at" &&
    changeInfo.cookie.domain.includes("linkedin.com")
  ) {
    sendToken(changeInfo.cookie.value);
  }
});

// Send existing cookie on startup so the app gets it without waiting for a change.
chrome.cookies.get(
  { url: "https://www.linkedin.com", name: "li_at" },
  (cookie) => {
    if (cookie) {
      sendToken(cookie.value);
    }
  }
);
