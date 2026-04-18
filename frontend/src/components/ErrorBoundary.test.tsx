import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import ErrorBoundary from "./ErrorBoundary";

// Suppress console.error output from ErrorBoundary's componentDidCatch during tests
beforeEach(() => {
  vi.spyOn(console, "error").mockImplementation(() => {});
});

function Bomb(): never {
  throw new Error("kaboom");
}

describe("ErrorBoundary", () => {
  it("renders children when no error occurs", () => {
    render(
      <ErrorBoundary>
        <p>All good</p>
      </ErrorBoundary>,
    );

    expect(screen.getByText("All good")).toBeInTheDocument();
  });

  it("shows error UI when a child throws", () => {
    render(
      <ErrorBoundary>
        <Bomb />
      </ErrorBoundary>,
    );

    expect(screen.getByText("Something went wrong")).toBeInTheDocument();
    expect(screen.getByText("kaboom")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry" })).toBeInTheDocument();
  });

  it("resets error state when Retry is clicked", () => {
    // Child that throws on first render, then succeeds after reset
    let shouldThrow = true;

    function ConditionalBomb(): string {
      if (shouldThrow) {
        throw new Error("boom");
      }
      return "Recovered";
    }

    render(
      <ErrorBoundary>
        <ConditionalBomb />
      </ErrorBoundary>,
    );

    expect(screen.getByText("Something went wrong")).toBeInTheDocument();

    // Fix the child before retrying so it doesn't throw again
    shouldThrow = false;

    fireEvent.click(screen.getByRole("button", { name: "Retry" }));

    expect(screen.getByText("Recovered")).toBeInTheDocument();
  });
});
