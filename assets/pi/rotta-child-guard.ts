// Child-only guard, also auto-discovered by top-level Pi sessions.
import * as fs from "node:fs";
import * as path from "node:path";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

const role = process.env.ROTTA_CHILD_ROLE || "";
const root = canonical(
  path.resolve(process.env.ROTTA_WORK_ROOT || process.cwd()),
);
const allowed: Record<string, string[]> = {
  implementation: ["read", "write", "edit"],
  reviewer: ["read", "grep", "find", "ls"],
  exploration: ["read", "grep", "find", "ls"],
  operations: ["read"],
};
const mcpAllowed: Record<string, Record<string, string[]>> = {
  implementation: {
    ancora: ["save", "summarize", "start", "end", "search", "context", "get"],
    context7: ["resolve_library_id", "query_docs"],
  },
  reviewer: {
    ancora: ["save", "summarize", "start", "end", "search", "context", "get"],
    context7: ["resolve_library_id", "query_docs"],
  },
  exploration: {
    ancora: ["save", "summarize", "start", "end", "search", "context", "get"],
    vela: [
      "explore",
      "lookup",
      "dependencies",
      "reverse_dependencies",
      "impact",
      "path",
      "explain",
      "rank",
      "hotspots",
      "module_summary",
      "status",
    ],
    context7: ["resolve_library_id", "query_docs"],
  },
  operations: {
    ancora: ["save", "summarize", "start", "end", "search", "context", "get"],
  },
};
export function isAllowedChildMCP(childRole: string, name: string) {
  const match = /^rotta_(ancora|vela|context7)_(.+)$/.exec(name);
  return !!match && !!mcpAllowed[childRole]?.[match[1]]?.includes(match[2]);
}
function allowedMCP(name: string) {
  return isAllowedChildMCP(role, name);
}

// realpath only works for existing paths. Resolve the deepest existing ancestor
// first, then restore the missing suffix so a symlinked .rotta or workspace
// cannot bypass the parent-only work-record boundary.
function canonical(input: string) {
  const suffix: string[] = [];
  let cursor = path.resolve(input);
  while (!fs.existsSync(cursor)) {
    const parent = path.dirname(cursor);
    if (parent === cursor) break;
    suffix.unshift(path.basename(cursor));
    cursor = parent;
  }
  try {
    cursor = fs.realpathSync.native(cursor);
  } catch { /* retain resolved path */ }
  return path.join(cursor, ...suffix);
}

function target(input: Record<string, unknown>) {
  const raw = input.path || input.file_path;
  return typeof raw === "string" ? canonical(path.resolve(root, raw)) : "";
}

export function isProtectedWorkPath(input: string, workspace = root) {
  const work = canonical(path.join(canonical(workspace), ".rotta", "work"));
  const candidate = canonical(input);
  return candidate === work || candidate.startsWith(work + path.sep);
}

export default function guard(pi: ExtensionAPI) {
  // The extension directory is auto-loaded by top-level Pi sessions.
  // Enforce child restrictions only when delegation assigned a child role.
  if (!role) return;

  pi.on("tool_call", async (event: { toolName: string; input: unknown }) => {
    if (
      !allowed[role]?.includes(event.toolName) && !allowedMCP(event.toolName)
    ) {
      return { block: true, reason: "Rotta child guard denied tool" };
    }
    if (
      ["write", "edit"].includes(event.toolName) &&
      isProtectedWorkPath(target(event.input as Record<string, unknown>))
    ) return { block: true, reason: "Only the parent may write .rotta/work" };
  });
}
