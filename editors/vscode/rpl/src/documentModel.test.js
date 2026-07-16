const test = require("node:test");
const assert = require("node:assert/strict");
const { parseRplDocument } = require("./documentModel");

test("parseRplDocument exposes project structure for VS Code UX", () => {
  const schema = `package account

target(lang: golang)

attrs (
  "rpl:sql"
  "rpl:validate"
)

import (
  "time"
)

@sql(table: "users")
model User {
  @sql(primaryKey: true)
  Id int64
  Email string
  CreatedAt time.Time
}
`;

  const parsed = parseRplDocument(schema);
  assert.equal(parsed.packageName, "account");
  assert.equal(parsed.target, "golang");
  assert.deepEqual(parsed.attrs, ["rpl:sql", "rpl:validate"]);
  assert.deepEqual(parsed.imports, ["time"]);
  assert.equal(parsed.models.length, 1);
  assert.equal(parsed.models[0].name, "User");
  assert.deepEqual(parsed.models[0].fields.map((field) => [field.name, field.type]), [
    ["Id", "int64"],
    ["Email", "string"],
    ["CreatedAt", "time.Time"]
  ]);
});

test("parseRplDocument ignores braces in strings and comments", () => {
  const parsed = parseRplDocument(`model Event {
  // this brace is harmless: }
  Value string = "}"
  Enabled bool = true
}

model Audit {
  Id int64
}`);

  assert.deepEqual(parsed.models.map((model) => model.name), ["Event", "Audit"]);
  assert.deepEqual(parsed.models[0].fields.map((field) => field.name), ["Value", "Enabled"]);
});
