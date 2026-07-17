"use strict";

function uriKey(uri) {
  if (!uri) return "";
  return typeof uri.toString === "function" ? uri.toString() : String(uri);
}

class WorkspaceDiagnosticController {
  constructor(options) {
    this.diagnostics = options.diagnostics;
    this.enabled = options.enabled;
    this.validate = options.validate;
    this.openDocument = options.openDocument;
    this.findFiles = options.findFiles;
    this.isRplDocument = options.isRplDocument;
    this.onError = options.onError || (() => {});
    this.delay = Number.isFinite(options.delay) ? options.delay : 180;
    this.timers = new Map();
    this.disposed = false;
  }

  scheduleDocument(document, delay = this.delay) {
    if (this.disposed || !document || !this.isRplDocument(document)) return;
    if (!this.enabled()) {
      this.diagnostics.delete(document.uri);
      return;
    }

    const key = uriKey(document.uri);
    this.cancel(key);
    const timer = setTimeout(async () => {
      this.timers.delete(key);
      if (this.disposed || !this.enabled()) return;
      try {
        await this.validate(document);
      } catch (error) {
        this.onError(error);
      }
    }, Math.max(0, delay));
    this.timers.set(key, timer);
  }

  async scheduleUri(uri, delay = this.delay) {
    if (this.disposed || !uri) return;
    if (!this.enabled()) {
      this.diagnostics.delete(uri);
      return;
    }
    try {
      const document = await this.openDocument(uri);
      this.scheduleDocument(document, delay);
    } catch (error) {
      this.onError(error);
    }
  }

  handleClose(document) {
    // A DiagnosticCollection is URI-based and remains valid after a tab closes.
    // Validate the closing snapshot once more, but never delete or reopen it:
    // reopening from onDidClose can itself produce another close event.
    this.scheduleDocument(document, 0);
  }

  handleDelete(uri) {
    const key = uriKey(uri);
    this.cancel(key);
    this.diagnostics.delete(uri);
  }

  async validateWorkspace() {
    if (this.disposed) return 0;
    if (!this.enabled()) {
      this.clear();
      return 0;
    }

    let uris;
    try {
      uris = await this.findFiles();
    } catch (error) {
      this.onError(error);
      return 0;
    }

    let checked = 0;
    for (const uri of uris || []) {
      if (this.disposed || !this.enabled()) break;
      try {
        const document = await this.openDocument(uri);
        if (!this.isRplDocument(document)) continue;
        this.cancel(uriKey(uri));
        const result = await this.validate(document);
        checked += 1;
        if (result && result.runtimeError) break;
      } catch (error) {
        this.onError(error);
      }
    }
    return checked;
  }

  clear() {
    for (const timer of this.timers.values()) clearTimeout(timer);
    this.timers.clear();
    this.diagnostics.clear();
  }

  cancel(key) {
    const timer = this.timers.get(key);
    if (!timer) return;
    clearTimeout(timer);
    this.timers.delete(key);
  }

  dispose() {
    this.disposed = true;
    for (const timer of this.timers.values()) clearTimeout(timer);
    this.timers.clear();
  }
}

module.exports = { WorkspaceDiagnosticController, uriKey };
