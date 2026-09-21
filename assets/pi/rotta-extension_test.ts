import { EventEmitter } from "node:events";
import { createHash } from "node:crypto";
import registerRottaExtension, { registerRotta } from "./rotta-extension.ts";
import { isAllowedChildMCP, isProtectedWorkPath } from "./rotta-child-guard.ts";

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
    mock.tools.length === 3,
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
    args.includes("--tools") &&
      args.includes(
        "read,grep,find,ls,rotta_ancora_save,rotta_ancora_summarize,rotta_ancora_start,rotta_ancora_end,rotta_ancora_search,rotta_ancora_context,rotta_ancora_get,rotta_context7_resolve_library_id,rotta_context7_query_docs",
      ),
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
  assert(
    args.includes("/test-home/.pi/agent/rotta-next/rotta-mcp-bridge.ts"),
    "child MCP bridge not activated",
  );

  const exploration = fakeSpawn({
    stdout:
      '{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"{\\"status\\":\\"success\\",\\"output\\":\\"explored\\"}"}]}}\n',
  });
  const explorationHost = host();
  registerRotta(explorationHost.pi as any, {
    spawn: exploration.spawn,
    home: () => "/test-home",
  });
  await tool(explorationHost.tools, "rotta_delegate").execute(
    "call",
    { role: "exploration", task: "trace dependencies" },
    signal().signal,
    () => {},
    explorationHost.ctx,
  );
  const explorationArgs = exploration.calls[0][1] as string[];
  const allowedTools = explorationArgs[explorationArgs.indexOf("--tools") + 1]
    .split(",");
  assert(allowedTools.includes("rotta_vela_dependencies"));
  assert(allowedTools.includes("rotta_ancora_context"));
  assert(allowedTools.includes("rotta_context7_query_docs"));
  assert(!allowedTools.includes("bash"));
});

Deno.test("Pi routing profile applies complete roles, but explicit and inheritance win safely", async () => {
  const home = Deno.makeTempDirSync();
  const root = `${home}/.pi/agent/rotta-next`;
  Deno.mkdirSync(root, { recursive: true });
  const profile: any = {
    version: 1,
    roles: {
      implementation: "openai-codex/gpt-5.6-terra",
      reviewer: "openai-codex/gpt-5.6-sol",
      exploration: "openai-codex/gpt-5.6-luna",
      operations: "openai-codex/gpt-5.6-luna",
    },
  };
  try {
    Deno.writeTextFileSync(
      `${root}/model-routing.json`,
      JSON.stringify(profile),
    );
    const invoke = async (
      params: Record<string, unknown>,
      ctx = host().ctx,
    ) => {
      const fixture = fakeSpawn({
        stdout: '{"status":"success","output":"ok"}\n',
      });
      const mock = host();
      registerRotta(mock.pi as any, { home: () => home, spawn: fixture.spawn });
      await tool(mock.tools, "rotta_delegate").execute(
        "x",
        params,
        signal().signal,
        () => {},
        ctx,
      );
      return fixture.calls[0][1] as string[];
    };
    for (
      const [role, expected] of Object.entries(profile.roles) as [
        string,
        string,
      ][]
    ) {
      const args = await invoke({ role, task: "delegate" });
      assert(
        args.includes("--model") && args.includes(expected),
        `${role} routing was not applied`,
      );
    }
    let args: string[];
    args = await invoke({
      role: "reviewer",
      task: "review",
      model: "custom/once",
    });
    assert(
      args.includes("custom/once") &&
        !args.includes("openai-codex/gpt-5.6-sol"),
    );
    profile.roles = { implementation: "only/one" } as any;
    Deno.writeTextFileSync(
      `${root}/model-routing.json`,
      JSON.stringify(profile),
    );
    args = await invoke({ role: "reviewer", task: "review" });
    assert(
      args.includes("test/parent-model"),
      "invalid profile did not inherit parent",
    );
    args = await invoke(
      { role: "reviewer", task: "review" },
      { ...host().ctx, model: undefined } as any,
    );
    assert(!args.includes("--model"), "absent parent should leave model unset");
  } finally {
    Deno.removeSync(home, { recursive: true });
  }
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

Deno.test("default parent entrypoint loads the installed bridge once and reports observed failure", async () => {
  const home = Deno.makeTempDirSync();
  const events: Record<string, any[]> = {}, tools: any[] = [];
  let spawns = 0;
  const pi = {
    registerTool: (entry: any) => tools.push(entry),
    on: (name: string, handler: any) => (events[name] ??= []).push(handler),
  };
  const spawn = (() => {
    spawns++;
    const proc: any = new EventEmitter();
    proc.stdout = new EventEmitter();
    proc.stderr = new EventEmitter();
    proc.stdin = {
      write(line: string, done?: () => void) {
        const request = JSON.parse(line);
        done?.();
        if (!request.id) return true;
        const response = request.method === "initialize"
          ? { protocolVersion: "2025-06-18", capabilities: {} }
          : request.method === "tools/list"
          ? { tools: [{ name: "ancora_search" }] }
          : {
            error: {
              code: -32000,
              message: "Bearer synthetic-key unavailable",
            },
          };
        queueMicrotask(() =>
          proc.stdout.emit(
            "data",
            Buffer.from(
              JSON.stringify({
                jsonrpc: "2.0",
                id: request.id,
                ...(response.error ? response : { result: response }),
              }) + "\n",
            ),
          )
        );
        return true;
      },
    };
    proc.kill = () => true;
    return proc;
  }) as any;
  try {
    const root = `${home}/.pi/agent/rotta-next`;
    Deno.mkdirSync(`${root}/rotta-core`, { recursive: true });
    Deno.mkdirSync(`${root}/rotta-orchestrator`, { recursive: true });
    Deno.writeTextFileSync(`${root}/rotta-core/SKILL.md`, "CORE");
    Deno.writeTextFileSync(
      `${root}/rotta-orchestrator/SKILL.md`,
      "ORCHESTRATOR",
    );
    Deno.writeTextFileSync(
      `${root}/mcp.json`,
      JSON.stringify({
        version: 1,
        services: {
          ancora: { enabled: true },
          vela: { enabled: false },
          context7: { enabled: false },
        },
      }),
    );
    Deno.writeTextFileSync(
      `${root}/rotta-mcp-bridge.ts`,
      await Deno.readTextFile(
        new URL("./rotta-mcp-bridge.ts", import.meta.url),
      ),
    );
    registerRottaExtension(pi as any, {
      home: () => home,
      spawn,
      mcp: { exists: () => true, env: (key) => key === "PATH" ? "/bin" : "" },
    });
    const event = { systemPrompt: "BASE" };
    await events.before_agent_start[0](event);
    await Promise.all(
      events.before_agent_start.map((handler) => handler(event)),
    );
    const search = tools.find((entry) => entry.name === "rotta_ancora_search");
    const status = tools.find((entry) => entry.name === "rotta_mcp_status");
    assert(
      search && status && spawns === 1,
      "default consumer did not load one installed bridge",
    );
    assert((await status.execute()).details.status.ancora.state === "healthy");
    await search.execute("x", {}, new AbortController().signal).then(() => {
      throw Error("failed service call succeeded");
    }, () => {});
    const observed = await status.execute();
    assert(
      observed.details.status.ancora.state === "unavailable" &&
        !observed.content[0].text.includes("synthetic-key"),
    );
  } finally {
    Deno.removeSync(home, { recursive: true });
  }
});

Deno.test("default parent entrypoint reports missing installed bridge separately", async () => {
  const home = Deno.makeTempDirSync();
  try {
    for (const name of ["rotta-core", "rotta-orchestrator"]) {
      Deno.mkdirSync(`${home}/.pi/agent/rotta-next/${name}`, {
        recursive: true,
      });
      Deno.writeTextFileSync(
        `${home}/.pi/agent/rotta-next/${name}/SKILL.md`,
        name,
      );
    }
    const mock = host();
    registerRottaExtension(mock.pi as any, { home: () => home });
    await (mock.events.before_agent_start as any)({ systemPrompt: "BASE" });
    const result = await tool(mock.tools, "rotta_mcp_status").execute();
    assert(result.details.status.bridge.state === "unavailable");
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
    const allowed = args[args.indexOf("--tools") + 1].split(",");
    assert(
      tools.split(",").every((name) => allowed.includes(name)) &&
        allowed.includes("rotta_ancora_context") &&
        args.includes("--no-session"),
      `${role} process was not isolated/allowlisted`,
    );
    assert(
      allowed.includes("rotta_vela_explore") === (role === "exploration"),
      `${role} received the wrong Vela permission`,
    );
    assert(!allowed.includes("bash"), `${role} received shell access`);
  }
});

Deno.test("child receives Context7 key only when trusted managed config enables docs", async () => {
  const home = Deno.makeTempDirSync();
  const enabled = `${home}/.pi/agent/rotta-next/mcp.json`;
  Deno.mkdirSync(`${home}/.pi/agent/rotta-next`, { recursive: true });
  const fixture = fakeSpawn({ stdout: '{"status":"success","output":"ok"}\n' });
  try {
    Deno.writeTextFileSync(
      enabled,
      JSON.stringify({
        version: 1,
        services: {
          ancora: { enabled: false },
          vela: { enabled: false },
          context7: { enabled: true },
        },
      }),
    );
    const mock = host();
    registerRotta(mock.pi as any, {
      spawn: fixture.spawn,
      home: () => home,
      env: (key) =>
        key === "CONTEXT7_API_KEY"
          ? "synthetic-key"
          : key === "PATH"
          ? "/bin"
          : "",
    });
    await tool(mock.tools, "rotta_delegate").execute(
      "x",
      { role: "reviewer", task: "docs" },
      signal().signal,
      () => {},
      mock.ctx,
    );
    const childEnv = fixture.calls[0][2] as { env: Record<string, string> };
    assert(
      childEnv.env.CONTEXT7_API_KEY === "synthetic-key" &&
        !childEnv.env.OTHER_SECRET,
      "selected optional key was not narrowly forwarded",
    );
    Deno.writeTextFileSync(
      enabled,
      JSON.stringify({
        version: 1,
        services: {
          ancora: { enabled: false },
          vela: { enabled: false },
          context7: { enabled: false },
        },
      }),
    );
    const disabled = fakeSpawn({
      stdout: '{"status":"success","output":"ok"}\n',
    });
    const disabledHost = host();
    registerRotta(disabledHost.pi as any, {
      spawn: disabled.spawn,
      home: () => home,
      env: (key) => key === "CONTEXT7_API_KEY" ? "synthetic-key" : "",
    });
    await tool(disabledHost.tools, "rotta_delegate").execute(
      "x",
      { role: "reviewer", task: "docs" },
      signal().signal,
      () => {},
      disabledHost.ctx,
    );
    assert(
      !(disabled.calls[0][2] as any).env.CONTEXT7_API_KEY,
      "disabled service leaked key",
    );
  } finally {
    Deno.removeSync(home, { recursive: true });
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
  const verbose = fakeSpawn({
    stdout:
      `${JSON.stringify({ type: "session", detail: "x".repeat(70_000) })}\n` +
      '{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"{\\"status\\":\\"success\\",\\"output\\":\\"tail result\\"}"}]}}\n',
  });
  const verboseHost = host();
  registerRotta(verboseHost.pi as any, {
    spawn: verbose.spawn,
    home: () => "/test-home",
  });
  const verboseResult = await tool(verboseHost.tools, "rotta_delegate").execute(
    "call",
    { role: "exploration", task: "inspect graph" },
    signal().signal,
    () => {},
    verboseHost.ctx,
  );
  assert(verboseResult.content[0].text === "tail result");
  const invalid = fakeSpawn({ stdout: "not-json\n" });
  registerRotta(mock.pi as any, {
    spawn: invalid.spawn,
    home: () => "/test-home",
  });
  await tool(mock.tools.slice(3), "rotta_delegate").execute(
    "call",
    { role: "reviewer", task: "review" },
    signal().signal,
    () => {},
    mock.ctx,
  ).then(() => {
    throw new Error("invalid protocol succeeded");
  }, () => {});
  const fenced = fakeSpawn({
    stdout:
      '{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"```json\\n{\\"status\\":\\"success\\",\\"output\\":\\"bounded report\\"}\\n```"}]}}\n',
  });
  const fencedHost = host();
  registerRotta(fencedHost.pi as any, {
    spawn: fenced.spawn,
    home: () => "/test-home",
  });
  const fencedResult = await tool(fencedHost.tools, "rotta_delegate").execute(
    "call",
    { role: "operations", task: "report status" },
    signal().signal,
    () => {},
    fencedHost.ctx,
  );
  assert(fencedResult.content[0].text === "bounded report");
  const plain = fakeSpawn({
    stdout:
      '{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"Review complete: no blockers."}]}}\n',
  });
  const plainHost = host();
  registerRotta(plainHost.pi as any, {
    spawn: plain.spawn,
    home: () => "/test-home",
  });
  const plainResult = await tool(plainHost.tools, "rotta_delegate").execute(
    "call",
    { role: "reviewer", task: "review" },
    signal().signal,
    () => {},
    plainHost.ctx,
  );
  assert(plainResult.content[0].text === "Review complete: no blockers.");
  const malformedEnvelope = fakeSpawn({
    stdout:
      '{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"{not-json}"}]}}\n',
  });
  const malformedHost = host();
  registerRotta(malformedHost.pi as any, {
    spawn: malformedEnvelope.spawn,
    home: () => "/test-home",
  });
  await tool(malformedHost.tools, "rotta_delegate").execute(
    "call",
    { role: "reviewer", task: "review" },
    signal().signal,
    () => {},
    malformedHost.ctx,
  ).then(() => {
    throw new Error("malformed envelope succeeded");
  }, () => {});
  const malformedFence = fakeSpawn({
    stdout:
      '{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"```json\\n{not-json}"}]}}\n',
  });
  const malformedFenceHost = host();
  registerRotta(malformedFenceHost.pi as any, {
    spawn: malformedFence.spawn,
    home: () => "/test-home",
  });
  await tool(malformedFenceHost.tools, "rotta_delegate").execute(
    "call",
    { role: "reviewer", task: "review" },
    signal().signal,
    () => {},
    malformedFenceHost.ctx,
  ).then(() => {
    throw new Error("malformed fenced envelope succeeded");
  }, () => {});
  const proseMalformedFence = fakeSpawn({
    stdout:
      '{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"Result:\\n```json\\n{not-json}\\n```"}]}}\n',
  });
  const proseMalformedFenceHost = host();
  registerRotta(proseMalformedFenceHost.pi as any, {
    spawn: proseMalformedFence.spawn,
    home: () => "/test-home",
  });
  await tool(proseMalformedFenceHost.tools, "rotta_delegate").execute(
    "call",
    { role: "reviewer", task: "review" },
    signal().signal,
    () => {},
    proseMalformedFenceHost.ctx,
  ).then(() => {
    throw new Error("prose plus malformed fenced envelope succeeded");
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

async function loadGuard(role?: string) {
  const previous = Deno.env.get("ROTTA_CHILD_ROLE");
  try {
    if (role === undefined) Deno.env.delete("ROTTA_CHILD_ROLE");
    else Deno.env.set("ROTTA_CHILD_ROLE", role);
    const module = await import(
      `./rotta-child-guard.ts?test=${crypto.randomUUID()}`
    );
    return module.default;
  } finally {
    if (previous === undefined) Deno.env.delete("ROTTA_CHILD_ROLE");
    else Deno.env.set("ROTTA_CHILD_ROLE", previous);
  }
}

Deno.test("auto-loaded guard leaves top-level Pi sessions unrestricted", async () => {
  for (const role of [undefined, ""]) {
    const guard = await loadGuard(role);
    const mock = host();
    guard(mock.pi as any);
    assert(
      !mock.events.tool_call,
      "parent session received child restrictions",
    );
  }
});

Deno.test("unknown child role still blocks tools", async () => {
  const guard = await loadGuard("unknown");
  const mock = host();
  guard(mock.pi as any);
  const handler = mock.events.tool_call as (event: unknown) => Promise<any>;
  assert((await handler({ toolName: "read", input: {} })).block);
});

Deno.test("child guard blocks all canonical and symlinked work-record write targets", async () => {
  const guard = await loadGuard("implementation");
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
    assert(handlers.length === 1, "child guard was not registered");
    assert(
      await handlers[0]({ toolName: "read", input: {} }) === undefined,
      "allowed child read was blocked",
    );
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

Deno.test("child MCP guard permits required memory lifecycle but never Vela outside exploration", () => {
  assert(isAllowedChildMCP("implementation", "rotta_ancora_summarize"));
  assert(!isAllowedChildMCP("implementation", "rotta_vela_explore"));
  assert(isAllowedChildMCP("exploration", "rotta_vela_module_summary"));
  assert(!isAllowedChildMCP("reviewer", "rotta_context7_delete"));
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
