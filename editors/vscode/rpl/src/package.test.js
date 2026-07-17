const test = require("node:test");
const assert = require("node:assert/strict");
const manifest = require("../package.json");

test("native Explorer exposes package compilation for RPL files", () => {
  const menus = manifest.contributes && manifest.contributes.menus;
  const explorer = menus && menus["explorer/context"];

  assert.ok(Array.isArray(explorer), "explorer/context menu must be contributed");
  assert.ok(explorer.some((item) =>
    item.command === "rpl.compilePackage"
    && item.when === "resourceExtname == .rpl"
  ));
});
