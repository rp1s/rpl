const test = require("node:test");
const assert = require("node:assert/strict");

const { mergeCatalogSpecEntries, normalizeCatalogSpec } = require("./attrSpecs");

test("mergeCatalogSpecEntries keeps args from duplicate namespace specs", () => {
  const manifest = { Author: "rpl", Name: "sql" };
  const modelSpec = normalizeCatalogSpec({
    namespace: "sql",
    help: "Model-level SQL attrs.",
    args: [
      { name: "db", types: ["string-like"] },
      { name: "table", types: ["string-like"] }
    ],
    snippets: [{ label: "@sql", insert: "@sql", help: "Model snippet." }]
  }, manifest, { analyze_model: true });
  const fieldSpec = normalizeCatalogSpec({
    namespace: "sql",
    help: "Field-level SQL attrs.",
    args: [
      { name: "default", types: ["string-like"] },
      { name: "index", types: ["bool"] },
      { name: "unique", types: ["bool"] },
      { name: "updatedAt", types: ["bool"] },
      { name: "ignore", types: ["bool", "string-like"] }
    ],
    snippets: [{ label: "@sql", insert: "@sql", help: "Field snippet." }]
  }, manifest, { generate_model: true });

  const merged = mergeCatalogSpecEntries(modelSpec, fieldSpec);

  assert.equal(merged.namespace, "sql");
  assert.deepEqual(
    merged.args.map((item) => item.name),
    ["db", "table", "default", "index", "unique", "updatedAt", "ignore"]
  );
  assert.match(merged.help, /Model-level SQL attrs\./);
  assert.match(merged.help, /Field-level SQL attrs\./);
  assert.deepEqual(merged.sources, ["rpl:sql"]);
  assert.equal(merged.capabilities.analyze_model, true);
  assert.equal(merged.capabilities.generate_model, true);
  assert.equal(merged.snippets.length, 1);
  assert.match(merged.snippets[0].help, /Model snippet\./);
  assert.match(merged.snippets[0].help, /Field snippet\./);
});

test("mergeCatalogSpecEntries merges duplicate arg definitions", () => {
  const manifest = { Author: "rpl", Name: "sql" };
  const left = normalizeCatalogSpec({
    namespace: "sql",
    args: [
      { name: "ignore", types: ["bool"], aliases: ["skip"], help: "Skip field." }
    ]
  }, manifest, {});
  const right = normalizeCatalogSpec({
    namespace: "sql",
    args: [
      { name: "ignore", types: ["string-like"], aliases: ["omit"], help: "Ignore by adapter." }
    ]
  }, manifest, {});

  const merged = mergeCatalogSpecEntries(left, right);

  assert.equal(merged.args.length, 1);
  assert.deepEqual(merged.args[0].types, ["bool", "string-like"]);
  assert.deepEqual(merged.args[0].aliases, ["skip", "omit"]);
  assert.match(merged.args[0].help, /Skip field\./);
  assert.match(merged.args[0].help, /Ignore by adapter\./);
});
