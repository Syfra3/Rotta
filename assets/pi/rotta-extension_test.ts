import { EventEmitter } from "node:events";
import { createHash } from "node:crypto";
import { registerRotta } from "./rotta-extension.ts";
import guard, { isProtectedWorkPath } from "./rotta-child-guard.ts";

function assert(value: unknown, message = "assertion failed"): asserts value {
  if (!value) throw new Error(message);
}
type Tool = { name: string; execute: (...args: any[]) => Promise<any> };
function host() {
  const tools: Tool[] = [];
  const selects: unknown[] = [];
  const events: Record<string, unknown> = {};
  const pi = {
    registerTool(tool: Tool) {
      tools.push(tool);
    },
    on(event: string, handler: unknown) {
      events[event] = handler;
    },
  };
  const ctx = {
    cwd: "/workspace",
    hasUI: true,
    model: { provider: "test", id: "parent-model" },
    sessionManager: { getSessionId: () => "trusted-session" },
    ui: {
      select: async (title: string, options: string[]) => {
        selects.push({ title, options, multiple: false, custom: false });
        return options[0];
      },
    },
  };
  return { pi, tools, selects, events, ctx };
}
function signal() {
  return new AbortController();
}
function fakeSpawn(
  events: { stdout?: string; stderr?: string; code?: number; wait?: boolean },
) {
  const calls: unknown[][] = [];
  let killed: string[] = [];
  const spawn = ((...args: unknown[]) => {
    calls.push(args);
    const proc: any = new EventEmitter();
    proc.stdout = new EventEmitter();
    proc.stderr = new EventEmitter();
    proc.killed = false;
    proc.kill = (kind: string) => {
      killed.push(kind);
      proc.killed = true;
      return true;
    };
    queueMicrotask(() => {
      if (events.stdout) proc.stdout.emit("data", Buffer.from(events.stdout));
      if (events.stderr) proc.stderr.emit("data", Buffer.from(events.stderr));
      if (!events.wait) proc.emit("close", events.code ?? 0);
    });
    return proc;
  }) as any;
  return { spawn, calls, killed };
}
function tool(tools: Tool[], name: string) {
  const found = tools.find((item) => item.name === name);
  if (!found) throw new Error(`missing ${name}`);
  return found;
}

Deno.test("installed entrypoint registers real Pi tools and isolates the child invocation", async () => {
  const fixture = fakeSpawn({
    stdout:
      '{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"{\\"status\\":\\"success\\",\\"output\\":\\"reviewed\\"}"}]}}\n',
  });
  const mock = host();
  registerRotta(mock.pi as any, {
    spawn: fixture.spawn,
    home: () => "/test-home",
  });
  assert(
    mock.tools.length === 2,
    "default registration did not expose both tools",
  );
  const result = await tool(mock.tools, "rotta_delegate").execute(
    "call",
    { role: "reviewer", task: "review" },
    signal().signal,
    () => {},
    mock.ctx,
  );
  assert(!result.isError && result.content[0].text === "reviewed");
  const args = fixture.calls[0][1] as string[];
  assert(
    args.includes("--no-extensions") && args.includes("--no-skills") &&
      args.includes("--no-context-files"),
  );
  assert(
    args.includes("--tools") && args.includes("read,grep,find,ls"),
    "reviewer received a shell-capable tool",
  );
  assert(
    args.includes("--model") && args.includes("test/parent-model"),
    "parent model was not inherited",
  );
  assert(
    args.includes("/test-home/.pi/agent/extensions/rotta-child-guard.ts"),
    "child guard not activated",
  );
});

Deno.test("parent activation injects only exact installed core and orchestrator policies", async () => {
  const home = Deno.makeTempDirSync();
  try {
    Deno.mkdirSync(`${home}/.pi/agent/rotta-next/rotta-core`, {
      recursive: true,
    });
    Deno.mkdirSync(`${home}/.pi/agent/rotta-next/rotta-orchestrator`, {
      recursive: true,
    });
    Deno.writeTextFileSync(
      `${home}/.pi/agent/rotta-next/rotta-core/SKILL.md`,
      "CORE",
    );
    Deno.writeTextFileSync(
      `${home}/.pi/agent/rotta-next/rotta-orchestrator/SKILL.md`,
      "ORCHESTRATOR",
    );
    const mock = host();
    registerRotta(mock.pi as any, { home: () => home });
    const activated = await (mock.events.before_agent_start as (
      event: { systemPrompt: string },
    ) => Promise<{ systemPrompt: string }>)({ systemPrompt: "BASE" });
    assert(
      activated.systemPrompt.includes("CORE") &&
        activated.systemPrompt.includes("ORCHESTRATOR") &&
        activated.systemPrompt.includes(`${home}/.pi/agent/rotta-next`),
    );
  } finally {
    Deno.removeSync(home, { recursive: true });
  }
});

Deno.test("every child role gets its own isolated process allowlist", async () => {
  for (
    const [role, tools] of Object.entries({
      implementation: "read,write,edit",
      reviewer: "read,grep,find,ls",
      exploration: "read,grep,find,ls",
      operations: "read",
    })
  ) {
    const fixture = fakeSpawn({
      stdout:
        '{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"{\\"status\\":\\"success\\",\\"output\\":\\"ok\\"}"}]}}\n',
    });
    const mock = host();
    registerRotta(mock.pi as any, {
      spawn: fixture.spawn,
      home: () => "/test-home",
    });
    await tool(mock.tools, "rotta_delegate").execute(
      "call",
      { role, task: "task" },
      signal().signal,
      () => {},
      mock.ctx,
    );
    const args = fixture.calls[0][1] as string[];
    assert(
      args.includes(tools) && args.includes("--no-session"),
      `${role} process was not isolated/allowlisted`,
    );
  }
});

Deno.test("transport bounds UTF-8 output, rejects invalid protocol, and terminates cancellation or timeout", async () => {
  const huge = "é".repeat(70_000);
  const capped = fakeSpawn({
    stdout: `{"status":"success","output":"${huge}"}\n`,
  });
  const mock = host();
  registerRotta(mock.pi as any, {
    spawn: capped.spawn,
    home: () => "/test-home",
  });
  const delegate = tool(mock.tools, "rotta_delegate");
  await delegate.execute(
    "call",
    { role: "reviewer", task: "review" },
    signal().signal,
    () => {},
    mock.ctx,
  ).then(() => {
    throw new Error("oversized protocol succeeded");
  }, () => {});
  const invalid = fakeSpawn({ stdout: "not-json\n" });
  registerRotta(mock.pi as any, {
    spawn: invalid.spawn,
    home: () => "/test-home",
  });
  await tool(mock.tools.slice(2), "rotta_delegate").execute(
    "call",
    { role: "reviewer", task: "review" },
    signal().signal,
    () => {},
    mock.ctx,
  ).then(() => {
    throw new Error("invalid protocol succeeded");
  }, () => {});
  const slow = fakeSpawn({ wait: true });
  const timeoutHost = host();
  registerRotta(timeoutHost.pi as any, {
    spawn: slow.spawn,
    home: () => "/test-home",
  });
  await tool(timeoutHost.tools, "rotta_delegate").execute(
    "call",
    { role: "reviewer", task: "review", timeoutMs: 5 },
    signal().signal,
    () => {},
    timeoutHost.ctx,
  ).then(() => {
    throw new Error("timeout succeeded");
  }, () => {});
  assert(slow.killed.includes("SIGTERM"));
  const cancelled = fakeSpawn({ wait: true });
  const cancelHost = host();
  registerRotta(cancelHost.pi as any, {
    spawn: cancelled.spawn,
    home: () => "/test-home",
  });
  const controller = signal();
  const pending = tool(cancelHost.tools, "rotta_delegate").execute(
    "call",
    { role: "reviewer", task: "review" },
    controller.signal,
    () => {},
    cancelHost.ctx,
  );
  controller.abort();
  await pending.then(() => {
    throw new Error("cancellation succeeded");
  }, () => {});
  assert(cancelled.killed.includes("SIGTERM"));

  const resistant = fakeSpawn({ wait: true });
  const resistantHost = host();
  registerRotta(resistantHost.pi as any, {
    spawn: resistant.spawn,
    home: () => "/test-home",
    defaultTimeoutMs: 2,
    maxTimeoutMs: 3,
    killGraceMs: 2,
  });
  await tool(resistantHost.tools, "rotta_delegate").execute(
    "call",
    { role: "reviewer", task: "review", timeoutMs: 999999 },
    signal().signal,
    () => {},
    resistantHost.ctx,
  ).then(() => {
    throw new Error("clamped timeout succeeded");
  }, () => {});
  await new Promise((resolve) => setTimeout(resolve, 5));
  assert(
    resistant.killed.includes("SIGTERM") &&
      resistant.killed.includes("SIGKILL"),
    "TERM-resistant child was not killed",
  );
  const spawnErrorHost = host();
  registerRotta(spawnErrorHost.pi as any, {
    spawn: (() => {
      throw new Error("no pi");
    }) as any,
    home: () => "/test-home",
  });
  await tool(spawnErrorHost.tools, "rotta_delegate").execute(
    "call",
    { role: "reviewer", task: "review" },
    signal().signal,
    () => {},
    spawnErrorHost.ctx,
  ).then(() => {
    throw new Error("spawn throw succeeded");
  }, () => {});
});

Deno.test("child guard blocks all canonical and symlinked work-record write targets", async () => {
  const fixture = Deno.makeTempDirSync();
  const external = Deno.makeTempDirSync();
  try {
    Deno.mkdirSync(`${fixture}/.rotta`, { recursive: true });
    Deno.symlinkSync(external, `${fixture}/.rotta/work`);
    assert(
      isProtectedWorkPath(`${fixture}/.rotta/work/new/missing.md`, fixture),
      "nonexistent child target escaped guard",
    );
    assert(
      isProtectedWorkPath(`${external}/existing.md`, fixture),
      "symlink target escaped guard",
    );
    assert(!isProtectedWorkPath(`${fixture}/notes.md`, fixture));
    const handlers:
      ((event: { toolName: string; input: unknown }) => Promise<unknown>)[] =
        [];
    guard({
      on(
        _event: string,
        handler: (
          event: { toolName: string; input: unknown },
        ) => Promise<unknown>,
      ) {
        handlers.push(handler);
      },
    } as any);
    const blocked = await handlers[0]({
      toolName: "write",
      input: { path: `${Deno.cwd()}/.rotta/work/new/missing.md` },
    });
    assert(
      (blocked as { block?: boolean }).block,
      "actual write route was not blocked",
    );
    const shell = await handlers[0]({
      toolName: "bash",
      input: { command: "true" },
    });
    assert((shell as { block?: boolean }).block, "shell route was not blocked");
  } finally {
    Deno.removeSync(fixture, { recursive: true });
    Deno.removeSync(external, { recursive: true });
  }
});

Deno.test("question adapter binds current session/cwd/action and fails closed", async () => {
  const mock = host();
  const project = Deno.makeTempDirSync();
  const contractPath = `${project}/.rotta/strict/new-feature.md`;
  Deno.mkdirSync(`${project}/.rotta/strict`, { recursive: true });
  const contractBytes = "# New feature\n\n**Revision:** `7`\n";
  Deno.writeTextFileSync(contractPath, contractBytes);
  mock.ctx.cwd = project;
  registerRotta(mock.pi as any);
  const question = tool(mock.tools, "rotta_question");
  const request = {
    trigger: "strict-approval",
    requestId: "request",
    workspace: project,
    action: "approve",
    decision: "Approve revision 7",
    options: ["Approve", "Stop"],
    safeOutcome: "Stop",
    contractPath: ".rotta/strict/new-feature.md",
    contractRevision: 7,
    contractDigest: createHash("sha256").update(contractBytes).digest("hex"),
  };
  for (
    const trigger of [
      "strict-clarification",
      "strict-approval",
      "policy-decision",
      "external-consent",
      "vela-unavailable",
    ]
  ) {
    const answer = await question.execute(
      "call-" + trigger,
      { ...request, trigger },
      signal().signal,
      () => {},
      mock.ctx,
    );
    assert(answer.content[0].text === "Approve", `${trigger} was not accepted`);
  }
  const accepted = await question.execute(
    "call",
    request,
    signal().signal,
    () => {},
    mock.ctx,
  );
  assert(
    !accepted.isError && accepted.details.session === "trusted-session" &&
      accepted.details.workspace === project,
  );
  assert(
    JSON.stringify(mock.selects[0]).includes('"multiple":false') &&
      JSON.stringify(mock.selects[0]).includes('"custom":false'),
  );
  await question.execute(
    "call",
    { ...request, workspace: "/other" },
    signal().signal,
    () => {},
    mock.ctx,
  ).then(() => {
    throw new Error("mismatched workspace was accepted");
  }, () => {});
  await question.execute(
    "call",
    request,
    signal().signal,
    () => {},
    { ...mock.ctx, hasUI: false },
  ).then(() => {
    throw new Error("headless succeeded");
  }, () => {});
  await question.execute(
    "call",
    { ...request, contractDigest: "wrong" },
    signal().signal,
    () => {},
    mock.ctx,
  ).then(() => {
    throw new Error("bad approval succeeded");
  }, () => {});

  const waiting = host();
  waiting.ctx.ui.select = (async (
    _title: string,
    _options: string[],
    options?: { signal: AbortSignal; timeout: number },
  ) =>
    await new Promise<string | null>((resolve) => {
      options?.signal.addEventListener("abort", () => resolve(null), {
        once: true,
      });
      setTimeout(() => resolve(null), options?.timeout);
    })) as any;
  registerRotta(waiting.pi as any, { questionTimeoutMs: 2 });
  const timedController = signal();
  await tool(waiting.tools, "rotta_question").execute(
    "timeout",
    request,
    timedController.signal,
    () => {},
    waiting.ctx,
  ).then(() => {
    throw new Error("question timeout succeeded");
  }, () => {});
  const abortController = signal();
  const aborted = tool(waiting.tools, "rotta_question").execute(
    "abort",
    request,
    abortController.signal,
    () => {},
    waiting.ctx,
  );
  abortController.abort();
  await aborted.then(() => {
    throw new Error("question abort succeeded");
  }, () => {});
  let release!: (answer: string) => void;
  const staleHost = host();
  staleHost.ctx.cwd = project;
  staleHost.ctx.ui.select =
    ((_: string, __: string[]) =>
      new Promise<string>((resolve) => {
        release = resolve;
      })) as any;
  registerRotta(staleHost.pi as any);
  const staleQuestion = tool(staleHost.tools, "rotta_question");
  const pendingApproval = staleQuestion.execute(
    "pending",
    request,
    signal().signal,
    () => {},
    staleHost.ctx,
  );
  Deno.writeTextFileSync(contractPath, "# New feature\n\n**Revision:** `8`\n");
  release("Approve");
  await pendingApproval.then(() => {
    throw new Error("changed contract was approved");
  }, () => {});
  await staleQuestion.execute(
    "escape",
    { ...request, contractPath: "../outside.md" },
    signal().signal,
    () => {},
    staleHost.ctx,
  ).then(() => {
    throw new Error("path escape approved");
  }, () => {});
});
