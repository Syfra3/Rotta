// Global Pi extension. Pi discovers this exact file at ~/.pi/agent/extensions/rotta.ts.
import { spawn as nodeSpawn } from "node:child_process";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { createHash } from "node:crypto";
import { StringDecoder } from "node:string_decoder";
import { Type } from "typebox";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

const MAX_OUTPUT_BYTES = 64 * 1024;
const KILL_GRACE_MS = 5_000;
const DEFAULT_TIMEOUT_MS = 60_000;
const MAX_TIMEOUT_MS = 120_000;
const QUESTION_TIMEOUT_MS = 30_000;
const roles = {
  implementation: { skill: "rotta-impl", tools: ["read", "write", "edit"] },
  reviewer: { skill: "rotta-review", tools: ["read", "grep", "find", "ls"] },
  exploration: {
    skill: "rotta-explore",
    tools: ["read", "grep", "find", "ls"],
  },
  operations: { skill: "rotta-ops", tools: ["read"] },
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
};
type DelegateParams = {
  role: string;
  task: string;
  model?: string;
  timeoutMs?: number;
};
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
    const revision = /\*\*Revision:\*\*\s*`?(\d+)`?/m.exec(
      bytes.toString("utf8"),
    )?.[1];
    return {
      path: candidate,
      digest: createHash("sha256").update(bytes).digest("hex"),
      revision: revision ? Number(revision) : undefined,
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
function policyPrompt(home: string, role: Role) {
  const root = path.join(home, ".pi", "agent", "rotta-next");
  return `Read and obey these exact installed policies before acting: ${
    path.join(root, "rotta-core", "SKILL.md")
  } and ${
    path.join(root, roles[role].skill, "SKILL.md")
  }. You are ${role}. Do not use unavailable tools or delegate. Return one JSON object only: {"status":"success","output":"..."} or {"status":"error","message":"..."}.`;
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
function validChildResult(stdout: string) {
  let finalAssistant = "";
  for (const line of stdout.split(/\r?\n/).reverse()) {
    try {
      const value = JSON.parse(line);
      if (
        value?.type === "message_end" && value.message?.role === "assistant"
      ) {
        const content = value.message.content;
        finalAssistant = typeof content === "string"
          ? content
          : Array.isArray(content)
          ? content.filter((part: unknown) =>
            typeof part === "object" && part !== null &&
            (part as { type?: string }).type === "text"
          ).map((part: { text?: unknown }) =>
            typeof part.text === "string" ? part.text : ""
          ).join("")
          : "";
        if (!finalAssistant) continue;
        try {
          const nested = JSON.parse(finalAssistant);
          if (
            nested && typeof nested === "object" &&
            ((nested.status === "success" &&
              typeof nested.output === "string") ||
              (nested.status === "error" && typeof nested.message === "string"))
          ) {
            return nested as { status: "success"; output: string } | {
              status: "error";
              message: string;
            };
          }
        } catch {
          return null;
        }
      }
      if (
        value && typeof value === "object" &&
        typeof value.status === "string" &&
        ((value.status === "success" && typeof value.output === "string") ||
          (value.status === "error" && typeof value.message === "string"))
      ) {
        return value as { status: "success"; output: string } | {
          status: "error";
          message: string;
        };
      }
    } catch { /* next line */ }
  }
  return finalAssistant ? null : null;
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
  args.push(task);
  try {
    return await new Promise<ReturnType<typeof toolResult>>((resolve) => {
      let done = false,
        closed = false,
        stopped = false,
        timer: ReturnType<typeof setTimeout> | undefined;
      let proc: ReturnType<Spawn> | undefined;
      let stdout = "", stderr = "";
      const outDecoder = new StringDecoder("utf8"),
        errDecoder = new StringDecoder("utf8");
      const finish = (text: string, error = false, reason?: string) => {
        if (!done) {
          done = true;
          if (timer) clearTimeout(timer);
          signal.removeEventListener("abort", stop);
          resolve(toolResult(trimUtf8(text), {
            role,
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
          env: { ...process.env, ROTTA_CHILD_ROLE: role, ROTTA_WORK_ROOT: cwd },
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
      {
        timer = setTimeout(() => {
          stop();
          finish("child timed out", true, "timeout");
        }, effectiveTimeout);
      }
      proc!.stdout!.on("data", (data) => {
        stdout = appendBounded(stdout, outDecoder.write(data));
        onUpdate(toolResult(stdout || "(running...)", { role, running: true }));
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
        stdout = appendBounded(stdout, outDecoder.end());
        stderr = appendBounded(stderr, errDecoder.end());
        if (done) return;
        const result = validChildResult(stdout);
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
  const homeDir = dependencies.home ?? os.homedir;
  const limits = {
    defaultTimeoutMs: dependencies.defaultTimeoutMs ?? DEFAULT_TIMEOUT_MS,
    maxTimeoutMs: dependencies.maxTimeoutMs ?? MAX_TIMEOUT_MS,
    killGraceMs: dependencies.killGraceMs ?? KILL_GRACE_MS,
  };
  const questionTimeout = dependencies.questionTimeoutMs ?? QUESTION_TIMEOUT_MS;
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
    return {
      systemPrompt:
        `${event.systemPrompt}\n\n<rotta-installed-policy source="${root}">\n${policies}\n</rotta-installed-policy>`,
    };
  });
  pi.registerTool({
    name: "rotta_delegate",
    label: "Rotta delegate",
    description: "Delegate only to a fixed isolated non-parent Rotta role.",
    parameters: Delegate,
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
      const result = await runChild(
        ctx.cwd,
        homeDir(),
        selected,
        params.task,
        params.model ?? inherited,
        params.timeoutMs,
        limits,
        signal,
        onUpdate,
        spawn,
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

export default function registerRottaExtension(pi: ExtensionAPI) {
  registerRotta(pi);
}
