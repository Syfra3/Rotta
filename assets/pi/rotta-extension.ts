// Global Pi extension. Pi discovers this exact file at ~/.pi/agent/extensions/rotta.ts.
import { spawn as nodeSpawn } from "node:child_process";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { pathToFileURL } from "node:url";
import { createHash } from "node:crypto";
import { StringDecoder } from "node:string_decoder";
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
const DETAIL_LIMIT_BYTES = 64 * 1024;
const PENDING_FAILURE_TTL_MS = 60_000;
const PENDING_FAILURE_CAPACITY = 64;
const DETAIL_SHORTCUT = "ctrl+shift+o";
const SUPPORTED_BUILTIN_PI_VERSION = "0.87.1";
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
export type RottaDependencies = {
  spawn?: Spawn;
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
  model?: string;
  timeoutMs?: number;
  operation?: { requestId: string; command: string; target: string; action: string; effect: string; artifactPath: string; revision: number; digest: string };
};
type RoleSelection = { model: string; effort?: string };
type QuestionParams = {
  trigger: string;
  requestId: string;
  workspace: string;
  action: string;
  decision: string;
  options: string[];
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
    const target = existingCanonical(fields.get("Target")!);
    if (!target || target !== fields.get("Target") ||
      !(target === workspace || target.startsWith(workspace + path.sep))) return null;
    return { path: candidate, digest: createHash("sha256").update(bytes).digest("hex"),
      revision, action: fields.get("Action")!, command: fields.get("Command")!,
      target, effect: fields.get("Effect")! };
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
  // Pi 0.87.1 formatOutput appends exactly one of these three terminal forms.
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
  const version = source.version ?? VERSION;
  if (version !== SUPPORTED_BUILTIN_PI_VERSION) {
    throw new Error(`unsupported Pi version ${version}; expected ${SUPPORTED_BUILTIN_PI_VERSION}`);
  }
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
    .replace(/Bearer\s+[^\s"']+/gi, "Bearer [redacted]")
    .replace(/(["']?(?:api[_-]?key|token|authorization|password)["']?\s*[:=]\s*)["']?[^\s,"'}]+["']?/gi, "$1[redacted]");
  return trimUtf8(redacted, DETAIL_LIMIT_BYTES);
}

export function detailOverlayComponent(
  tui: { requestRender(): void },
  theme: Theme,
  detail: ActionDetail,
  done: () => void,
) {
  let offset = 0;
  let maxOffset = 0;
  const body = [
    `${detail.service} / ${detail.action} / ${detail.state}`,
    "",
    "Request:",
    safeDetail(detail.request),
    ...(detail.response === undefined ? [] : ["", "Response:", safeDetail(detail.response)]),
    ...(detail.diagnostic === undefined ? [] : ["", "Diagnostic:", safeDetail(detail.diagnostic)]),
  ].join("\n");
  return {
    handleInput(data: string) {
      if (matchesKey(data, "escape") || matchesKey(data, "ctrl+c")) done();
      else if (matchesKey(data, "up") || matchesKey(data, "pageUp")) {
        offset = Math.max(0, offset - (matchesKey(data, "pageUp") ? 10 : 1));
        tui.requestRender();
      } else if (matchesKey(data, "down") || matchesKey(data, "pageDown")) {
        offset = Math.min(maxOffset, offset + (matchesKey(data, "pageDown") ? 10 : 1));
        tui.requestRender();
      }
    },
    render(width: number) {
      const inner = Math.max(1, width - 2);
      const wrapped = body.split("\n").flatMap((line) =>
        wrapTextWithAnsi(line, inner)
      );
      maxOffset = Math.max(0, wrapped.length - 20);
      offset = Math.min(offset, maxOffset);
      const page = wrapped.slice(offset, offset + 20);
      const border = theme.fg("border", "│");
      const lines = [
        theme.fg("border", `╭${"─".repeat(inner)}╮`),
        ...page.map((line) => `${border}${truncateToWidth(line, inner)}${border}`),
        `${border}${truncateToWidth(theme.fg("dim", " ↑↓/PgUp/PgDn scroll • Esc close"), inner)}${border}`,
        theme.fg("border", `╰${"─".repeat(inner)}╯`),
      ];
      return lines;
    },
    invalidate() {},
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
      activate: async () => {},
      close: async () => {},
      status: () => ({ bridge: { state: "unavailable", reason } }),
    };
  }
}
type ChildMetadata = {
  responseModel?: string;
  providerThinkingLevel?: string;
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
      const metadata = {
        ...(responseModel ? { responseModel } : {}),
        ...(providerThinkingLevel ? { providerThinkingLevel } : {}),
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
  args.push(task);
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
      const outDecoder = new StringDecoder("utf8"),
        errDecoder = new StringDecoder("utf8");
      const rolePrefix = `[${role}] `;
      const finish = (text: string, error = false, reason?: string) => {
        if (!done) {
          done = true;
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
            outcome: error ? "error" : "success",
            cancelled: signal.aborted,
            ...(stdout ? { diagnostic: stdout } : {}),
            ...(reason ? { reason } : {}),
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
        proc = spawn("pi", args, {
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
      if (signal.aborted) cancel();
      else signal.addEventListener("abort", cancel, { once: true });
      const effectiveTimeout = Math.min(
        limits.maxTimeoutMs,
        Math.max(
          1,
          timeoutMs && timeoutMs > 0 ? timeoutMs : limits.defaultTimeoutMs,
        ),
      );
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
            `child timed out after ${effectiveTimeout}ms`,
            true,
            "timeout",
          );
        }, effectiveTimeout);
      }
      const consumeLine = (line: string) => {
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
        onUpdate(
          toolResult("child running", {
            role,
            ...(routing ?? {}),
            running: true,
            timeoutMs: effectiveTimeout,
            elapsedMs: Date.now() - startedAt,
            diagnostic: stdout,
          }),
        );
      });
      proc!.stderr!.on("data", (data) => {
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
              ? "child returned invalid result protocol"
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
  model: Type.Optional(Type.String()),
  timeoutMs: Type.Optional(Type.Number()),
  operation: Type.Optional(Type.Object({ requestId: Type.String(), command: Type.String(), target: Type.String(), action: Type.String(), effect: Type.String(), artifactPath: Type.String(), revision: Type.Number(), digest: Type.String() })),
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
  decision: Type.String(),
  options: Type.Array(Type.String()),
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

export function registerRotta(
  pi: ExtensionAPI,
  dependencies: RottaDependencies = {},
) {
  const active = new Map<string, object>();
  const pendingOperations = new Map<string, { session: string; workspace: string; command: string; target: string; artifactPath: string; action: string; effect: string; revision: number; digest: string }>();
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
  const showLatest = async (ctx: any) => {
    if (ctx.mode !== "tui") return;
    if (!latestDetail) {
      ctx.ui.notify("No Rotta action details are available yet", "info");
      return;
    }
    await ctx.ui.custom(
      (tui: any, theme: Theme, _keys: unknown, done: () => void) =>
        detailOverlayComponent(tui, theme, latestDetail!, done),
      { overlay: true, overlayOptions: { width: "80%", maxHeight: 24, anchor: "center", margin: 1 } },
    );
  };
  pi.registerShortcut(DETAIL_SHORTCUT as any, {
    description: "Open latest Rotta action details",
    handler: showLatest,
  });
  pi.on("tool_execution_start", (event: any, ctx: any) => {
    const managed = managedAction(event.toolName);
    if (event.toolName !== "rotta_delegate" && !managed) return;
    latestDetail = {
      toolCallId: event.toolCallId,
      service: managed?.service ?? "delegate",
      action: managed?.action ?? roleLabel(event.args?.role),
      state: "running",
      request: event.args,
    };
    if (ctx.mode === "tui") {
      openLatestRottaDetail = () => showLatest(ctx);
      (globalThis as any)[DETAIL_OPENER] = openLatestRottaDetail;
    }
  });
  pi.on("tool_execution_update", (event: any) => {
    if (!latestDetail || latestDetail.toolCallId !== event.toolCallId) return;
    latestDetail = {
      ...latestDetail,
      state: "running",
      diagnostic: event.partialResult?.details,
    };
  });
  pi.on("tool_execution_end", (event: any) => {
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
    if (ctx.mode !== "tui") return;
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
    description: "Delegate only to a fixed isolated non-parent Rotta role.",
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
      let operationCommand: string | undefined;
      if (selected === "operations") {
        if (!params.operation) fail("safe stop: operation authorization missing or stale");
        const session = ctx.sessionManager.getSessionId();
        const workspace = existingCanonical(ctx.cwd);
        const pending = pendingOperations.get(params.operation.requestId);
        // Consume before dispatch, including failed starts and cancellations.
        pendingOperations.delete(params.operation.requestId);
        const requestedTarget = existingCanonical(params.operation.target);
        const artifact = currentOperation(workspace, params.operation.artifactPath);
        if (!pending || !session || !workspace || pending.session !== session || pending.workspace !== workspace ||
          existingCanonical(pending.target) !== pending.target || requestedTarget !== pending.target || params.operation.command !== pending.command || params.operation.action !== pending.action || params.operation.effect !== pending.effect ||
          !artifact || artifact.path !== pending.artifactPath || artifact.digest !== pending.digest || artifact.revision !== pending.revision ||
          params.operation.revision !== pending.revision || params.operation.digest !== pending.digest ||
          artifact.command !== pending.command || artifact.action !== pending.action || artifact.target !== pending.target || artifact.effect !== pending.effect) {
          fail("safe stop: operation authorization missing or stale");
        }
        operationCommand = pending.command;
      } else if (params.operation) fail("operation binding only valid for operations");
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
        ctx.cwd,
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
      );
      if (result.details.failed) {
        rememberDelegatedFailure(_id, result.details);
        fail(result.content[0].text);
      }
      return result;
    },
  });
  pi.registerTool({
    name: "rotta_question",
    label: "Rotta decision",
    description: "Ask one bound governance decision. For strict-approval supply contractDigest as SHA-256 of the exact file bytes. Pi reads the contract revision from those bytes; contractRevision is optional but, when supplied, must match. The contract needs one `Revision: 1` line or `(revision 1)` H1 suffix. A changed digest or conflicting revision fails safely.",
    parameters: Question,
    async execute(
      toolCallId: string,
      params: QuestionParams,
      signal: AbortSignal,
      _onUpdate: unknown,
      ctx: ToolContext,
    ) {
      if (
        !ctx.hasUI || signal.aborted || params.options.length === 0 ||
        new Set(params.options).size !== params.options.length
      ) {
        return fail(
          "safe stop: interactive UI or decision binding unavailable",
        );
      }
      const workspace = existingCanonical(ctx.cwd);
      const suppliedWorkspace = existingCanonical(params.workspace);
      if (!workspace || suppliedWorkspace !== workspace) {
        return fail("safe stop: approval identity mismatch (workspace)");
      }
      const executionFields = [params.command, params.target, params.operationPath,
        params.operationRevision, params.operationDigest, params.effect];
      const executable = params.trigger === "external-consent" &&
        executionFields.some((field) => field !== undefined);
      if (params.trigger === "external-consent" && executable) {
        // A replaced, rejected, or cancelled request cannot leave an earlier
        // authorization under the same identity available for later dispatch.
        pendingOperations.delete(params.requestId);
        const target = params.target && existingCanonical(params.target);
        const artifact = currentOperation(workspace, params.operationPath);
        if (!artifact || artifact.path !== path.resolve(workspace, params.operationPath!) ||
          artifact.digest !== params.operationDigest || artifact.revision !== params.operationRevision ||
          artifact.action !== params.action || artifact.command !== params.command ||
          artifact.target !== target || artifact.effect !== params.effect ||
          !target || !(target === workspace || target.startsWith(workspace + path.sep)) ||
          !params.command?.trim() || !Number.isSafeInteger(params.operationRevision) ||
          !/^[a-f0-9]{64}$/.test(params.operationDigest ?? "") ||
          params.options.length !== 2 || params.options[0] !== "Approve the exact rendered operation once" ||
          params.options[1] !== params.safeOutcome ||
          !params.decision.includes(`Action: ${artifact.action}\nCommand: ${artifact.command}\nTarget: ${artifact.target}\nEffect: ${artifact.effect}\nWorkspace: ${workspace}\nArtifact: ${artifact.path}\nDigest: ${artifact.digest}\nRevision: ${artifact.revision}\nScope: one execution`)) {
          return fail("safe stop: incomplete exact operation binding");
        }
      }
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
      const decision = contract
        ? `${params.decision}\n\nExact contract: ${contract.path}\nSHA-256: ${contract.digest}\nRevision: ${contract.revision}`
        : params.decision;
      const bindingTarget = executable ? existingCanonical(params.target!) : undefined;
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
        options: [...params.options],
        safeOutcome: params.safeOutcome,
        contractPath: contract?.path,
        contractRevision: contract?.revision,
        contractDigest: contract?.digest,
        operationTarget: bindingTarget,
        operationDigest: bindingDigest,
      });
      active.set(key, binding);
      const aborted = new Promise<null>((resolve) =>
        signal.addEventListener("abort", () => resolve(null), { once: true })
      );
      let choice: string | null | undefined;
      let uiFailed = false;
      pi.events.emit("herdr:blocked", {
        active: true,
        label: decision,
      });
      try {
        try {
          choice = await Promise.race([
            ctx.ui.select(decision, params.options, { signal }),
            aborted,
          ]);
        } catch {
          uiFailed = true;
        }
        if (uiFailed) {
          return fail(signal.aborted
            ? "safe stop: cancelled, stale, or invalid decision"
            : "safe stop: question UI failed");
        }
        if (
          signal.aborted || active.get(key) !== binding || !choice ||
          ctx.sessionManager.getSessionId() !== session ||
          existingCanonical(ctx.cwd) !== workspace ||
          (executable &&
            (existingCanonical(params.target!) !== bindingTarget ||
              params.operationDigest !== bindingDigest ||
              (() => { const latest = currentOperation(workspace, params.operationPath);
                return !latest || latest.path !== path.resolve(workspace, params.operationPath!) ||
                  latest.digest !== bindingDigest || latest.revision !== params.operationRevision ||
                  latest.action !== params.action || latest.command !== params.command ||
                  latest.target !== bindingTarget || latest.effect !== params.effect;
              })())) ||
          !params.options.includes(choice) ||
          (contract && (() => {
            const latest = currentContract(workspace, params.contractPath);
            return !latest || latest.path !== binding.contractPath ||
              latest.digest !== binding.contractDigest ||
              latest.revision !== binding.contractRevision;
          })())
        ) {
          return fail("safe stop: cancelled, stale, or invalid decision");
        }
        if (executable && choice === "Approve the exact rendered operation once") {
          pendingOperations.set(params.requestId, {
            session, workspace, command: params.command!, target: existingCanonical(params.target!),
            revision: params.operationRevision!, digest: params.operationDigest!,
            artifactPath: path.resolve(workspace, params.operationPath!), action: params.action, effect: params.effect!,
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
