// Global Pi extension. Pi discovers this exact file at ~/.pi/agent/extensions/rotta.ts.
import { execFile as nodeExecFile, execFileSync as nodeExecFileSync, spawn as nodeSpawn, spawnSync as nodeSpawnSync } from "node:child_process";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { pathToFileURL } from "node:url";
import { createHash } from "node:crypto";
import { StringDecoder } from "node:string_decoder";
import { promisify } from "node:util";
import { Type } from "typebox";
import {
  createBashToolDefinition,
  createEditToolDefinition,
  formatSize,
  type ExtensionAPI,
  type Theme,
  type ToolDefinition,
  VERSION,
} from "@earendil-works/pi-coding-agent";

const MAX_OUTPUT_BYTES = 64 * 1024;
const KILL_GRACE_MS = 5_000;
const DEFAULT_TIMEOUT_MS = 600_000;
const MAX_TIMEOUT_MS = 1_800_000;
const DETAIL_LIMIT_BYTES = 2048;
const PENDING_FAILURE_TTL_MS = 60_000;
const PENDING_FAILURE_CAPACITY = 64;
const DETAIL_SHORTCUT = "ctrl+shift+o";
const gitExec = promisify(nodeExecFile);
const ROTTA_VIEW_ROWS = 512;
const ROTTA_DIFF_BYTES = 64 * 1024;

type UsageFields = "input" | "output" | "reasoning" | "cacheRead" | "cacheWrite";
type UsageSummary = Partial<Record<UsageFields, number>> & { cost?: number; responses: number };
const usageFields: UsageFields[] = ["input", "output", "reasoning", "cacheRead", "cacheWrite"];
function measuredUsage(value: unknown): UsageSummary | undefined {
  if (!value || typeof value !== "object") return undefined;
  const usage = value as Record<string, unknown>;
  const totals: UsageSummary = { responses: 1 };
  let reported = false;
  for (const name of usageFields) {
    if (typeof usage[name] === "number" && Number.isFinite(usage[name]) && usage[name] >= 0) {
      totals[name] = usage[name] as number;
      reported = true;
    }
  }
  const cost = usage.cost;
  if (cost && typeof cost === "object" && typeof (cost as { total?: unknown }).total === "number") {
    const total = (cost as { total: number }).total;
    if (Number.isFinite(total) && total >= 0) { totals.cost = total; reported = true; }
  }
  return reported ? totals : undefined;
}
function addUsage(total: UsageSummary, next: UsageSummary): void {
  total.responses += next.responses;
  for (const field of usageFields) if (next[field] !== undefined) total[field] = (total[field] ?? 0) + next[field]!;
  if (next.cost !== undefined) total.cost = (total.cost ?? 0) + next.cost;
}
function usageLabel(value: number | undefined): string {
  return value === undefined ? "unavailable" : value.toLocaleString("en-US");
}

// A small, non-persistent Pi-themed panel. It never copies prompts or tool output
// into the session transcript; closing it releases the component and its lines.
async function showRottaPanel(ctx: any, title: string, lines: string[]): Promise<void> {
  if (ctx.mode !== "tui" || typeof ctx.ui?.custom !== "function") {
    ctx.ui?.notify?.("Rotta views require an interactive Pi terminal", "info");
    return;
  }
  const rows = lines.length > ROTTA_VIEW_ROWS
    ? [...lines.slice(0, ROTTA_VIEW_ROWS - 1), "… further rows omitted (local view limit)"] : lines;
  await ctx.ui.custom((tui: any, theme: Theme, _keys: unknown, done: (value: undefined) => void) => {
    let offset = 0;
    return {
      render(width: number) {
        const size = Math.max(1, width);
        return [truncateToWidth(theme.fg("accent", theme.bold(`Rotta • ${title}`)), size),
          ...rows.slice(offset, offset + 15).map((line) => truncateToWidth(theme.fg("text", line), size)),
          truncateToWidth(theme.fg("muted", `↑↓ scroll • ${rows.length ? offset + 1 : 0}-${Math.min(rows.length, offset + 15)}/${rows.length} • q / Esc close`), size)];
      },
      handleInput(data: string) {
        if (data === "q" || data === "\x1b") { done(undefined); return; }
        if (data === "\x1b[B" || data === "j") offset = Math.min(Math.max(0, rows.length - 15), offset + 1);
        if (data === "\x1b[A" || data === "k") offset = Math.max(0, offset - 1);
        tui.requestRender();
      },
      invalidate() { tui.requestRender(); },
    };
  });
}

function sessionUsage(ctx: any): { total: UsageSummary; byModel: Map<string, UsageSummary>; toolCalls: number } {
  const total: UsageSummary = { responses: 0 };
  const byModel = new Map<string, UsageSummary>();
  let toolCalls = 0;
  const entries = ctx.sessionManager?.getBranch?.() ?? [];
  const ids = new Set<string>();
  for (const entry of entries) {
    if (!entry || entry.type !== "message" || !entry.message) continue;
    if (typeof entry.id === "string") { if (ids.has(entry.id)) continue; ids.add(entry.id); }
    const message = entry.message;
    if (message.role !== "assistant") continue;
    toolCalls += Array.isArray(message.content) ? message.content.filter((part: any) => part?.type === "toolCall").length : 0;
    const usage = measuredUsage(message.usage);
    if (!usage) continue;
    addUsage(total, usage);
    const model = typeof message.model === "string" && message.model ? message.model : "unknown model";
    const row = byModel.get(model) ?? { responses: 0 };
    addUsage(row, usage);
    byModel.set(model, row);
  }
  return { total, byModel, toolCalls };
}

type DelegateActivity = { role: string; state: string; started: number; elapsedMs?: number; model?: string; requestedModel?: string; usage?: UsageSummary };
function delegateLines(jobs: readonly DelegateActivity[]): string[] {
  return jobs.length ? jobs.slice(-12).reverse().map((job) =>
    `${job.role} • ${job.state} • ${elapsedLabel(job.elapsedMs ?? (Date.now() - job.started))} • observed ${job.model ?? "unavailable"} / requested ${job.requestedModel ?? "unavailable"} • output ${usageLabel(job.usage?.output)}`)
    : ["No Rotta delegations observed in this session"];
}

type GitChange = { path: string; state: string; staged: boolean; untracked: boolean };
async function gitChanges(cwd: string, execute = gitExec): Promise<GitChange[]> {
  const { stdout } = await execute("git", ["--no-optional-locks", "status", "--porcelain=v1", "-z", "--untracked-files=normal"], {
    cwd, timeout: 5_000, maxBuffer: 128 * 1024, encoding: "utf8",
  });
  const tokens = stdout.split("\0");
  const changes: GitChange[] = [];
  for (let index = 0; index < tokens.length && changes.length < 80; index++) {
    const token = tokens[index];
    if (!token || token.length < 4 || token[2] !== " ") continue;
    const state = token.slice(0, 2);
    const file = token.slice(3);
    if (state.includes("R") || state.includes("C")) index++; // porcelain -z adds the other rename path
    changes.push({ path: file, state, staged: state[0] !== " " && state[0] !== "?", untracked: state === "??" });
  }
  return changes;
}
function terminalSafe(text: string): string {
  return text.replace(/\x1b\[[0-?]*[ -/]*[@-~]/g, "")
    .replace(/[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]/g, " ");
}
async function gitDiff(cwd: string, change: GitChange, execute = gitExec): Promise<string[]> {
  if (change.untracked) return ["Untracked file: diff unavailable until Git tracks it", "Author: unknown"];
  const args = ["--no-optional-locks", "diff", "--no-ext-diff", "--no-textconv", "--color=never"];
  const outputs: string[] = [];
  for (const staged of [false, true]) {
    if (staged && !change.staged) continue;
    const { stdout } = await execute("git", [...args, ...(staged ? ["--cached"] : []), "--", change.path], {
      cwd, timeout: 5_000, maxBuffer: ROTTA_DIFF_BYTES, encoding: "utf8",
    });
    if (stdout) outputs.push(`${staged ? "Staged" : "Unstaged"} diff:`, terminalSafe(stdout));
  }
  return ["Author: unknown (Git status does not prove who edited a file)", ...(outputs.length ? outputs : ["No textual diff available"])];
}
type WorkView = { id: string; phase: string; next: string; blocker: string };
function workViews(cwd: string): WorkView[] {
  const root = existingCanonical(path.join(cwd, ".rotta", "work"));
  if (!root) return [];
  try {
    const files = fs.readdirSync(root).filter((name) => /^[a-zA-Z0-9_-]+\.md$/.test(name))
      .sort((left, right) => {
        try { return fs.statSync(path.join(root, right)).mtimeMs - fs.statSync(path.join(root, left)).mtimeMs; }
        catch { return 0; }
      }).slice(0, 80);
    return files.map((name) => {
      try {
        const file = existingCanonical(path.join(root, name));
        if (!file.startsWith(root + path.sep) || fs.statSync(file).size > 64 * 1024) return null;
        const text = fs.readFileSync(file, "utf8");
        const field = (label: string) => terminalSafe(text.match(new RegExp(`^${label}: ([^\\r\\n]+)`, "m"))?.[1] ?? "unavailable").slice(0, 180);
        return { id: field("Work ID"), phase: field("Phase"), next: field("Next action"), blocker: field("Blocker") };
      } catch { return null; }
    }).filter((view): view is WorkView => view !== null);
  } catch { return []; }
}
const memoryTools = [
  "rotta_ancora_save",
  "rotta_ancora_summarize",
  "rotta_ancora_start",
  "rotta_ancora_end",
  "rotta_ancora_search",
  "rotta_ancora_context",
  "rotta_ancora_get",
] as const;
const docsTools = [
  "rotta_context7_resolve_library_id",
  "rotta_context7_query_docs",
] as const;
const velaTools = [
  "rotta_vela_explore",
  "rotta_vela_lookup",
  "rotta_vela_dependencies",
  "rotta_vela_reverse_dependencies",
  "rotta_vela_impact",
  "rotta_vela_path",
  "rotta_vela_explain",
  "rotta_vela_rank",
  "rotta_vela_hotspots",
  "rotta_vela_module_summary",
  "rotta_vela_status",
] as const;
const roles = {
  implementation: {
    skill: "rotta-impl",
    tools: [
      "read",
      "write",
      "edit",
      "bash",
      ...memoryTools,
      ...docsTools,
    ],
  },
  reviewer: {
    skill: "rotta-review",
    tools: [
      "read",
      "grep",
      "find",
      "ls",
      ...memoryTools,
      ...docsTools,
    ],
  },
  exploration: {
    skill: "rotta-explore",
    tools: [
      "read",
      "grep",
      "find",
      "ls",
      ...memoryTools,
      ...velaTools,
      ...docsTools,
    ],
  },
  operations: { skill: "rotta-ops", tools: ["read", "bash", ...memoryTools] },
} as const;
type Role = keyof typeof roles;
type Spawn = typeof nodeSpawn;
function currentPiRuntime(): { executable: string; entry?: string } | undefined {
  const executable = process.execPath;
  if (!path.isAbsolute(executable)) return;
  const name = path.basename(executable);
  if (/^pi(?:\.exe)?$/.test(name)) return { executable };
  if (/^(node|bun)(?:\.exe)?$/.test(name) && process.argv[1] && path.isAbsolute(process.argv[1]))
    return { executable, entry: process.argv[1] };
}
function compiledRuntime(file: string): boolean {
  // A name alone could identify a shell wrapper. Read only the magic bytes,
  // never the potentially large executable image.
  const fd = fs.openSync(file, "r");
  const magic = Buffer.alloc(4);
  let bytes: number;
  try { bytes = fs.readSync(fd, magic, 0, magic.length, 0); }
  finally { fs.closeSync(fd); }
  return bytes === 4 && (
    magic.toString("hex") === "7f454c46" || // ELF
    magic.subarray(0, 2).toString("hex") === "4d5a" || // PE MZ
    ["feedface", "cefaedfe", "feedfacf", "cffaedfe", // thin Mach-O
      "cafebabe", "bebafeca", "cafebabf", "bfbafeca"].includes(magic.toString("hex")) // universal Mach-O
  );
}
function trustedPiRuntime(identity: { executable: string; entry?: string } | undefined) {
  if (!identity?.executable || !path.isAbsolute(identity.executable)) return;
  try {
    const binary = fs.realpathSync(identity.executable);
    if (!fs.statSync(binary).isFile()) return;
    if (identity.entry) {
      if (!/^(node|bun)(?:\.exe)?$/.test(path.basename(binary)) ||
        !compiledRuntime(binary) || !path.isAbsolute(identity.entry)) return;
      const entry = fs.realpathSync(identity.entry);
      if (!fs.statSync(entry).isFile() || !/^(?:pi|cli)\.[cm]?js$/.test(path.basename(entry)) ||
        !entry.split(path.sep).some((part, index, parts) =>
          part === "pi" || (part === "node_modules" && parts[index + 1] === "@earendil-works" &&
            parts[index + 2] === "pi-coding-agent"))) return;
      return { executable: binary, prefix: [entry] };
    }
    if (/^pi(?:\.exe)?$/.test(path.basename(binary)) && compiledRuntime(binary))
      return { executable: binary, prefix: [] as string[] };
  } catch { /* fail closed */ }
}
export type RottaDependencies = {
  spawn?: Spawn;
  piRuntime?: () => { executable: string; entry?: string } | undefined;
  lazyGitSpawnSync?: typeof nodeSpawnSync;
  gitExec?: typeof gitExec;
  home?: () => string;
  defaultTimeoutMs?: number;
  maxTimeoutMs?: number;
  killGraceMs?: number;
  pendingFailureTtlMs?: number;
  pendingFailureCapacity?: number;
  // Test seams for deterministic pending-detail expiry and cleanup checks.
  setTimeout?: typeof globalThis.setTimeout;
  clearTimeout?: typeof globalThis.clearTimeout;
  // Test seam; production reads only the explicitly selected optional key.
  env?: (key: string) => string | undefined;
  // Test seams for the explicit Pi built-in renderer-shadow contract guard.
  piVersion?: string;
  builtinFactories?: {
    bash: typeof createBashToolDefinition;
    edit: typeof createEditToolDefinition;
  };
  // Parent-only seams for the real installed bridge's transport boundary.
  mcp?: {
    fetch?: typeof fetch;
    exists?: (file: string) => boolean;
    env?: (key: string) => string | undefined;
    timeoutMs?: number;
  };
};
type DelegateParams = {
  role: string;
  task: string;
  workspace?: string;
  model?: string;
  timeoutMs?: number;
  requiredTools?: string[];
  operation?: { requestId: string; command: string; target: string; action: string; effect: string; artifactPath: string; revision: number; digest: string };
};
type RoleSelection = { model: string; effort?: string };
type QuestionParams = {
  trigger: string;
  requestId: string;
  workspace: string;
  action: string;
  decision?: string;
  options?: string[];
  safeOutcome: string;
  contractPath?: string;
  contractRevision?: number;
  contractDigest?: string;
  command?: string;
  target?: string;
  operationRevision?: number;
  operationDigest?: string;
  operationPath?: string;
  effect?: string;
};
type QuestionDraftParams = {
  trigger: QuestionParams["trigger"];
  requestId: string;
  workspace?: string;
  action: string;
  decision: string;
  choices?: string[];
  safeOption: string;
  contractPath?: string;
  operationPath?: string;
};
type ToolContext = {
  cwd: string;
  hasUI: boolean;
  model?: { provider: string; id: string };
  ui: {
    select(
      title: string,
      options: string[],
      selectOptions?: { signal: AbortSignal; timeout?: number },
    ): Promise<string | null | undefined>;
  };
  sessionManager: { getSessionId(): string };
};

function toolResult(
  text: string,
  details: Record<string, unknown> = {},
) {
  return {
    content: [{ type: "text" as const, text }],
    details,
  };
}
function fail(message: string): never {
  throw new Error(trimUtf8(message, 1024));
}
function existingCanonical(input: string) {
  try {
    return fs.realpathSync.native(input);
  } catch {
    return "";
  }
}
// A sibling worktree is an explicitly selected project workspace, not an
// arbitrary path accepted because it shares a parent directory or .git file.
function selectedWorkspace(cwd: string, requested?: string): string {
  const base = existingCanonical(cwd);
  if (!base) fail("safe stop: question workspace unavailable");
  if (requested === undefined) return base;
  const target = existingCanonical(requested);
  if (!target || !path.isAbsolute(requested) || target !== path.normalize(requested)) {
    fail("safe stop: selected workspace must be an existing canonical path");
  }
  if (target === base) return base;
  try {
    const git = (dir: string, ...args: string[]) => nodeExecFileSync("git", ["-C", dir, ...args],
      { encoding: "utf8", timeout: 5000, maxBuffer: 1024 * 1024, stdio: ["ignore", "pipe", "pipe"] }).trim();
    const root = existingCanonical(git(base, "rev-parse", "--show-toplevel"));
    if (root !== base || existingCanonical(git(target, "rev-parse", "--show-toplevel")) !== target ||
      existingCanonical(path.resolve(base, git(base, "rev-parse", "--git-common-dir"))) !==
      existingCanonical(path.resolve(target, git(target, "rev-parse", "--git-common-dir"))) ||
      !git(base, "worktree", "list", "--porcelain").split("\n").some((line) =>
        line.startsWith("worktree ") && existingCanonical(line.slice(9)) === target)) {
      fail("safe stop: selected workspace is not a registered sibling Git worktree");
    }
  } catch {
    fail("safe stop: selected workspace is not a registered sibling Git worktree");
  }
  return target;
}
function operationTarget(workspace: string, supplied: string | undefined) {
  if (!supplied || !path.isAbsolute(supplied) || path.normalize(supplied) !== supplied ||
    !workspace) return null;
  // Require every existing component to be canonical. An absent leaf is allowed,
  // but a symlink or an existing target cannot silently change identity.
  let ancestor = supplied;
  while (true) {
    try { fs.lstatSync(ancestor); break; } catch (error) {
      if ((error as NodeJS.ErrnoException).code !== "ENOENT") return null;
    }
    const parent = path.dirname(ancestor);
    if (parent === ancestor) return null;
    ancestor = parent;
  }
  if (existingCanonical(ancestor) !== ancestor) return null;
  const chain: string[] = [];
  for (let current = ancestor; ; current = path.dirname(current)) {
    const stat = fs.lstatSync(current);
    chain.push(`${stat.dev}:${stat.ino}`);
    // An explicitly consented target may live outside the workspace. Bind its
    // canonical ancestry to the filesystem root so relocation is detected.
    if (current === workspace || path.dirname(current) === current) break;
  }
  const exists = ancestor === supplied;
  return { path: supplied, ancestor, exists, chain: chain.join("/") };
}
function currentContract(cwd: string, supplied: string | undefined) {
  if (!supplied) return null;
  const workspace = existingCanonical(cwd);
  const candidate = existingCanonical(path.resolve(workspace || cwd, supplied));
  if (
    !workspace || !candidate ||
    (candidate !== workspace && !candidate.startsWith(workspace + path.sep))
  ) return null;
  try {
    const bytes = fs.readFileSync(candidate);
    // A contract identity has exactly one metadata line, never an incidental
    // prose mention. The established forms are prefix-specific: plain uses an
    // unbackticked number; bold uses a balanced backtick wrapper.
    const contractText = bytes.toString("utf8");
    const revisionLines = [...contractText.matchAll(
      /^(?:[ \t]{0,3})(?:Revision:|\*\*Revision:\*\*)[^\r\n]*\r?$/gm,
    )];
    const revisions = [...contractText.matchAll(
      /^(?:[ \t]{0,3})(?:Revision:[ \t]*r?(\d+)|\*\*Revision:\*\*[ \t]*`r?(\d+)`)[ \t]*\r?$/gm,
    )].map((match) => match[1] ?? match[2]);
    // A title suffix is an alternative identity, not additional metadata.
    // Count candidate suffixes even when malformed so they cannot be ignored
    // in favor of a valid standalone line or another heading.
    const headingLines = [...contractText.matchAll(/^#(?!#)[ \t]+[^\r\n]*\r?$/gm)]
      .filter((match) => /\(revision\b/i.test(match[0]));
    const headingRevisions = headingLines.map((match) =>
      /^#[ \t]+[^\r\n]+[ \t]+\(revision[ \t]+r?(\d+)\)[ \t]*\r?$/i.exec(match[0])?.[1]
    );
    const identities = revisionLines.length + headingLines.length;
    return {
      path: candidate,
      digest: createHash("sha256").update(bytes).digest("hex"),
      revision: identities === 1 &&
          (revisionLines.length === 1 ? revisions.length === 1 : headingRevisions[0] !== undefined)
        ? Number(revisionLines.length ? revisions[0] : headingRevisions[0])
        : undefined,
    };
  } catch {
    return null;
  }
}
function currentOperation(workspace: string, supplied: string | undefined) {
  if (!supplied || !workspace) return null;
  const ops = existingCanonical(path.join(workspace, ".rotta", "ops"));
  const candidate = existingCanonical(path.resolve(workspace, supplied));
  if (!ops || !candidate || !candidate.startsWith(ops + path.sep) ||
    !candidate.endsWith(".md")) return null;
  try {
    const bytes = fs.readFileSync(candidate);
    const text = new TextDecoder("utf-8", { fatal: true }).decode(bytes);
    // Exactly five single-line fields, no markdown, CR, duplicate, or hidden data.
    const lines = text.endsWith("\n") ? text.slice(0, -1).split("\n") : text.split("\n");
    if (lines.length !== 5) return null;
    const fields = new Map<string, string>();
    for (const line of lines) {
      const match = /^(Action|Command|Target|Effect|Revision): (.+)$/.exec(line);
      if (!match || fields.has(match[1]) || /[\x00-\x1f\x7f]/.test(match[2])) return null;
      fields.set(match[1], match[2]);
    }
    const revisionText = fields.get("Revision");
    if (!revisionText || !/^[1-9][0-9]*$/.test(revisionText)) return null;
    const revision = Number(revisionText);
    if (!Number.isSafeInteger(revision)) return null;
    const identity = operationTarget(workspace, fields.get("Target"));
    if (!identity) return null;
    const target = identity.path;
    return { path: candidate, digest: createHash("sha256").update(bytes).digest("hex"),
      revision, action: fields.get("Action")!, command: fields.get("Command")!,
      target, targetAncestor: identity.ancestor, targetExists: identity.exists, targetChain: identity.chain,
      effect: fields.get("Effect")! };
  } catch { return null; }
}
function trimUtf8(text: string, limit = MAX_OUTPUT_BYTES) {
  let result = "";
  for (const char of text) {
    if (Buffer.byteLength(result + char, "utf8") > limit) break;
    result += char;
  }
  return result;
}
function appendBounded(current: string, next: string) {
  return trimUtf8(current + next);
}
function appendPrefixBounded(current: string, next: string) {
  let result = current;
  let bytes = Buffer.byteLength(result, "utf8");
  for (const char of next) {
    const charBytes = Buffer.byteLength(char, "utf8");
    if (bytes + charBytes > MAX_OUTPUT_BYTES) break;
    result += char;
    bytes += charBytes;
  }
  return result;
}
function appendTailBounded(current: string, next: string) {
  const bytes = Buffer.from(current + next, "utf8");
  if (bytes.length <= MAX_OUTPUT_BYTES) return bytes.toString("utf8");
  let start = bytes.length - MAX_OUTPUT_BYTES;
  while (start < bytes.length && (bytes[start] & 0xc0) === 0x80) start++;
  return bytes.subarray(start).toString("utf8");
}
function roleLabel(role: unknown) {
  return typeof role === "string" && role ? role : "unknown";
}
function summarizeTask(task: unknown, limit = 80) {
  if (typeof task !== "string" || task.trim() === "") return "no task supplied";
  const compact = task.replace(/\s+/g, " ").trim();
  return compact.length > limit ? `${compact.slice(0, limit - 1)}…` : compact;
}
function elapsedLabel(ms: number) {
  const safe = Math.max(0, Math.floor(ms));
  const seconds = Math.floor(safe / 1000);
  const minutes = Math.floor(seconds / 60);
  const rest = seconds % 60;
  if (minutes <= 0) return `${rest}s`;
  return `${minutes}m ${rest.toString().padStart(2, "0")}s`;
}
function executionStartedMs(value: unknown) {
  if (typeof value === "number") return value;
  if (value instanceof Date) return value.getTime();
  if (typeof value === "string") {
    const parsed = Date.parse(value);
    return Number.isFinite(parsed) ? parsed : Date.now();
  }
  return Date.now();
}
function contentText(result: { content?: unknown }) {
  const content =
    (result as { content?: Array<{ type?: string; text?: string }> }).content;
  if (!Array.isArray(content)) return "";
  return content.filter((part) =>
    part?.type === "text" && typeof part.text === "string"
  ).map((part) => part.text).join("\n");
}
function characterWidth(character: string) {
  const code = character.codePointAt(0) ?? 0;
  if (/\p{Mark}/u.test(character) || code === 0x200d || code === 0xfe0f) return 0;
  return code >= 0x1100 && (
      code <= 0x115f || code === 0x2329 || code === 0x232a ||
      (code >= 0x2e80 && code <= 0xa4cf) ||
      (code >= 0xac00 && code <= 0xd7a3) ||
      (code >= 0xf900 && code <= 0xfaff) ||
      (code >= 0xfe10 && code <= 0xfe6f) ||
      (code >= 0xff00 && code <= 0xff60) ||
      (code >= 0xffe0 && code <= 0xffe6) || code >= 0x1f300
    ) ? 2 : 1;
}
function truncateAnsiLine(line: string, width: number, suffix = ""): string {
  if (width <= 0) return "";
  const tokens = line.match(/\x1b\[[0-?]*[ -/]*[@-~]|[^\x1b]/gu) ?? [];
  const visible = tokens.reduce((sum, token) =>
    sum + (token.startsWith("\x1b[") ? 0 : characterWidth(token)), 0);
  if (visible <= width) return line;
  const suffixWidth = [...suffix].reduce((sum, char) => sum + characterWidth(char), 0);
  let result = "", used = 0;
  for (const token of tokens) {
    if (token.startsWith("\x1b[")) {
      result += token;
      continue;
    }
    const tokenWidth = characterWidth(token);
    if (used + tokenWidth > Math.max(0, width - suffixWidth)) break;
    result += token;
    used += tokenWidth;
  }
  return result + (suffixWidth <= width ? suffix : "");
}
function truncateToWidth(line: string, width: number, suffix = "") {
  return truncateAnsiLine(line, width, suffix);
}
function wrapTextWithAnsi(text: string, width: number): string[] {
  if (width <= 0) return [""];
  const tokens = text.match(/\x1b\[[0-?]*[ -/]*[@-~]|[^\x1b]/gu) ?? [];
  const lines: string[] = [];
  let line = "", visible = 0;
  for (const token of tokens) {
    if (token.startsWith("\x1b[")) {
      line += token;
    } else {
      const tokenWidth = characterWidth(token);
      if (visible > 0 && visible + tokenWidth > width) {
        lines.push(line);
        line = "";
        visible = 0;
      }
      line += token;
      visible += tokenWidth;
    }
  }
  lines.push(line);
  return lines;
}
function matchesKey(data: string, key: string) {
  const sequences: Record<string, readonly string[]> = {
    escape: ["\x1b"],
    "ctrl+c": ["\x03"],
    up: ["\x1b[A", "\x1bOA"],
    down: ["\x1b[B", "\x1bOB"],
    pageUp: ["\x1b[5~"],
    pageDown: ["\x1b[6~"],
  };
  return sequences[key]?.includes(data) ?? false;
}

function rottaVersionLabel(output: string): string | undefined {
  const match = /^\s*rotta\s+v?(\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?)\s*$/i
    .exec(output);
  return match ? `Rotta v${match[1]}` : undefined;
}

async function installedRottaLabel(pi: ExtensionAPI): Promise<string> {
  try {
    const result = await pi.exec("rotta", ["--version"], { timeout: 5_000 });
    return result.code === 0
      ? rottaVersionLabel(result.stdout) ?? "Rotta"
      : "Rotta";
  } catch {
    return "Rotta";
  }
}

export function renderRottaHeader(
  theme: Theme,
  width: number,
  rottaLabel = "Rotta",
): string[] {
  const logo = theme.fg("accent", "╭─╮");
  const title = theme.fg("accent", theme.bold("R O T T A"));
  const greeting = theme.fg("text", "Welcome to Rotta");
  const version = theme.fg("muted", `Pi v${VERSION} • ${rottaLabel}`);
  const startupHelp = theme.fg(
    "dim",
    " • ctrl+o for full startup help • /hotkeys",
  );
  const lines = [
    `${logo}  ${title}`,
    `${theme.fg("accent", "│◉│")}  ${greeting}`,
    `${theme.fg("accent", "╰─╯")}  ${version}${startupHelp}`,
  ];
  return lines.map((line) => truncateAnsiLine(line, width));
}

const DETAIL_OPENER = Symbol.for("rotta.pi.openLatestDetail");
let openLatestRottaDetail: (() => Promise<void>) | undefined;

function textComponent(text: string, activate?: () => void) {
  return {
    render(width: number) {
      return text.split("\n").map((line) => truncateToWidth(line, width, "…"));
    },
    handleMouse(event: { button?: string; type?: string }) {
      if (activate && event.button === "left" && event.type === "click") {
        activate();
        return { handled: true };
      }
      return undefined;
    },
    invalidate() {},
  };
}

type PiToolDefinition = ToolDefinition<any, any, any>;

function plainComponent(text: string) {
  return {
    render(width: number) {
      return text.split("\n").map((line) => truncateToWidth(line, width, "…"));
    },
    invalidate() {},
  };
}

function completeComponent(text: string) {
  return {
    render(width: number) {
      return text.split("\n").flatMap((line) => wrapTextWithAnsi(line, Math.max(1, width)));
    },
    invalidate() {},
  };
}

function compactCommand(command: unknown, limit = 72) {
  if (typeof command !== "string" || !command.trim()) return "…";
  const compact = command.replace(/\s+/g, " ").trim();
  return compact.length > limit ? `${compact.slice(0, limit - 1)}…` : compact;
}

function bashDuration(context: any, partial: boolean) {
  const state = context.state ?? (context.state = {});
  if (context.executionStarted && state.rottaStartedAt === undefined) {
    state.rottaStartedAt = Date.now();
  }
  if (!partial && state.rottaEndedAt === undefined) state.rottaEndedAt = Date.now();
  const start = state.rottaStartedAt;
  if (typeof start !== "number") return "0s";
  return elapsedLabel((partial ? Date.now() : state.rottaEndedAt) - start);
}

function bashStatus(result: any, options: any, context: any) {
  if (options.isPartial || context.isPartial) return "running";
  if (!context.isError) return "exit 0";
  const text = contentText(result);
  const timeout = /Command timed out after ([0-9.]+) seconds/.exec(text);
  if (timeout) return `timeout ${timeout[1]}s`;
  const exit = /Command exited with code (\d+)/.exec(text);
  return exit ? `exit ${exit[1]}` : "failed";
}

function bashWarnings(result: any, pathOverride?: string) {
  const truncation = result.details?.truncation;
  const fullOutputPath = pathOverride ?? result.details?.fullOutputPath;
  const warnings: string[] = [];
  if (truncation?.truncated) {
    const shown = Number.isFinite(truncation.outputLines) ? truncation.outputLines : "?";
    if (truncation.truncatedBy === "lines") {
      const total = Number.isFinite(truncation.totalLines) ? truncation.totalLines : "?";
      warnings.push(`truncated: ${shown}/${total} lines`);
    } else {
      const byteCount = truncation.maxBytes ?? truncation.outputBytes;
      warnings.push(`truncated: ${shown} lines, ${Number.isFinite(byteCount) ? formatSize(byteCount) : "byte limit"}`);
    }
  }
  if (fullOutputPath) warnings.push(`Full output: ${fullOutputPath}`);
  return warnings.length ? `[${warnings.join(". ")}]` : "";
}

function generatedBashFooter(output: string, result: any) {
  if (!result.details?.truncation?.truncated) return null;
  // Recognized Pi formatOutput variants append exactly one of these three terminal forms.
  const match = /\n\n\[(?:Showing lines \d+-\d+ of \d+(?: \(50\.0KB limit\))?|Showing last (?:\d+B|\d+\.\dKB|\d+\.\dMB) of line \d+ \(line is (?:\d+B|\d+\.\dKB|\d+\.\dMB)\))\. Full output: ([^\]\r\n]+)\]$/.exec(output);
  if (!match) return null;
  return { content: output.slice(0, match.index), path: match[1] };
}

function withoutGeneratedBashFooter(output: string, result: any) {
  return generatedBashFooter(output, result)?.content ?? output;
}

function diagnosticExcerpt(result: any, limit = 120) {
  const sanitized = safeDetail(withoutGeneratedBashFooter(contentText(result), result))
    .replace(/\x1b\[[0-?]*[ -/]*[@-~]/g, "")
    .replace(/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f]/g, " ")
    .replace(/\s+/g, " ")
    .trim();
  if (!sanitized) return "no diagnostic output";
  return sanitized.length > limit ? `${sanitized.slice(0, limit - 1)}…` : sanitized;
}

function expandedBashText(result: any) {
  const raw = contentText(result);
  const footer = generatedBashFooter(raw, result);
  const output = footer?.content ?? raw;
  const warning = bashWarnings(result, footer?.path);
  if (!warning) return output;
  return output ? `${output}\n${warning}` : warning;
}

function diffCounts(diff: unknown) {
  if (typeof diff !== "string") return "";
  let added = 0, removed = 0;
  for (const line of diff.split("\n")) {
    if (line.startsWith("+") && !line.startsWith("+++")) added++;
    else if (line.startsWith("-") && !line.startsWith("---")) removed++;
  }
  return ` +${added} -${removed}`;
}

type BuiltinContractSource = {
  version?: string;
  factories?: {
    bash: typeof createBashToolDefinition;
    edit: typeof createEditToolDefinition;
  };
};

function assertBuiltinFactoryResult(name: "bash" | "edit", definition: any) {
  if (
    !definition || typeof definition !== "object" || definition.name !== name ||
    typeof definition.execute !== "function" || !("parameters" in definition) ||
    typeof definition.renderCall !== "function" ||
    typeof definition.renderResult !== "function"
  ) throw new Error(`unsupported Pi ${name} factory result shape`);
}

export function compactBuiltinDefinitions(cwd: string, source: BuiltinContractSource = {}): {
  bash: PiToolDefinition;
  edit: PiToolDefinition;
  originals: { bash: PiToolDefinition; edit: PiToolDefinition };
} {
  const factories = source.factories ?? { bash: createBashToolDefinition, edit: createEditToolDefinition };
  const originalBash = factories.bash(cwd);
  const originalEdit = factories.edit(cwd);
  assertBuiltinFactoryResult("bash", originalBash);
  assertBuiltinFactoryResult("edit", originalEdit);
  const bash: PiToolDefinition = {
    ...originalBash,
    renderCall(args: any, theme: any, context: any) {
      bashDuration(context, true);
      return plainComponent(`${theme.fg("toolTitle", theme.bold("$"))} ${compactCommand(args?.command)}`);
    },
    renderResult(result: any, options: any, theme: any, context: any) {
      const status = bashStatus(result, options, context);
      const duration = bashDuration(context, options.isPartial);
      const warning = bashWarnings(result, generatedBashFooter(contentText(result), result)?.path);
      const diagnostic = context.isError ? ` • ${diagnosticExcerpt(result)}` : "";
      const hint = options.expanded ? "" : warning ? ` • ${warning} • Ctrl+O details` : " • Ctrl+O details";
      const summary = `${compactCommand(context.args?.command)} • ${status} • ${duration}${diagnostic}${hint}`;
      if (!options.expanded) return plainComponent(theme.fg(context.isError ? "error" : status === "running" ? "warning" : "success", summary));
      const output = expandedBashText(result);
      return completeComponent(output ? `${summary}\n${output}` : summary);
    },
  };
  const edit: PiToolDefinition = {
    ...originalEdit,
    renderCall(args: any, theme: any) {
      return plainComponent(`${theme.fg("toolTitle", theme.bold("edit"))} ${String(args?.path ?? args?.file_path ?? "…")}`);
    },
    renderResult(result: any, options: any, theme: any, context: any) {
      const path = String(context.args?.path ?? context.args?.file_path ?? "…");
      const count = Array.isArray(context.args?.edits) ? context.args.edits.length : 0;
      const state = options.isPartial || context.isPartial ? "running" : context.isError ? "failed" : "success";
      const diagnostic = context.isError && !options.expanded ? ` • ${diagnosticExcerpt(result)}` : "";
      const line = `${path} • ${state} • ${count} replacement${count === 1 ? "" : "s"}${diffCounts(result.details?.diff)}${diagnostic}${options.expanded ? "" : " • Ctrl+O details"}`;
      if (!options.expanded) return plainComponent(theme.fg(context.isError ? "error" : state === "running" ? "warning" : "success", line));
      const detail = context.isError ? contentText(result) : typeof result.details?.diff === "string" ? result.details.diff : contentText(result);
      return completeComponent(detail ? `${line}\n${detail}` : line);
    },
  };
  return { bash, edit, originals: { bash: originalBash, edit: originalEdit } };
}

type ActionDetail = {
  toolCallId?: string;
  service: string;
  action: string;
  state: string;
  request: unknown;
  response?: unknown;
  diagnostic?: unknown;
};

function safeDetail(value: unknown) {
  const raw = typeof value === "string" ? value : JSON.stringify(value, null, 2);
  const redacted = (raw ?? "")
    .replace(/\b(Bearer|Basic)\s+[^\s"']+/gi, "$1 [redacted]")
    .replace(/(["']?(?:api[_-]?key|token|authorization|password)["']?\s*[:=]\s*)["']?[^\s,"'}]+["']?/gi, "$1[redacted]");
  return trimUtf8(redacted.replace(/[\x00-\x1f\x7f-\x9f]/g, " "), DETAIL_LIMIT_BYTES);
}

export function detailOverlayComponent(
  tui: { requestRender(): void },
  theme: Theme,
  getDetail: () => ActionDetail,
  expanded: () => boolean,
) {
  return {
    render(width: number) {
      const detail = getDetail();
      const lines = [`Rotta ${safeDetail(detail.service).slice(0, 40)} / ${safeDetail(detail.action).slice(0, 60)} • ${detail.state}`];
      if (expanded()) {
        lines.push(`Diagnostic: ${safeDetail(detail.diagnostic ?? "none")}`);
      } else {
        const summary = detail.state === "running" ? "Action in progress" :
          detail.state === "failed" ? "Action failed" : "Action completed";
        lines.push(`${summary} • ${DETAIL_SHORTCUT} hide • ctrl+shift+d diagnostic`);
      }
      return lines.slice(0, expanded() ? 8 : 2).flatMap((line) =>
        wrapTextWithAnsi(line, Math.max(1, width)).slice(0, expanded() ? 4 : 1).map((part) => truncateToWidth(theme.fg("dim", part), width))
      );
    },
    invalidate() { tui.requestRender(); },
  };
}
function renderDelegateCall(args: Record<string, unknown>, theme: any) {
  const role = roleLabel(args.role);
  const task = summarizeTask(args.task);
  const model = typeof args.model === "string" && args.model
    ? args.model
    : "resolved at run";
  return textComponent(
    `${theme.fg("toolTitle", theme.bold("delegate"))} ${
      theme.fg("accent", role)
    } ${theme.fg("muted", `model=${model} effort=${args.model ? "default" : "resolved at run"}`)} ${theme.fg("dim", task)}`,
    () => void openLatestRottaDetail?.(),
  );
}
function renderDelegateResult(
  result: {
    content?: unknown;
    details?: Record<string, unknown>;
    isError?: boolean;
  },
  options: { expanded?: boolean; isPartial?: boolean },
  theme: any,
  context: {
    args?: Record<string, unknown>;
    executionStarted?: unknown;
    invalidate?: () => void;
    isError?: boolean;
  },
) {
  const role = roleLabel(result.details?.role ?? context.args?.role);
  const reportedModel = typeof result.details?.responseModel === "string"
    ? result.details.responseModel
    : "?";
  const reportedEffort = typeof result.details?.providerThinkingLevel === "string"
    ? result.details.providerThinkingLevel
    : "?";
  const routing = `model=${reportedModel} effort=${reportedEffort}`;
  const elapsed = elapsedLabel(
    typeof result.details?.elapsedMs === "number"
      ? result.details.elapsedMs
      : Date.now() - executionStartedMs(context.executionStarted),
  );
  if (options.isPartial || result.details?.running === true) {
    setTimeout(() => context.invalidate?.(), 1000);
    const timeout = typeof result.details?.timeoutMs === "number"
      ? ` / timeout ${elapsedLabel(result.details.timeoutMs)}`
      : "";
    const requestedModel = typeof result.details?.requestedModel === "string"
      ? result.details.requestedModel
      : "unavailable";
    const requestedEffort = typeof result.details?.requestedEffort === "string"
      ? result.details.requestedEffort
      : "unavailable";
    const line = `${theme.fg("warning", "● running")} ${
      theme.fg("accent", role)
    } ${theme.fg("muted", `model=? effort=? ${elapsed + timeout}`)} ${
      theme.fg("dim", `requested ${requestedModel}/${requestedEffort} • details`)
    }`;
    return textComponent(line, () => void openLatestRottaDetail?.());
  }
  const failed = context.isError === true ||
    result.details?.outcome === "error" ||
    (result.details?.outcome === undefined && result.details?.failed === true);
  const icon = failed
    ? theme.fg("error", "✗ failed")
    : theme.fg("success", "✓ complete");
  const reasonText = typeof result.details?.reason === "string"
    ? result.details.reason
    : failed
    ? summarizeTask(contentText(result), 120)
    : "";
  const reason = reasonText && reasonText !== "no task supplied"
    ? ` ${theme.fg("dim", reasonText)}`
    : "";
  const line = `${icon} ${theme.fg("accent", role)} ${
    theme.fg("muted", `${routing} ${elapsed}`)
  }${reason} ${theme.fg("dim", "ctrl+shift+o details")}`;
  const response = contentText(result) || "(no delegation output)";
  if (!failed) {
    return textComponent(`${line}\n${theme.fg("toolOutput", response)}`, () => void openLatestRottaDetail?.());
  }
  // Child text is arbitrary output, not a safe one-line status for the TUI.
  // Keep the model-facing failure in the tool result and the detail overlay.
  return textComponent(line, () => void openLatestRottaDetail?.());
}
function policyPrompt(home: string, role: Role) {
  const root = path.join(home, ".pi", "agent", "rotta-next");
  return `Read and obey these exact installed policies before acting: ${
    path.join(root, "rotta-core", "SKILL.md")
  } and ${
    path.join(root, roles[role].skill, "SKILL.md")
  }. You are ${role}. Managed MCP tools, if discovered, are named rotta_ancora_*, rotta_vela_*, and rotta_context7_*. Use only names permitted by your installed role policy. Do not use unavailable tools or delegate. Return one JSON object only: {"status":"success","output":"..."} or {"status":"error","message":"..."}.`;
}
function policyPaths(home: string, role: Role) {
  const root = path.join(home, ".pi", "agent", "rotta-next");
  return [
    path.join(root, "rotta-core", "SKILL.md"),
    path.join(root, roles[role].skill, "SKILL.md"),
  ];
}
function childGuard(home: string) {
  return path.join(home, ".pi", "agent", "extensions", "rotta-child-guard.ts");
}
function mcpBridge(home: string) {
  return path.join(home, ".pi", "agent", "rotta-next", "rotta-mcp-bridge.ts");
}
async function loadMCPBridge(
  pi: ExtensionAPI,
  home: string,
  dependencies: RottaDependencies,
) {
  // The bridge is intentionally a sibling managed resource, not an
  // auto-discovered global extension.  Dynamic loading keeps this source
  // directly testable while installed imports resolve from ~/.pi/agent.
  try {
    const module = await import(pathToFileURL(mcpBridge(home)).href);
    return module.registerMCPBridge(pi, {
      spawn: dependencies.spawn,
      home: () => home,
      ...dependencies.mcp,
      role: "parent",
    });
  } catch (error) {
    const reason = trimUtf8(
      error instanceof Error ? error.message : String(error),
      1024,
    );
    return {
      start: async () => {},
      activate: async () => {},
      close: async () => {},
      status: () => ({ bridge: { state: "unavailable", reason } }),
      nativeStatus: () => "native MCP bridge unavailable",
    };
  }
}
type ChildMetadata = {
  responseModel?: string;
  providerThinkingLevel?: string;
  responseUsage?: UsageSummary;
};
type ChildResult = ({ status: "success"; output: string } | {
  status: "error";
  message: string;
}) & ChildMetadata;

function parseChildEnvelope(text: string): ChildResult | null {
  const trimmed = text.trim();
  const fenced = /^```(?:json)?\s*\n([\s\S]*?)\n```$/i.exec(trimmed);
  try {
    const value = JSON.parse(fenced ? fenced[1] : trimmed);
    if (
      value && typeof value === "object" &&
      ((value.status === "success" && typeof value.output === "string") ||
        (value.status === "error" && typeof value.message === "string"))
    ) {
      return value.status === "success"
        ? { status: "success", output: trimUtf8(value.output) }
        : { status: "error", message: trimUtf8(value.message) };
    }
  } catch { /* invalid envelope */ }
  return null;
}
function envelopeLike(text: string) {
  return /^\s*[\[{]/.test(text) || /```(?:json)?(?:\s|$)/i.test(text);
}

// `undefined` means this line is not a result-bearing protocol record; null
// means it is an authoritative but malformed final assistant result.
// Pi JSONL emits these discriminator fields at the start of message_end records.
// This intentionally examines only the already bounded line prefix: oversized
// records must not make us retain unbounded child output.
function isAuthoritativeAssistantPrefix(line: string) {
  return /^\s*\{\s*"type"\s*:\s*"message_end"\s*,\s*"message"\s*:\s*\{\s*"role"\s*:\s*"assistant"(?:\s*,|\s*\})/
    .test(
      line,
    );
}

function childResultForLine(line: string): ChildResult | null | undefined {
  try {
    const value = JSON.parse(line);
    if (
      value?.type === "message_end" && value.message?.role === "assistant"
    ) {
      const content = value.message.content;
      const responseModel = typeof value.message.responseModel === "string" &&
          value.message.responseModel.trim()
        ? value.message.responseModel
        : typeof value.message.model === "string" && value.message.model.trim()
        ? value.message.model
        : undefined;
      const providerThinkingLevel =
        typeof value.message.providerThinkingLevel === "string" &&
          value.message.providerThinkingLevel.trim()
          ? value.message.providerThinkingLevel
          : undefined;
      const responseUsage = measuredUsage(value.message.usage);
      const metadata = {
        ...(responseModel ? { responseModel } : {}),
        ...(providerThinkingLevel ? { providerThinkingLevel } : {}),
        ...(responseUsage ? { responseUsage } : {}),
      };
      const finalAssistant = typeof content === "string"
        ? content
        : Array.isArray(content)
        ? content.filter((part: unknown) =>
          typeof part === "object" && part !== null &&
          (part as { type?: string }).type === "text"
        ).map((part: { text?: unknown }) =>
          typeof part.text === "string" ? part.text : ""
        ).join("")
        : "";
      if (!finalAssistant) return undefined;
      const envelope = parseChildEnvelope(finalAssistant);
      if (envelope) return { ...envelope, ...metadata };
      // Only message_end may carry plain prose. JSON- or fence-shaped failures
      // invalidate an earlier result rather than being accepted as prose.
      return envelopeLike(finalAssistant)
        ? null
        : { status: "success", output: trimUtf8(finalAssistant), ...metadata };
    }
    // Retain the legacy direct envelope transport, but never treat an
    // arbitrary Pi event as a result merely because it has similar fields.
    return value?.type === undefined ? parseChildEnvelope(line) : undefined;
  } catch { /* malformed JSONL record */ }
  return undefined;
}

function reportChildTelemetry(event: { phase: string; role: string; reason?: string; responseUsage?: UsageSummary; responseModel?: string }) {
  // Telemetry is optional and must never change child dispatch or its result.
  try { (globalThis as any)[Symbol.for("rotta.pi.child.telemetry")]?.(event); } catch { /* optional observer */ }
}

async function runChild(
  cwd: string,
  home: string,
  role: Role,
  task: string,
  model: string | undefined,
  effort: string | undefined,
  timeoutMs: number | undefined,
  limits: {
    defaultTimeoutMs: number;
    maxTimeoutMs: number;
    killGraceMs: number;
  },
  signal: AbortSignal,
  onUpdate: (result: ReturnType<typeof toolResult>) => void,
  spawn: Spawn,
  context7Key?: string,
  routing?: { source: string; requestedModel: string; requestedEffort: string },
  operationCommand?: string,
  runtime?: () => { executable: string; entry?: string } | undefined,
) {
  const promptDir = await fs.promises.mkdtemp(
    path.join(os.tmpdir(), "rotta-pi-"),
  );
  const prompt = path.join(promptDir, "policy.md");
  await fs.promises.writeFile(prompt, policyPrompt(home, role), {
    mode: 0o600,
  });
  const [core, roleSkill] = policyPaths(home, role);
  const args = [
    "--mode",
    "json",
    "-p",
    "--no-session",
    "--no-extensions",
    "-e",
    childGuard(home),
    "-e",
    mcpBridge(home),
    "--no-skills",
    "--skill",
    core,
    "--skill",
    roleSkill,
    "--no-prompt-templates",
    "--no-context-files",
    "--tools",
    roles[role].tools.join(","),
    "--append-system-prompt",
    prompt,
  ];
  if (model) args.push("--model", model);
  if (model && effort) args.push("--thinking", effort);
  args.push(operationCommand
    ? `${task}\n\nParent-validated one-time operation binding: execute exactly this command once with the bash tool (as the entire command argument):\n${operationCommand}\nThe child guard independently enforces this exact command and rejects any other bash invocation. Do not run a preparatory bash check or alter the command.`
    : task);
  try {
    const startedAt = Date.now();
    return await new Promise<ReturnType<typeof toolResult>>((resolve) => {
      let done = false,
        closed = false,
        stopped = false,
        timer: ReturnType<typeof setTimeout> | undefined;
      let proc: ReturnType<Spawn> | undefined;
      let stdout = "", stderr = "", pendingLine = "";
      let pendingLineTooLarge = false;
      let childResult: ChildResult | null = null;
      let stage = "starting";
      let outputObserved = false;
      const outDecoder = new StringDecoder("utf8"),
        errDecoder = new StringDecoder("utf8");
      const rolePrefix = `[${role}] `;
      const finish = (text: string, error = false, reason?: string) => {
        if (!done) {
          done = true;
          reportChildTelemetry({ phase: "finished", role,
            reason: reason ?? (error ? "child_error" : "success"),
            responseUsage: reason === "timeout" || reason === "cancelled" || reason === "invalid_result"
              ? undefined : childResult?.responseUsage,
            responseModel: childResult?.responseModel,
          });
          if (timer) clearTimeout(timer);
          signal.removeEventListener("abort", cancel);
          resolve(toolResult(trimUtf8(error ? rolePrefix + text : text), {
            role,
            elapsedMs: Date.now() - startedAt,
            ...(routing ?? {}),
            ...(childResult?.responseModel
              ? { responseModel: childResult.responseModel }
              : {}),
            ...(childResult?.providerThinkingLevel
              ? { providerThinkingLevel: childResult.providerThinkingLevel }
              : {}),
            ...(childResult?.responseUsage
              ? { responseUsage: childResult.responseUsage }
              : {}),
            outcome: error ? "error" : "success",
            cancelled: signal.aborted,
            stage,
            outputObserved,
            // Partial JSONL may contain unredacted tool arguments or secrets.
            // Timeout/invalid protocol evidence is structural, not a handoff.
            ...(stdout && reason !== "timeout" && reason !== "invalid_result" ? { diagnostic: stdout } : {}),
            ...(reason ? { reason } : {}),
            ...(reason === "timeout" || reason === "invalid_result"
              ? { checkpointRequired: true,
                nextAction: "Verify the persisted work record and current diff before a changed-hypothesis retry; inspect any operation effect before considering another authorization. Partial child output is not a handoff." }
              : {}),
            ...(error ? { failed: true } : {}),
          }));
        }
      };
      const stop = () => {
        if (stopped) return;
        stopped = true;
        proc?.kill("SIGTERM");
        setTimeout(() => {
          if (!closed) proc?.kill("SIGKILL");
        }, limits.killGraceMs);
      };
      const cancel = () => {
        stop();
        finish("child cancelled", true, "cancelled");
      };
      try {
        const selectedRuntime = trustedPiRuntime((runtime ?? currentPiRuntime)());
        if (!selectedRuntime && (runtime || spawn === nodeSpawn)) {
          finish("child start failed: trusted current Pi runtime unavailable", true, "start_failed");
          return;
        }
        proc = spawn(selectedRuntime?.executable ?? "pi", [...(selectedRuntime?.prefix ?? []), ...args], {
          cwd,
          shell: false,
          stdio: ["ignore", "pipe", "pipe"],
          // Do not forward ambient credentials or arbitrary host settings to
          // the isolated child; only Pi discovery and the fixed guard need
          // these values.
          env: {
            PATH: process.env.PATH ?? "",
            HOME: process.env.HOME ?? home,
            XDG_CONFIG_HOME: process.env.XDG_CONFIG_HOME ?? "",
            ROTTA_CHILD_ROLE: role,
            ROTTA_WORK_ROOT: cwd,
            ...(operationCommand ? { ROTTA_OPERATION_COMMAND: operationCommand } : {}),
            ...(context7Key ? { CONTEXT7_API_KEY: context7Key } : {}),
          },
        });
      } catch (error) {
        finish(
          `child start failed: ${
            error instanceof Error ? error.message : String(error)
          }`,
          true,
          "start_failed",
        );
        return;
      }
      stage = "spawned; no JSON session startup evidence";
      reportChildTelemetry({ phase: "started", role });
      if (signal.aborted) cancel();
      else signal.addEventListener("abort", cancel, { once: true });
      const effectiveTimeout = Math.min(
        limits.maxTimeoutMs,
        Math.max(
          1,
          timeoutMs && timeoutMs > 0 ? timeoutMs : limits.defaultTimeoutMs,
        ),
      );
      if (done) return;
      onUpdate(toolResult("child running", {
        role,
        ...(routing ?? {}),
        running: true,
        timeoutMs: effectiveTimeout,
        elapsedMs: Date.now() - startedAt,
      }));
      {
        timer = setTimeout(() => {
          stop();
          finish(
            `child timed out after ${effectiveTimeout}ms; ${outputObserved ? "partial child output observed" : "no child output observed"}; last known stage: ${stage}; no final handoff accepted. Verify the persisted checkpoint and any operation effect before retry`,
            true,
            "timeout",
          );
        }, effectiveTimeout);
      }
      const consumeLine = (line: string) => {
        try {
          const event = JSON.parse(line);
          if (event?.type === "session") stage = "session started; awaiting model";
          else if (event?.type === "agent_start" || event?.type === "turn_start" || event?.type === "message_start")
            stage = "model request in progress";
          else if (event?.type === "tool_execution_start") stage = "tool execution in progress";
          else if (event?.type === "tool_execution_end") stage = "tool completed; awaiting final handoff";
        } catch { /* non-JSON output is not startup evidence */ }
        const result = childResultForLine(line);
        if (result !== undefined) childResult = result;
      };
      const discardOversizedLine = () => {
        // A too-large authoritative final assistant record cannot be accepted
        // or ignored: it supersedes any earlier result, but we only retain its
        // bounded prefix to classify it.
        if (isAuthoritativeAssistantPrefix(pendingLine)) childResult = null;
      };
      const consumeStdout = (text: string) => {
        if (text) outputObserved = true;
        stdout = appendTailBounded(stdout, text);
        const fragments = text.split("\n");
        for (let index = 0; index < fragments.length; index++) {
          const fragment = fragments[index];
          if (!pendingLineTooLarge) {
            if (
              Buffer.byteLength(pendingLine, "utf8") +
                  Buffer.byteLength(fragment, "utf8") > MAX_OUTPUT_BYTES
            ) {
              pendingLine = appendPrefixBounded(pendingLine, fragment);
              discardOversizedLine();
              pendingLine = "";
              pendingLineTooLarge = true;
            } else pendingLine += fragment;
          }
          if (index < fragments.length - 1) {
            if (!pendingLineTooLarge) {
              consumeLine(pendingLine.replace(/\r$/, ""));
            }
            pendingLine = "";
            pendingLineTooLarge = false;
          }
        }
      };
      proc!.stdout!.on("data", (data) => {
        consumeStdout(outDecoder.write(data));
        // Pi replaces the visible partial result on every update. Publishing
        // the growing diagnostic tail for every JSONL chunk repeatedly copies
        // up to 64 KiB through the event bus and TUI; the collapsed running
        // view does not consume it. Keep the bounded tail for the final result.
      });
      proc!.stderr!.on("data", (data) => {
        if (data.length) outputObserved = true;
        stderr = appendBounded(stderr, errDecoder.write(data));
      });
      proc!.on(
        "error",
        (err) =>
          finish(`child start failed: ${err.message}`, true, "start_failed"),
      );
      proc!.on("close", (code) => {
        closed = true;
        consumeStdout(outDecoder.end());
        // Pi may close without a final newline; it is still one complete JSONL
        // record at EOF. Oversized non-authoritative records stay discarded.
        if (!pendingLineTooLarge && pendingLine) consumeLine(pendingLine);
        stderr = appendBounded(stderr, errDecoder.end());
        if (done) return;
        const result = childResult;
        if (signal.aborted) finish("child cancelled", true, "cancelled");
        else if (code === 0 && result?.status === "success") {
          finish(result.output);
        } else if (result?.status === "error") {
          finish(result.message || "child reported an error without a message", true, "child_error");
        } else {finish(
            code === 0
              ? "child returned invalid result protocol; no final handoff accepted. Verify the persisted checkpoint and any operation effect before retry"
              : `child exited ${code}; inspect bounded child diagnostic and local Pi logs`,
            true,
            code === 0 ? "invalid_result" : "child_error",
          );}
      });
    });
  } finally {
    await fs.promises.rm(promptDir, { recursive: true, force: true });
  }
}

function selectedContext7Key(
  home: string,
  role: Role,
  env: (key: string) => string | undefined,
) {
  // The managed config, rather than a caller/model hint, is the authority for
  // forwarding this optional credential.  Operations cannot use docs tools.
  if (role === "operations") return undefined;
  try {
    const value = JSON.parse(fs.readFileSync(
      path.join(home, ".pi", "agent", "rotta-next", "mcp.json"),
      "utf8",
    ));
    return value?.version === 1 && value?.services?.context7?.enabled === true
      ? env("CONTEXT7_API_KEY")
      : undefined;
  } catch {
    return undefined;
  }
}

const piRoutingRoles = [
  "implementation",
  "reviewer",
  "exploration",
  "operations",
] as const;

// A routing profile is all-or-nothing. A malformed, incomplete, or manually
// edited file must not silently send one child to a managed model while the
// others inherit a different parent model.
function validPiModel(model: unknown): model is string {
  return typeof model === "string" && /^[^\s/|\\]+\/[^\s/|\\]+$/.test(model);
}
function validPiEffort(effort: unknown): effort is string {
  return typeof effort === "string" &&
    ["off", "minimal", "low", "medium", "high", "xhigh", "max"].includes(
      effort,
    );
}
function configuredRoleSelection(
  home: string,
  role: Role,
): RoleSelection | undefined {
  try {
    const value: unknown = JSON.parse(fs.readFileSync(
      path.join(home, ".pi", "agent", "rotta-next", "model-routing.json"),
      "utf8",
    ));
    if (
      !value || typeof value !== "object" ||
      (value as { version?: unknown }).version !== 1
    ) return undefined;
    const roles = (value as { roles?: unknown }).roles;
    if (!roles || typeof roles !== "object" || Array.isArray(roles)) {
      return undefined;
    }
    const entries = Object.entries(roles as Record<string, unknown>);
    if (entries.length === 0) return undefined;
    if (
      entries.length !== piRoutingRoles.length ||
      !piRoutingRoles.every((name) => Object.hasOwn(roles, name))
    ) {
      return undefined;
    }
    const selections: Partial<Record<Role, RoleSelection>> = {};
    for (const [name, assignment] of entries) {
      if (!piRoutingRoles.includes(name as Role)) return undefined;
      if (validPiModel(assignment)) {
        selections[name as Role] = { model: assignment };
        continue;
      }
      if (!assignment || typeof assignment !== "object") return undefined;
      const model = (assignment as { model?: unknown }).model;
      const effort = (assignment as { effort?: unknown }).effort;
      if (
        !validPiModel(model) || (effort !== undefined && !validPiEffort(effort))
      ) {
        return undefined;
      }
      selections[name as Role] = { model, ...(effort ? { effort } : {}) };
    }
    return selections[role];
  } catch {
    return undefined;
  }
}

const Delegate = Type.Object({
  role: Type.Union([
    Type.Literal("implementation"),
    Type.Literal("reviewer"),
    Type.Literal("exploration"),
    Type.Literal("operations"),
  ]),
  task: Type.String(),
  workspace: Type.Optional(Type.String()),
  model: Type.Optional(Type.String()),
  timeoutMs: Type.Optional(Type.Number()),
  requiredTools: Type.Optional(Type.Array(Type.String())),
  operation: Type.Optional(Type.Object({ requestId: Type.String(), command: Type.String(), target: Type.String(), action: Type.String(), effect: Type.String(), artifactPath: Type.String(), revision: Type.Number(), digest: Type.String() })),
});
const CopyWorktreeEnv = Type.Object({
  sourceWorkspace: Type.String(),
  targetWorkspace: Type.String(),
  files: Type.Array(Type.String()),
});
const Question = Type.Object({
  trigger: Type.Union([
    Type.Literal("strict-clarification"),
    Type.Literal("strict-approval"),
    Type.Literal("policy-decision"),
    Type.Literal("external-consent"),
    Type.Literal("vela-unavailable"),
  ]),
  requestId: Type.String(),
  workspace: Type.String(),
  action: Type.String(),
  decision: Type.Optional(Type.String()),
  options: Type.Optional(Type.Array(Type.String())),
  safeOutcome: Type.String(),
  contractPath: Type.Optional(Type.String()),
  contractRevision: Type.Optional(Type.Number()),
  contractDigest: Type.Optional(Type.String()),
  command: Type.Optional(Type.String()),
  target: Type.Optional(Type.String()),
  operationRevision: Type.Optional(Type.Number()),
  operationDigest: Type.Optional(Type.String()),
  operationPath: Type.Optional(Type.String()),
  effect: Type.Optional(Type.String()),
});
const QuestionDraft = Type.Object({
  trigger: Type.Union([
    Type.Literal("strict-clarification"), Type.Literal("strict-approval"),
    Type.Literal("policy-decision"), Type.Literal("external-consent"),
    Type.Literal("vela-unavailable"),
  ]),
  requestId: Type.String(),
  workspace: Type.Optional(Type.String()),
  action: Type.String(),
  decision: Type.String(),
  choices: Type.Optional(Type.Array(Type.String())),
  safeOption: Type.String(),
  contractPath: Type.Optional(Type.String()),
  operationPath: Type.Optional(Type.String()),
});
const IndependentClarifications = Type.Object({
  independent: Type.Literal(true),
  questions: Type.Array(QuestionDraft),
});

async function collectClarifications(ctx: any, questions: QuestionParams[], signal: AbortSignal): Promise<string[] | undefined> {
  if (ctx.mode !== "tui" || typeof ctx.ui.custom !== "function") {
    const answers: string[] = [];
    for (const question of questions) {
      if (signal.aborted) return undefined;
      const answer = await ctx.ui.select(question.decision!, question.options!, { signal });
      if (!answer || answer === question.safeOutcome || !question.options!.includes(answer)) return undefined;
      answers.push(answer);
    }
    return answers;
  }
  let removeAbort = () => {};
  try {
    return await ctx.ui.custom((tui: any, theme: Theme, _keys: unknown, done: (answer: string[] | undefined) => void) => {
    const answers: string[] = [];
    let cursor = 0;
    let selected = 0;
    let completed = false;
    const finish = (value: string[] | undefined) => { if (!completed) { completed = true; done(value); } };
    const onAbort = () => finish(undefined);
    signal.addEventListener("abort", onAbort, { once: true });
    removeAbort = () => signal.removeEventListener("abort", onAbort);
    return {
      render(width: number) {
        const question = questions[cursor];
        const size = Math.max(1, width);
        return [
          truncateToWidth(theme.fg("accent", theme.bold(`Rotta clarification ${cursor + 1}/${questions.length}`)), size),
          ...wrapTextWithAnsi(terminalSafe(question.decision!), size).slice(0, 8).map((line) => truncateToWidth(theme.fg("text", line), size)),
          ...question.options!.map((option, index) => truncateToWidth(theme.fg(index === selected ? "accent" : "muted", `${index === selected ? "›" : " "} ${index + 1}. ${terminalSafe(option)}`), size)),
          truncateToWidth(theme.fg("dim", "↑↓ / 1-4 select • Enter accept • Esc stop"), size),
        ];
      },
      handleInput(data: string) {
        if (completed) return;
        if (data === "\x1b") { finish(undefined); return; }
        if (data === "\x1b[B" || data === "j") selected = (selected + 1) % questions[cursor].options!.length;
        if (data === "\x1b[A" || data === "k") selected = (selected + questions[cursor].options!.length - 1) % questions[cursor].options!.length;
        if (/^[1-4]$/.test(data) && Number(data) <= questions[cursor].options!.length) selected = Number(data) - 1;
        if (data === "\r" || data === "\n") {
          const chosen = questions[cursor].options![selected];
          if (chosen === questions[cursor].safeOutcome) { finish(undefined); return; }
          answers.push(chosen);
          cursor++;
          selected = 0;
          if (cursor === questions.length) finish(answers);
        }
        tui.requestRender();
      },
      invalidate() { tui.requestRender(); },
      dispose() { removeAbort(); },
    };
    });
  } finally { removeAbort(); }
}

function prepareQuestion(draft: QuestionDraftParams, cwd: string): QuestionParams {
  const workspace = selectedWorkspace(cwd, draft.workspace);
  for (const field of ["requestId", "action", "decision", "safeOption"] as const) {
    if (!draft[field]?.trim()) fail(`safe stop: invalid question draft (${field})`);
  }
  const executable = draft.trigger === "external-consent" && !!draft.operationPath;
  const choices = draft.choices ?? [];
  // An exact operation has one runtime-owned approval label. Accept a draft
  // that redundantly names only that label, but never accept custom approvals.
  const fixedApproval = "Approve the exact rendered operation once";
  if (choices.some((choice) => !choice.trim()) ||
    new Set([...choices, draft.safeOption]).size !== choices.length + 1) {
    fail("safe stop: invalid question draft (choices/safeOption)");
  }
  if (!executable && !choices.length) fail("safe stop: question requires a named decision alternative");
  if (executable && choices.length && (choices.length !== 1 || choices[0] !== fixedApproval)) {
    fail("safe stop: exact operation options are fixed by the runtime");
  }
  const prepared: QuestionParams = {
    trigger: draft.trigger, requestId: draft.requestId, workspace,
    action: draft.action, decision: draft.decision,
    options: executable ? [fixedApproval, draft.safeOption] : [...choices, draft.safeOption],
    safeOutcome: draft.safeOption,
  };
  if (draft.trigger === "strict-approval") {
    const contract = currentContract(workspace, draft.contractPath);
    if (!contract || contract.revision === undefined) fail("safe stop: contract identity missing or ambiguous");
    Object.assign(prepared, { contractPath: contract.path,
      contractRevision: contract.revision, contractDigest: contract.digest });
  } else if (draft.contractPath) fail("safe stop: contract artifact only valid for strict approval");
  if (executable) {
    const artifact = currentOperation(workspace, draft.operationPath);
    if (!artifact || artifact.action !== draft.action ||
      draft.safeOption === "Approve the exact rendered operation once") {
      fail(`safe stop: invalid operation draft (${!artifact ? "operationPath/Target/artifact" : artifact.action !== draft.action ? "action" : "safeOption"})`);
    }
    Object.assign(prepared, { operationPath: artifact.path, operationRevision: artifact.revision,
      operationDigest: artifact.digest, command: artifact.command, target: artifact.target,
      effect: artifact.effect });
  } else if (draft.operationPath || draft.trigger === "external-consent") {
    fail("safe stop: exact operation artifact required for external consent");
  }
  return prepared;
}

export function registerRotta(
  pi: ExtensionAPI,
  dependencies: RottaDependencies = {},
) {
  pi.registerTool({
    name: "rotta_copy_worktree_env",
    label: "Copy local worktree env files",
    description: "Copy explicitly named, Git-ignored .env files between registered worktrees of this repository. Never reads values into the agent transcript, overwrites destination files, or connects to external services. Use only for a user-requested local worktree env setup.",
    parameters: CopyWorktreeEnv,
    async execute(_id: string, params: { sourceWorkspace: string; targetWorkspace: string; files: string[] },
      _signal: AbortSignal, _onUpdate: unknown, ctx: ToolContext) {
      const source = selectedWorkspace(ctx.cwd, params.sourceWorkspace);
      const target = selectedWorkspace(ctx.cwd, params.targetWorkspace);
      if (source === target || !params.files.length || params.files.length > 32 ||
        new Set(params.files).size !== params.files.length) {
        fail("safe stop: env copy requires distinct worktrees and 1-32 distinct file paths");
      }
      // Preflight the entire batch before writing anything. File names, never
      // contents, are exposed; production-named files are still only local copies.
      for (const relative of params.files) {
        const parts = relative.split("/");
        const name = parts.at(-1) ?? "";
        if (!parts.every((part) => part && part !== "." && part !== ".." && part !== ".git") ||
          !(name === ".env" || /^\.env\.[a-zA-Z0-9_-]+$/.test(name))) {
          fail("safe stop: env copy accepts only relative .env file paths");
        }
        const from = path.join(source, ...parts), to = path.join(target, ...parts);
        const fromParent = existingCanonical(path.dirname(from));
        const toParent = existingCanonical(path.dirname(to));
        if (!fromParent || !toParent ||
          !(fromParent === source || fromParent.startsWith(source + path.sep)) ||
          !(toParent === target || toParent.startsWith(target + path.sep))) {
          fail("safe stop: env copy parent missing or outside selected worktree");
        }
        let stat: fs.Stats;
        try { stat = fs.lstatSync(from); }
        catch { fail("safe stop: requested env source missing"); }
        if (!stat.isFile() || stat.size > 1024 * 1024 || fs.existsSync(to)) {
          fail("safe stop: env source is not a bounded regular file or destination already exists");
        }
        const ignored = nodeSpawnSync("git", ["-C", source, "check-ignore", "-q", "--", relative],
          { encoding: "utf8", timeout: 5000, stdio: ["ignore", "pipe", "pipe"] });
        if (ignored.status !== 0) fail("safe stop: env source must be Git-ignored");
      }
      const copied: string[] = [];
      for (const relative of params.files) {
        selectedWorkspace(ctx.cwd, source);
        selectedWorkspace(ctx.cwd, target);
        const from = path.join(source, relative), to = path.join(target, relative);
        if (existingCanonical(path.dirname(from)) !== path.dirname(from) ||
          existingCanonical(path.dirname(to)) !== path.dirname(to)) fail("safe stop: env copy path changed");
        const fd = fs.openSync(from, fs.constants.O_RDONLY | fs.constants.O_NOFOLLOW);
        let bytes: Buffer;
        try {
          const stat = fs.fstatSync(fd);
          if (!stat.isFile() || stat.size > 1024 * 1024) fail("safe stop: env source changed");
          bytes = fs.readFileSync(fd);
        } finally { fs.closeSync(fd); }
        // Exclusive creation and a restrictive mode keep existing files intact.
        // Values remain inside this host tool and are never returned to the agent.
        fs.writeFileSync(to, bytes, { flag: "wx", mode: 0o600 });
        copied.push(relative);
      }
      return toolResult(`Copied ${copied.length} local env file(s) into the selected worktree`,
        { copiedPaths: copied, sourceWorkspace: source, targetWorkspace: target });
    },
  });
  pi.registerCommand("rotta:lazygit", {
    description: "Open LazyGit in the current workspace to inspect changes",
    handler: async (_args, ctx) => {
      if (ctx.mode !== "tui" || !ctx.hasUI) {
        ctx.ui.notify("Rotta LazyGit requires an interactive Pi terminal", "warning");
        return;
      }
      if (typeof ctx.ui.custom !== "function") {
        ctx.ui.notify("Rotta LazyGit requires Pi terminal UI support", "warning");
        return;
      }
      try {
        const result = await ctx.ui.custom<{ status: number | null; signal?: string | null; error?: Error }>((tui, _theme, _keys, done) => {
          tui.stop();
          let launched: { status: number | null; signal?: string | null; error?: Error };
          try {
            if (process.stdout.isTTY) process.stdout.write("\x1b[2J\x1b[H");
            launched = (dependencies.lazyGitSpawnSync ?? nodeSpawnSync)("lazygit", [], {
              cwd: ctx.cwd,
              stdio: "inherit",
              env: process.env,
            });
          } catch (error) {
            launched = { status: null, error: error instanceof Error ? error : new Error(String(error)) };
          } finally {
            tui.start();
            tui.requestRender(true);
          }
          done(launched);
          return { render: () => [], invalidate() {} };
        });
        if (result?.error) {
          ctx.ui.notify(result.error.message, "error");
        } else if (result?.signal) {
          ctx.ui.notify(`LazyGit exited on ${result.signal}`, "warning");
        } else if (result?.status !== 0) {
          ctx.ui.notify(`LazyGit exited with code ${result?.status ?? "unknown"}`, "warning");
        }
      } catch (error) {
        ctx.ui.notify(`Unable to open LazyGit: ${error instanceof Error ? error.message : String(error)}`, "error");
      }
    },
  });
  const active = new Map<string, object>();
  const pendingOperations = new Map<string, { session: string; workspace: string; command: string; target: string; targetAncestor: string; targetExists: boolean; targetChain: string; artifactPath: string; action: string; effect: string; revision: number; digest: string }>();
  const preparedOperations = new Map<string, { session: string; workspace: string; request: string; artifactPath: string; digest: string; target: string; ancestor: string; exists: boolean; chain: string }>();
  const preparedIds = new Set<string>();
  const jobs = new Map<string, DelegateActivity>();
  const pendingEdits = new Map<string, string>();
  const observedEdits = new Set<string>();
  let currentSession = "";
  let lastCtx: any;
  const prompts: string[] = [];
  const notifiedDecisions = new Set<string>();
  let lastSettledEntry = "";
  const refreshStatus = (ctx: any) => {
    if (ctx?.mode !== "tui" || typeof ctx.ui?.setStatus !== "function") return;
    const usage = ctx.getContextUsage?.();
    const percent = typeof usage?.percent === "number" ? `${Math.round(usage.percent)}%` : "?";
    const filled = typeof usage?.percent === "number" ? Math.max(0, Math.min(8, Math.round(usage.percent / 12.5))) : null;
    const gauge = filled === null ? "" : ` ${"█".repeat(filled)}${"░".repeat(8 - filled)}`;
    const model = typeof ctx.model?.id === "string" ? ctx.model.id : "model ?";
    const running = [...jobs.values()].filter((job) => job.state === "running").length;
    ctx.ui.setStatus("rotta", `Rotta ${model} • context${gauge} ${percent}${running ? ` • ${running} child running` : ""}`);
  };
  pi.on("session_shutdown", () => {
    active.clear();
    pendingOperations.clear();
    preparedOperations.clear();
    // Keep used request IDs tombstoned across sessions: a serialized old
    // prepared request must not become an unprepared direct call after logout.
    jobs.clear();
    pendingEdits.clear(); observedEdits.clear();
    prompts.length = 0;
    notifiedDecisions.clear();
    lastSettledEntry = "";
    lastCtx?.ui?.setStatus?.("rotta", undefined);
    currentSession = "";
  });
  pi.registerCommand("rotta:stats", {
    description: "Show local Pi session usage and Rotta child activity",
    handler: async (_args, ctx) => {
      const { total, byModel, toolCalls } = sessionUsage(ctx);
      await showRottaPanel(ctx, "Session stats", [
        `Assistant responses: ${total.responses || "usage unavailable"} • model tool calls: ${toolCalls}`,
        `Input ${usageLabel(total.input)} • output ${usageLabel(total.output)} • reasoning ${usageLabel(total.reasoning)} (included in output)`,
        `Cache read ${usageLabel(total.cacheRead)} • write ${usageLabel(total.cacheWrite)}`,
        `Cost ${total.cost === undefined ? "unavailable" : `$${total.cost.toFixed(4)}`} (reported estimate)`,
        "Parent session only; child usage is separate, never added twice",
        ...[...byModel.entries()].slice(0, 10).map(([model, usage]) =>
          `${model}: ${usage.responses} responses • input ${usageLabel(usage.input)} • output ${usageLabel(usage.output)}`),
        "Children:", ...delegateLines([...jobs.values()]),
      ]);
    },
  });
  pi.registerCommand("rotta:agents", {
    description: "Show recent Rotta delegated-role activity",
    handler: async (_args, ctx) => showRottaPanel(ctx, "Agents", delegateLines([...jobs.values()])),
  });
  pi.registerCommand("rotta:status", {
    description: "Show local model, context and active Rotta roles",
    handler: async (_args, ctx) => {
      if (ctx.mode !== "tui") return;
      const context = ctx.getContextUsage?.();
      const { total } = sessionUsage(ctx);
      let branch = "unavailable";
      try {
        const result = await (dependencies.gitExec ?? gitExec)("git", ["--no-optional-locks", "symbolic-ref", "--short", "HEAD"], {
          cwd: ctx.cwd, timeout: 5_000, maxBuffer: 4096, encoding: "utf8",
        });
        branch = terminalSafe(result.stdout).trim() || "detached or unavailable";
      } catch { /* not a Git repository or detached HEAD */ }
      await showRottaPanel(ctx, "Status", [
        `Model: ${ctx.model?.provider && ctx.model?.id ? `${ctx.model.provider}/${ctx.model.id}` : "unavailable"}`,
        `Effort: ${ctx.thinkingLevel ?? "unavailable"}`,
        `Branch: ${branch}`,
        `Context: ${context?.percent === null || context?.percent === undefined ? "unavailable" : `${context.percent.toFixed(1)}%`}`,
        `Reported parent cost: ${total.cost === undefined ? "unavailable" : `$${total.cost.toFixed(4)}`}`,
        `Current session: ${ctx.sessionManager?.getSessionId?.() ?? "unavailable"}`,
        ...delegateLines([...jobs.values()]),
      ]);
    },
  });
  pi.registerCommand("rotta:changes", {
    description: "Inspect read-only Git status and bounded diffs in the current workspace",
    handler: async (_args, ctx) => {
      if (ctx.mode !== "tui") return;
      try {
        const changes = await gitChanges(ctx.cwd, dependencies.gitExec ?? gitExec);
        if (!changes.length) { await showRottaPanel(ctx, "Changes", ["No Git changes observed"]); return; }
        const labels = changes.map((change, index) => `${index + 1}. ${terminalSafe(change.state)} ${terminalSafe(change.path)}`);
        const selected = await ctx.ui.select("Rotta changes • select a file for a bounded diff", labels);
        if (selected === undefined) return;
        const index = labels.indexOf(selected);
        if (index < 0) return;
        const change = changes[index];
        const lines = await gitDiff(ctx.cwd, change, dependencies.gitExec ?? gitExec);
        const canonical = existingCanonical(path.resolve(ctx.cwd, change.path));
        const observation = canonical && observedEdits.has(canonical)
          ? ["Pi parent edit observed; current diff may include other authors"] : [];
        await showRottaPanel(ctx, `Changes • ${terminalSafe(change.path)}`, [...observation, ...lines.flatMap((line) => line.split("\n"))]);
      } catch (error) {
        ctx.ui.notify(`Rotta changes unavailable: ${error instanceof Error && (error as NodeJS.ErrnoException).code === "ERR_CHILD_PROCESS_STDIO_MAXBUFFER" ? "diff exceeded the local display limit" : "Git unavailable or not a repository"}`, "warning");
      }
    },
  });
  pi.registerCommand("rotta:work", {
    description: "Inspect existing Rotta work-record progress without changing it",
    handler: async (_args, ctx) => {
      const records = workViews(ctx.cwd);
      await showRottaPanel(ctx, "Recent work records (activity unverified)", records.length
        ? records.slice(0, 12).flatMap((record) => [
          `${record.id} • ${record.phase}`, `  Next: ${record.next}`, `  Blocker: ${record.blocker}`,
        ]) : ["No readable Rotta work records in this workspace"]);
    },
  });
  pi.registerCommand("rotta:history", {
    description: "View opt-in in-memory Pi prompt navigation for this session",
    handler: async (_args, ctx) => {
      if (process.env.ROTTA_PI_HISTORY !== "1") {
        ctx.ui.notify("Rotta prompt history is off; set ROTTA_PI_HISTORY=1 before starting Pi to opt in", "info");
        return;
      }
      await showRottaPanel(ctx, "Prompt history", prompts.length
        ? prompts.slice().reverse().map((prompt, index) => `${index + 1}. ${prompt}`)
        : ["No prompts captured in this session"]);
    },
  });
  pi.registerCommand("rotta:history:clear", {
    description: "Clear only Rotta's opt-in, in-memory prompt navigation",
    handler: async (_args, ctx) => {
      prompts.length = 0;
      ctx.ui.notify("Rotta in-memory prompt navigation cleared; Pi's session history is unchanged", "info");
    },
  });
  pi.on("input", (event: any, ctx: any) => {
    if (process.env.ROTTA_PI_HISTORY !== "1" || ctx?.mode !== "tui" || event?.source !== "interactive" || typeof event?.text !== "string") return;
    const prompt = terminalSafe(event.text).replace(/\s+/g, " ").trim().slice(0, 240);
    if (!prompt) return;
    prompts.push(prompt);
    if (prompts.length > 20) prompts.shift();
  });
  pi.on("agent_settled", (event: any, ctx: any) => {
    if (process.env.ROTTA_PI_NOTIFICATIONS !== "1" || ctx?.mode !== "tui" || event?.aborted) return;
    const entry = ctx.sessionManager?.getBranch?.()?.at(-1)?.id;
    const key = `${ctx.sessionManager?.getSessionId?.() ?? ""}:${entry ?? ""}`;
    if (!entry || lastSettledEntry === key) return;
    lastSettledEntry = key;
    ctx.ui.notify("Rotta: Pi run finished; verify the work outcome before delivery", "info");
  });
  pi.on("message_end", (event: any, ctx: any) => {
    if (event.message?.role === "assistant") refreshStatus(ctx);
  });
  let compactBuiltinsRegistered = false;
  type PendingFailure = {
    details: Record<string, unknown>;
    timer: ReturnType<typeof globalThis.setTimeout>;
  };
  const delegatedFailures = new Map<string, PendingFailure>();
  const pendingFailureTtlMs = dependencies.pendingFailureTtlMs ??
    PENDING_FAILURE_TTL_MS;
  const pendingFailureCapacity = dependencies.pendingFailureCapacity ??
    PENDING_FAILURE_CAPACITY;
  const scheduleTimeout = dependencies.setTimeout ?? globalThis.setTimeout;
  const cancelTimeout = dependencies.clearTimeout ?? globalThis.clearTimeout;
  const deleteDelegatedFailure = (toolCallId: string) => {
    const pending = delegatedFailures.get(toolCallId);
    if (!pending) return undefined;
    delegatedFailures.delete(toolCallId);
    cancelTimeout(pending.timer);
    return pending.details;
  };
  const rememberDelegatedFailure = (
    toolCallId: string,
    details: Record<string, unknown>,
  ) => {
    deleteDelegatedFailure(toolCallId);
    while (delegatedFailures.size >= pendingFailureCapacity) {
      const oldest = delegatedFailures.keys().next().value;
      if (oldest === undefined) break;
      deleteDelegatedFailure(oldest);
    }
    if (pendingFailureCapacity <= 0) return;
    let pending: PendingFailure;
    const timer = scheduleTimeout(() => {
      if (delegatedFailures.get(toolCallId) === pending) {
        delegatedFailures.delete(toolCallId);
      }
    }, pendingFailureTtlMs);
    pending = { details, timer };
    delegatedFailures.set(toolCallId, pending);
    const unref = (timer as any)?.unref;
    if (typeof unref === "function") unref.call(timer);
  };
  let latestDetail: ActionDetail | undefined;
  const managedAction = (name: string) => {
    const match = /^rotta_(ancora|vela|context7)_(.+)$/.exec(name);
    return match ? { service: match[1], action: match[2] } : undefined;
  };
  let detailOpen = false;
  let diagnosticOpen = false;
  let detailCtx: any;
  const refreshDetail = () => {
    if (detailOpen && latestDetail && detailCtx?.mode === "tui") {
      detailCtx.ui.setWidget("rotta-action-detail", (tui: any, theme: Theme) =>
        detailOverlayComponent(tui, theme, () => latestDetail!, () => diagnosticOpen));
    }
  };
  const showLatest = async (ctx: any) => {
    if (ctx.mode !== "tui") return;
    if (!latestDetail && !detailOpen) {
      ctx.ui.notify("No Rotta action details are available yet", "info");
      return;
    }
    detailOpen = !detailOpen;
    detailCtx = ctx;
    if (!detailOpen) diagnosticOpen = false;
    ctx.ui.setWidget("rotta-action-detail", undefined);
    refreshDetail();
  };
  pi.registerShortcut("ctrl+shift+d" as any, {
    description: "Toggle bounded Rotta diagnostic detail",
    handler: (ctx: any) => {
      if (!detailOpen || ctx.mode !== "tui") return;
      diagnosticOpen = !diagnosticOpen;
      refreshDetail();
    },
  });
  pi.registerShortcut(DETAIL_SHORTCUT as any, {
    description: "Open latest Rotta action details",
    handler: showLatest,
  });
  pi.on("tool_execution_start", (event: any, ctx: any) => {
    if ((event.toolName === "edit" || event.toolName === "write") && typeof event.args?.path === "string" && typeof event.toolCallId === "string" && typeof ctx?.cwd === "string") {
      pendingEdits.set(event.toolCallId, path.resolve(ctx.cwd, event.args.path));
      if (pendingEdits.size > 80) pendingEdits.delete(pendingEdits.keys().next().value!);
    }
    if (event.toolName === "rotta_delegate" && ctx?.mode === "tui") {
      const role = roleLabel(event.args?.role);
      jobs.set(event.toolCallId, { role, state: "running", started: Date.now(),
        requestedModel: typeof event.args?.model === "string" ? event.args.model : undefined });
      if (jobs.size > 24) jobs.delete(jobs.keys().next().value!);
      refreshStatus(ctx);
    }
    const managed = managedAction(event.toolName);
    if (event.toolName !== "rotta_delegate" && !managed) return;
    latestDetail = {
      toolCallId: event.toolCallId,
      service: managed?.service ?? "delegate",
      action: managed?.action ?? roleLabel(event.args?.role),
      state: "running",
      request: event.args,
    };
    refreshDetail();
    if (ctx.mode === "tui") {
      openLatestRottaDetail = () => showLatest(ctx);
      (globalThis as any)[DETAIL_OPENER] = openLatestRottaDetail;
    }
  });
  pi.on("tool_execution_update", (event: any) => {
    if (event.toolName === "rotta_delegate" && jobs.has(event.toolCallId)) {
      const current = jobs.get(event.toolCallId)!;
      const requested = event.partialResult?.details?.requestedModel;
      if (typeof requested === "string" && requested !== "default" && requested !== "unavailable") {
        jobs.set(event.toolCallId, { ...current, requestedModel: requested });
      }
    }
    if (!latestDetail || latestDetail.toolCallId !== event.toolCallId) return;
    latestDetail = {
      ...latestDetail,
      state: "running",
      diagnostic: event.partialResult?.details,
    };
    refreshDetail();
  });
  pi.on("tool_execution_end", (event: any) => {
    const edited = pendingEdits.get(event.toolCallId);
    pendingEdits.delete(event.toolCallId);
    if (edited && !event.isError && lastCtx?.cwd) {
      const file = existingCanonical(edited);
      const root = existingCanonical(lastCtx.cwd);
      if (file && root && file.startsWith(root + path.sep)) {
        observedEdits.add(file);
        if (observedEdits.size > 80) observedEdits.delete(observedEdits.values().next().value!);
      }
    }
    if (event.toolName === "rotta_delegate") {
      const current = jobs.get(event.toolCallId);
      if (current) {
        const details = event.result?.details;
        jobs.set(event.toolCallId, { ...current,
          state: details?.cancelled ? "cancelled" : event.isError ? "failed" : "complete",
          elapsedMs: typeof details?.elapsedMs === "number" ? details.elapsedMs : Date.now() - current.started,
          model: typeof details?.responseModel === "string" ? details.responseModel : undefined,
          requestedModel: typeof details?.requestedModel === "string" && details.requestedModel !== "default"
            ? details.requestedModel : current.requestedModel,
          usage: measuredUsage(details?.responseUsage),
        });
        refreshStatus(lastCtx);
      }
    }
    const managed = managedAction(event.toolName);
    if (event.toolName !== "rotta_delegate" && !managed) return;
    if (latestDetail?.toolCallId && latestDetail.toolCallId !== event.toolCallId) return;
    latestDetail = {
      ...(latestDetail ?? { service: managed?.service ?? "delegate", action: managed?.action ?? "unknown", request: event.args ?? {} }),
      state: event.isError ? "failed" : "complete",
      response: event.result?.content,
      diagnostic: event.result?.details?.diagnostic ??
        event.result?.details ?? latestDetail?.diagnostic,
    };
    refreshDetail();
  });
  // Pi correctly turns thrown tool errors into red error results, but that
  // synthesis cannot retain custom details from the value that caused the
  // throw. Restore those details through Pi's supported tool_result transform
  // while leaving its authoritative isError flag untouched.
  pi.on("tool_result", (event: any) => {
    if (event.toolName !== "rotta_delegate") return;
    const details = deleteDelegatedFailure(event.toolCallId);
    if (!event.isError || !details) return;
    return { details };
  });
  const spawn = dependencies.spawn ?? nodeSpawn;
  const env = dependencies.env ?? ((key: string) => process.env[key]);
  const homeDir = dependencies.home ?? os.homedir;
  const limits = {
    defaultTimeoutMs: dependencies.defaultTimeoutMs ?? DEFAULT_TIMEOUT_MS,
    maxTimeoutMs: dependencies.maxTimeoutMs ?? MAX_TIMEOUT_MS,
    killGraceMs: dependencies.killGraceMs ?? KILL_GRACE_MS,
  };
  // One bridge belongs to this extension lifetime, not to every agent turn.
  const bridgeHome = dependencies.home
    ? homeDir()
    : process.env.HOME ?? os.homedir();
  const bridge = loadMCPBridge(pi, bridgeHome, dependencies);
  pi.on("session_start", async (_event: unknown, ctx: any) => {
    // Await dynamic bridge loading: its own session_start hook may be registered
    // after dispatch has already begun. Native rows must exist before a prompt.
    await (await bridge).start();
    if (ctx.mode !== "tui") return;
    const session = ctx.sessionManager?.getSessionId?.() ?? "";
    if (session !== currentSession) {
      jobs.clear(); pendingEdits.clear(); observedEdits.clear();
      prompts.length = 0; notifiedDecisions.clear(); lastSettledEntry = "";
      currentSession = session;
    }
    lastCtx = ctx;
    refreshStatus(ctx);
    if (!compactBuiltinsRegistered) {
      try {
        const definitions = compactBuiltinDefinitions(ctx.cwd, {
          version: dependencies.piVersion,
          factories: dependencies.builtinFactories,
        });
        // Shadow only after TUI mode and the exact Pi version/factory-result
        // contract are known. Spreading retains execution and schema identity.
        pi.registerTool(definitions.bash);
        pi.registerTool(definitions.edit);
      } catch (error) {
        const reason = error instanceof Error ? error.message : String(error);
        ctx.ui.notify(`Rotta compact built-ins disabled: ${reason}`, "error");
      }
      compactBuiltinsRegistered = true;
    }
    const rottaLabel = await installedRottaLabel(pi);
    ctx.ui.setHeader((_tui: unknown, theme: Theme) => ({
      render: (width: number) => renderRottaHeader(theme, width, rottaLabel),
      invalidate() {},
    }));
  });
  pi.on("before_agent_start", async (event: { systemPrompt: string }) => {
    const home = homeDir();
    const root = path.join(home, ".pi", "agent", "rotta-next");
    const core = path.join(root, "rotta-core", "SKILL.md");
    const orchestrator = path.join(root, "rotta-orchestrator", "SKILL.md");
    let policies: string;
    try {
      policies = await Promise.all(
        [core, orchestrator].map(async (file) =>
          await fs.promises.readFile(file, "utf8")
        ),
      ).then((parts) => parts.join("\n\n"));
    } catch {
      return fail(
        "safe stop: installed Rotta core/orchestrator policy is missing or unreadable",
      );
    }
    try {
      await (await bridge).activate();
    } catch {
      /* status tool exposes boot failure without preventing Pi startup */
    }
    return {
      systemPrompt:
        `${event.systemPrompt}\n\n<rotta-installed-policy source="${root}">\n${policies}\n</rotta-installed-policy>\nManaged MCP tools, when discovered, use rotta_ancora_*, rotta_vela_*, and rotta_context7_* names; only call role-permitted tools.`,
    };
  });
  pi.registerTool({
    name: "rotta_mcp_status",
    label: "Rotta MCP status",
    description:
      "Show observed managed MCP service state and sanitized failure reason.",
    parameters: Type.Object({}),
    async execute() {
      try {
        return toolResult(JSON.stringify((await bridge).status()), {
          status: (await bridge).status(),
          native: (await bridge).nativeStatus(),
        });
      } catch (error) {
        return toolResult("MCP bridge unavailable", {
          status: {
            bridge: "unavailable",
            reason: trimUtf8(
              error instanceof Error ? error.message : String(error),
              1024,
            ),
          },
        });
      }
    },
  });
  pi.registerTool({
    name: "rotta_delegate",
    label: "Rotta delegate (role-isolated child)",
    description: "Delegate to an isolated role. The operations role requires a fresh one-time operation binding and executes its exact command; never use operations for diagnostic-only inspection. Use read or a non-operations role for local diagnostics.",
    parameters: Delegate,
    renderCall: renderDelegateCall,
    renderResult: renderDelegateResult,
    async execute(
      _id: string,
      params: DelegateParams,
      signal: AbortSignal,
      onUpdate: (result: ReturnType<typeof toolResult>) => void,
      ctx: ToolContext,
    ) {
      const selected = params.role as Role;
      if (!roles[selected]) fail("unknown child role");
      const missing = [...new Set(params.requiredTools ?? [])].filter((name) =>
        !roles[selected].tools.some((tool) => tool === name)
      );
      if (missing.length) fail(`safe stop: ${selected} child lacks required tool(s): ${missing.slice(0, 8).join(", ")}; use an authorized parent/source fallback or stop. No child started`);
      let operationCommand: string | undefined;
      if (selected === "operations") {
        if (!params.operation) fail("safe stop: operations delegation requires a fresh one-time operation binding; diagnostic-only inspection must use read or a non-operations role. No child started and no operation ran");
        const session = ctx.sessionManager.getSessionId();
        const workspace = selectedWorkspace(ctx.cwd, params.workspace);
        const pending = pendingOperations.get(params.operation.requestId);
        // Consume before dispatch, including failed starts and cancellations.
        pendingOperations.delete(params.operation.requestId);
        const requestedTarget = operationTarget(workspace, params.operation.target);
        const artifact = currentOperation(workspace, params.operation.artifactPath);
        if (!pending || !session || !workspace || pending.session !== session || pending.workspace !== workspace ||
          !requestedTarget || requestedTarget.path !== pending.target || requestedTarget.ancestor !== pending.targetAncestor || requestedTarget.exists !== pending.targetExists || requestedTarget.chain !== pending.targetChain || params.operation.command !== pending.command || params.operation.action !== pending.action || params.operation.effect !== pending.effect ||
          !artifact || artifact.path !== pending.artifactPath || artifact.digest !== pending.digest || artifact.revision !== pending.revision ||
          params.operation.revision !== pending.revision || params.operation.digest !== pending.digest ||
          artifact.command !== pending.command || artifact.action !== pending.action || artifact.target !== pending.target || artifact.targetAncestor !== pending.targetAncestor || artifact.targetExists !== pending.targetExists || artifact.targetChain !== pending.targetChain || artifact.effect !== pending.effect) {
          fail("safe stop: operation binding missing or stale at dispatch (session, request, target, or artifact identity); authorization consumed; no child started and no operation ran");
        }
        operationCommand = pending.command;
      } else if (params.operation) fail("operation binding only valid for operations");
      const childWorkspace = params.workspace ? selectedWorkspace(ctx.cwd, params.workspace) : ctx.cwd;
      const inherited = ctx.model
        ? `${ctx.model.provider}/${ctx.model.id}`
        : undefined;
      const configured = configuredRoleSelection(homeDir(), selected);
      const resolvedModel = params.model ?? configured?.model ?? inherited;
      const resolvedEffort = params.model ? undefined : configured?.effort;
      const routingSource = params.model
        ? "explicit"
        : configured
        ? "role profile"
        : inherited
        ? "parent model"
        : "child default";
      const result = await runChild(
        childWorkspace,
        homeDir(),
        selected,
        params.task,
        resolvedModel,
        resolvedEffort,
        params.timeoutMs,
        limits,
        signal,
        onUpdate,
        spawn,
        selectedContext7Key(homeDir(), selected, env),
        {
          source: routingSource,
          requestedModel: resolvedModel ?? "default",
          requestedEffort: resolvedEffort ??
            (routingSource === "parent model" ? "unavailable" : "default"),
        },
        operationCommand,
        dependencies.piRuntime,
      );
      if (result.details.failed) {
        rememberDelegatedFailure(_id, result.details);
        fail(result.content[0].text);
      }
      return result;
    },
  });
  pi.registerTool({
    name: "rotta_prepare_question",
    label: "Prepare Rotta question",
    description: "Read-only preparation before rotta_question: provide decision alternatives and a separate safeOption. For external-consent with operationPath omit choices: the runtime supplies the fixed approval option. Derives canonical workspace and artifact identity; pass its request unchanged to rotta_question. Preparation never asks the user or records approval; a changed artifact requires new preparation.",
    parameters: QuestionDraft,
    async execute(_id: string, draft: QuestionDraftParams, _signal: AbortSignal, _onUpdate: unknown, ctx: ToolContext) {
      // Replacement attempts revoke earlier consent before draft validation,
      // even when the new operation is malformed and cannot be prepared.
      if (draft.trigger === "external-consent" || draft.operationPath) pendingOperations.delete(draft.requestId);
      preparedOperations.delete(draft.requestId);
      const request = prepareQuestion(draft, ctx.cwd);
      if (request.trigger === "external-consent") {
        preparedIds.add(request.requestId);
        const session = ctx.sessionManager.getSessionId();
        const identity = operationTarget(request.workspace, request.target);
        if (!session || !identity) fail("safe stop: prepared operation identity unavailable");
        preparedOperations.set(request.requestId, {
          session, workspace: request.workspace, request: JSON.stringify(request),
          artifactPath: request.operationPath!, digest: request.operationDigest!,
          target: identity.path, ancestor: identity.ancestor, exists: identity.exists, chain: identity.chain,
        });
      }
      return toolResult(JSON.stringify(request), { request });
    },
  });
  pi.registerTool({
    name: "rotta_clarify_independent",
    label: "Rotta independent clarifications",
    description: "Collect two or three independently prepared non-binding Strict clarifications in one Pi view. Requires explicit independence; never approve contracts, policy or operations.",
    parameters: IndependentClarifications,
    async execute(_id: string, params: { independent: true; questions: QuestionDraftParams[] }, signal: AbortSignal, _onUpdate: unknown, ctx: ToolContext) {
      if (!ctx.hasUI || signal.aborted || params.independent !== true || params.questions.length < 2 || params.questions.length > 3 ||
        params.questions.some((question) => question.trigger !== "strict-clarification" || (question.choices?.length ?? 0) > 3 || !question.requestId?.trim() ||
          question.safeOption !== "Stop / use safe defaults" || question.decision.length > 480 || question.choices?.some((choice) => choice.length > 120)) ||
        new Set(params.questions.map((question) => question.requestId)).size !== params.questions.length) {
        fail("safe stop: independent clarifications require two or three distinct non-binding questions");
      }
      const prepared = params.questions.map((question) => prepareQuestion(question, ctx.cwd));
      const session = ctx.sessionManager.getSessionId();
      if (!session) fail("safe stop: active session unavailable");
      const workspace = selectedWorkspace(ctx.cwd, params.questions[0].workspace);
      if (prepared.some((question) => question.workspace !== workspace)) fail("safe stop: independent clarifications require one workspace");
      const answers = await collectClarifications(ctx, prepared, signal);
      if (!answers || signal.aborted || ctx.sessionManager.getSessionId() !== session || selectedWorkspace(ctx.cwd, params.questions[0].workspace) !== workspace ||
        answers.length !== prepared.length || answers.some((answer, index) => !prepared[index].options?.includes(answer))) {
        fail("safe stop: clarification cancelled or current session/workspace changed");
      }
      return toolResult(JSON.stringify(prepared.map((question, index) => ({ requestId: question.requestId, answer: answers[index] }))), {
        answers: prepared.map((question, index) => ({ requestId: question.requestId, answer: answers[index] })),
        authority: "clarification only",
      });
    },
  });
  pi.registerTool({
    name: "rotta_question",
    label: "Rotta decision",
    description: "Ask one bound governance decision. Use rotta_prepare_question first with explicit decision, choices and safeOption; pass its request unchanged. This tool revalidates the current artifact and session before accepting an answer. Direct callers must supply safeOutcome as an exact option. A changed digest or conflicting revision fails safely.",
    parameters: Question,
    async execute(
      toolCallId: string,
      params: QuestionParams,
      signal: AbortSignal,
      _onUpdate: unknown,
      ctx: ToolContext,
    ) {
      // Revoke same-ID consent before UI/workspace/preparation checks can fail.
      // No other request's pending authorization is affected.
      if (params.trigger === "external-consent" || params.operationPath) pendingOperations.delete(params.requestId);
      if (
        !ctx.hasUI || signal.aborted
      ) {
        return fail(
          "safe stop: interactive UI or decision binding unavailable",
        );
      }
      let workspace: string;
      try { workspace = selectedWorkspace(ctx.cwd, params.workspace); }
      catch { return fail("safe stop: approval identity mismatch (workspace)"); }
      if (!workspace || existingCanonical(params.workspace) !== workspace) {
        return fail("safe stop: approval identity mismatch (workspace)");
      }
      const executable = params.trigger === "external-consent";
      if (executable) {
        const prepared = preparedOperations.get(params.requestId);
        preparedOperations.delete(params.requestId);
        // A known prepared request cannot fall back to the legacy direct-call
        // route, even after a rejected or already consumed preparation.
        if (preparedIds.has(params.requestId)) {
          const identity = operationTarget(workspace, params.target);
          if (!prepared || prepared.session !== ctx.sessionManager.getSessionId() ||
            prepared.workspace !== workspace || prepared.request !== JSON.stringify(params) ||
            prepared.artifactPath !== params.operationPath || prepared.digest !== params.operationDigest ||
            !identity || identity.path !== prepared.target || identity.ancestor !== prepared.ancestor ||
            identity.exists !== prepared.exists || identity.chain !== prepared.chain) {
            return fail("safe stop: stale prepared operation identity (request, session, target, or artifact)");
          }
        }
        const target = operationTarget(workspace, params.target);
        const artifact = currentOperation(workspace, params.operationPath);
        const mismatches: string[] = [];
        // A malformed packet has no trusted fields to compare. Name only its
        // structural source, never echo any portion of the rejected bytes.
        if (!params.operationPath || !artifact) mismatches.push("operationPath");
        if (!/^[a-f0-9]{64}$/.test(params.operationDigest ?? "") ||
          (artifact && artifact.digest !== params.operationDigest)) mismatches.push("operationDigest");
        if (!Number.isSafeInteger(params.operationRevision) ||
          (artifact && artifact.revision !== params.operationRevision)) mismatches.push("operationRevision");
        if (!params.action?.trim() || (artifact && artifact.action !== params.action)) mismatches.push("action");
        if (!params.command?.trim() || (artifact && artifact.command !== params.command)) mismatches.push("command");
        if (!target || (artifact && artifact.target !== target.path)) mismatches.push("target");
        if (!params.effect || (artifact && artifact.effect !== params.effect)) mismatches.push("effect");
        if (mismatches.length) return fail(`safe stop: incomplete exact operation binding (${mismatches.join(", ")})`);
      }
      const bindingIdentity = executable ? operationTarget(workspace, params.target) : undefined;
      const bindingTarget = bindingIdentity?.path;
      const operationArtifact = executable ? currentOperation(workspace, params.operationPath) : undefined;
      const options = executable
        ? ["Approve the exact rendered operation once", params.safeOutcome]
        : params.options;
      const missingQuestion = [
        ...(!params.requestId?.trim() ? ["requestId"] : []),
        ...(!params.action?.trim() ? ["action"] : []),
        ...(!params.safeOutcome?.trim() ? ["safeOutcome"] : []),
        ...(!executable && !params.decision?.trim() ? ["decision"] : []),
        ...(!options?.length ? ["options"] : []),
        ...(options?.length && !options.includes(params.safeOutcome) ? ["safeOutcome"] : []),
      ];
      if (missingQuestion.length) return fail(`safe stop: incomplete question binding (${[...new Set(missingQuestion)].join(", ")})`);
      if (!options) return fail("safe stop: incomplete question binding (options)");
      if (new Set(options).size !== options.length ||
        (executable && params.safeOutcome === options[0])) return fail("safe stop: invalid question options");
      const contract = params.trigger === "strict-approval"
        ? currentContract(workspace, params.contractPath)
        : undefined;
      if (params.trigger === "strict-approval") {
        const mismatches: string[] = [];
        if (!contract) mismatches.push("contractPath");
        else {
          if (contract.digest !== params.contractDigest) {
            mismatches.push("digest");
          }
          if (contract.revision === undefined) {
            mismatches.push("revision metadata missing or ambiguous");
          } else if (params.contractRevision !== undefined && contract.revision !== params.contractRevision) {
            mismatches.push("revision");
          }
        }
        if (mismatches.length > 0) {
          return fail(
            `safe stop: approval identity mismatch (${mismatches.join(", ")})`,
          );
        }
      }
      // The verified file, not a duplicated model-supplied number, defines the
      // revision shown to the user and bound to their answer.
      const decision = executable
        ? `${params.decision?.trim() ? `${params.decision.trim()}\n\n` : ""}Action: ${params.action}\nCommand: ${params.command}\nTarget: ${bindingTarget ?? existingCanonical(params.target!)}\nEffect: ${params.effect}\nWorkspace: ${workspace}\nArtifact: ${operationArtifact!.path}\nDigest: ${params.operationDigest}\nRevision: ${params.operationRevision}\nScope: one execution`
        : contract
        ? `${params.decision}\n\nExact contract: ${contract.path}\nSHA-256: ${contract.digest}\nRevision: ${contract.revision}`
        : params.decision!;
      const bindingDigest = params.operationDigest;
      const session = ctx.sessionManager.getSessionId();
      if (!session) return fail("safe stop: active session unavailable");
      const key = `${session}\0${workspace}\0${params.action}`;
      const binding = Object.freeze({
        toolCallId,
        requestId: params.requestId,
        session,
        workspace,
        action: params.action,
        decision,
        options: [...options],
        safeOutcome: params.safeOutcome,
        contractPath: contract?.path,
        contractRevision: contract?.revision,
        contractDigest: contract?.digest,
        operationTarget: bindingTarget,
        operationDigest: bindingDigest,
      });
      active.set(key, binding);
      let resolveAbort!: (value: null) => void;
      const aborted = new Promise<null>((resolve) => { resolveAbort = resolve; });
      const onAbort = () => resolveAbort(null);
      signal.addEventListener("abort", onAbort, { once: true });
      let choice: string | null | undefined;
      let uiFailed = false;
      pi.events.emit("herdr:blocked", { active: true, label: decision });
      if ((ctx as any).mode === "tui" && !notifiedDecisions.has(params.requestId) && typeof (ctx.ui as any).notify === "function") {
        notifiedDecisions.add(params.requestId);
        if (notifiedDecisions.size > 32) notifiedDecisions.delete(notifiedDecisions.values().next().value!);
        (ctx.ui as any).notify("Rotta decision needs your answer", "info");
      }
      try {
        try {
          choice = await Promise.race([
            ctx.ui.select(decision, options, { signal }),
            aborted,
          ]);
        } catch {
          uiFailed = true;
        }
        if (uiFailed) {
          return fail(signal.aborted ? "safe stop: decision cancelled" : "safe stop: question UI failed");
        }
        if (signal.aborted || !choice) return fail("safe stop: decision cancelled");
        if (active.get(key) !== binding || ctx.sessionManager.getSessionId() !== session || selectedWorkspace(ctx.cwd, params.workspace) !== workspace) {
          return fail("safe stop: stale decision session or workspace");
        }
        if (!options.includes(choice)) return fail("safe stop: invalid decision selection");
        if (executable) {
          const latest = currentOperation(workspace, params.operationPath);
          const latestTarget = operationTarget(workspace, params.target);
          if (!latestTarget || latestTarget.path !== bindingTarget || latestTarget.ancestor !== bindingIdentity?.ancestor || latestTarget.exists !== bindingIdentity?.exists || latestTarget.chain !== bindingIdentity?.chain || params.operationDigest !== bindingDigest ||
            !latest || latest.path !== operationArtifact?.path || latest.digest !== bindingDigest ||
            latest.revision !== params.operationRevision || latest.action !== params.action ||
            latest.command !== params.command || latest.target !== bindingTarget || latest.targetAncestor !== bindingIdentity?.ancestor || latest.targetExists !== bindingIdentity?.exists || latest.targetChain !== bindingIdentity?.chain || latest.effect !== params.effect) {
            return fail("safe stop: stale operation identity");
          }
        }
        if (contract) {
          const latest = currentContract(workspace, params.contractPath);
          if (!latest || latest.path !== binding.contractPath || latest.digest !== binding.contractDigest ||
            latest.revision !== binding.contractRevision) return fail("safe stop: stale contract identity");
        }
        if (executable && choice === "Approve the exact rendered operation once") {
          pendingOperations.set(params.requestId, {
            session, workspace, command: params.command!, target: bindingTarget!,
            targetAncestor: bindingIdentity!.ancestor, targetExists: bindingIdentity!.exists, targetChain: bindingIdentity!.chain,
            revision: params.operationRevision!, digest: params.operationDigest!,
            artifactPath: operationArtifact!.path, action: params.action, effect: params.effect!,
          });
        }
        return toolResult(choice, {
          toolCallId,
          requestId: params.requestId,
          session,
          workspace,
          action: params.action,
          decision,
        });
      } finally {
        signal.removeEventListener("abort", onAbort);
        pi.events.emit("herdr:blocked", {
          active: false,
          label: decision,
        });
        if (active.get(key) === binding) active.delete(key);
      }
    },
  });
}

export default function registerRottaExtension(
  pi: ExtensionAPI,
  dependencies: RottaDependencies = {},
) {
  registerRotta(pi, dependencies);
}
