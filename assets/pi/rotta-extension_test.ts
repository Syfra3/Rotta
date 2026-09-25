import { EventEmitter } from "node:events";
import { createHash } from "node:crypto";
import registerRottaExtension, {
  compactBuiltinDefinitions,
  detailOverlayComponent,
  registerRotta,
  renderRottaHeader,
} from "./rotta-extension.ts";
import { isAllowedChildMCP, isProtectedWorkPath, operationGate } from "./rotta-child-guard.ts";

function assert(value: unknown, message = "assertion failed"): asserts value {
  if (!value) throw new Error(message);
}
type Tool = { name: string; execute: (...args: any[]) => Promise<any> };
type Exec = (
  command: string,
  args: string[],
  options?: { timeout?: number },
) => Promise<{ stdout: string; stderr: string; code: number; killed: boolean }>;

function host(
  exec: Exec = async () => ({
    stdout: "rotta 1.16.2\n",
    stderr: "",
    code: 0,
    killed: false,
  }),
) {
  const tools: Tool[] = [];
  const selects: unknown[] = [];
  const headers: unknown[] = [];
  const shortcuts: any[] = [];
  const overlays: any[] = [];
  const notifications: Array<{ message: string; level: string }> = [];
  const events: Record<string, unknown> = {};
  const emittedEvents: Array<{ event: string; data: unknown }> = [];
  const extensionEventHandlers: Record<string, Array<(data: unknown) => void>> = {};
  const activity: string[] = [];
  const execCalls: unknown[][] = [];
  const pi = {
    events: {
      on(event: string, handler: (data: unknown) => void) {
        (extensionEventHandlers[event] ??= []).push(handler);
      },
      emit(event: string, data: unknown) {
        emittedEvents.push({ event, data });
        activity.push(`emit:${event}:${(data as { active?: unknown })?.active}`);
        for (const handler of extensionEventHandlers[event] ?? []) handler(data);
      },
    },
    exec(command: string, args: string[], options?: { timeout?: number }) {
      execCalls.push([command, args, options]);
      return exec(command, args, options);
    },
    registerTool(tool: Tool) {
      tools.push(tool);
    },
    registerShortcut(shortcut: string, options: unknown) {
      shortcuts.push({ shortcut, ...(options as object) });
    },
    on(event: string, handler: unknown) {
      events[event] = handler;
    },
  };
  const ctx = {
    cwd: "/workspace",
    mode: "tui",
    hasUI: true,
    model: { provider: "test", id: "parent-model" },
    sessionManager: { getSessionId: () => "trusted-session" },
    ui: {
      setHeader: (factory: unknown) => headers.push(factory),
      custom: async (factory: any) => {
        let closed = false;
        const component = factory(
          { requestRender() {} },
          { fg: (_: string, text: string) => text },
          {},
          () => (closed = true),
        );
        overlays.push({ component, get closed() { return closed; } });
      },
      notify: (message: string, level: string) => notifications.push({ message, level }),
      select: async (
        title: string,
        options: string[],
        selectOptions?: { signal: AbortSignal; timeout?: number },
      ) => {
        selects.push({ title, options, selectOptions, multiple: false, custom: false });
        activity.push("select");
        return options[0];
      },
    },
  };
  return {
    pi,
    tools,
    selects,
    headers,
    shortcuts,
    overlays,
    notifications,
    events,
    emittedEvents,
    extensionEventHandlers,
    activity,
    ctx,
    execCalls,
  };
}

function visibleWidth(text: string) {
  return text.replace(/\x1b\[[0-?]*[ -/]*[@-~]/g, "").length;
}
function signal() {
  return new AbortController();
}
function fakeSpawn(
  events: {
    stdout?: string | Uint8Array[];
    stderr?: string;
    code?: number;
    wait?: boolean;
  },
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
      if (events.stdout) {
        for (
          const chunk of Array.isArray(events.stdout)
            ? events.stdout
            : [Buffer.from(events.stdout)]
        ) {
          proc.stdout.emit("data", chunk);
        }
      }
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
function controlledTimers() {
  let nextId = 0;
  const callbacks = new Map<number, () => void>();
  const cleared: number[] = [];
  const unrefed: number[] = [];
  return {
    callbacks,
    cleared,
    unrefed,
    setTimeout: ((callback: () => void) => {
      const id = ++nextId;
      callbacks.set(id, callback);
      return { id, unref: () => unrefed.push(id) } as any;
    }) as typeof globalThis.setTimeout,
    clearTimeout: ((timer: { id?: number }) => {
      if (timer?.id !== undefined) {
        cleared.push(timer.id);
        callbacks.delete(timer.id);
      }
    }) as any,
    expire(id: number) {
      const callback = callbacks.get(id);
      callbacks.delete(id);
      callback?.();
    },
  };
}
async function stageDelegateFailure(
  mock: ReturnType<typeof host>,
  toolCallId: string,
) {
  await tool(mock.tools, "rotta_delegate").execute(
    toolCallId,
    { role: "reviewer", task: `fail ${toolCallId}` },
    signal().signal,
    () => {},
    mock.ctx,
  ).then(() => {
    throw new Error("failed delegation unexpectedly succeeded");
  }, () => {});
}
function delegateToolResult(
  mock: ReturnType<typeof host>,
  toolCallId: string,
  isError = true,
) {
  return (mock.events.tool_result as any)({
    toolCallId,
    toolName: "rotta_delegate",
    input: {},
    content: [{ type: "text", text: "failed" }],
    details: undefined,
    isError,
  }, mock.ctx);
}
async function recoveredDelegateFailure(
  mock: ReturnType<typeof host>,
  params: Record<string, unknown>,
  controller = signal(),
) {
  const delegate = tool(mock.tools, "rotta_delegate") as any;
  let error: Error | undefined;
  try {
    await delegate.execute(
      "failed-call",
      params,
      controller.signal,
      () => {},
      mock.ctx,
    );
  } catch (caught) {
    error = caught as Error;
  }
  assert(error, "failed delegation did not remain a genuine thrown Pi error");
  const patch = await (mock.events.tool_result as any)({
    toolCallId: "failed-call",
    toolName: "rotta_delegate",
    input: params,
    content: [{ type: "text", text: error.message }],
    details: undefined,
    isError: true,
  }, mock.ctx);
  assert(patch?.details, "Pi tool_result path did not recover failure details");
  const result = {
    content: [{ type: "text", text: error.message }],
    details: patch.details,
  };
  const rendered = delegate.renderResult(
    result,
    { expanded: false, isPartial: false },
    { bold: (s: string) => s, fg: (_: string, s: string) => s },
    { args: params, toolCallId: "failed-call", isError: true },
  ).render(200).join("\n");
  return { error, details: patch.details, rendered };
}

const plainTheme = {
  bold: (text: string) => text,
  fg: (_name: string, text: string) => text,
};

Deno.test("compact built-ins preserve factory execution contracts exactly", () => {
  const { bash, edit, originals } = compactBuiltinDefinitions("/workspace");
  for (const [wrapped, original] of [[bash, originals.bash], [edit, originals.edit]]) {
    assert(wrapped.name === original.name);
    assert(wrapped.description === original.description);
    assert(wrapped.parameters === original.parameters, "schema identity changed");
    assert(wrapped.execute === original.execute, "execute identity changed");
    for (const key of Object.keys(original)) {
      if (key !== "renderCall" && key !== "renderResult") {
        assert((wrapped as any)[key] === (original as any)[key], `${key} changed`);
      }
    }
  }
});

Deno.test("compact built-ins register only after a TUI session starts", async () => {
  for (const mode of ["json", "print", "rpc", "tui"]) {
    const mock = host();
    mock.ctx.mode = mode;
    registerRotta(mock.pi as any, { home: () => "/test-home" });
    assert(!mock.tools.some((entry) => ["bash", "edit"].includes(entry.name)));
    await (mock.events.session_start as any)({}, mock.ctx);
    const names = mock.tools.map((entry) => entry.name);
    assert(names.includes("bash") === (mode === "tui"), `${mode} bash shadow mismatch`);
    assert(names.includes("edit") === (mode === "tui"), `${mode} edit shadow mismatch`);
  }
});

Deno.test("compact built-ins visibly disable shadows on Pi version or factory-shape mismatch", async () => {
  const cases = [
    { piVersion: "0.88.0" },
    {
      piVersion: "0.87.1",
      builtinFactories: {
        bash: (() => ({ name: "bash", parameters: {}, execute: async () => ({}) })) as any,
        edit: (() => ({ name: "edit", parameters: {}, execute: async () => ({}) })) as any,
      },
    },
  ];
  for (const dependencies of cases) {
    const mock = host();
    registerRotta(mock.pi as any, { home: () => "/test-home", ...dependencies });
    await (mock.events.session_start as any)({}, mock.ctx);
    assert(!mock.tools.some((entry) => ["bash", "edit"].includes(entry.name)), "mismatched shadows activated");
    assert(mock.notifications.length === 1 && mock.notifications[0].level === "error");
    assert(mock.notifications[0].message.includes("compact built-ins disabled"));
  }
});

Deno.test("compact bash covers running success failure expansion and truncation without mutation", () => {
  const bash = compactBuiltinDefinitions("/workspace").bash as any;
  const state: any = {};
  const context: any = { args: { command: "printf 'hello world'" }, state, executionStarted: true, isPartial: true, isError: false };
  const partial = { content: [{ type: "text", text: "partial output" }], details: undefined };
  const partialSnapshot = JSON.stringify(partial);
  const running = bash.renderResult(partial, { expanded: false, isPartial: true }, plainTheme, context).render(200);
  assert(running.length === 1 && running[0].includes("running") && running[0].includes("printf 'hello world'") && running[0].includes("0s"));
  assert(JSON.stringify(partial) === partialSnapshot, "partial result mutated");

  const success: any = { content: [{ type: "text", text: "all output" }], details: undefined };
  context.isPartial = false;
  const successLine = bash.renderResult(success, { expanded: false, isPartial: false }, plainTheme, context).render(200);
  assert(successLine.length === 1 && successLine[0].includes("exit 0"));
  const expanded = bash.renderResult(success, { expanded: true, isPartial: false }, plainTheme, context).render(200).join("\n");
  assert(expanded.split("all output").length === 2, "expanded output missing or duplicated");

  const path = "/tmp/pi-bash-full";
  const footer = `[Showing lines 6-10 of 10. Full output: ${path}]`;
  const truncated: any = {
    content: [{ type: "text", text: `tail output\n\n${footer}` }],
    details: { truncation: { truncated: true, truncatedBy: "lines", outputLines: 5, totalLines: 10 }, fullOutputPath: path },
  };
  const snapshot = JSON.stringify(truncated);
  const collapsed = bash.renderResult(truncated, { expanded: false, isPartial: false }, plainTheme, context).render(200);
  assert(collapsed.length === 1 && collapsed[0].includes("truncated"));
  const full = bash.renderResult(truncated, { expanded: true, isPartial: false }, plainTheme, context).render(300).join("\n");
  assert(full.split(path).length === 2, "full-output path was missing or duplicated");
  assert(full.split("tail output").length === 2);
  assert(JSON.stringify(truncated) === snapshot, "truncated result/details mutated");

  const failed: any = { content: [{ type: "text", text: "stderr\n\nCommand exited with code 7" }], details: undefined };
  context.isError = true;
  assert(bash.renderResult(failed, { expanded: false, isPartial: false }, plainTheme, context).render(200)[0].includes("exit 7"));
  const timeout: any = { content: [{ type: "text", text: "Command timed out after 3 seconds" }], details: undefined };
  assert(bash.renderResult(timeout, { expanded: false, isPartial: false }, plainTheme, context).render(200)[0].includes("timeout 3s"));
});

Deno.test("compact bash preserves transcript whitespace and removes only its recognized footer", () => {
  const bash = compactBuiltinDefinitions("/workspace").bash as any;
  const context: any = { args: { command: "printf" }, state: {}, isError: false };
  const exact = "\n\n  leading  \nbody\ntrailing  \n\n";
  const expanded = bash.renderResult(
    { content: [{ type: "text", text: exact }], details: undefined },
    { expanded: true, isPartial: false }, plainTheme, context,
  ).render(300).join("\n");
  assert(expanded.slice(expanded.indexOf("\n") + 1) === exact, "expanded transcript whitespace changed");

  const path = "/tmp/full.out";
  const footer = `[Showing lines 6-10 of 10. Full output: ${path}]`;
  const result = {
    content: [{ type: "text", text: `  output  \n\n${footer}` }],
    details: { truncation: { truncated: true, truncatedBy: "lines", outputLines: 5, totalLines: 10 }, fullOutputPath: path },
  };
  const rendered = bash.renderResult(result, { expanded: true, isPartial: false }, plainTheme, context).render(300).join("\n");
  assert(rendered.includes("  output  \n[truncated:"), "footer removal trimmed output spaces");
  assert(rendered.split(path).length === 2, "recognized footer was not deduplicated");
  const similar = { ...result, content: [{ type: "text", text: `keep\n\n[User note. Full output: ${path}]` }] };
  const kept = bash.renderResult(similar, { expanded: true, isPartial: false }, plainTheme, context).render(300).join("\n");
  assert(kept.includes("[User note."), "non-generated footer was removed");

  const footerOnly = { content: [{ type: "text", text: `\n  output  \n\n${footer}` }], details: { truncation: result.details.truncation } };
  const fromFooter = bash.renderResult(footerOnly, { expanded: true, isPartial: false }, plainTheme, context).render(300).join("\n");
  assert(fromFooter.split(path).length === 2, "footer-only path missing or duplicated");
  assert(fromFooter.includes("\n  output  \n[truncated:"), "footer-only removal changed transcript whitespace");
  const otherPath = { ...result, details: { ...result.details, fullOutputPath: "/tmp/other.out" } };
  const retained = bash.renderResult(otherPath, { expanded: true, isPartial: false }, plainTheme, context).render(300).join("\n");
  assert(!retained.includes(footer), "recognized footer with conflicting details remained");
  assert(retained.split(path).length === 2 && !retained.includes("/tmp/other.out"), "footer path did not win conflict");
});

Deno.test("Pi 0.87.1 byte-limit and partial-line footers preserve transcript and deduplicate paths", () => {
  const bash = compactBuiltinDefinitions("/workspace").bash as any;
  const prefix = "  first  \n\nlast  \n\n";
  const footerPath = "/tmp/pi-footer";
  const cases = [
    { footer: `[Showing lines 4-5 of 5 (50.0KB limit). Full output: ${footerPath}]`, truncation: { truncated: true, truncatedBy: "bytes", outputLines: 2, maxBytes: 51200 } },
    { footer: `[Showing last 49.9KB of line 5 (line is 80.0KB). Full output: ${footerPath}]`, truncation: { truncated: true, truncatedBy: "bytes", lastLinePartial: true, outputLines: 1, maxBytes: 51200 } },
  ];
  for (const { footer, truncation } of cases) {
    for (const fullOutputPath of [undefined, footerPath, "/tmp/conflicting"]) {
      const result = { content: [{ type: "text", text: prefix + footer }], details: { truncation, fullOutputPath } };
      const context = { args: { command: "x" }, state: {}, isError: false };
      const expanded = bash.renderResult(result, { expanded: true, isPartial: false }, plainTheme, context).render(500).join("\n");
      assert(expanded.includes(`\n${prefix.slice(0, -2)}\n[truncated:`), "prefix whitespace was not preserved before warning");
      assert(!expanded.includes(footer) && expanded.split(footerPath).length === 2, "footer was not replaced with one path");
      assert(!expanded.includes("/tmp/conflicting"), "conflicting details path won");
      const errorContext = { args: { command: "x" }, state: {}, isError: true };
      const collapsed = bash.renderResult(result, { expanded: false, isPartial: false }, plainTheme, errorContext).render(500)[0];
      assert(!collapsed.includes("Showing") && collapsed.split(footerPath).length === 2, "error diagnostic duplicated footer path");
      assert(!collapsed.includes("/tmp/conflicting") && collapsed.includes("first last"), "error diagnostic or path lost");
    }
  }
});

Deno.test("compact bash and edit failures expose sanitized width-bounded diagnostics", () => {
  const { bash, edit } = compactBuiltinDefinitions("/workspace") as any;
  const bashContext: any = { args: { command: "x" }, state: {}, isError: true };
  for (const [text, expected] of [
    ["generic boom token=secret", "generic boom token=[redacted]"],
    ["stderr detail\n\nCommand exited with code 9", "stderr detail"],
    ["Command timed out after 4 seconds", "Command timed out after 4 seconds"],
  ]) {
    const lines = bash.renderResult({ content: [{ type: "text", text }], details: undefined }, { expanded: false, isPartial: false }, plainTheme, bashContext).render(70);
    assert(lines.length === 1 && visibleWidth(lines[0]) <= 70 && lines[0].includes(expected), `missing diagnostic: ${text}`);
    assert(!lines[0].includes("secret"), "diagnostic leaked secret");
  }
  const editContext: any = { args: { path: "a", edits: [{}] }, isError: true, isPartial: false };
  const line = edit.renderResult({ content: [{ type: "text", text: "oldText mismatch password=hunter2" }] }, { expanded: false, isPartial: false }, plainTheme, editContext).render(70)[0];
  assert(visibleWidth(line) <= 70 && line.includes("oldText mismatch") && !line.includes("hunter2"));
});

Deno.test("compact bash reports line and byte truncation counts and paths on one line", () => {
  const bash = compactBuiltinDefinitions("/workspace").bash as any;
  const context: any = { args: { command: "x" }, state: {}, isError: false };
  for (const [truncation, expected] of [
    [{ truncated: true, truncatedBy: "lines", outputLines: 8, totalLines: 20 }, "8/20 lines"],
    [{ truncated: true, truncatedBy: "bytes", outputLines: 3, maxBytes: 2048 }, "3 lines, 2.0 KB"],
  ] as const) {
    const line = bash.renderResult({ content: [], details: { truncation, fullOutputPath: "/tmp/full" } }, { expanded: false, isPartial: false }, plainTheme, context).render(240);
    assert(line.length === 1 && line[0].includes(expected) && line[0].includes("/tmp/full"));
  }
});

Deno.test("compact edit covers counts, running/errors, expansion, and immutable details", () => {
  const edit = compactBuiltinDefinitions("/workspace").edit as any;
  const context: any = { args: { path: "src/a.ts", edits: [{}, {}] }, state: {}, isPartial: false, isError: false };
  const result: any = { content: [{ type: "text", text: "Successfully replaced 2 block(s)" }], details: { diff: "@@ -1 +1,2 @@\n-old\n+new\n+more", patch: "patch" } };
  const snapshot = JSON.stringify(result);
  const collapsed = edit.renderResult(result, { expanded: false, isPartial: false }, plainTheme, context).render(200);
  assert(collapsed.length === 1 && collapsed[0].includes("src/a.ts • success • 2 replacements +2 -1"));
  const expanded = edit.renderResult(result, { expanded: true, isPartial: false }, plainTheme, context).render(200).join("\n");
  assert(expanded.split("@@ -1 +1,2 @@").length === 2, "diff missing or duplicated");
  assert(JSON.stringify(result) === snapshot, "edit result/details mutated");
  context.isPartial = true;
  assert(edit.renderResult(result, { expanded: false, isPartial: true }, plainTheme, context).render(200)[0].includes("running"));
  const error: any = { content: [{ type: "text", text: "oldText was not unique\nfull diagnostic" }], details: undefined };
  context.isPartial = false;
  context.isError = true;
  const failed = edit.renderResult(error, { expanded: false, isPartial: false }, plainTheme, context).render(200);
  assert(failed.length === 1 && failed[0].includes("failed"));
  const fullError = edit.renderResult(error, { expanded: true, isPartial: false }, plainTheme, context).render(200).join("\n");
  assert(fullError.split("oldText was not unique").length === 2, "error missing or duplicated");
});

Deno.test(
  "TUI session startup shows the installed Rotta version in a width-safe header",
  async () => {
    const mock = host();
    registerRotta(mock.pi as any, { home: () => "/test-home" });
    const start = mock.events.session_start as (
      event: unknown,
      ctx: unknown,
    ) => Promise<void>;
    await start({}, mock.ctx);
    assert(mock.headers.length === 1, "TUI startup did not install a header");
    assert(
      JSON.stringify(mock.execCalls) ===
        JSON.stringify([["rotta", ["--version"], { timeout: 5_000 }]]),
      "TUI startup did not perform the bounded Rotta version lookup",
    );

    const theme = {
      bold: (text: string) => `\x1b[1m${text}\x1b[22m`,
      fg: (_name: string, text: string) => `\x1b[36m${text}\x1b[39m`,
    };
    const factory = mock.headers[0] as (_tui: unknown, theme: unknown) => {
      render(width: number): string[];
    };
    const wide = factory({}, theme).render(80);
    assert(wide.join("\n").includes("R O T T A"));
    assert(wide.join("\n").includes("Welcome to Rotta"));
    assert(wide.join("\n").includes("Pi v"));
    assert(wide.join("\n").includes("Rotta v1.16.2"));
    assert(wide.join("\n").includes("ctrl+o for full startup help"));
    assert(wide.join("\n").includes("/hotkeys"));
    for (const width of [0, 1, 8, 20, 80]) {
      for (
        const line of renderRottaHeader(theme as any, width, "Rotta v1.16.2")
      ) {
        assert(visibleWidth(line) <= width, `header overflowed width ${width}`);
      }
    }
  },
);

Deno.test(
  "failed or invalid Rotta version lookup keeps a truthful Rotta label",
  async () => {
    for (
      const exec of [
        async () => ({
          stdout: "rotta 1.16.2\n",
          stderr: "failed",
          code: 1,
          killed: false,
        }),
        async () => ({
          stdout: "not a version",
          stderr: "",
          code: 0,
          killed: false,
        }),
        async () => await Promise.reject(new Error("not found")),
      ]
    ) {
      const mock = host(exec);
      registerRotta(mock.pi as any, { home: () => "/test-home" });
      await (mock.events.session_start as any)({}, mock.ctx);
      const theme = {
        bold: (text: string) => text,
        fg: (_: string, text: string) => text,
      };
      const header = (mock.headers[0] as any)({}, theme).render(80).join("\n");
      assert(header.includes("Rotta"), "fallback omitted the Rotta label");
      assert(
        !header.includes("Rotta v"),
        "fallback fabricated a Rotta version",
      );
    }
  },
);

Deno.test(
  "non-TUI session startup does not install the Rotta header or look up Rotta",
  async () => {
    for (const mode of ["rpc", "json", "print"]) {
      const mock = host();
      mock.ctx.mode = mode;
      registerRotta(mock.pi as any, { home: () => "/test-home" });
      await (mock.events.session_start as any)({}, mock.ctx);
      assert(mock.headers.length === 0, `${mode} installed a TUI header`);
      assert(
        mock.execCalls.length === 0,
        `${mode} looked up the Rotta version`,
      );
    }
  },
);

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
  assert(
    result.details.outcome === "success" && result.details.failed === undefined,
    "successful delegate metadata was not authoritatively success-only",
  );
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
    "reviewer received the wrong allowlist",
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
      implementation: { model: "openai-codex/gpt-5.6-sol", effort: "low" },
      reviewer: { model: "openai-codex/gpt-5.6-sol", effort: "medium" },
      exploration: { model: "openai-codex/gpt-5.6-sol", effort: "high" },
      operations: { model: "openai-codex/gpt-5.6-sol", effort: "low" },
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
      const [role, expected] of Object.entries(profile.roles).filter(([role]) => role !== "operations") as [
        string,
        { model: string; effort: string },
      ][]
    ) {
      const args = await invoke({ role, task: "delegate" });
      assert(
        args.includes("--model") && args.includes(expected.model) &&
          args.includes("--thinking") && args.includes(expected.effort),
        `${role} routing was not applied`,
      );
    }
    let args: string[];
    profile.roles = {
      implementation: "legacy/impl",
      reviewer: "legacy/reviewer",
      exploration: "legacy/explore",
      operations: "legacy/ops",
    };
    Deno.writeTextFileSync(
      `${root}/model-routing.json`,
      JSON.stringify(profile),
    );
    args = await invoke({ role: "reviewer", task: "review" });
    assert(args.includes("legacy/reviewer") && !args.includes("--thinking"));
    args = await invoke({
      role: "reviewer",
      task: "review",
      model: "custom/once",
    });
    assert(
      args.includes("custom/once") &&
        !args.includes("--thinking"),
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

Deno.test("delegate preserves requested routing and prefers authoritative child model and effort", async () => {
  const fixture = fakeSpawn({
    stdout: JSON.stringify({
      type: "message_end",
      message: {
        role: "assistant",
        model: "provider/base-model",
        responseModel: "provider/actual-model",
        providerThinkingLevel: "medium",
        content: "done",
      },
    }) + "\n",
  });
  const mock = host();
  const updates: any[] = [];
  registerRotta(mock.pi as any, {
    spawn: fixture.spawn,
    home: () => "/test-home",
  });
  const delegate = tool(mock.tools, "rotta_delegate") as any;
  const result = await delegate.execute(
    "call",
    { role: "reviewer", task: "review", model: "requested/model" },
    signal().signal,
    (update: unknown) => updates.push(update),
    mock.ctx,
  );
  assert(
    updates[0].details.requestedModel === "requested/model" &&
      updates[0].details.requestedEffort === "default" &&
      updates[0].details.source === "explicit",
    "running details lost requested CLI routing",
  );
  assert(
    updates[0].details.responseModel === undefined &&
      updates[0].details.providerThinkingLevel === undefined,
    "running details fabricated actual routing",
  );
  assert(
    result.details.responseModel === "provider/actual-model" &&
      result.details.providerThinkingLevel === "medium" &&
      result.details.requestedModel === "requested/model",
    "final assistant metadata was not authoritative or retained",
  );
  const theme = { bold: (s: string) => s, fg: (_: string, s: string) => s };
  const rendered = delegate.renderResult(
    result,
    { expanded: false, isPartial: false },
    theme,
    { args: { role: "reviewer" }, isError: false },
  ).render(160).join("\n");
  assert(
    rendered.includes(
      "✓ complete reviewer model=provider/actual-model effort=medium",
    ),
    "completed row omitted authoritative routing metadata",
  );
  const explicitCall = delegate.renderCall(
    { role: "reviewer", task: "review", model: "requested/model" },
    theme,
    {},
  ).render(160).join("\n");
  assert(explicitCall.includes("model=requested/model effort=default"));
  const defaultCall = delegate.renderCall(
    { role: "reviewer", task: "review" },
    theme,
    {},
  ).render(160).join("\n");
  assert(
    defaultCall.includes("model=resolved at run effort=resolved at run"),
    "call row inferred omitted routing",
  );
});

Deno.test("delegate uses message model fallback and leaves unreported actual values unknown", async () => {
  for (
    const [message, expectedModel, expectedEffort] of [
      [{ model: "provider/fallback", providerThinkingLevel: "low" }, "provider/fallback", "low"],
      [{}, undefined, undefined],
    ] as const
  ) {
    const fixture = fakeSpawn({
      stdout: JSON.stringify({
        type: "message_end",
        message: { role: "assistant", content: "done", ...message },
      }) + "\n",
    });
    const mock = host();
    registerRotta(mock.pi as any, {
      spawn: fixture.spawn,
      home: () => "/test-home",
    });
    const delegate = tool(mock.tools, "rotta_delegate") as any;
    const result = await delegate.execute(
      "call",
      { role: "reviewer", task: "review" },
      signal().signal,
      () => {},
      { ...mock.ctx, model: undefined },
    );
    assert(result.details.responseModel === expectedModel);
    assert(result.details.providerThinkingLevel === expectedEffort);
    assert(
      result.details.requestedModel === "default" &&
        result.details.requestedEffort === "default" &&
        result.details.source === "child default",
    );
    const rendered = delegate.renderResult(
      result,
      { expanded: false, isPartial: false },
      { bold: (s: string) => s, fg: (_: string, s: string) => s },
      { args: { role: "reviewer" }, isError: false },
    ).render(160).join("\n");
    assert(
      rendered.includes(
        `model=${expectedModel ?? "?"} effort=${expectedEffort ?? "?"}`,
      ),
      "unknown/default actual values were inferred",
    );
  }
});

Deno.test("timeout remains a Pi error and recovers requested routing without fabricating actual routing", async () => {
  const originalSetTimeout = globalThis.setTimeout;
  try {
    (globalThis as any).setTimeout = (fn: () => void) => {
      queueMicrotask(fn);
      return 0 as any;
    };
    const fixture = fakeSpawn({ wait: true });
    const mock = host();
    const pendingDetailTimers = controlledTimers();
    registerRotta(mock.pi as any, {
      spawn: fixture.spawn,
      home: () => "/test-home",
      defaultTimeoutMs: 5,
      setTimeout: pendingDetailTimers.setTimeout,
      clearTimeout: pendingDetailTimers.clearTimeout,
    });
    const failure = await recoveredDelegateFailure(mock, {
      role: "reviewer",
      task: "slow review",
      model: "requested/model",
    });
    assert(failure.details.reason === "timeout");
    assert(
      pendingDetailTimers.cleared.length === 1,
      "tool_result did not consume and clear the independent detail TTL",
    );
    assert(failure.details.role === "reviewer" && typeof failure.details.elapsedMs === "number");
    assert(failure.details.requestedModel === "requested/model" && failure.details.requestedEffort === "default");
    assert(failure.details.responseModel === undefined && failure.details.providerThinkingLevel === undefined);
    assert(failure.rendered.includes("✗ failed reviewer model=? effort=?") && failure.rendered.includes("timeout"));
  } finally {
    globalThis.setTimeout = originalSetTimeout;
  }
});

Deno.test("cancellation remains a Pi error with structured requested context and unknown actual routing", async () => {
  const fixture = fakeSpawn({ wait: true });
  const mock = host();
  registerRotta(mock.pi as any, { spawn: fixture.spawn, home: () => "/test-home" });
  const controller = signal();
  controller.abort();
  const failure = await recoveredDelegateFailure(mock, {
    role: "implementation",
    task: "cancel me",
    model: "requested/cancel-model",
  }, controller);
  assert(failure.details.reason === "cancelled" && failure.details.cancelled === true);
  assert(failure.details.requestedModel === "requested/cancel-model" && failure.details.requestedEffort === "default");
  assert(failure.rendered.includes("✗ failed implementation model=? effort=?") && failure.rendered.includes("cancelled"));
});

Deno.test("child start failure remains a Pi error with timing and requested context only", async () => {
  const mock = host();
  registerRotta(mock.pi as any, {
    spawn: (() => { throw new Error("pi unavailable"); }) as any,
    home: () => "/test-home",
  });
  const failure = await recoveredDelegateFailure(mock, {
    role: "reviewer",
    task: "report",
    model: "requested/start-model",
  });
  assert(failure.details.reason === "start_failed");
  assert(failure.details.role === "reviewer" && typeof failure.details.elapsedMs === "number");
  assert(failure.details.requestedModel === "requested/start-model" && failure.details.requestedEffort === "default");
  assert(failure.rendered.includes("✗ failed reviewer model=? effort=?") && failure.rendered.includes("start_failed"));
});

Deno.test("child error remains a Pi error and renders authoritative reported model and effort", async () => {
  const fixture = fakeSpawn({
    stdout: JSON.stringify({
      type: "message_end",
      message: {
        role: "assistant",
        responseModel: "provider/actual-error-model",
        providerThinkingLevel: "high",
        content: '{"status":"error","message":"review failed"}',
      },
    }) + "\n",
  });
  const mock = host();
  registerRotta(mock.pi as any, { spawn: fixture.spawn, home: () => "/test-home" });
  const failure = await recoveredDelegateFailure(mock, {
    role: "reviewer",
    task: "review",
    model: "requested/error-model",
  });
  assert(failure.details.reason === "child_error");
  assert(failure.details.requestedModel === "requested/error-model" && failure.details.requestedEffort === "default");
  assert(failure.details.responseModel === "provider/actual-error-model" && failure.details.providerThinkingLevel === "high");
  assert(failure.rendered.includes("✗ failed reviewer model=provider/actual-error-model effort=high"));
});

Deno.test("pending delegate failure details are consumed once and clear their unrefed timer", async () => {
  const timers = controlledTimers();
  const fixture = fakeSpawn({
    stdout: '{"status":"error","message":"failed"}\n',
  });
  const mock = host();
  registerRotta(mock.pi as any, {
    spawn: fixture.spawn,
    home: () => "/test-home",
    setTimeout: timers.setTimeout,
    clearTimeout: timers.clearTimeout,
  });
  await stageDelegateFailure(mock, "consume");
  assert(timers.unrefed.length === 1, "pending timer was not unrefed");
  const recovered = await delegateToolResult(mock, "consume");
  assert(recovered?.details?.reason === "child_error");
  assert(timers.cleared.length === 1, "consumption did not clear its timer");
  assert(
    await delegateToolResult(mock, "consume") === undefined,
    "consumed details were returned twice",
  );
});

Deno.test("unmatched and non-error delegate results receive no details or retained state", async () => {
  const timers = controlledTimers();
  const clearedTimerCount = () => timers.cleared.length;
  const fixture = fakeSpawn({
    stdout: '{"status":"error","message":"failed"}\n',
  });
  const mock = host();
  registerRotta(mock.pi as any, {
    spawn: fixture.spawn,
    home: () => "/test-home",
    setTimeout: timers.setTimeout,
    clearTimeout: timers.clearTimeout,
  });
  await stageDelegateFailure(mock, "non-error");
  assert(await delegateToolResult(mock, "missing") === undefined);
  assert(clearedTimerCount() === 0, "unmatched ID altered pending state");
  assert(await delegateToolResult(mock, "non-error", false) === undefined);
  assert(clearedTimerCount() === 1, "non-error result leaked pending state");
  assert(await delegateToolResult(mock, "non-error") === undefined);
});

Deno.test("pending delegate failure details expire after their finite TTL", async () => {
  const timers = controlledTimers();
  const fixture = fakeSpawn({
    stdout: '{"status":"error","message":"failed"}\n',
  });
  const mock = host();
  registerRotta(mock.pi as any, {
    spawn: fixture.spawn,
    home: () => "/test-home",
    pendingFailureTtlMs: 25,
    setTimeout: timers.setTimeout,
    clearTimeout: timers.clearTimeout,
  });
  await stageDelegateFailure(mock, "expired");
  const timerId = [...timers.callbacks.keys()][0];
  assert(timerId !== undefined, "expiry timer was not scheduled");
  timers.expire(timerId);
  assert(
    await delegateToolResult(mock, "expired") === undefined,
    "expired details were retained",
  );
});

Deno.test("pending delegate failure capacity evicts the deterministic oldest entry", async () => {
  const timers = controlledTimers();
  const fixture = fakeSpawn({
    stdout: '{"status":"error","message":"failed"}\n',
  });
  const mock = host();
  registerRotta(mock.pi as any, {
    spawn: fixture.spawn,
    home: () => "/test-home",
    pendingFailureCapacity: 2,
    setTimeout: timers.setTimeout,
    clearTimeout: timers.clearTimeout,
  });
  await stageDelegateFailure(mock, "oldest");
  await stageDelegateFailure(mock, "middle");
  await stageDelegateFailure(mock, "newest");
  assert(timers.cleared.includes(1), "oldest entry timer was not cleared");
  assert(await delegateToolResult(mock, "oldest") === undefined);
  assert((await delegateToolResult(mock, "middle"))?.details);
  assert((await delegateToolResult(mock, "newest"))?.details);
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
    registerShortcut: () => {},
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
      implementation: "read,write,edit,bash",
      reviewer: "read,grep,find,ls",
      exploration: "read,grep,find,ls",
      operations: "read,bash",
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
    if (role === "operations") {
      await tool(mock.tools, "rotta_delegate").execute("call", { role, task: "task" }, signal().signal, () => {}, mock.ctx)
        .then(() => { throw Error("unbound operations spawned"); }, () => {});
      assert(fixture.calls.length === 0);
      continue;
    }
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
    assert(allowed.includes("bash") === ["implementation", "operations"].includes(role), `${role} received wrong shell access`);
  }
});

Deno.test("operations gate denies missing, modified and replayed commands", () => {
  assert(!operationGate(undefined)({ command: "git status" }));
  const gate = operationGate("git status");
  assert(!gate({ command: "git status --short" }));
  assert(gate({ command: "git status" }));
  assert(!gate({ command: "git status" }));
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

Deno.test("delegate renderer stays compact until expanded", () => {
  const mock = host();
  registerRotta(mock.pi as any, { home: () => "/test-home" });
  const delegate = tool(mock.tools, "rotta_delegate") as any;
  const theme = {
    bold: (s: string) => s,
    fg: (_name: string, s: string) => s,
  };
  const callLines = delegate.renderCall(
    { role: "reviewer", task: "review the whole diff for blockers" },
    theme,
    {},
  ).render(120);
  assert(callLines.join("\n").includes("delegate reviewer"));
  const collapsed = delegate.renderResult(
    {
      content: [{ type: "text", text: "full child transcript" }],
      details: {
        role: "reviewer",
        running: true,
        timeoutMs: 300_000,
        elapsedMs: 12_000,
      },
    },
    { expanded: false, isPartial: true },
    theme,
    { args: { role: "reviewer" }, invalidate: () => {} },
  ).render(120).join("\n");
  assert(
    collapsed.includes("● running reviewer model=? effort=? 12s / timeout 5m 00s"),
  );
  assert(collapsed.includes("requested unavailable/unavailable"));
  assert(!collapsed.includes("full child transcript"));
  const expanded = delegate.renderResult(
    {
      content: [{ type: "text", text: "full child transcript" }],
      details: {
        role: "reviewer",
        elapsedMs: 15_000,
        outcome: "success",
        failed: true,
      },
    },
    { expanded: true, isPartial: false },
    theme,
    { args: { role: "reviewer" }, isError: false },
  ).render(120).join("\n");
  assert(expanded.includes("✓ complete reviewer model=? effort=? 15s"));
  assert(expanded.endsWith("full child transcript"));
  assert(
    expanded.split("\n").filter((line: string) => line.includes("full child transcript")).length === 1,
    "successful result duplicated or wrapped the child response",
  );
  const failed = delegate.renderResult(
    {
      content: [{ type: "text", text: "child timed out after 300000ms" }],
      details: { outcome: "error", reason: "timed out" },
    },
    { expanded: false, isPartial: false },
    theme,
    { args: { role: "reviewer" }, isError: true },
  ).render(120).join("\n");
  assert(
    failed.includes("✗ failed reviewer model=? effort=?") &&
      failed.includes("timed out"),
  );
});

Deno.test("latest Rotta action detail shortcut opens a bounded Escape-close overlay", async () => {
  const mock = host();
  registerRotta(mock.pi as any, { home: () => "/test-home" });
  assert(mock.shortcuts[0]?.shortcut === "ctrl+shift+o");
  (mock.events.tool_execution_start as any)(
    { toolName: "rotta_delegate", args: { role: "reviewer", task: "secret-free task" } },
    mock.ctx,
  );
  const delegate = tool(mock.tools, "rotta_delegate") as any;
  const clickable = delegate.renderCall(
    { role: "reviewer", task: "open details" },
    { bold: (s: string) => s, fg: (_: string, s: string) => s },
    {},
  );
  assert(
    JSON.stringify(clickable.handleMouse({ type: "click", button: "left" })) ===
      JSON.stringify({ handled: true }),
    "normalized left click did not return Pi's handled result",
  );
  assert(
    clickable.handleMouse({ type: "move", button: "left" }) === undefined,
    "nonmatching mouse event returned a handled result",
  );
  await Promise.resolve();
  assert(mock.overlays.length === 1, "normalized click did not open details");
  await mock.shortcuts[0].handler(mock.ctx);
  const shown = mock.overlays[1];
  const lines = shown.component.render(24);
  assert(lines.every((line: string) => visibleWidth(line) <= 24));
  shown.component.handleInput("\x1b");
  assert(shown.closed, "Escape did not close detail overlay");
});

Deno.test("detail overlay clamps repeated scrolling to non-empty content bounds", () => {
  const component = detailOverlayComponent(
    { requestRender() {} },
    { fg: (_: string, text: string) => text } as any,
    {
      service: "delegate",
      action: "reviewer",
      state: "complete",
      request: "request ".repeat(80),
      response: "response ".repeat(80),
    },
    () => {},
  );
  component.render(20);
  for (let index = 0; index < 100; index++) component.handleInput("\x1b[6~");
  const lines = component.render(20);
  assert(lines.length > 3, "overscroll emptied the detail viewport");
  assert(lines.some((line: string) => line.includes("response")));
});

Deno.test("delegate timeout status names role and honors bounded explicit timeout", async () => {
  const fixture = fakeSpawn({ wait: true });
  const mock = host();
  const updates: any[] = [];
  const originalSetTimeout = globalThis.setTimeout;
  const delays: number[] = [];
  try {
    (globalThis as any).setTimeout = (
      fn: (...args: unknown[]) => void,
      delay?: number,
    ) => {
      delays.push(Number(delay));
      queueMicrotask(fn);
      return 0 as any;
    };
    registerRotta(mock.pi as any, {
      spawn: fixture.spawn,
      home: () => "/test-home",
      defaultTimeoutMs: 1,
      maxTimeoutMs: 600_000,
    });
    await tool(mock.tools, "rotta_delegate").execute(
      "call",
      { role: "reviewer", task: "slow review", timeoutMs: 300_000 },
      signal().signal,
      (update: unknown) => updates.push(update),
      mock.ctx,
    ).then(() => {
      throw new Error("timeout unexpectedly succeeded");
    }, (error: Error) => {
      assert(
        error.message.includes("[reviewer] child timed out after 300000ms"),
      );
    });
    assert(
      delays.includes(300_000),
      `explicit timeout was not honored: ${delays}`,
    );
    assert(
      updates.some((update) =>
        update.content?.[0]?.text === "child running" &&
        update.details?.role === "reviewer" &&
        update.details?.timeoutMs === 300_000
      ) && updates.every((update) => !update.content?.[0]?.text?.includes("[reviewer]")),
      "running update exposed child output or omitted role/timeout metadata",
    );
  } finally {
    globalThis.setTimeout = originalSetTimeout;
  }
});

Deno.test("delegate defaults to ten minutes and clamps explicit timeout at thirty minutes", async () => {
  const originalSetTimeout = globalThis.setTimeout;
  const delays: number[] = [];
  try {
    (globalThis as any).setTimeout = (
      fn: (...args: unknown[]) => void,
      delay?: number,
    ) => {
      delays.push(Number(delay));
      queueMicrotask(fn);
      return 0 as any;
    };
    for (const [timeoutMs, expected] of [
      [undefined, 600_000],
      [9_999_999, 1_800_000],
    ] as const) {
      const fixture = fakeSpawn({ wait: true });
      const mock = host();
      const updates: any[] = [];
      registerRotta(mock.pi as any, {
        spawn: fixture.spawn,
        home: () => "/test-home",
      });
      await tool(mock.tools, "rotta_delegate").execute(
        "call",
        { role: "reviewer", task: "hard review", ...(timeoutMs ? { timeoutMs } : {}) },
        signal().signal,
        (update: unknown) => updates.push(update),
        mock.ctx,
      ).then(() => {
        throw new Error("timeout unexpectedly succeeded");
      }, (error: Error) => {
        assert(error.message.includes(`timed out after ${expected}ms`));
      });
      assert(delays.includes(expected), `missing wall-clock timeout ${expected}`);
      assert(updates[0]?.details?.timeoutMs === expected);
      assert(fixture.killed.includes("SIGTERM"));
    }
  } finally {
    globalThis.setTimeout = originalSetTimeout;
  }
});

Deno.test("delegate accepts ordinary final assistant text while preserving envelope validation", async () => {
  const assistantOutput = (content: string) =>
    JSON.stringify({
      type: "message_end",
      message: { role: "assistant", content },
    }) + "\n";
  const delegate = async (content: string) => {
    const fixture = fakeSpawn({ stdout: assistantOutput(content) });
    const mock = host();
    registerRotta(mock.pi as any, {
      spawn: fixture.spawn,
      home: () => "/test-home",
    });
    return await tool(mock.tools, "rotta_delegate").execute(
      "call",
      { role: "reviewer", task: "review" },
      signal().signal,
      () => {},
      mock.ctx,
    );
  };

  const prose = await delegate("Review complete: no blockers found.");
  assert(
    prose.content[0].text === "Review complete: no blockers found.",
    "ordinary final assistant text was not returned",
  );
  const bracketedProse = await delegate(
    "Review [routing] found {no blockers}.",
  );
  assert(
    bracketedProse.content[0].text ===
      "Review [routing] found {no blockers}.",
    "ordinary prose containing brackets or braces was rejected",
  );
  const json = await delegate('{"status":"success","output":"envelope"}');
  assert(json.content[0].text === "envelope");
  const fenced = await delegate(
    '```json\n{"status":"success","output":"fenced envelope"}\n```',
  );
  assert(fenced.content[0].text === "fenced envelope");
  await delegate('{"status":"error","message":"JSON child error"}').then(
    () => {
      throw new Error("JSON error envelope succeeded");
    },
    (error: Error) => assert(error.message.includes("JSON child error")),
  );
  await delegate(
    '```json\n{"status":"error","message":"fenced child error"}\n```',
  ).then(
    () => {
      throw new Error("fenced error envelope succeeded");
    },
    (error: Error) => assert(error.message.includes("fenced child error")),
  );
  await delegate('{"status":"success",}').then(() => {
    throw new Error("malformed JSON-looking output succeeded");
  }, (error: Error) => {
    assert(error.message.includes("child returned invalid result protocol"));
  });
  await delegate(
    '```json\n{"status":"success","output":"missing fence"}',
  ).then(
    () => {
      throw new Error("malformed fenced output succeeded");
    },
    (error: Error) => {
      assert(error.message.includes("child returned invalid result protocol"));
    },
  );
});

Deno.test("delegate invalidates stale results for oversized assistant records only", async () => {
  const earlier = JSON.stringify({
    type: "message_end",
    message: {
      role: "assistant",
      content: '{"status":"success","output":"earlier result"}',
    },
  });
  const oversizedMalformedAssistant = JSON.stringify({
    type: "message_end",
    message: {
      role: "assistant",
      content: '{"status":"success",' + "x".repeat(70_000),
    },
  });
  const fixture = fakeSpawn({
    stdout: `${earlier}\n${oversizedMalformedAssistant}\n`,
  });
  const mock = host();
  registerRotta(mock.pi as any, {
    spawn: fixture.spawn,
    home: () => "/test-home",
  });
  await tool(mock.tools, "rotta_delegate").execute(
    "call",
    { role: "reviewer", task: "review" },
    signal().signal,
    () => {},
    mock.ctx,
  ).then(() => {
    throw new Error("oversized authoritative record preserved stale result");
  }, (error: Error) => {
    assert(error.message.includes("child returned invalid result protocol"));
  });
});

Deno.test("delegate parses fragmented JSONL and UTF-8 but rejects nonzero child exits", async () => {
  const record = Buffer.from(
    JSON.stringify({
      type: "message_end",
      message: { role: "assistant", content: "fragmented café result" },
    }) + "\n",
  );
  const splitUtf8 = record.indexOf(Buffer.from("é")) + 1;
  const fragmented = fakeSpawn({
    stdout: [record.subarray(0, splitUtf8), record.subarray(splitUtf8)],
  });
  const mock = host();
  registerRotta(mock.pi as any, {
    spawn: fragmented.spawn,
    home: () => "/test-home",
  });
  const result = await tool(mock.tools, "rotta_delegate").execute(
    "call",
    { role: "reviewer", task: "review" },
    signal().signal,
    () => {},
    mock.ctx,
  );
  assert(result.content[0].text === "fragmented café result");

  const nonzero = fakeSpawn({
    stdout: JSON.stringify({
      type: "message_end",
      message: { role: "assistant", content: "valid but nonzero" },
    }) + "\n",
    code: 7,
  });
  const nonzeroHost = host();
  registerRotta(nonzeroHost.pi as any, {
    spawn: nonzero.spawn,
    home: () => "/test-home",
  });
  await tool(nonzeroHost.tools, "rotta_delegate").execute(
    "call",
    { role: "reviewer", task: "review" },
    signal().signal,
    () => {},
    nonzeroHost.ctx,
  ).then(() => {
    throw new Error("nonzero child exit succeeded after valid result");
  }, (error: Error) => {
    assert(error.message.includes("child exited 7"));
  });
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
  const finalAssistant = JSON.stringify({
    type: "message_end",
    message: {
      role: "assistant",
      content: '{"status":"success","output":"authoritative result"}',
    },
  });
  const trailingAgentEnd = JSON.stringify({
    type: "agent_end",
    history: "x".repeat(70_000),
  });
  const oversizedTrailing = fakeSpawn({
    stdout: `${finalAssistant}\n${trailingAgentEnd}`,
  });
  const oversizedHost = host();
  const updates: any[] = [];
  registerRotta(oversizedHost.pi as any, {
    spawn: oversizedTrailing.spawn,
    home: () => "/test-home",
  });
  const oversizedResult = await tool(
    oversizedHost.tools,
    "rotta_delegate",
  ).execute(
    "call",
    { role: "reviewer", task: "review" },
    signal().signal,
    (update: unknown) => updates.push(update),
    oversizedHost.ctx,
  );
  assert(
    oversizedResult.content[0].text === "authoritative result",
    "oversized trailing agent_end evicted the final assistant result",
  );
  assert(
    updates.every((update) =>
      Buffer.byteLength(update.content?.[0]?.text ?? "", "utf8") <= 65_536 +
          "[reviewer] ".length
    ),
    "model-facing child output exceeded its 64 KiB bound",
  );
  const unterminated = fakeSpawn({
    stdout: JSON.stringify({
      type: "message_end",
      message: { role: "assistant", content: "EOF final assistant result" },
    }),
  });
  const unterminatedHost = host();
  registerRotta(unterminatedHost.pi as any, {
    spawn: unterminated.spawn,
    home: () => "/test-home",
  });
  const unterminatedResult = await tool(
    unterminatedHost.tools,
    "rotta_delegate",
  ).execute(
    "call",
    { role: "reviewer", task: "review" },
    signal().signal,
    () => {},
    unterminatedHost.ctx,
  );
  assert(
    unterminatedResult.content[0].text ===
      "EOF final assistant result",
    "unterminated final JSONL record was not processed at close",
  );
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
    { role: "reviewer", task: "report status" },
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
  assert(
    plainResult.content[0].text === "Review complete: no blockers.",
  );
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
    assert(shell === undefined, "implementation development shell was blocked");
    for (const role of ["reviewer", "exploration", "operations"]) {
      const other = await loadGuard(role);
      const guarded = host();
      other(guarded.pi as any);
      const decision = await (guarded.events.tool_call as any)({
        toolName: "bash", input: { command: "true" },
      });
      assert(decision?.block, `${role} received unbound shell`);
    }
    const gate = operationGate("printf safe");
    assert(!gate({ command: "printf changed" }), "altered command accepted");
    assert(gate({ command: "printf safe" }), "matching command rejected");
    assert(!gate({ command: "printf safe" }), "operation replay accepted");
  } finally {
    Deno.removeSync(fixture, { recursive: true });
    Deno.removeSync(external, { recursive: true });
  }
});

Deno.test("exact operation consent binds dispatch once without executing on consent", async () => {
  const project = Deno.makeTempDirSync();
  const target = `${project}/target`;
  Deno.mkdirSync(target);
  try {
    const fake = fakeSpawn({ stdout: JSON.stringify({ status: "success", output: "safe result" }) + "\n" });
    const mock = host();
    mock.ctx.cwd = project;
    registerRotta(mock.pi as any, { spawn: fake.spawn });
    const question = tool(mock.tools, "rotta_question");
    const delegate = tool(mock.tools, "rotta_delegate");
    const command = "printf safe";
    Deno.mkdirSync(`${project}/.rotta/ops`, { recursive: true });
    const artifactPath = `${project}/.rotta/ops/exact.md`;
    const action = "external command";
    const effect = "harmless output";
    const bytes = `Action: ${action}\nCommand: ${command}\nTarget: ${target}\nEffect: ${effect}\nRevision: 1\n`;
    Deno.writeTextFileSync(artifactPath, bytes);
    const revision = 1;
    const digest = createHash("sha256").update(bytes).digest("hex");
    const operation = { requestId: "exact", command, target, action, effect, artifactPath, revision, digest };
    const request = {
      trigger: "external-consent", requestId: operation.requestId, workspace: project,
      action, effect, decision: `Action: ${action}\nCommand: ${command}\nTarget: ${target}\nEffect: ${effect}\nWorkspace: ${project}\nArtifact: ${artifactPath}\nDigest: ${digest}\nRevision: ${revision}\nScope: one execution`,
      options: ["Approve the exact rendered operation once", "Stop"], safeOutcome: "Stop",
      command, target, operationPath: artifactPath, operationRevision: revision, operationDigest: digest,
    };
    const dispatch = (binding: typeof operation, ctx = mock.ctx) => delegate.execute(
      "dispatch", { role: "operations", task: "run exact harmless command", operation: binding },
      signal().signal, () => {}, ctx,
    );
    const callCount = () => fake.calls.length;
    const denied = async (binding: typeof operation, ctx = mock.ctx) => {
      const before = callCount();
      await dispatch(binding, ctx).then(() => { throw new Error("invalid operation dispatched"); }, () => {});
      assert(fake.calls.length === before, "invalid operation spawned a child");
    };
    await denied(operation);
    await denied({ ...operation, artifactPath: `${project}/.rotta/ops/missing.md` });
    const answer = await question.execute("consent", request, signal().signal, () => {}, mock.ctx);
    assert(answer.content[0].text === request.options[0]);
    assert(callCount() === 0, "consent executed an operation");
    await denied({ ...operation, command: "printf changed" });
    await denied(operation); // mismatch consumes authorization before dispatch
    await question.execute("consent-again", request, signal().signal, () => {}, mock.ctx);
    await denied({ ...operation, target: project });
    await question.execute("consent-third", request, signal().signal, () => {}, mock.ctx);
    await denied(operation, { ...mock.ctx, sessionManager: { getSessionId: () => "other-session" } });
    await question.execute("consent-final", request, signal().signal, () => {}, mock.ctx);
    const result = await dispatch(operation);
    assert(result.content[0].text === "safe result", "matching child result missing");
    assert(callCount() === 1, "matching operation did not dispatch once");
    const childEnv = (fake.calls[0][2] as { env: Record<string, string> }).env;
    assert(childEnv.ROTTA_OPERATION_COMMAND === command, "child lacks exact command binding");
    await denied(operation);
    for (const field of ["command", "action", "target", "effect"] as const) {
      const alternate = field === "target" ? project : `changed ${field}`;
      await question.execute(`mismatch-${field}`, { ...request, [field]: alternate }, signal().signal, () => {}, mock.ctx)
        .then(() => { throw Error(`modified ${field} was approved`); }, () => {});
      assert(callCount() === 1, `modified ${field} spawned child`);
      await question.execute(`dispatch-${field}`, request, signal().signal, () => {}, mock.ctx);
      await denied({ ...operation, [field]: alternate });
    }
    for (const invalid of [
      bytes + "Command: printf safe\n", bytes.replace(`Effect: ${effect}\n`, ""),
      bytes.replace("Revision: 1", "Revision: 2"),
    ]) {
      Deno.writeTextFileSync(artifactPath, invalid);
      await question.execute("invalid-artifact", { ...request, operationDigest: createHash("sha256").update(invalid).digest("hex") }, signal().signal, () => {}, mock.ctx)
        .then(() => { throw Error("invalid artifact approved"); }, () => {});
      assert(callCount() === 1);
    }
    Deno.writeTextFileSync(artifactPath, bytes);
    await question.execute("consent-revision", request, signal().signal, () => {}, mock.ctx);
    await denied({ ...operation, revision: 2 });
    await question.execute("consent-artifact", request, signal().signal, () => {}, mock.ctx);
    Deno.writeTextFileSync(artifactPath, bytes + "changed\n");
    await denied(operation);
    await question.execute("changed-artifact", request, signal().signal, () => {}, mock.ctx)
      .then(() => { throw Error("changed artifact approved"); }, () => {});
    const outside = Deno.makeTempDirSync();
    try {
      await question.execute("external-target", { ...request, target: outside }, signal().signal, () => {}, mock.ctx)
        .then(() => { throw new Error("external target approved"); }, () => {});
      assert(callCount() === 1, "invalid target caused execution");
    } finally { Deno.removeSync(outside, { recursive: true }); }
  } finally { Deno.removeSync(project, { recursive: true }); }
});

Deno.test("child MCP guard permits required memory lifecycle but never Vela outside exploration", () => {
  assert(isAllowedChildMCP("implementation", "rotta_ancora_summarize"));
  assert(!isAllowedChildMCP("implementation", "rotta_vela_explore"));
  assert(isAllowedChildMCP("exploration", "rotta_vela_module_summary"));
  assert(!isAllowedChildMCP("reviewer", "rotta_context7_delete"));
});

Deno.test("strict approval accepts one established contract revision identity", async () => {
  const project = Deno.makeTempDirSync();
  const contractPath = `${project}/.rotta/strict/contract.md`;
  Deno.mkdirSync(`${project}/.rotta/strict`, { recursive: true });
  try {
    for (
      const [name, contractBytes] of [
        ["plain", "Revision: 1\n"],
        ["plain spacing", "  Revision:   1  \n"],
        ["plain release prefix", "Revision: r1\n"],
        ["bold", "**Revision:** `1`\n"],
        ["bold release prefix", "**Revision:** `r1`\n"],
        ["bold spacing", "**Revision:**\t `1` \n"],
        ["heading observed", "# Recurring payment sources — execution contract (revision 1)\n"],
        ["heading release prefix", "# Contract (revision r1)\n"],
      ]
    ) {
      Deno.writeTextFileSync(contractPath, contractBytes);
      const mock = host();
      mock.ctx.cwd = project;
      registerRotta(mock.pi as any);
      const answer = await tool(mock.tools, "rotta_question").execute(
        name,
        {
          trigger: "strict-approval",
          requestId: name,
          workspace: project,
          action: "approve",
          decision: "Approve revision 1",
          options: ["Approve", "Stop"],
          safeOutcome: "Stop",
          contractPath: ".rotta/strict/contract.md",
          ...(name === "heading observed" ? {} : { contractRevision: 1 }),
          contractDigest: createHash("sha256").update(contractBytes).digest(
            "hex",
          ),
        },
        signal().signal,
        () => {},
        mock.ctx,
      );
      assert(answer.content[0].text === "Approve", `${name} was rejected`);
      if (name === "heading observed") {
        const title = (mock.selects[0] as { title: string }).title;
        assert(title.includes(`Exact contract: ${contractPath}`), "canonical contract path not rendered");
        assert(title.includes(`SHA-256: ${createHash("sha256").update(contractBytes).digest("hex")}`), "digest not rendered");
        assert(title.includes("Revision: 1"), "verified revision not rendered");
        for (const override of [{ contractRevision: 2 }, { contractDigest: "0".repeat(64) }]) {
          await tool(mock.tools, "rotta_question").execute(
            `mismatch-${Object.keys(override)[0]}`,
            {
              trigger: "strict-approval", requestId: "heading mismatch", workspace: project,
              action: "approve", decision: "Approve revision 1", options: ["Approve", "Stop"],
              safeOutcome: "Stop", contractPath: ".rotta/strict/contract.md",
              contractRevision: 1, contractDigest: createHash("sha256").update(contractBytes).digest("hex"),
              ...override,
            }, signal().signal, () => {}, mock.ctx,
          ).then(() => { throw Error("mismatched heading identity accepted"); },
            (error: Error) => assert(error.message.includes("safe stop: approval identity mismatch")));
        }
      }
    }
  } finally {
    Deno.removeSync(project, { recursive: true });
  }
});

Deno.test("strict approval rejects malformed, ambiguous, and prose revision identity", async () => {
  const project = Deno.makeTempDirSync();
  const contractPath = `${project}/.rotta/strict/contract.md`;
  Deno.mkdirSync(`${project}/.rotta/strict`, { recursive: true });
  try {
    for (
      const [name, contractBytes] of [
        ["missing", "# Contract\n"],
        ["nonnumeric", "Revision: one\n"],
        ["invalid release prefix", "Revision: release1\n"],
        ["nonnumeric alongside valid", "Revision: one\nRevision: 1\n"],
        ["unbalanced backticks", "Revision: `1\n"],
        ["plain backticked", "Revision: `1`\n"],
        ["bold unbackticked", "**Revision:** 1\n"],
        ["conflicting", "Revision: 1\n**Revision:** `2`\n"],
        ["duplicate", "Revision: 1\nRevision: 1\n"],
        ["plain prose", "This sentence mentions Revision: 1.\n"],
        ["bold prose", "This sentence mentions **Revision:** `1`.\n"],
        ["heading malformed", "# Contract (revision one)\n"],
        ["heading trailing prose", "# Contract (revision 1) extra\n"],
        ["heading duplicate", "# Contract (revision 1)\n# Other (revision 1)\n"],
        ["heading conflicting", "# Contract (revision 1)\n# Other (revision 2)\n"],
        ["heading plus standalone", "# Contract (revision 1)\nRevision: 1\n"],
        ["malformed heading plus standalone", "# Contract (revision one)\nRevision: 1\n"],
      ]
    ) {
      Deno.writeTextFileSync(contractPath, contractBytes);
      const mock = host();
      mock.ctx.cwd = project;
      registerRotta(mock.pi as any);
      await tool(mock.tools, "rotta_question").execute(
        name,
        {
          trigger: "strict-approval",
          requestId: name,
          workspace: project,
          action: "approve",
          decision: "Approve revision 1",
          options: ["Approve", "Stop"],
          safeOutcome: "Stop",
          contractPath: ".rotta/strict/contract.md",
          contractRevision: 1,
          contractDigest: createHash("sha256").update(contractBytes).digest(
            "hex",
          ),
        },
        signal().signal,
        () => {},
        mock.ctx,
      ).then(
        () => {
          throw new Error(`${name} revision identity was accepted`);
        },
        (error: Error) => {
          assert(
            error.message.includes("safe stop: approval identity mismatch"),
            `${name} failed for the wrong reason: ${error.message}`,
          );
        },
      );
      assert(mock.selects.length === 0, `${name} reached the approval UI`);
    }
  } finally {
    Deno.removeSync(project, { recursive: true });
  }
});

Deno.test("validated questions bracket every UI outcome with Herdr blocked events", async () => {
  const project = Deno.makeTempDirSync();
  const request = {
    trigger: "policy-decision",
    requestId: "attention",
    workspace: project,
    action: "choose",
    decision: "Choose the safe policy",
    options: ["Apply", "Stop"],
    safeOutcome: "Stop",
  };
  const blockedEvents = (mock: ReturnType<typeof host>) =>
    mock.emittedEvents.filter((entry) => entry.event === "herdr:blocked");
  const assertLifecycle = (mock: ReturnType<typeof host>, label: string) => {
    const events = blockedEvents(mock);
    assert(events.length === 2, `${label} emitted ${events.length} blocked events`);
    assert(
      JSON.stringify(events.map((entry) => entry.data)) === JSON.stringify([
        { active: true, label: request.decision },
        { active: false, label: request.decision },
      ]),
      `${label} emitted the wrong blocked lifecycle`,
    );
  };
  try {
    const success = host();
    success.ctx.cwd = project;
    registerRotta(success.pi as any);
    const answer = await tool(success.tools, "rotta_question").execute(
      "success",
      request,
      signal().signal,
      () => {},
      success.ctx,
    );
    assert(answer.content[0].text === "Apply");
    assertLifecycle(success, "success");
    assert(
      success.activity.indexOf("emit:herdr:blocked:true") <
        success.activity.indexOf("select"),
      "active event was not emitted immediately before selection",
    );

    const racedAbort = new AbortController();
    const abortedChoice = host();
    abortedChoice.ctx.cwd = project;
    abortedChoice.ctx.ui.select = async () => {
      racedAbort.abort();
      return "Apply";
    };
    registerRotta(abortedChoice.pi as any);
    await tool(abortedChoice.tools, "rotta_question").execute(
      "aborted-choice",
      request,
      racedAbort.signal,
      () => {},
      abortedChoice.ctx,
    ).then(() => {
      throw new Error("choice returned after abort unexpectedly succeeded");
    }, () => {});
    assertLifecycle(abortedChoice, "choice returned after abort");

    for (const [label, select] of [
      ["invalid answer", async () => "invalid"],
      ["cancellation", async () => null],
      ["UI throw", async () => { throw new Error("renderer exploded"); }],
    ] as const) {
      const mock = host();
      mock.ctx.cwd = project;
      mock.ctx.ui.select = select as any;
      registerRotta(mock.pi as any);
      await tool(mock.tools, "rotta_question").execute(
        label,
        request,
        signal().signal,
        () => {},
        mock.ctx,
      ).then(() => {
        throw new Error(`${label} unexpectedly succeeded`);
      }, () => {});
      assertLifecycle(mock, label);
    }

    for (const [label, params, ctx] of [
      ["bad workspace", { ...request, workspace: `${project}/missing` }, undefined],
      ["headless", request, { hasUI: false }],
      ["invalid binding", { ...request, options: ["Stop", "Stop"] }, undefined],
    ] as const) {
      const mock = host();
      mock.ctx.cwd = project;
      registerRotta(mock.pi as any);
      await tool(mock.tools, "rotta_question").execute(
        label,
        params,
        signal().signal,
        () => {},
        ctx ? { ...mock.ctx, ...ctx } : mock.ctx,
      ).then(() => {
        throw new Error(`${label} unexpectedly succeeded`);
      }, () => {});
      assert(blockedEvents(mock).length === 0, `${label} emitted a Herdr event`);
    }
  } finally {
    Deno.removeSync(project, { recursive: true });
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
      { ...request, trigger: trigger === "external-consent" ? "policy-decision" : trigger },
      signal().signal,
      () => {},
      mock.ctx,
    );
    assert(answer.content[0].text === "Approve", `${trigger} was not accepted`);
  }
  const legacy = await question.execute("legacy-consent", { ...request, trigger: "external-consent" },
    signal().signal, () => {}, mock.ctx);
  assert(legacy.content[0].text === "Approve");
  await question.execute("partial-consent", { ...request, trigger: "external-consent", command: "printf x" },
    signal().signal, () => {}, mock.ctx).then(() => {
    throw new Error("partial consent was accepted");
  }, (error: Error) => assert(error.message.includes("incomplete exact operation binding")));
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
  }, (error: Error) => {
    assert(error.message.includes("(workspace)"));
    assert(!error.message.includes(project));
  });
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
    { ...request, contractDigest: "wrong", contractRevision: 99 },
    signal().signal,
    () => {},
    mock.ctx,
  ).then(() => {
    throw new Error("bad approval succeeded");
  }, (error: Error) => {
    assert(error.message.includes("(digest, revision)"));
    assert(!error.message.includes(request.contractDigest));
    assert(!error.message.includes("wrong"));
  });

  const waiting = host();
  waiting.ctx.cwd = project;
  let releaseWaiting!: (answer: string) => void;
  let waitingOptions: { signal: AbortSignal; timeout?: number } | undefined;
  waiting.ctx.ui.select = (async (
    _title: string,
    _options: string[],
    options?: { signal: AbortSignal; timeout?: number },
  ) => {
    waitingOptions = options;
    return await new Promise<string>((resolve) => {
      releaseWaiting = resolve;
    });
  }) as any;
  registerRotta(waiting.pi as any);
  let settled = false;
  const waitingController = signal();
  const stillWaiting = tool(waiting.tools, "rotta_question").execute(
    "waiting",
    request,
    waitingController.signal,
    () => {},
    waiting.ctx,
  ).finally(() => (settled = true));
  await new Promise((resolve) => setTimeout(resolve, 35));
  assert(!settled, "question retained the old short timeout seam");
  assert(
    waitingOptions?.signal === waitingController.signal &&
      waitingOptions.timeout === undefined,
    "question select did not omit the UI timeout while forwarding cancellation",
  );
  releaseWaiting("Approve");
  assert((await stillWaiting).content[0].text === "Approve");

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
  }, (error: Error) => {
    assert(error.message.includes("cancelled, stale, or invalid decision"));
    assert(!error.message.includes("question UI failed"));
  });
  const invalidHost = host();
  invalidHost.ctx.cwd = project;
  invalidHost.ctx.ui.select = (async () => "not an option") as any;
  registerRotta(invalidHost.pi as any);
  await tool(invalidHost.tools, "rotta_question").execute(
    "invalid",
    request,
    signal().signal,
    () => {},
    invalidHost.ctx,
  ).then(() => {
    throw new Error("invalid selection succeeded");
  }, (error: Error) => {
    assert(error.message.includes("cancelled, stale, or invalid decision"));
    assert(!error.message.includes("question UI failed"));
  });
  const brokenHost = host();
  brokenHost.ctx.cwd = project;
  brokenHost.ctx.ui.select = (async () => {
    throw new Error("renderer exploded");
  }) as any;
  registerRotta(brokenHost.pi as any);
  await tool(brokenHost.tools, "rotta_question").execute(
    "broken",
    request,
    signal().signal,
    () => {},
    brokenHost.ctx,
  ).then(() => {
    throw new Error("broken UI succeeded");
  }, (error: Error) => {
    assert(error.message.includes("question UI failed"));
  });
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
  }, (error: Error) => {
    assert(error.message.includes("cancelled, stale, or invalid decision"));
    assert(!error.message.includes("question UI failed"));
  });
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
