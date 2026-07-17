"use strict";

const assert = require("node:assert/strict");
const test = require("node:test");
const { WorkspaceDiagnosticController } = require("./diagnosticController");

function uri(value) {
  return { value, toString() { return value; } };
}

function fakeController(overrides = {}) {
  const deleted = [];
  const diagnostics = {
    cleared: 0,
    delete(value) { deleted.push(value.toString()); },
    clear() { this.cleared += 1; }
  };
  const documents = new Map();
  const validated = [];
  const controller = new WorkspaceDiagnosticController({
    diagnostics,
    enabled: () => true,
    validate: async (document) => { validated.push(document.uri.toString()); return {}; },
    openDocument: async (value) => documents.get(value.toString()),
    findFiles: async () => Array.from(documents.values(), (document) => document.uri),
    isRplDocument: (document) => document && document.languageId === "rpl",
    delay: 0,
    ...overrides
  });
  return { controller, diagnostics, deleted, documents, validated };
}

test("closing a document keeps diagnostics and revalidates its URI", async () => {
  const state = fakeController();
  const file = uri("file:///workspace/schema.rpl");
  state.documents.set(file.toString(), { uri: file, languageId: "rpl" });

  state.controller.handleClose(state.documents.get(file.toString()));
  await new Promise((resolve) => setTimeout(resolve, 10));

  assert.deepEqual(state.deleted, []);
  assert.deepEqual(state.validated, [file.toString()]);
  state.controller.dispose();
});

test("workspace validation checks unopened RPL files and stops on runtime failure", async () => {
  const first = uri("file:///workspace/a.rpl");
  const second = uri("file:///workspace/b.rpl");
  const third = uri("file:///workspace/c.rpl");
  const calls = [];
  const state = fakeController({
    findFiles: async () => [first, second, third],
    openDocument: async (value) => ({ uri: value, languageId: "rpl" }),
    validate: async (document) => {
      calls.push(document.uri.toString());
      return { runtimeError: document.uri.toString() === second.toString() };
    }
  });

  const checked = await state.controller.validateWorkspace();

  assert.equal(checked, 2);
  assert.deepEqual(calls, [first.toString(), second.toString()]);
  state.controller.dispose();
});

test("deleting a file removes only its persisted diagnostics", () => {
  const state = fakeController();
  const file = uri("file:///workspace/deleted.rpl");

  state.controller.handleDelete(file);

  assert.deepEqual(state.deleted, [file.toString()]);
  assert.equal(state.diagnostics.cleared, 0);
  state.controller.dispose();
});

test("disabled workspace diagnostics clear every persisted marker", async () => {
  const state = fakeController({ enabled: () => false });

  const checked = await state.controller.validateWorkspace();

  assert.equal(checked, 0);
  assert.equal(state.diagnostics.cleared, 1);
  state.controller.dispose();
});
