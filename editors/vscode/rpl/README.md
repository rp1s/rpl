# RPL Language Support

Полная IDE-интеграция RPL для VS Code: от редактирования схемы до генерации package.

## UX и навигация

- панель **RPL** в Explorer показывает схемы workspace, установленные attrs и состояние toolchain;
- CodeLens над каждой моделью: **Generate package**, **Check schema**, **Docs**;
- Outline и `Go to Symbol` видят package, target, модели и поля;
- кнопки Generate и Check доступны в заголовке редактора;
- статус RPL показывает текущую проверку, число ошибок и недоступный runtime;
- клик по статусу открывает единый центр действий;
- VS Code Tasks автоматически получает задачи Generate, Docs и Format для схем.

## Языковые возможности

- подсветка `.rpl`, snippets и автодополнение ключевых слов, моделей и target-типов;
- динамический каталог attrs из установленного RPL;
- completion и hover по `AttrSpec`: аргументы, типы, help и capabilities;
- живая диагностика настоящего parser/compiler RPL;
- quick fixes для отсутствующих attrs и импортов;
- formatter и автоматическое форматирование при сохранении;
- автоматическая установка `attrs (...)` и Go `import (...)` при сохранении.

## Команды

- `RPL: Show Actions` - центр основных действий;
- `RPL: Compile Package` - `rpl run <schema> out <dir>`;
- `RPL: Check Schema` - немедленная проверка текущей схемы;
- `RPL: Generate Documentation` - `rpl docs <schema>` и открытие README;
- `RPL: Auto Set Imports` - установка attrs и импортов;
- `RPL: Show Toolchain Status` - версия CLI, runtime и количество attrs;
- `RPL: Select CLI Binary` - выбор бинаря без ручного редактирования settings;
- `RPL: Restart Runtime`, `RPL: Refresh Attr Catalog`, `RPL: Open Output`.

## Установка toolchain

Расширению нужен `rpl` в `PATH`. Если он находится в другом месте, выполните
`RPL: Select CLI Binary` или задайте `rpl.binaryPath`.

Расширение держит фоновый процесс `rpl runtime` и использует его действия
`check`, `format`, `auto.set.import`, `attrs.search`, `attrs.get` и
`types.catalog`. Генерация package и README выполняется отдельными CLI-командами.

## Настройки

- `rpl.binaryPath` - путь к RPL CLI;
- `rpl.enableDiagnostics` - живая диагностика;
- `rpl.enableCompletions` - completion provider;
- `rpl.autoSetImportsOnSave` - attrs и imports при сохранении;
- `rpl.formatOnSave` - форматирование при сохранении; выбранная форма attrs
  сохраняется (`Field T @attr(...)` или отдельный `{ ... }` блок);
- `rpl.enableCodeLens` - действия над моделями;
- `rpl.showStatusBar` - состояние RPL в status bar;
- `rpl.enableTaskProvider` - автоматические VS Code Tasks.

## Локальная сборка

```bash
cd editors/vscode/rpl
npm install
npx @vscode/vsce package
```

Установите получившийся `.vsix` командой `Extensions: Install from VSIX...`.
