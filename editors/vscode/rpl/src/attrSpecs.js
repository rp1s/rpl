function sourceIdFromManifest(manifest) {
  const author = String(manifest && manifest.Author || "").trim();
  const name = String(manifest && manifest.Name || "").trim();
  if (!author || !name) {
    return "";
  }

  return `${author}:${name}`;
}

function uniqueNonEmptyStrings(items) {
  const result = [];
  const seen = new Set();

  for (const item of items) {
    const value = String(item || "").trim();
    if (!value || seen.has(value)) {
      continue;
    }

    seen.add(value);
    result.push(value);
  }

  return result;
}

function mergeText(left, right) {
  return uniqueNonEmptyStrings([left, right]).join("\n\n");
}

function normalizeArg(arg) {
  if (!arg || !arg.name) {
    return null;
  }

  return {
    name: String(arg.name).trim(),
    types: uniqueNonEmptyStrings(Array.isArray(arg.types) ? arg.types : []),
    help: String(arg.help || "").trim(),
    aliases: uniqueNonEmptyStrings(Array.isArray(arg.aliases) ? arg.aliases : []),
    conflicts_with: uniqueNonEmptyStrings(Array.isArray(arg.conflicts_with) ? arg.conflicts_with : []),
    positional: Boolean(arg.positional)
  };
}

function mergeArgSpecs(left, right) {
  if (!left) {
    return right;
  }
  if (!right) {
    return left;
  }

  return {
    name: left.name || right.name,
    types: uniqueNonEmptyStrings([...(left.types || []), ...(right.types || [])]),
    help: mergeText(left.help, right.help),
    aliases: uniqueNonEmptyStrings([...(left.aliases || []), ...(right.aliases || [])]),
    conflicts_with: uniqueNonEmptyStrings([...(left.conflicts_with || []), ...(right.conflicts_with || [])]),
    positional: Boolean(left.positional || right.positional)
  };
}

function mergeArgs(leftArgs, rightArgs) {
  const merged = [];
  const indexByName = new Map();

  for (const item of Array.isArray(leftArgs) ? leftArgs : []) {
    const normalized = normalizeArg(item);
    if (!normalized) {
      continue;
    }
    indexByName.set(normalized.name, merged.length);
    merged.push(normalized);
  }

  for (const item of Array.isArray(rightArgs) ? rightArgs : []) {
    const normalized = normalizeArg(item);
    if (!normalized) {
      continue;
    }

    const existingIndex = indexByName.get(normalized.name);
    if (existingIndex === undefined) {
      indexByName.set(normalized.name, merged.length);
      merged.push(normalized);
      continue;
    }

    merged[existingIndex] = mergeArgSpecs(merged[existingIndex], normalized);
  }

  return merged;
}

function normalizeSnippet(snippet) {
  if (!snippet) {
    return null;
  }

  const label = String(snippet.label || "").trim();
  const insert = String(snippet.insert || "").trim();
  if (!label && !insert) {
    return null;
  }

  return {
    label,
    insert,
    help: String(snippet.help || "").trim()
  };
}

function mergeSnippets(leftSnippets, rightSnippets) {
  const merged = [];
  const indexByKey = new Map();

  const append = (snippet) => {
    const normalized = normalizeSnippet(snippet);
    if (!normalized) {
      return;
    }

    const key = `${normalized.label}\u0000${normalized.insert}`;
    const existingIndex = indexByKey.get(key);
    if (existingIndex === undefined) {
      indexByKey.set(key, merged.length);
      merged.push(normalized);
      return;
    }

    merged[existingIndex] = {
      ...merged[existingIndex],
      help: mergeText(merged[existingIndex].help, normalized.help)
    };
  };

  for (const item of Array.isArray(leftSnippets) ? leftSnippets : []) {
    append(item);
  }
  for (const item of Array.isArray(rightSnippets) ? rightSnippets : []) {
    append(item);
  }

  return merged;
}

function mergeCapabilities(left, right) {
  const first = left && typeof left === "object" ? left : {};
  const second = right && typeof right === "object" ? right : {};
  const keys = new Set([...Object.keys(first), ...Object.keys(second)]);
  const result = {};

  for (const key of keys) {
    result[key] = Boolean(first[key] || second[key]);
  }

  return result;
}

function normalizeCatalogSpec(spec, manifest, capabilities) {
  const namespace = String(spec && spec.namespace || "").trim();
  if (!namespace) {
    return null;
  }

  const source = sourceIdFromManifest(manifest);

  return {
    namespace,
    help: String(spec && spec.help || "").trim(),
    args: mergeArgs([], spec && spec.args),
    snippets: mergeSnippets([], spec && spec.snippets),
    manifest: manifest && typeof manifest === "object" ? manifest : {},
    sources: source ? [source] : [],
    capabilities: mergeCapabilities({}, capabilities)
  };
}

function mergeCatalogSpecEntries(left, right) {
  if (!left) {
    return right;
  }
  if (!right) {
    return left;
  }

  return {
    namespace: left.namespace || right.namespace,
    help: mergeText(left.help, right.help),
    args: mergeArgs(left.args, right.args),
    snippets: mergeSnippets(left.snippets, right.snippets),
    manifest: sourceIdFromManifest(left.manifest) ? left.manifest : right.manifest,
    sources: uniqueNonEmptyStrings([...(left.sources || []), ...(right.sources || [])]),
    capabilities: mergeCapabilities(left.capabilities, right.capabilities)
  };
}

module.exports = {
  mergeCatalogSpecEntries,
  normalizeCatalogSpec
};
