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
  type ExtensionAPI,
  type Theme,
  VERSION,
} from "@earendil-works/pi-coding-agent";

const MAX_OUTPUT_BYTES = 64 * 1024;
const KILL_GRACE_MS = 5_000;
const DEFAULT_TIMEOUT_MS = 180_000;
const MAX_TIMEOUT_MS = 600_000;
const QUESTION_TIMEOUT_MS = 30_000;
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
  operations: { skill: "rotta-ops", tools: ["read", ...memoryTools] },
} as const;
type Role = keyof typeof roles;
type Spawn = typeof nodeSpawn;
export type RottaDependencies = {
  spawn?: Spawn;
  home?: () => string;
  defaultTimeoutMs?: number;
  maxTimeoutMs?: number;
  killGraceMs?: number;
  questionTimeoutMs?: number;
  // Test seam; production reads only the explicitly selected optional key.
  env?: (key: string) => string | undefined;
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
};
type ToolContext = {
  cwd: string;
  hasUI: boolean;
  model?: { provider: string; id: string };
  ui: {
    select(
      title: string,
      options: string[],
      selectOptions?: { signal: AbortSignal; timeout: number },
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
      /^(?:[ \t]{0,3})(?:Revision:[ \t]*(\d+)|\*\*Revision:\*\*[ \t]*`(\d+)`)[ \t]*\r?$/gm,
    )].map((match) => match[1] ?? match[2]);
    return {
      path: candidate,
      digest: createHash("sha256").update(bytes).digest("hex"),
      revision: revisionLines.length === 1 && revisions.length === 1
        ? Number(revisions[0])
        : undefined,
    };
  } catch {
    return null;
  }
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
function truncateAnsiLine(line: string, width: number): string {
  if (width <= 0) return "";
  let result = "";
  let visible = 0;
  for (let index = 0; index < line.length && visible < width;) {
    const escape = /^\x1b\[[0-?]*[ -/]*[@-~]/.exec(line.slice(index));
    if (escape) {
      result += escape[0];
      index += escape[0].length;
      continue;
    }
    const character = String.fromCodePoint(line.codePointAt(index)!);
    result += character;
    index += character.length;
    visible++;
  }
  return result;
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

function textComponent(text: string) {
  return {
    render(width: number) {
      return text.split("\n").map((line) =>
        line.length > width ? line.slice(0, Math.max(0, width - 1)) + "…" : line
      );
    },
    invalidate() {},
  };
}
function renderDelegateCall(args: Record<string, unknown>, theme: any) {
  const role = roleLabel(args.role);
  const task = summarizeTask(args.task);
  return textComponent(
    `${theme.fg("toolTitle", theme.bold("delegate"))} ${
      theme.fg("accent", role)
    } ${theme.fg("dim", task)}`,
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
  },
) {
  const role = roleLabel(result.details?.role ?? context.args?.role);
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
    const line = `${theme.fg("warning", "● running")} ${
      theme.fg("accent", role)
    } ${theme.fg("muted", elapsed + timeout)} ${
      theme.fg("dim", "expand for details")
    }`;
    if (!options.expanded) return textComponent(line);
    return textComponent(
      `${line}\n${
        theme.fg(
          "toolOutput",
          contentText(result) || "waiting for child output…",
        )
      }`,
    );
  }
  const failed = result.isError || result.details?.failed === true;
  const icon = failed
    ? theme.fg("error", "✗ failed")
    : theme.fg("success", "✓ complete");
  const reason = typeof result.details?.reason === "string"
    ? ` ${theme.fg("dim", result.details.reason)}`
    : "";
  const line = `${icon} ${theme.fg("accent", role)} ${
    theme.fg("muted", elapsed)
  }${reason} ${theme.fg("dim", "expand for details")}`;
  if (!options.expanded) return textComponent(line);
  return textComponent(
    `${line}\n${
      theme.fg("toolOutput", contentText(result) || "(no delegation output)")
    }`,
  );
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
type ChildResult = { status: "success"; output: string } | {
  status: "error";
  message: string;
};

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
      if (envelope) return envelope;
      // Only message_end may carry plain prose. JSON- or fence-shaped failures
      // invalidate an earlier result rather than being accepted as prose.
      return envelopeLike(finalAssistant)
        ? null
        : { status: "success", output: trimUtf8(finalAssistant) };
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
  effort?: string,
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
          signal.removeEventListener("abort", stop);
          resolve(toolResult(trimUtf8(rolePrefix + text), {
            role,
            elapsedMs: Date.now() - startedAt,
            cancelled: signal.aborted,
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
      onUpdate(toolResult(`${rolePrefix}child running`, {
        role,
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
          toolResult(rolePrefix + (stdout || "child running"), {
            role,
            running: true,
            timeoutMs: effectiveTimeout,
            elapsedMs: Date.now() - startedAt,
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
          finish(result.message, true, "child_error");
        } else {finish(
            code === 0
              ? "child returned invalid result protocol"
              : stderr || `child exited ${code}`,
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
});

export function registerRotta(
  pi: ExtensionAPI,
  dependencies: RottaDependencies = {},
) {
  const active = new Map<string, object>();
  const spawn = dependencies.spawn ?? nodeSpawn;
  const env = dependencies.env ?? ((key: string) => process.env[key]);
  const homeDir = dependencies.home ?? os.homedir;
  const limits = {
    defaultTimeoutMs: dependencies.defaultTimeoutMs ?? DEFAULT_TIMEOUT_MS,
    maxTimeoutMs: dependencies.maxTimeoutMs ?? MAX_TIMEOUT_MS,
    killGraceMs: dependencies.killGraceMs ?? KILL_GRACE_MS,
  };
  const questionTimeout = dependencies.questionTimeoutMs ?? QUESTION_TIMEOUT_MS;
  // One bridge belongs to this extension lifetime, not to every agent turn.
  const bridgeHome = dependencies.home
    ? homeDir()
    : process.env.HOME ?? os.homedir();
  const bridge = loadMCPBridge(pi, bridgeHome, dependencies);
  pi.on("session_start", async (_event, ctx) => {
    if (ctx.mode !== "tui") return;
    const rottaLabel = await installedRottaLabel(pi);
    ctx.ui.setHeader((_tui, theme) => ({
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
      const inherited = ctx.model
        ? `${ctx.model.provider}/${ctx.model.id}`
        : undefined;
      const configured = configuredRoleSelection(homeDir(), selected);
      const resolvedModel = params.model ?? configured?.model ?? inherited;
      const resolvedEffort = params.model ? undefined : configured?.effort;
      const result = await runChild(
        ctx.cwd,
        homeDir(),
        selected,
        params.task,
        resolvedModel,
        params.timeoutMs,
        limits,
        signal,
        onUpdate,
        spawn,
        selectedContext7Key(homeDir(), selected, env),
        resolvedEffort,
      );
      if (result.details.failed) fail(result.content[0].text);
      return result;
    },
  });
  pi.registerTool({
    name: "rotta_question",
    label: "Rotta decision",
    description: "Ask one bound governance decision.",
    parameters: Question,
    async execute(
      toolCallId: string,
      params: QuestionParams,
      signal: AbortSignal,
      _onUpdate: unknown,
      ctx: ToolContext,
    ) {
      if (
        !ctx.hasUI || signal.aborted || params.workspace !== ctx.cwd ||
        params.options.length === 0 ||
        new Set(params.options).size !== params.options.length
      ) {
        return fail(
          "safe stop: interactive UI or decision binding unavailable",
        );
      }
      const contract = params.trigger === "strict-approval"
        ? currentContract(ctx.cwd, params.contractPath)
        : undefined;
      if (
        params.trigger === "strict-approval" &&
        (!contract || contract.digest !== params.contractDigest ||
          contract.revision !== params.contractRevision)
      ) return fail("safe stop: approval identity mismatch");
      const session = ctx.sessionManager.getSessionId();
      if (!session) return fail("safe stop: active session unavailable");
      const key = `${session}\0${ctx.cwd}\0${params.action}`;
      const binding = Object.freeze({
        toolCallId,
        requestId: params.requestId,
        session,
        workspace: ctx.cwd,
        action: params.action,
        decision: params.decision,
        options: [...params.options],
        safeOutcome: params.safeOutcome,
        contractPath: contract?.path,
        contractRevision: contract?.revision,
        contractDigest: contract?.digest,
      });
      active.set(key, binding);
      const aborted = new Promise<null>((resolve) =>
        signal.addEventListener("abort", () => resolve(null), { once: true })
      );
      try {
        const choice = await Promise.race([
          ctx.ui.select(params.decision, params.options, {
            signal,
            timeout: questionTimeout,
          }),
          aborted,
        ]);
        if (
          active.get(key) !== binding || !choice ||
          !params.options.includes(choice) ||
          (contract && (() => {
            const latest = currentContract(ctx.cwd, params.contractPath);
            return !latest || latest.path !== binding.contractPath ||
              latest.digest !== binding.contractDigest ||
              latest.revision !== binding.contractRevision;
          })())
        ) {
          return fail("safe stop: cancelled, stale, or invalid decision");
        }
        return toolResult(choice, {
          toolCallId,
          requestId: params.requestId,
          session,
          workspace: ctx.cwd,
          action: params.action,
          decision: params.decision,
        });
      } catch {
        return fail("safe stop: question UI failed");
      } finally {
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
