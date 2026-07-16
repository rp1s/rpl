"use strict";

function parseRplDocument(text) {
  const source = String(text || "");
  const result = {
    packageName: firstCapture(source, /^\s*package\s+([A-Za-z_][A-Za-z0-9_]*)/m),
    target: firstCapture(source, /\btarget\s*\([\s\S]*?\blang\s*:\s*([A-Za-z0-9_+.-]+)/m) || "golang",
    attrs: collectQuotedBlockValues(source, "attrs"),
    imports: collectQuotedBlockValues(source, "import"),
    models: []
  };

  const modelPattern = /(^|\n)([ \t]*)(?:@[A-Za-z_][^\n]*\n[ \t]*)*model\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{/g;
  let match;
  while ((match = modelPattern.exec(source)) !== null) {
    const modelToken = match[0].lastIndexOf("model");
    const start = match.index + modelToken;
    const open = source.indexOf("{", start);
    const close = findMatchingBrace(source, open);
    const end = close >= 0 ? close + 1 : source.length;
    const name = match[3];
    result.models.push({
      name,
      start,
      nameStart: source.indexOf(name, start),
      end,
      fields: collectModelFields(source, open + 1, close >= 0 ? close : source.length)
    });
    if (end > modelPattern.lastIndex) {
      modelPattern.lastIndex = end;
    }
  }

  return result;
}

function collectModelFields(source, start, end) {
  const fields = [];
  const body = source.slice(start, end);
  const linePattern = /(^|\n)([ \t]*)(?:@[A-Za-z_][^\n]*\n[ \t]*)*([A-Z][A-Za-z0-9_]*)\s+([^\s=\n}]+(?:\??))(?:\s*=\s*([^\n]+))?/g;
  let match;
  while ((match = linePattern.exec(body)) !== null) {
    const name = match[3];
    const lineOffset = match.index + match[1].length;
    const nameOffset = body.indexOf(name, lineOffset);
    fields.push({
      name,
      type: match[4],
      defaultValue: String(match[5] || "").trim(),
      start: start + nameOffset,
      end: start + nameOffset + name.length
    });
  }
  return fields;
}

function collectQuotedBlockValues(source, keyword) {
  const pattern = new RegExp(`\\b${keyword}\\s*\\(`, "g");
  const match = pattern.exec(source);
  if (!match) return [];
  const open = source.indexOf("(", match.index);
  const close = findMatchingParen(source, open);
  const body = source.slice(open + 1, close >= 0 ? close : source.length);
  const values = [];
  const quoted = /"((?:\\.|[^"\\])*)"/g;
  let value;
  while ((value = quoted.exec(body)) !== null) {
    values.push(value[1]);
  }
  return values;
}

function findMatchingBrace(source, open) {
  return findMatching(source, open, "{", "}");
}

function findMatchingParen(source, open) {
  return findMatching(source, open, "(", ")");
}

function findMatching(source, open, left, right) {
  if (open < 0) return -1;
  let depth = 0;
  let quote = "";
  let escaped = false;
  let lineComment = false;
  for (let index = open; index < source.length; index += 1) {
    const char = source[index];
    const next = source[index + 1];
    if (lineComment) {
      if (char === "\n") lineComment = false;
      continue;
    }
    if (quote) {
      if (escaped) escaped = false;
      else if (char === "\\") escaped = true;
      else if (char === quote) quote = "";
      continue;
    }
    if (char === "/" && next === "/") {
      lineComment = true;
      index += 1;
      continue;
    }
    if (char === '"' || char === "'") {
      quote = char;
      continue;
    }
    if (char === left) depth += 1;
    if (char === right) {
      depth -= 1;
      if (depth === 0) return index;
    }
  }
  return -1;
}

function firstCapture(source, pattern) {
  const match = source.match(pattern);
  return match ? match[1] : "";
}

module.exports = { parseRplDocument };
