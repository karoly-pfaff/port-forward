import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { NestExpressApplication } from "@nestjs/platform-express";
import type { Request, Response } from "express";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  configureStaticAssets,
  createStaticFallback,
  disabledStaticFallback,
  hasStaticClient,
  resolveStaticOptions,
  STATIC_INDEX_FILE,
} from "./static-serving.js";

let dir: string;

function makeClientDir(withIndex = true): string {
  const d = mkdtempSync(join(tmpdir(), "portier-static-"));
  if (withIndex) {
    writeFileSync(join(d, STATIC_INDEX_FILE), "<html><body>Portier</body></html>");
  }
  return d;
}

beforeEach(() => {
  dir = makeClientDir();
});

afterEach(() => {
  rmSync(dir, { recursive: true, force: true });
});

describe("hasStaticClient / resolveStaticOptions", () => {
  it("hasStaticClient is true only when index.html exists", () => {
    expect(hasStaticClient(dir)).toBe(true);
    const empty = makeClientDir(false);
    try {
      expect(hasStaticClient(empty)).toBe(false);
    } finally {
      rmSync(empty, { recursive: true, force: true });
    }
  });

  it("resolves enabled with the index path when the dir has an index.html", () => {
    expect(resolveStaticOptions(dir)).toEqual({
      enabled: true,
      staticDir: dir,
      indexPath: join(dir, STATIC_INDEX_FILE),
    });
  });

  it("resolves disabled (keeping the dir) when the dir exists but has no index.html", () => {
    const empty = makeClientDir(false);
    try {
      expect(resolveStaticOptions(empty)).toEqual({ enabled: false, staticDir: empty, indexPath: null });
    } finally {
      rmSync(empty, { recursive: true, force: true });
    }
  });

  it("resolves disabled with a null dir when none is supplied", () => {
    expect(resolveStaticOptions()).toEqual({ enabled: false, staticDir: null, indexPath: null });
  });

  it("resolves disabled (keeping the dir) for a nonexistent dir", () => {
    expect(resolveStaticOptions("/no/such/portier/dir")).toEqual({
      enabled: false,
      staticDir: "/no/such/portier/dir",
      indexPath: null,
    });
  });
});

describe("disabledStaticFallback", () => {
  it("never handles a request", () => {
    expect(disabledStaticFallback.handle({} as Request, {} as Response)).toBe(false);
  });
});

describe("createStaticFallback", () => {
  it("serves index.html and returns true for a GET", () => {
    const sendFile = vi.fn();
    const handled = createStaticFallback(dir).handle({ method: "GET" } as Request, { sendFile } as unknown as Response);
    expect(handled).toBe(true);
    expect(sendFile).toHaveBeenCalledWith(join(dir, STATIC_INDEX_FILE));
  });

  it("serves index.html and returns true for a HEAD", () => {
    const sendFile = vi.fn();
    const handled = createStaticFallback(dir).handle({ method: "HEAD" } as Request, { sendFile } as unknown as Response);
    expect(handled).toBe(true);
    expect(sendFile).toHaveBeenCalledTimes(1);
  });

  it("does not handle a non-GET/HEAD method", () => {
    const sendFile = vi.fn();
    const handled = createStaticFallback(dir).handle({ method: "POST" } as Request, { sendFile } as unknown as Response);
    expect(handled).toBe(false);
    expect(sendFile).not.toHaveBeenCalled();
  });

  it("returns the disabled fallback when the dir has no index.html", () => {
    const empty = makeClientDir(false);
    try {
      expect(createStaticFallback(empty)).toBe(disabledStaticFallback);
    } finally {
      rmSync(empty, { recursive: true, force: true });
    }
  });
});

describe("configureStaticAssets", () => {
  it("registers asset serving when the dir is enabled and returns the resolved state", () => {
    const useStaticAssets = vi.fn();
    const app = { useStaticAssets } as unknown as NestExpressApplication;
    const resolved = configureStaticAssets(app, dir);
    expect(useStaticAssets).toHaveBeenCalledWith(dir);
    expect(resolved.enabled).toBe(true);
    expect(resolved.staticDir).toBe(dir);
  });

  it("does not register asset serving when there is no usable dir", () => {
    const useStaticAssets = vi.fn();
    const app = { useStaticAssets } as unknown as NestExpressApplication;
    const resolved = configureStaticAssets(app, undefined);
    expect(useStaticAssets).not.toHaveBeenCalled();
    expect(resolved.enabled).toBe(false);
  });

  it("does not register asset serving for a dir without an index.html", () => {
    const empty = makeClientDir(false);
    const useStaticAssets = vi.fn();
    const app = { useStaticAssets } as unknown as NestExpressApplication;
    try {
      const resolved = configureStaticAssets(app, empty);
      expect(useStaticAssets).not.toHaveBeenCalled();
      expect(resolved.enabled).toBe(false);
    } finally {
      rmSync(empty, { recursive: true, force: true });
    }
  });
});
