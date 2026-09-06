import "@testing-library/jest-dom/vitest";
import { vi } from "vitest";

// Mock scrollTo which is not implemented in jsdom
Object.defineProperty(window, "scrollTo", { value: vi.fn(), writable: true });
