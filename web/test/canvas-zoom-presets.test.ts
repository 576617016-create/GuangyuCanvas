import { expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

test("光宇无限画布 canvas precision zoom exposes the compact preset set", () => {
    const source = readFileSync(resolve(import.meta.dir, "../src/components/canvas/canvas-zoom-controls.tsx"), "utf8");
    expect(source).toContain("const QUICK_ZOOM_LEVELS = [0.5, 1, 8] as const;");
    expect(source).toContain("canvas-zoom-menu-presets");
});
