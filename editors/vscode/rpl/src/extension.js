const vscode = require("vscode");
const childProcess = require("node:child_process");
const path = require("node:path");
const readline = require("node:readline");
const { mergeCatalogSpecEntries, normalizeCatalogSpec } = require("./attrSpecs");
const { parseRplDocument } = require("./documentModel");
const { WorkspaceDiagnosticController } = require("./diagnosticController");

const LANGUAGE_ID = "rpl";
const DEFAULT_RUNTIME_IDS = [
  "\"rpl:std\"",
  "\"rpl:grpc\"",
  "\"rpl:mongodb\"",
  "\"rpl:sql\"",
  "\"rpl:validate\"",
  "\"rpl:redis\"",
  "\"rpl:transport\""
];
const KEYWORDS = ["target", "import", "attrs", "model", "field", "func", "return"];
const COMPILE_OUTPUT_STATE_KEY = "rpl.compileOutputDirs";
const FALLBACK_TARGET_CATALOGS = {
  golang: {
    lang: "golang",
    label: "Go",
    types: [
      { name: "string", category: "scalar", help: "Standard Go string." },
      { name: "bool", category: "scalar", help: "Boolean value." },
      { name: "byte", category: "scalar", help: "Single byte value." },
      { name: "int", category: "scalar", help: "Platform-sized Go integer." },
      { name: "int32", category: "scalar", help: "Fixed 32-bit signed integer." },
      { name: "int64", category: "scalar", help: "Fixed 64-bit signed integer." },
      { name: "uint32", category: "scalar", help: "Fixed 32-bit unsigned integer." },
      { name: "uint64", category: "scalar", help: "Fixed 64-bit unsigned integer." },
      { name: "float32", category: "scalar", help: "32-bit floating point number." },
      { name: "float64", category: "scalar", help: "64-bit floating point number." },
      { name: "[]byte", category: "binary", help: "Raw binary payload." },
      { name: "time.Time", category: "time", help: "Timestamp type used by Go-oriented attrs." }
    ],
    structures: [
      { label: "Type Alias", insert: "type ${1:UserID} ${2:int64}", category: "type", help: "Reusable alias for domain identifiers." },
      { label: "Time Model", insert: "model ${1:Event} {\n\tId int64\n\tCreatedAt time.Time\n\tUpdatedAt time.Time?\n}", category: "model", help: "Model template with timestamp fields." }
    ]
  }
};

class RplRuntimeClient {
  constructor(output) {
    this.output = output;
    this.process = null;
    this.stdout = null;
    this.pending = [];
  }

  async send(action, data) {
    await this.ensureStarted();

    return new Promise((resolve, reject) => {
      this.pending.push({ resolve, reject, action });
      this.process.stdin.write(JSON.stringify({ action, data }) + "\n", (error) => {
        if (error) {
          const item = this.pending.shift();
          if (item) {
            item.reject(error);
          }
        }
      });
    });
  }

  async ensureStarted() {
    if (this.process && !this.process.killed && this.process.exitCode === null) {
      return;
    }

    const config = vscode.workspace.getConfiguration("rpl");
    const binaryPath = config.get("binaryPath", "rpl");
    const activeDocument = vscode.window.activeTextEditor ? vscode.window.activeTextEditor.document : undefined;

    this.process = childProcess.spawn(binaryPath, ["runtime"], {
      cwd: workspaceRootFor(activeDocument),
      stdio: ["pipe", "pipe", "pipe"]
    });

    this.process.on("error", (error) => {
      this.rejectAll(new Error(`failed to start RPL runtime: ${error.message}`));
    });

    this.process.on("exit", (code, signal) => {
      const reason = code !== null ? `exit code ${code}` : `signal ${signal || "unknown"}`;
      this.rejectAll(new Error(`RPL runtime stopped with ${reason}`));
      this.process = null;
      this.stdout = null;
    });

    this.process.stderr.on("data", (chunk) => {
      const text = chunk.toString("utf8").trim();
      if (text) {
        this.output.appendLine(text);
      }
    });

    this.stdout = readline.createInterface({ input: this.process.stdout });
    this.stdout.on("line", (line) => this.handleLine(line));
  }

  handleLine(line) {
    let payload;
    try {
      payload = JSON.parse(line);
    } catch (error) {
      this.output.appendLine(`RPL runtime returned invalid JSON: ${line}`);
      return;
    }

    const pending = this.pending.shift();
    if (!pending) {
      return;
    }

    if (payload && typeof payload.error === "string" && payload.error.trim() !== "") {
      pending.reject(new Error(payload.error));
      return;
    }

    if (payload && Object.prototype.hasOwnProperty.call(payload, "result")) {
      pending.resolve(payload.result);
      return;
    }

    pending.resolve(payload);
  }

  rejectAll(error) {
    const items = this.pending.splice(0);
    for (const item of items) {
      item.reject(error);
    }
  }

  async restart() {
    if (this.stdout) {
      this.stdout.close();
      this.stdout = null;
    }
    if (this.process && !this.process.killed && this.process.exitCode === null) {
      this.process.kill();
    }
    this.process = null;
  }

  dispose() {
    this.restart();
  }
}

class RplAttrCatalog {
  constructor(client, output) {
    this.client = client;
    this.output = output;
    this.items = [];
    this.loadedAt = 0;
    this.loadedPath = "";
    this.typeCatalog = fallbackTargetCatalog("golang");
    this.typeCatalogLoadedAt = 0;
    this.typeCatalogKey = "";
  }

  invalidate() {
    this.items = [];
    this.loadedAt = 0;
    this.loadedPath = "";
    this.typeCatalog = fallbackTargetCatalog("golang");
    this.typeCatalogLoadedAt = 0;
    this.typeCatalogKey = "";
  }

  async load(force = false) {
    const now = Date.now();
    const currentPath = currentDocumentPath();
    if (!force && this.loadedAt > 0 && now - this.loadedAt < 15000 && currentPath === this.loadedPath) {
      return this.items;
    }

    try {
      const response = await this.client.send("attrs.search", { value: "", path: currentPath });
      this.items = Array.isArray(response && response.items) ? response.items : [];
      this.loadedAt = now;
      this.loadedPath = currentPath;
    } catch (error) {
      this.output.appendLine(`Failed to refresh attr catalog: ${String(error && error.message ? error.message : error)}`);
      this.items = [];
      this.loadedAt = now;
      this.loadedPath = currentPath;
    }

    return this.items;
  }

  async runtimeIds() {
    const values = new Set(DEFAULT_RUNTIME_IDS);
    for (const item of await this.load()) {
      const manifest = item && item.Manifest ? item.Manifest : {};
      if (!manifest.Author || !manifest.Name) {
        continue;
      }
      values.add(JSON.stringify(`${manifest.Author}:${manifest.Name}`));
    }

    return Array.from(values.values()).sort();
  }

  async attrSpecs() {
    const merged = new Map();
    for (const item of await this.load()) {
      const manifest = item && item.Manifest ? item.Manifest : {};
      const specs = Array.isArray(item && item.specs) ? item.specs : [];
      for (const spec of specs) {
        if (!spec || !spec.namespace) {
          continue;
        }
        const namespace = String(spec.namespace).trim();
        if (!namespace) {
          continue;
        }

        const entry = normalizeCatalogSpec(
          spec,
          manifest,
          item && item.capabilities ? item.capabilities : {}
        );
        if (!entry) {
          continue;
        }

        const current = merged.get(namespace);
        merged.set(namespace, current ? mergeCatalogSpecEntries(current, entry) : entry);
      }
    }

    const values = Array.from(merged.values());
    values.sort((left, right) => left.namespace.localeCompare(right.namespace));
    return values;
  }

  async loadTypeCatalog(document, force = false) {
    const now = Date.now();
    const currentPath = documentPath(document);
    const lang = targetLanguageForDocument(document);
    const cacheKey = `${lang}|${currentPath}`;
    if (!force && this.typeCatalogLoadedAt > 0 && now - this.typeCatalogLoadedAt < 15000 && cacheKey === this.typeCatalogKey) {
      return this.typeCatalog;
    }

    try {
      const response = await this.client.send("types.catalog", { lang, path: currentPath });
      const rawCatalog = response && response.catalog ? response.catalog : response;
      this.typeCatalog = normalizeTargetCatalog(rawCatalog, lang);
      this.typeCatalogLoadedAt = now;
      this.typeCatalogKey = cacheKey;
    } catch (error) {
      this.output.appendLine(`Failed to refresh target type catalog: ${String(error && error.message ? error.message : error)}`);
      this.typeCatalog = fallbackTargetCatalog(lang);
      this.typeCatalogLoadedAt = now;
      this.typeCatalogKey = cacheKey;
    }

    return this.typeCatalog;
  }

  async findSpec(namespace) {
    const trimmed = String(namespace || "").trim();
    if (!trimmed) {
      return null;
    }

    const specs = await this.attrSpecs();
    return specs.find((item) => item.namespace === trimmed) || null;
  }
}

class RplWorkspaceProvider {
  constructor(catalog) {
    this.catalog = catalog;
    this.changed = new vscode.EventEmitter();
    this.onDidChangeTreeData = this.changed.event;
  }

  refresh() {
    this.changed.fire(undefined);
  }

  getTreeItem(element) {
    return element;
  }

  async getChildren(element) {
    if (!element) {
      return [
        treeNode("Схемы", "schemas", "files", vscode.TreeItemCollapsibleState.Expanded),
        treeNode("Attrs", "attrs", "extensions", vscode.TreeItemCollapsibleState.Collapsed),
        treeNode("Toolchain", "toolchain", "tools", vscode.TreeItemCollapsibleState.Expanded)
      ];
    }

    if (element.rplKind === "schemas") {
      const files = await vscode.workspace.findFiles("**/*.rpl", "**/{.git,node_modules,vendor,build,out}/**", 300);
      return files.sort((left, right) => left.fsPath.localeCompare(right.fsPath)).map((uri) => {
        const folder = vscode.workspace.getWorkspaceFolder(uri);
        const label = folder ? path.relative(folder.uri.fsPath, uri.fsPath) : path.basename(uri.fsPath);
        const item = treeNode(label, "schema", "symbol-file", vscode.TreeItemCollapsibleState.None);
        item.resourceUri = uri;
        item.tooltip = uri.fsPath;
        item.command = { command: "vscode.open", title: "Открыть схему", arguments: [uri] };
        item.contextValue = "rplSchema";
        return item;
      });
    }

    if (element.rplKind === "attrs") {
      const attrs = await this.catalog.load();
      return attrs.map((entry) => {
        const manifest = entry && entry.Manifest ? entry.Manifest : {};
        const id = `${manifest.Author || "?"}:${manifest.Name || "?"}`;
        const item = treeNode(id, "attr", "symbol-method", vscode.TreeItemCollapsibleState.None);
        item.description = manifest.Version || "";
        item.tooltip = manifest.Description || id;
        item.command = { command: "rpl.showAttrInfo", title: "Информация об attr", arguments: [entry] };
        return item;
      });
    }

    if (element.rplKind === "toolchain") {
      const binary = vscode.workspace.getConfiguration("rpl").get("binaryPath", "rpl");
      const binaryItem = treeNode(`Binary: ${binary}`, "binary", "terminal", vscode.TreeItemCollapsibleState.None);
      binaryItem.command = { command: "rpl.showToolchain", title: "Проверить RPL toolchain" };
      const runtimeItem = treeNode("Проверить runtime", "runtime", "pulse", vscode.TreeItemCollapsibleState.None);
      runtimeItem.command = { command: "rpl.showToolchain", title: "Проверить RPL toolchain" };
      return [binaryItem, runtimeItem];
    }

    return [];
  }

  dispose() {
    this.changed.dispose();
  }
}

function treeNode(label, rplKind, icon, state) {
  const item = new vscode.TreeItem(label, state);
  item.rplKind = rplKind;
  item.iconPath = new vscode.ThemeIcon(icon);
  return item;
}

class RplStatus {
  constructor() {
    this.statusBar = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 25);
    this.statusBar.command = "rpl.showMenu";
    this.language = vscode.languages.createLanguageStatusItem("rpl.toolchain", { language: LANGUAGE_ID });
    this.language.name = "RPL Toolchain";
    this.language.command = { command: "rpl.showMenu", title: "Открыть центр действий RPL" };
    this.setIdle();
  }

  setIdle(detail = "RPL готов") {
    this.statusBar.text = "$(bracket-dot) RPL";
    this.statusBar.tooltip = detail;
    this.language.text = "RPL готов";
    this.language.detail = detail;
    this.language.severity = vscode.LanguageStatusSeverity.Information;
    this.updateVisibility();
  }

  setBusy(detail) {
    this.statusBar.text = "$(sync~spin) RPL";
    this.statusBar.tooltip = detail;
    this.language.text = "RPL работает…";
    this.language.detail = detail;
    this.language.severity = vscode.LanguageStatusSeverity.Information;
    this.updateVisibility();
  }

  setProblems(count, detail) {
    if (count > 0) {
      this.statusBar.text = `$(error) RPL ${count}`;
      this.language.text = `${count} проблем`;
      this.language.severity = vscode.LanguageStatusSeverity.Error;
    } else {
      this.statusBar.text = "$(check) RPL";
      this.language.text = "Схема корректна";
      this.language.severity = vscode.LanguageStatusSeverity.Information;
    }
    this.statusBar.tooltip = detail;
    this.language.detail = detail;
    this.updateVisibility();
  }

  setError(detail) {
    this.statusBar.text = "$(warning) RPL";
    this.statusBar.tooltip = detail;
    this.language.text = "Toolchain недоступен";
    this.language.detail = detail;
    this.language.severity = vscode.LanguageStatusSeverity.Error;
    this.updateVisibility();
  }

  updateVisibility() {
    const enabled = vscode.workspace.getConfiguration("rpl").get("showStatusBar", true);
    const editor = vscode.window.activeTextEditor;
    if (enabled && editor && editor.document.languageId === LANGUAGE_ID) this.statusBar.show();
    else this.statusBar.hide();
  }

  dispose() {
    this.statusBar.dispose();
    this.language.dispose();
  }
}

function activate(context) {
  const output = vscode.window.createOutputChannel("RPL");
  const client = new RplRuntimeClient(output);
  const catalog = new RplAttrCatalog(client, output);
  const diagnostics = vscode.languages.createDiagnosticCollection("rpl");
  const status = new RplStatus();
  const workspaceProvider = new RplWorkspaceProvider(catalog);
  const suggestTimers = new Map();

  const diagnosticController = new WorkspaceDiagnosticController({
    diagnostics,
    enabled: () => vscode.workspace.getConfiguration("rpl").get("enableDiagnostics", true),
    validate: (document) => validateDocument(document, client, diagnostics, output, status),
    openDocument: (uri) => vscode.workspace.openTextDocument(uri),
    findFiles: () => vscode.workspace.findFiles("**/*.rpl", "**/{.git,node_modules,vendor,build,out}/**"),
    isRplDocument: (document) => document && document.languageId === LANGUAGE_ID,
    onError: (error) => output.appendLine(`Workspace diagnostics: ${errorMessage(error)}`),
    delay: 180
  });

  const scheduleValidation = (document) => {
    diagnosticController.scheduleDocument(document);
  };

  const scheduleAttrArgSuggest = (document) => {
    if (!document || document.languageId !== LANGUAGE_ID) {
      return;
    }

    const editor = vscode.window.activeTextEditor;
    if (!editor || editor.document.uri.toString() !== document.uri.toString()) {
      return;
    }

    const key = document.uri.toString();
    const current = suggestTimers.get(key);
    if (current) {
      clearTimeout(current);
    }

    const timer = setTimeout(async () => {
      suggestTimers.delete(key);

      const active = vscode.window.activeTextEditor;
      if (!active || active.document.uri.toString() !== document.uri.toString()) {
        return;
      }

      const position = active.selection.active;
      if (!isAttrArgContext(document, position)) {
        return;
      }

      await vscode.commands.executeCommand("editor.action.triggerSuggest");
    }, 1000);
    suggestTimers.set(key, timer);
  };

  context.subscriptions.push(
    output,
    diagnostics,
    diagnosticController,
    status,
    workspaceProvider,
    vscode.window.registerTreeDataProvider("rpl.workspace", workspaceProvider),
    vscode.commands.registerCommand("rpl.compilePackage", async (uri) => {
      const document = await resolveRplDocument(uri);
      if (!document) return;
      if (document.isUntitled || document.uri.scheme !== "file") {
        vscode.window.showErrorMessage("RPL compile работает только для сохранённых файлов.");
        return;
      }

      await document.save();

      try {
        status.setBusy("Генерация package…");
        const outputDir = await resolveCompileOutputDir(context, document);
        if (!outputDir) {
          status.setIdle();
          return;
        }

        const binaryPath = vscode.workspace.getConfiguration("rpl").get("binaryPath", "rpl");
        const args = ["run", document.uri.fsPath, "out", outputDir];
        output.appendLine(`$ ${binaryPath} ${args.join(" ")}`);

        const result = await execRpl(binaryPath, args, workspaceRootFor(document));
        if (result.stdout) {
          output.appendLine(result.stdout.trim());
        }
        if (result.stderr) {
          output.appendLine(result.stderr.trim());
        }

        status.setIdle(`Package сгенерирован: ${outputDir}`);
        workspaceProvider.refresh();
        const choice = await vscode.window.showInformationMessage(`RPL compiled to ${outputDir}`, "Показать", "Сменить output");
        if (choice === "Показать") await vscode.commands.executeCommand("revealFileInOS", vscode.Uri.file(outputDir));
        if (choice === "Сменить output") await vscode.commands.executeCommand("rpl.configureOutput", document.uri);
      } catch (error) {
        const message = String(error && error.message ? error.message : error);
        output.appendLine(message);
        status.setError(message);
        vscode.window.showErrorMessage(`RPL compile failed: ${message}`);
      }
    }),
    vscode.commands.registerCommand("rpl.showMenu", async () => {
      const actions = [
        { label: "$(play) Сгенерировать package", description: "rpl run", command: "rpl.compilePackage" },
        { label: "$(folder) Настроить папку генерации", description: "Output для текущей схемы", command: "rpl.configureOutput" },
        { label: "$(check-all) Проверить схему", description: "Диагностика компилятором", command: "rpl.checkDocument" },
        { label: "$(symbol-keyword) Настроить импорты", description: "attrs и Go imports", command: "rpl.autoSetImports" },
        { label: "$(book) Сгенерировать документацию", description: "README из схемы", command: "rpl.generateDocs" },
        { label: "$(tools) Состояние toolchain", description: "Binary, runtime и attrs", command: "rpl.showToolchain" },
        { label: "$(file-binary) Выбрать RPL binary", description: "Настроить rpl.binaryPath", command: "rpl.selectBinary" },
        { label: "$(output) Открыть RPL Output", description: "Логи расширения", command: "rpl.openOutput" },
        { label: "$(refresh) Перезапустить runtime", description: "Перезапуск фонового процесса", command: "rpl.restartRuntime" }
      ];
      const selected = await vscode.window.showQuickPick(actions, {
        title: "RPL - центр действий",
        placeHolder: "Выберите действие для текущей схемы",
        matchOnDescription: true
      });
      if (selected) await vscode.commands.executeCommand(selected.command);
    }),
    vscode.commands.registerCommand("rpl.checkDocument", async (uri) => {
      const document = await resolveRplDocument(uri);
      if (!document) return;
      status.setBusy("Проверка схемы…");
      await validateDocument(document, client, diagnostics, output, status);
      const count = (diagnostics.get(document.uri) || []).length;
      if (count === 0) vscode.window.showInformationMessage("RPL: схема корректна.");
      else vscode.window.showWarningMessage(`RPL: найдено проблем - ${count}.`);
    }),
    vscode.commands.registerCommand("rpl.configureOutput", async (uri) => {
      const document = await resolveRplDocument(uri);
      if (!document || document.uri.scheme !== "file") return;
      const outputDir = await resolveCompileOutputDir(context, document, true);
      if (outputDir) vscode.window.showInformationMessage(`RPL output: ${outputDir}`);
    }),
    vscode.commands.registerCommand("rpl.generateDocs", async (uri) => {
      const document = await resolveRplDocument(uri);
      if (!document || document.isUntitled || document.uri.scheme !== "file") return;
      await document.save();
      const binaryPath = vscode.workspace.getConfiguration("rpl").get("binaryPath", "rpl");
      status.setBusy("Генерация README…");
      try {
        const result = await execRpl(binaryPath, ["docs", document.uri.fsPath], workspaceRootFor(document));
        appendCommandResult(output, binaryPath, ["docs", document.uri.fsPath], result);
        const readmePath = documentationPathFor(document.uri.fsPath);
        status.setIdle(`Документация: ${readmePath}`);
        workspaceProvider.refresh();
        const readme = await vscode.workspace.openTextDocument(vscode.Uri.file(readmePath));
        await vscode.window.showTextDocument(readme, { preview: false });
      } catch (error) {
        const message = errorMessage(error);
        output.appendLine(message);
        status.setError(message);
        vscode.window.showErrorMessage(`RPL docs failed: ${message}`);
      }
    }),
    vscode.commands.registerCommand("rpl.selectBinary", async () => {
      const picked = await vscode.window.showOpenDialog({
        title: "Выберите RPL CLI binary",
        canSelectFiles: true,
        canSelectFolders: false,
        canSelectMany: false,
        openLabel: "Использовать этот binary"
      });
      if (!picked || picked.length === 0) return;
      await vscode.workspace.getConfiguration("rpl").update("binaryPath", picked[0].fsPath, vscode.ConfigurationTarget.Workspace);
      vscode.window.showInformationMessage(`RPL binary: ${picked[0].fsPath}`);
    }),
    vscode.commands.registerCommand("rpl.showToolchain", async () => {
      const binaryPath = vscode.workspace.getConfiguration("rpl").get("binaryPath", "rpl");
      status.setBusy("Проверка RPL toolchain…");
      try {
        const result = await execRpl(binaryPath, ["help"], workspaceRootFor(vscode.window.activeTextEditor && vscode.window.activeTextEditor.document));
        const versionLine = stripAnsi(result.stdout).split(/\r?\n/).find((line) => /^RPL\s+/i.test(line.trim())) || "RPL CLI доступен";
        const attrs = await catalog.load(true);
        const detail = `${versionLine.trim()} • attrs: ${attrs.length} • ${binaryPath}`;
        status.setIdle(detail);
        workspaceProvider.refresh();
        vscode.window.showInformationMessage(detail, "Открыть Output").then((choice) => {
          if (choice) output.show(true);
        });
      } catch (error) {
        const message = errorMessage(error);
        status.setError(message);
        vscode.window.showErrorMessage(`RPL toolchain недоступен: ${message}`, "Выбрать binary").then((choice) => {
          if (choice) vscode.commands.executeCommand("rpl.selectBinary");
        });
      }
    }),
    vscode.commands.registerCommand("rpl.openOutput", () => output.show(true)),
    vscode.commands.registerCommand("rpl.showAttrInfo", async (entry) => {
      const manifest = entry && entry.Manifest ? entry.Manifest : {};
      const id = `${manifest.Author || "?"}:${manifest.Name || "?"}`;
      const specs = Array.isArray(entry && entry.specs) ? entry.specs : [];
      const capabilities = formatCapabilities(entry && entry.capabilities);
      const lines = [`${id}${manifest.Version ? ` v${manifest.Version}` : ""}`];
      if (manifest.Description) lines.push(manifest.Description);
      if (specs.length) lines.push(`Namespaces: ${specs.map((spec) => `@${spec.namespace}`).join(", ")}`);
      if (capabilities) lines.push(`Capabilities: ${capabilities}`);
      vscode.window.showInformationMessage(lines.join(" • "));
    }),
    vscode.commands.registerCommand("rpl.addAttrDeclaration", async (uri, runtimeId) => {
      if (!uri || !runtimeId) {
        return;
      }

      const document = await vscode.workspace.openTextDocument(uri);
      const editor = await vscode.window.showTextDocument(document, { preview: false });
      const updated = ensureAttrDeclaration(document.getText(), runtimeId);
      if (updated !== document.getText()) {
        await replaceDocumentText(editor, updated);
      }
    }),
    vscode.commands.registerCommand("rpl.insertAttrsBlockQuickFix", async (uri) => {
      if (!uri) {
        return;
      }

      const document = await vscode.workspace.openTextDocument(uri);
      const editor = await vscode.window.showTextDocument(document, { preview: false });
      const updated = ensureAttrBlockText(document.getText());
      if (updated !== document.getText()) {
        await replaceDocumentText(editor, updated);
      }
    }),
    vscode.commands.registerCommand("rpl.autoSetImports", async () => {
      const editor = vscode.window.activeTextEditor;
      if (!editor || editor.document.languageId !== LANGUAGE_ID) {
        return;
      }

      try {
        const response = await client.send("auto.set.import", {
          code: editor.document.getText(),
          path: editor.document.uri.scheme === "file" ? editor.document.uri.fsPath : ""
        });
        if (!response || typeof response.code !== "string" || response.code === editor.document.getText()) {
          return;
        }
        await replaceDocumentText(editor, response.code);
      } catch (error) {
        output.appendLine(String(error && error.message ? error.message : error));
      }
    }),
    vscode.commands.registerCommand("rpl.insertAttrsBlock", async () => {
      const editor = vscode.window.activeTextEditor;
      if (!editor || editor.document.languageId !== LANGUAGE_ID) {
        return;
      }

      await insertTopLevelBlock(editor, "attrs (\n\t\n)\n\n", /\battrs\s*\(/);
    }),
    vscode.commands.registerCommand("rpl.insertImportBlock", async () => {
      const editor = vscode.window.activeTextEditor;
      if (!editor || editor.document.languageId !== LANGUAGE_ID) {
        return;
      }

      await insertTopLevelBlock(editor, "import (\n\t\n)\n\n", /\bimport\s*\(/);
    }),
    vscode.commands.registerCommand("rpl.insertModelScaffold", async () => {
      const editor = vscode.window.activeTextEditor;
      if (!editor || editor.document.languageId !== LANGUAGE_ID) {
        return;
      }

      await insertAtCursor(editor, "model ModelName {\n\tName string\n}\n");
    }),
    vscode.commands.registerCommand("rpl.restartRuntime", async () => {
      catalog.invalidate();
      await client.restart();
      output.appendLine("RPL runtime restarted.");
      status.setIdle("Runtime перезапущен");
      await diagnosticController.validateWorkspace();
    }),
    vscode.commands.registerCommand("rpl.refreshAttrs", async () => {
      catalog.invalidate();
      const items = await catalog.load(true);
      output.appendLine(`RPL attr catalog refreshed (${items.length} attrs).`);
      workspaceProvider.refresh();
      status.setIdle(`Каталог attrs: ${items.length}`);
    }),
    vscode.workspace.onDidOpenTextDocument(scheduleValidation),
    vscode.workspace.onDidSaveTextDocument(scheduleValidation),
    vscode.workspace.onDidCloseTextDocument((document) => {
      if (document.languageId === LANGUAGE_ID && document.uri.scheme === "file") {
        diagnosticController.handleClose(document);
      }
      const key = document.uri.toString();
      const timer = suggestTimers.get(key);
      if (timer) {
        clearTimeout(timer);
        suggestTimers.delete(key);
      }
      status.updateVisibility();
    }),
    vscode.workspace.onDidChangeTextDocument((event) => {
      scheduleValidation(event.document);
      scheduleAttrArgSuggest(event.document);
    }),
    vscode.window.onDidChangeActiveTextEditor((editor) => {
      status.updateVisibility();
    }),
    vscode.workspace.onDidChangeConfiguration(async (event) => {
      if (event.affectsConfiguration("rpl.showStatusBar")) status.updateVisibility();
      if (event.affectsConfiguration("rpl.binaryPath") || event.affectsConfiguration("rpl.enableDiagnostics")) {
        catalog.invalidate();
        await client.restart();
        workspaceProvider.refresh();
        status.updateVisibility();
        await diagnosticController.validateWorkspace();
      }
    })
  );

  context.subscriptions.push(
    vscode.workspace.onWillSaveTextDocument((event) => {
      const document = event.document;
      if (!document || document.languageId !== LANGUAGE_ID) {
        return;
      }

      const config = vscode.workspace.getConfiguration("rpl");
      const formatEnabled = config.get("formatOnSave", true);
      const autoImportEnabled = config.get("autoSetImportsOnSave", true);
      if (!formatEnabled && !autoImportEnabled) {
        return;
      }

      event.waitUntil(provideSaveEdits(document, client, output, {
        autoImport: autoImportEnabled,
        format: formatEnabled
      }));
    })
  );

  if (vscode.workspace.getConfiguration("rpl").get("enableCompletions", true)) {
    context.subscriptions.push(
      vscode.languages.registerCompletionItemProvider(
        { language: LANGUAGE_ID },
        {
          async provideCompletionItems(document, position) {
            return buildCompletions(document, position, catalog);
          }
        },
        "@",
        "\"",
        ".",
        "(",
        ","
      )
    );
  }

  context.subscriptions.push(
    vscode.languages.registerCodeActionsProvider({ language: LANGUAGE_ID }, {
      async provideCodeActions(document, range, context) {
        return buildQuickFixes(document, context, catalog);
      }
    }, {
      providedCodeActionKinds: [vscode.CodeActionKind.QuickFix]
    }),
    vscode.languages.registerHoverProvider({ language: LANGUAGE_ID }, {
      async provideHover(document, position) {
        return provideAttrHover(document, position, catalog);
      }
    }),
    vscode.languages.registerDocumentFormattingEditProvider({ language: LANGUAGE_ID }, {
      async provideDocumentFormattingEdits(document) {
        return provideFormattingEdits(document, client, output);
      }
    }),
    vscode.languages.registerDocumentSymbolProvider({ language: LANGUAGE_ID }, {
      provideDocumentSymbols(document) {
        return provideRplDocumentSymbols(document);
      }
    }),
    vscode.languages.registerCodeLensProvider({ language: LANGUAGE_ID }, {
      provideCodeLenses(document) {
        if (!vscode.workspace.getConfiguration("rpl").get("enableCodeLens", true)) return [];
        return provideRplCodeLenses(document);
      }
    }),
    vscode.languages.registerDocumentSemanticTokensProvider({ language: LANGUAGE_ID }, {
      provideDocumentSemanticTokens(document) {
        return provideRplSemanticTokens(document);
      }
    }, new vscode.SemanticTokensLegend(["class", "property"], []))
  );

  const workspaceWatcher = vscode.workspace.createFileSystemWatcher("**/*.rpl");
  context.subscriptions.push(
    workspaceWatcher,
    workspaceWatcher.onDidCreate((uri) => {
      workspaceProvider.refresh();
      diagnosticController.scheduleUri(uri);
    }),
    workspaceWatcher.onDidDelete((uri) => {
      workspaceProvider.refresh();
      diagnosticController.handleDelete(uri);
    }),
    workspaceWatcher.onDidChange((uri) => {
      workspaceProvider.refresh();
      diagnosticController.scheduleUri(uri);
    })
  );

  context.subscriptions.push(vscode.tasks.registerTaskProvider("rpl", {
    async provideTasks() {
      if (!vscode.workspace.getConfiguration("rpl").get("enableTaskProvider", true)) return [];
      const files = await vscode.workspace.findFiles("**/*.rpl", "**/{.git,node_modules,vendor,build,out}/**", 100);
      const tasks = [];
      for (const uri of files) {
        tasks.push(createRplTask(uri, "generate"));
        tasks.push(createRplTask(uri, "docs"));
        tasks.push(createRplTask(uri, "format"));
      }
      return tasks;
    },
    resolveTask(task) {
      const definition = task && task.definition ? task.definition : {};
      if (!definition.schema || !definition.command) return undefined;
      return createRplTask(vscode.Uri.file(definition.schema), definition.command);
    }
  }));

  diagnosticController.validateWorkspace();
}

function provideRplDocumentSymbols(document) {
  const parsed = parseRplDocument(document.getText());
  const symbols = [];
  const text = document.getText();
  const packageMatch = text.match(/^\s*package\s+([A-Za-z_][A-Za-z0-9_]*)/m);
  if (packageMatch && parsed.packageName) {
    symbols.push(symbolAtMatch(document, packageMatch, parsed.packageName, vscode.SymbolKind.Package));
  }
  const targetMatch = text.match(/\btarget\s*\([^)]*\)/m);
  if (targetMatch) {
    symbols.push(symbolAtMatch(document, targetMatch, `target: ${parsed.target}`, vscode.SymbolKind.Namespace));
  }
  for (const model of parsed.models) {
    const range = new vscode.Range(document.positionAt(model.start), document.positionAt(model.end));
    const selection = new vscode.Range(document.positionAt(model.nameStart), document.positionAt(model.nameStart + model.name.length));
    const symbol = new vscode.DocumentSymbol(model.name, "RPL model", vscode.SymbolKind.Class, range, selection);
    for (const field of model.fields) {
      const fieldRange = new vscode.Range(document.positionAt(field.start), document.positionAt(field.end));
      symbol.children.push(new vscode.DocumentSymbol(field.name, field.type, vscode.SymbolKind.Field, fieldRange, fieldRange));
    }
    symbols.push(symbol);
  }
  return symbols.filter(Boolean);
}

function createRplTask(uri, command) {
  const folder = vscode.workspace.getWorkspaceFolder(uri);
  const scope = folder || vscode.TaskScope.Workspace;
  const relative = folder ? path.relative(folder.uri.fsPath, uri.fsPath) : path.basename(uri.fsPath);
  const binaryPath = vscode.workspace.getConfiguration("rpl").get("binaryPath", "rpl");
  const commandArgs = command === "generate"
    ? ["run", uri.fsPath]
    : command === "docs"
      ? ["docs", uri.fsPath]
      : ["fmt", uri.fsPath];
  const definition = { type: "rpl", command, schema: uri.fsPath };
  const task = new vscode.Task(
    definition,
    scope,
    `${command}: ${relative}`,
    "rpl",
    new vscode.ProcessExecution(binaryPath, commandArgs, { cwd: folder ? folder.uri.fsPath : path.dirname(uri.fsPath) })
  );
  if (command === "generate") task.group = vscode.TaskGroup.Build;
  return task;
}

function symbolAtMatch(document, match, name, kind) {
  if (!match || typeof match.index !== "number") return null;
  const range = new vscode.Range(document.positionAt(match.index), document.positionAt(match.index + match[0].length));
  return new vscode.DocumentSymbol(name, "", kind, range, range);
}

function provideRplCodeLenses(document) {
  const lenses = [];
  for (const model of parseRplDocument(document.getText()).models) {
    const position = document.positionAt(model.nameStart);
    const range = new vscode.Range(position, position);
    lenses.push(new vscode.CodeLens(range, {
      command: "rpl.compilePackage",
      title: "$(play) Generate package",
      arguments: [document.uri]
    }));
    lenses.push(new vscode.CodeLens(range, {
      command: "rpl.checkDocument",
      title: "$(check) Check schema",
      arguments: [document.uri]
    }));
    lenses.push(new vscode.CodeLens(range, {
      command: "rpl.generateDocs",
      title: "$(book) Docs",
      arguments: [document.uri]
    }));
  }
  return lenses;
}

function provideRplSemanticTokens(document) {
  const builder = new vscode.SemanticTokensBuilder();
  for (const model of parseRplDocument(document.getText()).models) {
    const modelStart = document.positionAt(model.nameStart);
    builder.push(modelStart.line, modelStart.character, model.name.length, 0, 0);
    for (const field of model.fields) {
      const fieldStart = document.positionAt(field.start);
      builder.push(fieldStart.line, fieldStart.character, field.name.length, 1, 0);
    }
  }
  return builder.build();
}

async function resolveRplDocument(uri) {
  if (uri && uri.uri) uri = uri.uri;
  if (uri && uri.scheme) {
    const document = await vscode.workspace.openTextDocument(uri);
    return document.languageId === LANGUAGE_ID ? document : null;
  }
  const editor = vscode.window.activeTextEditor;
  return editor && editor.document.languageId === LANGUAGE_ID ? editor.document : null;
}

function documentationPathFor(schemaPath) {
  const schemaDir = path.dirname(schemaPath);
  return path.basename(schemaDir) === "src"
    ? path.join(path.dirname(schemaDir), "README.md")
    : path.join(schemaDir, "README.md");
}

function appendCommandResult(output, binaryPath, args, result) {
  output.appendLine(`$ ${binaryPath} ${args.join(" ")}`);
  if (result && result.stdout) output.appendLine(result.stdout.trim());
  if (result && result.stderr) output.appendLine(result.stderr.trim());
}

function errorMessage(error) {
  return String(error && error.message ? error.message : error);
}

function stripAnsi(value) {
  return String(value || "").replace(/[\u001b\u009b][[\]()#;?]*(?:(?:(?:[a-zA-Z\d]*(?:;[-a-zA-Z\d\/#&.:=?%@~_]+)*)?\u0007)|(?:(?:\d{1,4}(?:[;:]\d{0,4})*)?[\dA-PR-TZcf-nq-uy=><~]))/g, "");
}

async function resolveCompileOutputDir(context, document, force = false) {
  const all = context.workspaceState.get(COMPILE_OUTPUT_STATE_KEY, {});
  const key = document.uri.toString();
  const existing = typeof all[key] === "string" ? all[key].trim() : "";
  if (existing && !force) {
    return existing;
  }

  const schemaDir = document.uri.scheme === "file" ? path.dirname(document.uri.fsPath) : "";
  const suggested = "models";
  const value = await vscode.window.showInputBox({
    title: "RPL Compile Package",
    prompt: "Куда сохранить generated package для этого файла",
    placeHolder: "Например: models или src/models",
    value: existing || suggested,
    ignoreFocusOut: true,
    validateInput(input) {
      return String(input || "").trim() === "" ? "Нужна папка вывода." : null;
    }
  });
  if (!value) {
    return "";
  }

  const trimmed = value.trim();
  const resolved = path.isAbsolute(trimmed)
    ? trimmed
    : path.resolve(schemaDir, trimmed);

  all[key] = resolved;
  await context.workspaceState.update(COMPILE_OUTPUT_STATE_KEY, all);
  return resolved;
}

function execRpl(binaryPath, args, cwd) {
  return new Promise((resolve, reject) => {
    childProcess.execFile(binaryPath, args, { cwd }, (error, stdout, stderr) => {
      if (error) {
        const detail = [stderr, stdout].filter(Boolean).join("\n").trim();
        if (detail) {
          reject(new Error(detail));
          return;
        }
        reject(error);
        return;
      }

      resolve({
        stdout: String(stdout || ""),
        stderr: String(stderr || "")
      });
    });
  });
}

function workspaceRootFor(document) {
  if (!document) {
    return undefined;
  }
  const folder = vscode.workspace.getWorkspaceFolder(document.uri);
  return folder ? folder.uri.fsPath : undefined;
}

function currentDocumentPath() {
  const editor = vscode.window.activeTextEditor;
  if (!editor || !editor.document || editor.document.uri.scheme !== "file") {
    return "";
  }
  return editor.document.uri.fsPath;
}

function documentPath(document) {
  if (!document || !document.uri || document.uri.scheme !== "file") {
    return currentDocumentPath();
  }
  return document.uri.fsPath;
}

function normalizeTargetLanguage(value) {
  const lang = String(value || "").trim().toLowerCase();
  if (!lang || lang === "go" || lang === "golang") {
    return "golang";
  }
  return lang;
}

function targetLanguageForDocument(document) {
  const text = document && typeof document.getText === "function" ? document.getText() : "";
  const match = text.match(/\btarget\s*\((?:[^()]|\([^)]*\))*?\blang\s*:\s*([A-Za-z0-9_+\-]+)\b/);
  return normalizeTargetLanguage(match ? match[1] : "");
}

function normalizeTargetType(item) {
  if (!item) {
    return null;
  }

  const name = String(item.name || item.insert || "").trim();
  if (!name) {
    return null;
  }

  return {
    name,
    insert: String(item.insert || name).trim(),
    category: String(item.category || "").trim(),
    help: String(item.help || "").trim(),
    aliases: Array.isArray(item.aliases) ? item.aliases.map((entry) => String(entry || "").trim()).filter(Boolean) : []
  };
}

function normalizeTargetStructure(item) {
  if (!item) {
    return null;
  }

  const label = String(item.label || item.insert || "").trim();
  const insert = String(item.insert || "").trim();
  if (!label || !insert) {
    return null;
  }

  return {
    label,
    insert,
    category: String(item.category || "").trim(),
    help: String(item.help || "").trim()
  };
}

function normalizeTargetCatalog(rawCatalog, fallbackLang) {
  const catalog = rawCatalog && typeof rawCatalog === "object" ? rawCatalog : {};
  const lang = normalizeTargetLanguage(catalog.lang || fallbackLang);
  const fallback = FALLBACK_TARGET_CATALOGS[lang] || FALLBACK_TARGET_CATALOGS.golang;

  const types = [];
  const seenTypes = new Set();
  for (const source of [fallback.types, catalog.types]) {
    for (const item of Array.isArray(source) ? source : []) {
      const normalized = normalizeTargetType(item);
      if (!normalized) {
        continue;
      }
      if (seenTypes.has(normalized.name)) {
        continue;
      }
      seenTypes.add(normalized.name);
      types.push(normalized);
    }
  }

  const structures = [];
  const seenStructures = new Set();
  for (const source of [fallback.structures, catalog.structures]) {
    for (const item of Array.isArray(source) ? source : []) {
      const normalized = normalizeTargetStructure(item);
      if (!normalized) {
        continue;
      }
      const key = `${normalized.label}\u0000${normalized.insert}`;
      if (seenStructures.has(key)) {
        continue;
      }
      seenStructures.add(key);
      structures.push(normalized);
    }
  }

  return {
    lang,
    label: String(catalog.label || fallback.label || lang).trim(),
    help: String(catalog.help || fallback.help || "").trim(),
    types,
    structures
  };
}

function fallbackTargetCatalog(lang) {
  return normalizeTargetCatalog(FALLBACK_TARGET_CATALOGS[normalizeTargetLanguage(lang)], lang);
}

async function buildQuickFixes(document, context, catalog) {
  const actions = [];
  const diagnostics = Array.isArray(context && context.diagnostics) ? context.diagnostics : [];

  for (const diagnostic of diagnostics) {
    const message = String(diagnostic.message || "");

    const missingNamespace = message.match(/attr namespace "([^"]+)" is not declared in attrs \(\)/i);
    if (missingNamespace) {
      const namespace = missingNamespace[1];
      const owner = await findAttrOwner(catalog, namespace);
      if (owner) {
        const action = new vscode.CodeAction(`Add "${owner}" to attrs (...)`, vscode.CodeActionKind.QuickFix);
        action.diagnostics = [diagnostic];
        action.command = {
          command: "rpl.addAttrDeclaration",
          title: `Add "${owner}" to attrs (...)`,
          arguments: [document.uri, owner]
        };
        actions.push(action);
      }

      if (!hasAttrsBlock(document.getText())) {
        const action = new vscode.CodeAction("Insert attrs (...) block", vscode.CodeActionKind.QuickFix);
        action.diagnostics = [diagnostic];
        action.command = {
          command: "rpl.insertAttrsBlockQuickFix",
          title: "Insert attrs (...) block",
          arguments: [document.uri]
        };
        actions.push(action);
      }
    }

    if (isImportRelatedDiagnostic(message)) {
      const action = new vscode.CodeAction("Run RPL: Auto Set Imports", vscode.CodeActionKind.QuickFix);
      action.diagnostics = [diagnostic];
      action.command = {
        command: "rpl.autoSetImports",
        title: "RPL: Auto Set Imports"
      };
      actions.push(action);
    }
  }

  return actions;
}

async function findAttrOwner(catalog, namespace) {
  const trimmed = String(namespace || "").trim();
  if (!trimmed) {
    return "";
  }

  const items = await catalog.load();
  for (const item of items) {
    const manifest = item && item.Manifest ? item.Manifest : {};
    const author = String(manifest.Author || "").trim();
    const name = String(manifest.Name || "").trim();
    if (!author || !name) {
      continue;
    }

    if (name === trimmed) {
      return `${author}:${name}`;
    }

    const specs = Array.isArray(item && item.specs) ? item.specs : [];
    for (const spec of specs) {
      if (String(spec && spec.namespace || "").trim() === trimmed) {
        return `${author}:${name}`;
      }
    }
  }

  return "";
}

function isImportRelatedDiagnostic(message) {
  const text = String(message || "").toLowerCase();
  return text.includes("unresolved imported type")
    || text.includes("неразрешимый импортированный тип")
    || text.includes("неразрешимый внешний go-тип")
    || text.includes("uses an unresolved external go type")
    || text.includes("check the alias in `import (...)`")
    || text.includes("проверьте alias в `import (...)`");
}

function hasAttrsBlock(text) {
  return findBlockSpan(text, "attrs") !== null;
}

function ensureAttrBlockText(text) {
  if (hasAttrsBlock(text)) {
    return text;
  }

  const block = "attrs (\n\t\n)\n\n";
  const targetMatch = text.match(/target\s*\([^)]*\)\s*/);
  if (!targetMatch || typeof targetMatch.index !== "number") {
    return block + text;
  }

  let offset = targetMatch.index + targetMatch[0].length;
  let prefix = "";
  if (text.slice(offset, offset + 2) !== "\n\n") {
    prefix = "\n\n";
  }
  return text.slice(0, offset) + prefix + block + text.slice(offset);
}

function ensureAttrDeclaration(text, runtimeId) {
  const runtimeLiteral = JSON.stringify(String(runtimeId).trim());
  if (!runtimeLiteral || runtimeLiteral === "\"\"") {
    return text;
  }

  const withBlock = ensureAttrBlockText(text);
  const span = findBlockSpan(withBlock, "attrs");
  if (!span) {
    return withBlock;
  }

  const inner = withBlock.slice(span.innerStart, span.innerEnd);
  const existing = new Set();
  const pattern = /"([^"]+)"/g;
  let match;
  while ((match = pattern.exec(inner)) !== null) {
    existing.add(match[1]);
  }
  if (existing.has(JSON.parse(runtimeLiteral))) {
    return withBlock;
  }

  const values = Array.from(existing);
  values.push(JSON.parse(runtimeLiteral));
  values.sort((left, right) => left.localeCompare(right));

  const rendered = values.length === 0
    ? "\n\t\n"
    : "\n" + values.map((value) => `\t${JSON.stringify(value)}`).join("\n") + "\n";

  return withBlock.slice(0, span.innerStart) + rendered + withBlock.slice(span.innerEnd);
}

function findBlockSpan(text, keyword) {
  const matcher = new RegExp(`\\b${keyword}\\s*\\(`, "g");
  const match = matcher.exec(text);
  if (!match) {
    return null;
  }

  const openIndex = text.indexOf("(", match.index);
  if (openIndex < 0) {
    return null;
  }

  let depth = 0;
  for (let index = openIndex; index < text.length; index += 1) {
    const char = text[index];
    if (char === "(") {
      depth += 1;
      continue;
    }
    if (char === ")") {
      depth -= 1;
      if (depth === 0) {
        return {
          start: match.index,
          innerStart: openIndex + 1,
          innerEnd: index,
          end: index + 1
        };
      }
    }
  }

  return null;
}

async function provideFormattingEdits(document, client, output) {
  return provideSaveEdits(document, client, output, {
    autoImport: false,
    format: true
  });
}

async function provideSaveEdits(document, client, output, options) {
  try {
    const filePath = document.uri.scheme === "file" ? document.uri.fsPath : "";
    let code = document.getText();

    if (options && options.autoImport) {
      const autoImportResponse = await client.send("auto.set.import", {
        code,
        path: filePath
      });
      if (autoImportResponse && typeof autoImportResponse.code === "string") {
        code = autoImportResponse.code;
      }
    }

    if (options && options.format) {
      const formatResponse = await client.send("format", {
        code,
        path: filePath
      });
      if (formatResponse && typeof formatResponse.code === "string") {
        code = formatResponse.code;
      }
    }

    if (code === document.getText()) {
      return [];
    }

    const endLine = Math.max(0, document.lineCount - 1);
    const endChar = document.lineCount > 0 ? document.lineAt(endLine).text.length : 0;
    const fullRange = new vscode.Range(new vscode.Position(0, 0), new vscode.Position(endLine, endChar));
    return [vscode.TextEdit.replace(fullRange, code)];
  } catch (error) {
    output.appendLine(String(error && error.message ? error.message : error));
    return [];
  }
}

async function replaceDocumentText(editor, text) {
  const document = editor.document;
  const endLine = Math.max(0, document.lineCount - 1);
  const endChar = document.lineCount > 0 ? document.lineAt(endLine).text.length : 0;
  const fullRange = new vscode.Range(new vscode.Position(0, 0), new vscode.Position(endLine, endChar));
  await editor.edit((editBuilder) => {
    editBuilder.replace(fullRange, text);
  });
}

async function insertTopLevelBlock(editor, block, existsPattern) {
  const document = editor.document;
  if (existsPattern.test(document.getText())) {
    return;
  }

  const text = document.getText();
  const targetMatch = text.match(/target\s*\([^)]*\)\s*/);
  let offset = 0;
  if (targetMatch && typeof targetMatch.index === "number") {
    offset = targetMatch.index + targetMatch[0].length;
    if (text.slice(offset, offset + 2) !== "\n\n") {
      block = "\n\n" + block;
    }
  }

  const position = document.positionAt(offset);
  await editor.edit((editBuilder) => {
    editBuilder.insert(position, block);
  });
}

async function insertAtCursor(editor, text) {
  const position = editor.selection.active;
  await editor.edit((editBuilder) => {
    editBuilder.insert(position, text);
  });
}

async function validateDocument(document, client, diagnostics, output, status) {
  try {
    const response = await client.send("check", {
      code: document.getText(),
      path: document.uri.scheme === "file" ? document.uri.fsPath : ""
    });

    const items = Array.isArray(response && response.diagnostics) ? response.diagnostics : [];
    if (items.length === 0 && response && response.ok === false) {
      diagnostics.set(document.uri, [runtimeDiagnostic(document, "RPL не смог проверить файл.", "Компилятор вернул неуспешный ответ без списка diagnostics.")]);
      if (status) status.setProblems(1, "Компилятор вернул ошибку без diagnostics");
      return { count: 1, runtimeError: false };
    }

    const merged = mergeDiagnostics(document, items);
    diagnostics.set(document.uri, merged);
    if (status && vscode.window.activeTextEditor && vscode.window.activeTextEditor.document.uri.toString() === document.uri.toString()) {
      status.setProblems(merged.length, merged.length === 0 ? "Схема проверена компилятором RPL" : `Проблем в схеме: ${merged.length}`);
    }
    return { count: merged.length, runtimeError: false };
  } catch (error) {
    const message = String(error && error.message ? error.message : error);
    output.appendLine(message);
    diagnostics.set(document.uri, [runtimeDiagnostic(
      document,
      "RPL runtime недоступен, поэтому проверка схемы не выполнена.",
      `${message}\nПроверь настройку rpl.binaryPath и команду \`rpl runtime\`.`
    )]);
    if (status) status.setError(message);
    return { count: 1, runtimeError: true };
  }
}

function mergeDiagnostics(document, items) {
  const groups = new Map();

  for (const item of items) {
    const diagnostic = toDiagnostic(document, item);
    const key = [
      diagnostic.range.start.line,
      diagnostic.range.start.character,
      diagnostic.range.end.line,
      diagnostic.range.end.character,
      diagnostic.severity
    ].join(":");

    const current = groups.get(key);
    if (!current) {
      groups.set(key, {
        diagnostic,
        messages: [diagnostic.message]
      });
      continue;
    }

    current.messages.push(diagnostic.message);
  }

  return Array.from(groups.values()).map((group) => {
    if (group.messages.length <= 1) {
      return group.diagnostic;
    }

    group.diagnostic.message = `RPL нашёл несколько проблем:\n- ${group.messages.join("\n- ")}`;
    return group.diagnostic;
  });
}

function runtimeDiagnostic(document, message, detail) {
  const range = new vscode.Range(new vscode.Position(0, 0), new vscode.Position(0, 1));
  const diagnostic = new vscode.Diagnostic(
    range,
    detail ? `${message}\n${detail}` : message,
    vscode.DiagnosticSeverity.Error
  );
  diagnostic.source = "rpl";
  return diagnostic;
}

function toDiagnostic(document, item) {
  const location = resolveDiagnosticLocation(document, item);
  const safeLine = location.line;
  const lineText = document.lineCount > 0 ? document.lineAt(safeLine).text : "";
  const safeColumn = Math.min(location.column, lineText.length);
  const endColumn = Math.min(lineText.length, Math.max(safeColumn + 1, inferTokenEnd(lineText, safeColumn)));
  const range = new vscode.Range(safeLine, safeColumn, safeLine, endColumn);

  let message = String(item.message || "RPL error");
  if (item.hint) {
    message += `\nHint: ${item.hint}`;
  }
  if (item.detail) {
    message += `\n${item.detail}`;
  }

  const diagnostic = new vscode.Diagnostic(range, message, vscode.DiagnosticSeverity.Error);
  diagnostic.source = "rpl";
  return diagnostic;
}

function resolveDiagnosticLocation(document, item) {
  const explicitLine = Number(item.line || 0);
  const explicitColumn = Number(item.column || 0);
  if (explicitLine > 0) {
    const zeroLine = Math.max(0, explicitLine - 1);
    const safeLine = Math.min(zeroLine, Math.max(0, document.lineCount - 1));
    const zeroColumn = Math.max(0, explicitColumn - 1);
    return { line: safeLine, column: zeroColumn };
  }

  const message = String(item && item.message ? item.message : "");
  const missingAttrMatch = message.match(/attr "([^"]+)" by author "([^"]+)"/i);
  if (missingAttrMatch) {
    const fullId = `"${missingAttrMatch[2]}:${missingAttrMatch[1]}"`;
    const text = document.getText();
    const offset = text.indexOf(fullId);
    if (offset >= 0) {
      const position = document.positionAt(offset + 1);
      return { line: position.line, column: position.character };
    }
  }

  const attrsMatch = document.getText().match(/\battrs\s*\(/);
  if (attrsMatch && typeof attrsMatch.index === "number") {
    const position = document.positionAt(attrsMatch.index);
    return { line: position.line, column: position.character };
  }

  return { line: 0, column: 0 };
}

function inferTokenEnd(lineText, column) {
  if (column >= lineText.length) {
    return column + 1;
  }

  let cursor = column;
  while (cursor < lineText.length) {
    const char = lineText[cursor];
    if (/\s|[(){}[\],]/.test(char)) {
      break;
    }
    cursor += 1;
  }

  return cursor > column ? cursor : column + 1;
}

async function buildCompletions(document, position, catalog) {
  const linePrefix = document.lineAt(position.line).text.slice(0, position.character);
  const items = [];

  const attrArgContext = getAttrArgContext(document, position);
  if (attrArgContext) {
    const spec = await catalog.findSpec(attrArgContext.namespace);
    if (spec) {
      return buildAttrArgCompletions(spec, document, position, attrArgContext);
    }
  }

  if (/@[\w.]*$/.test(linePrefix)) {
    return buildAttrCompletions(await catalog.attrSpecs(), document, position);
  }

  if (/\battrs\s*\([^)]*$/.test(document.getText(new vscode.Range(new vscode.Position(0, 0), position)))) {
    const runtimeReplaceRange = getRuntimeIdReplaceRange(document, position);
    for (const runtimeId of await catalog.runtimeIds()) {
      const item = new vscode.CompletionItem(runtimeId, vscode.CompletionItemKind.Value);
      item.insertText = runtimeId;
      item.range = runtimeReplaceRange;
      items.push(item);
    }
  }

  for (const keyword of KEYWORDS) {
    items.push(new vscode.CompletionItem(keyword, vscode.CompletionItemKind.Keyword));
  }

  const targetCatalog = await catalog.loadTypeCatalog(document);
  for (const typeSpec of Array.isArray(targetCatalog && targetCatalog.types) ? targetCatalog.types : []) {
    items.push(buildTargetTypeCompletion(typeSpec, targetCatalog));
  }

  for (const typeName of collectModelNames(document)) {
    items.push(new vscode.CompletionItem(typeName, vscode.CompletionItemKind.Class));
  }

  items.push(modelSnippet());
  items.push(attrsSnippet());
  items.push(targetSnippet());
  items.push(...buildTargetStructureCompletions(targetCatalog));

  return items;
}

function buildTargetTypeCompletion(typeSpec, catalog) {
  const item = new vscode.CompletionItem(typeSpec.name, vscode.CompletionItemKind.Class);
  item.insertText = typeSpec.insert || typeSpec.name;
  item.sortText = `10-type-${typeSpec.name}`;
  const detail = [];
  if (catalog && catalog.label) {
    detail.push(catalog.label);
  }
  if (typeSpec.category) {
    detail.push(typeSpec.category);
  }
  item.detail = detail.join(" • ");
  item.documentation = buildTargetTypeMarkdown(typeSpec, catalog);
  return item;
}

function buildTargetStructureCompletions(catalog) {
  const items = [];
  for (const structure of Array.isArray(catalog && catalog.structures) ? catalog.structures : []) {
    const item = new vscode.CompletionItem(
      {
        label: structure.label,
        description: structure.category || (catalog && catalog.label ? catalog.label : "")
      },
      vscode.CompletionItemKind.Snippet
    );
    item.insertText = new vscode.SnippetString(structure.insert);
    item.sortText = `20-structure-${structure.label}`;
    const detail = [];
    if (catalog && catalog.label) {
      detail.push(catalog.label);
    }
    if (structure.category) {
      detail.push(structure.category);
    }
    item.detail = detail.join(" • ");
    item.documentation = buildTargetStructureMarkdown(structure, catalog);
    items.push(item);
  }
  return items;
}

function buildAttrCompletions(specs, document, position) {
  const items = [];
  const seen = new Set();
  const replaceRange = getAttrReplaceRange(document, position);

  for (const spec of specs) {
    const snippets = Array.isArray(spec.snippets) && spec.snippets.length > 0
      ? spec.snippets
      : [{ label: `@${spec.namespace}`, insert: `@${spec.namespace}`, help: spec.help || "" }];

    for (const snippetSpec of snippets) {
      const snippet = normalizeAttrInsertText(document, replaceRange, String(snippetSpec.insert || `@${spec.namespace}`));
      const label = String(snippetSpec.label || snippet);
      const key = `${spec.namespace}|${label}|${snippet}`;
      if (seen.has(key)) {
        continue;
      }
      seen.add(key);

      const item = new vscode.CompletionItem({
        label,
        description: formatSpecSource(spec)
      }, vscode.CompletionItemKind.Function);
      item.insertText = snippet;
      item.range = replaceRange;
      item.filterText = label;
      item.sortText = `00-${spec.namespace}-${label}`;
      item.detail = buildCompletionDetail(spec);
      item.documentation = buildSpecMarkdown(spec);
      if (snippetSpec.help) {
        item.documentation = buildSpecMarkdown(spec, snippetSpec);
      }
      items.push(item);
    }
  }

  return items;
}

function getAttrReplaceRange(document, position) {
  const line = document.lineAt(position.line).text;
  const prefix = line.slice(0, position.character);
  const suffix = line.slice(position.character);
  const leftMatch = prefix.match(/@+[\w.]*$/);
  if (!leftMatch) {
    return new vscode.Range(position, position);
  }

  const start = position.character - leftMatch[0].length;
  const rightMatch = suffix.match(/^[\w.]*/);
  const rightLength = rightMatch ? rightMatch[0].length : 0;
  return new vscode.Range(position.line, start, position.line, position.character + rightLength);
}

function normalizeAttrInsertText(document, replaceRange, snippet) {
  const existing = document.getText(replaceRange);
  const existingPrefix = existing.match(/^@+/);
  const snippetPrefix = snippet.match(/^@+/);
  if (!snippetPrefix) {
    return snippet;
  }

  const existingCount = existingPrefix ? existingPrefix[0].length : 0;
  const snippetCount = snippetPrefix[0].length;
  const preservedCount = existingCount > 0 ? existingCount : snippetCount;
  return "@".repeat(Math.max(1, preservedCount)) + snippet.slice(snippetCount);
}

function getRuntimeIdReplaceRange(document, position) {
  const line = document.lineAt(position.line).text;
  const prefix = line.slice(0, position.character);
  const suffix = line.slice(position.character);

  const leftMatch = prefix.match(/"[^"]*$/);
  if (!leftMatch) {
    return new vscode.Range(position, position);
  }

  const start = position.character - leftMatch[0].length;
  const rightMatch = suffix.match(/^[^"]*"?/);
  const end = position.character + (rightMatch ? rightMatch[0].length : 0);
  return new vscode.Range(position.line, start, position.line, end);
}

function buildAttrArgCompletions(spec, document, position, context) {
  const items = [];
  const args = Array.isArray(spec.args) ? spec.args : [];
  const replaceRange = context.argRange || new vscode.Range(position, position);

  for (const arg of args) {
    if (!arg || !arg.name) {
      continue;
    }
    const positional = isPositionalArg(spec, arg);

    const item = new vscode.CompletionItem(
      buildArgCompletionLabel(spec, arg),
      positional ? vscode.CompletionItemKind.Value : vscode.CompletionItemKind.Property
    );
    item.insertText = buildArgInsertText(spec, arg);
    item.range = replaceRange;
    item.filterText = arg.name;
    item.sortText = `00-${positional ? "value" : arg.name}`;
    item.detail = `${formatSpecSource(spec) || spec.namespace} • ${positional ? "позиционный" : "именованный"} аргумент @${spec.namespace}`;
    item.documentation = buildArgMarkdown(spec, arg);
    items.push(item);
  }

  return items;
}

function buildCompletionDetail(spec) {
  const source = formatSpecSource(spec) || "RPL attr";
  const args = Array.isArray(spec.args) ? spec.args.map((item) => item.name).filter(Boolean) : [];
  const capabilities = formatCapabilities(spec.capabilities);
  if (args.length === 0) {
    return capabilities ? `${source} • без аргументов • ${capabilities}` : `${source} • без аргументов`;
  }
  return capabilities
    ? `${source} • аргументы: ${args.join(", ")} • ${capabilities}`
    : `${source} • аргументы: ${args.join(", ")}`;
}

function buildSpecMarkdown(spec, snippetSpec = null) {
  const markdown = new vscode.MarkdownString();
  markdown.isTrusted = false;

  markdown.appendMarkdown(`**@${spec.namespace}**`);
  const source = formatSpecSource(spec);
  if (source) {
    markdown.appendMarkdown(`  \nИсточник: \`${source}\``);
  }
  if (spec.help) {
    markdown.appendMarkdown(`\n\n${escapeMarkdown(spec.help)}`);
  }
  if (snippetSpec && snippetSpec.help) {
    markdown.appendMarkdown(`\n\n**Вариант**  \n${escapeMarkdown(snippetSpec.help)}`);
  }
  const capabilities = formatCapabilities(spec.capabilities);
  if (capabilities) {
    markdown.appendMarkdown(`\n\n**Возможности**  \n${escapeMarkdown(capabilities)}`);
  }

  const args = Array.isArray(spec.args) ? spec.args : [];
  if (args.length > 0) {
    markdown.appendMarkdown("\n\n**Допустимые аргументы**");
    for (const arg of args) {
      markdown.appendMarkdown(`\n- \`${arg.name}\``);
      const types = Array.isArray(arg.types) && arg.types.length > 0 ? arg.types.join(", ") : "any";
      markdown.appendMarkdown(`: ${escapeMarkdown(types)}`);
      if (arg.help) {
        markdown.appendMarkdown(`  \n  ${escapeMarkdown(arg.help)}`);
      }
      if (Array.isArray(arg.aliases) && arg.aliases.length > 0) {
        markdown.appendMarkdown(`  \n  aliases: \`${arg.aliases.join("`, `")}\``);
      }
    }
  }

  return markdown;
}

function buildTargetTypeMarkdown(typeSpec, catalog) {
  const markdown = new vscode.MarkdownString();
  markdown.isTrusted = false;

  markdown.appendMarkdown(`**${escapeMarkdown(typeSpec.name)}**`);
  if (catalog && catalog.label) {
    markdown.appendMarkdown(`  \nTarget: \`${escapeMarkdown(catalog.label)}\``);
  }
  if (typeSpec.category) {
    markdown.appendMarkdown(`  \nCategory: \`${escapeMarkdown(typeSpec.category)}\``);
  }
  if (typeSpec.help) {
    markdown.appendMarkdown(`\n\n${escapeMarkdown(typeSpec.help)}`);
  }
  if (typeSpec.insert && typeSpec.insert !== typeSpec.name) {
    markdown.appendCodeblock(typeSpec.insert, "rpl");
  }

  return markdown;
}

function buildTargetStructureMarkdown(structure, catalog) {
  const markdown = new vscode.MarkdownString();
  markdown.isTrusted = false;

  markdown.appendMarkdown(`**${escapeMarkdown(structure.label)}**`);
  if (catalog && catalog.label) {
    markdown.appendMarkdown(`  \nTarget: \`${escapeMarkdown(catalog.label)}\``);
  }
  if (structure.category) {
    markdown.appendMarkdown(`  \nCategory: \`${escapeMarkdown(structure.category)}\``);
  }
  if (structure.help) {
    markdown.appendMarkdown(`\n\n${escapeMarkdown(structure.help)}`);
  }
  markdown.appendCodeblock(structure.insert, "rpl");

  return markdown;
}

function buildArgMarkdown(spec, arg) {
  const markdown = new vscode.MarkdownString();
  markdown.isTrusted = false;
  const positional = isPositionalArg(spec, arg);

  markdown.appendMarkdown(`**${positional ? "значение" : arg.name}** для \`@${spec.namespace}\``);
  const source = formatSpecSource(spec);
  if (source) {
    markdown.appendMarkdown(`  \nИсточник: \`${source}\``);
  }

  const types = formatArgTypes(arg);
  if (types) {
    markdown.appendMarkdown(`\n\n**Принимает**  \n${escapeMarkdown(types)}`);
  }
  markdown.appendMarkdown(`\n\n**Форма**  \n${positional ? "позиционный аргумент" : "именованный аргумент"}`);
  if (arg.help) {
    markdown.appendMarkdown(`\n\n${escapeMarkdown(arg.help)}`);
  }
  if (Array.isArray(arg.aliases) && arg.aliases.length > 0) {
    markdown.appendMarkdown(`\n\n**Алиасы**  \n\`${arg.aliases.join("`, `")}\``);
  }

  return markdown;
}

function formatSpecSource(spec) {
  if (Array.isArray(spec && spec.sources) && spec.sources.length > 0) {
    return spec.sources.join(", ");
  }
  if (!spec || !spec.manifest) {
    return "";
  }
  const author = String(spec.manifest.Author || "").trim();
  const name = String(spec.manifest.Name || "").trim();
  if (!author || !name) {
    return "";
  }
  return `${author}:${name}`;
}

function formatCapabilities(capabilities) {
  if (!capabilities || typeof capabilities !== "object") {
    return "";
  }
  const labels = [];
  if (capabilities.analyze_model) labels.push("analyze:model");
  if (capabilities.analyze_file) labels.push("analyze:file");
  if (capabilities.generate_model) labels.push("generate:model");
  if (capabilities.generate_file) labels.push("generate:file");
  if (capabilities.docs_model) labels.push("docs:model");
  if (capabilities.docs_file) labels.push("docs:file");
  return labels.join(", ");
}

function formatArgTypes(arg) {
  const types = Array.isArray(arg && arg.types) ? arg.types : [];
  if (types.length === 0) {
    return "any";
  }
  return types.join(", ");
}

function buildArgCompletionLabel(spec, arg) {
  if (!arg || !isPositionalArg(spec, arg)) {
    return {
      label: arg.name,
      description: formatArgTypes(arg)
    };
  }

  const types = Array.isArray(arg.types) ? arg.types : [];
  let label = "value";
  if (types.includes("string-like") || types.includes("string") || types.includes("name")) {
    label = "text";
  } else if (types.includes("number")) {
    label = "number";
  } else if (types.includes("bool")) {
    label = "bool";
  }

  return {
    label,
    detail: formatArgTypes(arg),
    description: "позиционный"
  };
}

function buildArgInsertText(spec, arg) {
  if (!arg || !isPositionalArg(spec, arg)) {
    return `${arg.name}: `;
  }

  const types = Array.isArray(arg.types) ? arg.types : [];
  if (types.includes("bool")) {
    return new vscode.SnippetString("${1|true,false|}");
  }
  if (types.includes("number")) {
    return new vscode.SnippetString("${1:0}");
  }
  return new vscode.SnippetString('"${1:value}"');
}

function isPositionalArg(spec, arg) {
  if (!arg) {
    return false;
  }
  if (arg.positional) {
    return true;
  }

  const args = Array.isArray(spec && spec.args) ? spec.args : [];
  return args.length === 1 && String(arg.name || "").trim() === "value";
}

function isAttrArgContext(document, position) {
  return getAttrArgContext(document, position) !== null;
}

function getAttrArgContext(document, position) {
  const linePrefix = document.lineAt(position.line).text.slice(0, position.character);
  const match = linePrefix.match(/@([A-Za-z_][A-Za-z0-9_.]*)\(([^()]*)$/);
  if (!match) {
    return null;
  }

  const namespace = String(match[1] || "").split(".")[0];
  if (!namespace) {
    return null;
  }

  const argsPrefix = String(match[2] || "");
  const rawArg = argsPrefix.split(",").pop() || "";
  const argMatch = rawArg.match(/([A-Za-z_][A-Za-z0-9_]*)?$/);
  const typedArg = argMatch ? (argMatch[1] || "") : "";
  const startCharacter = position.character - typedArg.length;

  return {
    namespace,
    typedArg,
    argRange: new vscode.Range(position.line, startCharacter, position.line, position.character)
  };
}

async function provideAttrHover(document, position, catalog) {
  const range = document.getWordRangeAtPosition(position, /@?[A-Za-z_][A-Za-z0-9_.]*/);
  if (!range) {
    return null;
  }

  const text = document.getText(range);
  if (!text.startsWith("@")) {
    return null;
  }

  const namespace = text.slice(1).split(".")[0];
  const spec = await catalog.findSpec(namespace);
  if (!spec) {
    return null;
  }

  return new vscode.Hover(buildSpecMarkdown(spec), range);
}

function escapeMarkdown(value) {
  return String(value || "").replace(/[\\`*_{}[\]()#+\-.!]/g, "\\$&");
}

function modelSnippet() {
  const item = new vscode.CompletionItem("model", vscode.CompletionItemKind.Snippet);
  item.insertText = new vscode.SnippetString("model ${1:User} {\n\t${2:Name string}\n}");
  item.detail = "Model block";
  return item;
}

function attrsSnippet() {
  const item = new vscode.CompletionItem("attrs block", vscode.CompletionItemKind.Snippet);
  item.insertText = new vscode.SnippetString("attrs (\n\t\"rpl:std\",\n\t\"rpl:${1:grpc}\",\n)");
  item.detail = "Attrs declaration block";
  return item;
}

function targetSnippet() {
  const item = new vscode.CompletionItem("target", vscode.CompletionItemKind.Snippet);
  item.insertText = new vscode.SnippetString("target(lang: golang)");
  item.detail = "Target language declaration";
  return item;
}

function collectModelNames(document) {
  const names = new Set();
  const pattern = /\bmodel\s+([A-Z][A-Za-z0-9_]*)\b/g;
  const text = document.getText();
  let match = pattern.exec(text);
  while (match) {
    names.add(match[1]);
    match = pattern.exec(text);
  }

  return Array.from(names.values()).sort();
}

function deactivate() {}

module.exports = {
  activate,
  deactivate
};
