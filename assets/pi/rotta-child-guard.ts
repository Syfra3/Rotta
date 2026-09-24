// Child-only guard, also auto-discovered by top-level Pi sessions.
import * as fs from "node:fs";
import * as path from "node:path";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

const role = process.env.ROTTA_CHILD_ROLE || "";
const root = canonical(
  path.resolve(process.env.ROTTA_WORK_ROOT || process.cwd()),
);
const allowed: Record<string, string[]> = {
  implementation: ["read", "write", "edit", "bash"],
  reviewer: ["read", "grep", "find", "ls"],
  exploration: ["read", "grep", "find", "ls"],
  operations: ["read", "bash"],
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

export function operationGate(command: string | undefined) {
  let used = false;
  return (input: unknown) => {
    if (used || !command || typeof input !== "object" || input === null ||
      (input as { command?: unknown }).command !== command) return false;
    used = true;
    return true;
  };
}

export default function guard(pi: ExtensionAPI) {
  // The extension directory is auto-loaded by top-level Pi sessions.
  // Enforce child restrictions only when delegation assigned a child role.
  if (!role) return;
  const acceptOperation = operationGate(process.env.ROTTA_OPERATION_COMMAND);

  pi.on("tool_call", async (event: { toolName: string; input: unknown }) => {
    if (
      !allowed[role]?.includes(event.toolName) && !allowedMCP(event.toolName)
    ) {
      return { block: true, reason: "Rotta child guard denied tool" };
    }
    if (role === "operations" && event.toolName === "bash" && !acceptOperation(event.input)) {
      return { block: true, reason: "Rotta operation command missing, altered, or already used" };
    }
    if (
      ["write", "edit"].includes(event.toolName) &&
      isProtectedWorkPath(target(event.input as Record<string, unknown>))
    ) return { block: true, reason: "Only the parent may write .rotta/work" };
  });
}
