// Explicit child-only guard loaded with -e after --no-extensions.
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
  pi.on("tool_call", async (event: { toolName: string; input: unknown }) => {
    if (!allowed[role]?.includes(event.toolName)) {
      return { block: true, reason: "Rotta child guard denied tool" };
    }
    if (
      ["write", "edit"].includes(event.toolName) &&
      isProtectedWorkPath(target(event.input as Record<string, unknown>))
    ) return { block: true, reason: "Only the parent may write .rotta/work" };
  });
}
