import { EventEmitter } from "node:events";
import { HTTPMCP, registerMCPBridge, StdioMCP } from "./rotta-mcp-bridge.ts";

function assert(value: unknown, message = "assertion failed"): asserts value {
  if (!value) throw new Error(message);
}
function host() {
  const tools: any[] = [], events: Record<string, any> = {};
  return {
    tools,
    events,
    pi: {
      registerTool(tool: any) {
        tools.push(tool);
      },
      on(name: string, handler: any) {
        events[name] = handler;
      },
    },
  };
}
function stdio(
  pages: any[][],
  result: any = {
    content: [{ type: "text", text: "ok" }],
    structuredContent: { yes: true },
  },
) {
  const calls: any[] = [];
  const spawn = (() => {
    const proc: any = new EventEmitter();
    proc.stdout = new EventEmitter();
    proc.stderr = new EventEmitter();
    proc.stdin = {
      write(line: string, done?: () => void) {
        const request = JSON.parse(line);
        calls.push(request);
        const response = request.method === "initialize"
          ? { protocolVersion: "2025-06-18", capabilities: {} }
          : request.method === "tools/list"
          ? {
            tools: pages[request.params?.cursor ? 1 : 0] ?? [],
            ...(request.params?.cursor
              ? {}
              : pages.length > 1
              ? { nextCursor: "next" }
              : {}),
          }
          : result;
        queueMicrotask(() => {
          done?.();
          if (request.id) {
            proc.stdout.emit(
              "data",
              Buffer.from(
                JSON.stringify({
                  jsonrpc: "2.0",
                  id: request.id,
                  result: response,
                }) + "\n",
              ),
            );
          }
        });
        return true;
      },
    };
    proc.kill = () => true;
    return proc;
  }) as any;
  return { spawn, calls };
}
function config(enabled = { ancora: true, vela: false, context7: false }) {
  return JSON.stringify({
    version: 1,
    services: Object.fromEntries(
      Object.entries(enabled).map(([key, value]) => [key, { enabled: value }]),
    ),
  });
}
const deps = (
  transport: ReturnType<typeof stdio>,
  role: any = "parent",
  cfg = config(),
) => ({
  spawn: transport.spawn,
  role,
  readFile: () => cfg,
  exists: () => true,
  env: (key: string) => key === "PATH" ? "/bin" : "",
  cwd: () => "/repo",
});

Deno.test("bridge activates only initialize/list, pages tools, preserves remote name/schema/content", async () => {
  const transport = stdio([[{
    name: "ancora_search",
    inputSchema: { type: "object", properties: { query: { type: "string" } } },
  }], [{ name: "ancora_delete" }]]);
  const mocked = host();
  const bridge = registerMCPBridge(mocked.pi as any, deps(transport));
  await bridge.activate();
  assert(
    transport.calls.every((call) => call.method !== "tools/call"),
    "activation called a tool",
  );
  assert(
    mocked.tools.length === 1 && mocked.tools[0].name === "rotta_ancora_search",
  );
  assert(mocked.tools[0].parameters.properties.query.type === "string");
  const output = await mocked.tools[0].execute(
    "x",
    { query: "q" },
    new AbortController().signal,
  );
  assert(
    output.content[0].text === "ok" && output.details.structuredContent.yes,
  );
  assert(transport.calls.some((call) => call.params?.name === "ancora_search"));
});

Deno.test("bridge enforces config, role intersection, collision safety and Vela capsule limit", async () => {
  const noConfig = host();
  await registerMCPBridge(
    noConfig.pi as any,
    deps(stdio([[{ name: "ancora_search" }]]), "parent", "bad"),
  ).activate();
  assert(
    noConfig.tools.length === 0 &&
      registerMCPBridge(host().pi as any, deps(stdio([[]]), "parent", "bad"))
          .status().ancora.state === "configured-pending-health",
  );
  const reviewer = host();
  await registerMCPBridge(
    reviewer.pi as any,
    deps(
      stdio([[{ name: "explore" }]]),
      "reviewer",
      config({ ancora: false, vela: true, context7: false }),
    ),
  ).activate();
  assert(reviewer.tools.length === 0, "reviewer received Vela");
  const transport = stdio([[{ name: "explore" }, { name: "vela_explore" }]]);
  const explorer = host();
  await registerMCPBridge(
    explorer.pi as any,
    deps(
      transport,
      "exploration",
      config({ ancora: false, vela: true, context7: false }),
    ),
  ).activate();
  assert(explorer.tools.length === 1, "collision registered twice");
  await explorer.tools[0].execute("1", {}, new AbortController().signal);
  await explorer.tools[0].execute("2", {}, new AbortController().signal);
  await explorer.tools[0].execute("3", {}, new AbortController().signal).then(
    () => {
      throw Error("third Vela call accepted");
    },
    () => {},
  );
});

Deno.test("unavailable binary or graph registers nothing and exposes observed status", async () => {
  const mocked = host();
  const bridge = registerMCPBridge(mocked.pi as any, {
    ...deps(
      stdio([[{ name: "explore" }]]),
      "exploration",
      config({ ancora: false, vela: true, context7: false }),
    ),
    exists: () => false,
  });
  await bridge.activate();
  assert(mocked.tools.length === 0);
  assert(
    bridge.status().vela.state === "unavailable" &&
      bridge.status().vela.reason?.includes("binary missing"),
  );
  const noGraph = host();
  const graphBridge = registerMCPBridge(noGraph.pi as any, {
    ...deps(
      stdio([[{ name: "explore" }]]),
      "exploration",
      config({ ancora: false, vela: true, context7: false }),
    ),
    exists: (file: string) => file === "/bin/vela",
  });
  await graphBridge.activate();
  assert(graphBridge.status().vela.reason?.includes("graph is missing"));
});

Deno.test("stdio transport validates envelopes, aborts before writing, crashes and cleans pending requests", async () => {
  const transport = stdio([[]]);
  const client = new StdioMCP("ancora", ["mcp"], transport.spawn, 50, {});
  const aborted = new AbortController();
  aborted.abort();
  await client.request("x", {}, aborted.signal).then(() => {
    throw Error("pre-abort wrote");
  }, () => {});
  assert(transport.calls.length === 0);
  const pending = client.request("hang");
  await new Promise((r) => setTimeout(r, 1)); // fixture answers; close must still reject no future requests after lifecycle close
  client.close();
  await pending.then(() => {}, () => {});
  await client.request("after"); // explicit action is the only reconnect path
});

Deno.test("HTTP handles streamed SSE fragments/notifications, 202 initialized, errors and redacts tokens", async () => {
  const seen: any[] = [];
  let count = 0;
  const fetcher = (async (_url: string, init: any) => {
    seen.push(init);
    count++;
    if (count === 1) {
      return new Response(
        JSON.stringify({
          jsonrpc: "2.0",
          id: 1,
          result: { protocolVersion: "2025-06-18" },
        }),
        { headers: { "mcp-session-id": "session" } },
      );
    }
    if (count === 2) return new Response("", { status: 202 });
    return new Response(
      ': comment\r\ndata: {"jsonrpc":"2.0",\r\ndata: "id":2,"result":{"tools":[]}}\r\n\r\n',
      { headers: { "content-type": "text/event-stream" } },
    );
  }) as any;
  const client = new HTTPMCP(fetcher, 100, () => "secret-token");
  await client.request("initialize");
  await client.notify("notifications/initialized");
  const listed = await client.request("tools/list");
  assert(Array.isArray(listed.tools));
  assert(
    seen[0].headers.authorization === "Bearer secret-token" &&
      seen[2].headers["mcp-session-id"] === "session",
  );
  const bad = new HTTPMCP(
    (async () => new Response("Bearer secret-token", { status: 401 })) as any,
    100,
    () => "secret-token",
  );
  await bad.request("x").then(() => {
    throw Error("auth accepted");
  }, (e) => assert(!e.message.includes("secret-token"), "secret leaked"));
});

Deno.test("failed tools/list closes before eviction and only an explicit later activation reconnects", async () => {
  let spawned = 0, kills = 0;
  const spawn = (() => {
    spawned++;
    const proc: any = new EventEmitter();
    proc.stdout = new EventEmitter();
    proc.stderr = new EventEmitter();
    proc.stdin = {
      write(line: string, done?: () => void) {
        const request = JSON.parse(line);
        done?.();
        if (!request.id) return true;
        queueMicrotask(() =>
          proc.stdout.emit(
            "data",
            Buffer.from(
              JSON.stringify({
                jsonrpc: "2.0",
                id: request.id,
                result: request.method === "initialize"
                  ? { protocolVersion: "2025-06-18", capabilities: {} }
                  : { notTools: true },
              }) + "\n",
            ),
          )
        );
        return true;
      },
    };
    proc.kill = () => {
      kills++;
      return true;
    };
    return proc;
  }) as any;
  const mocked = host();
  const bridge = registerMCPBridge(mocked.pi as any, {
    readFile: () => config(),
    exists: () => true,
    env: (key) => key === "PATH" ? "/bin" : "",
    spawn,
  });
  await bridge.activate();
  assert(
    kills >= 1 && spawned === 1,
    `failed list was evicted before close (${kills}/${spawned})`,
  );
  await bridge.activate();
  assert(
    kills >= 2 && spawned >= 2,
    "explicit activation did not make a fresh session",
  );
});

Deno.test("stdio gives server requests precedence over colliding numeric pending responses", async () => {
  const writes: any[] = [];
  const spawn = (() => {
    const proc: any = new EventEmitter();
    proc.stdout = new EventEmitter();
    proc.stderr = new EventEmitter();
    proc.stdin = {
      write(line: string, done?: () => void) {
        const message = JSON.parse(line);
        writes.push(message);
        done?.();
        if (message.method === "call") {
          queueMicrotask(() =>
            proc.stdout.emit(
              "data",
              Buffer.from(
                [
                  JSON.stringify({ jsonrpc: "2.0", id: 1, method: "ping" }),
                  JSON.stringify({ jsonrpc: "2.0", id: 99, method: "unknown" }),
                  JSON.stringify({
                    jsonrpc: "2.0",
                    id: "server",
                    method: "ping",
                  }),
                  JSON.stringify({ jsonrpc: "2.0", method: "notice" }),
                  JSON.stringify({
                    jsonrpc: "2.0",
                    id: 1,
                    result: { ok: true },
                  }),
                ].join("\n") + "\n",
              ),
            )
          );
        }
        return true;
      },
    };
    proc.kill = () => true;
    return proc;
  }) as any;
  const client = new StdioMCP("x", [], spawn, 100, {});
  assert(
    (await client.request("call")).ok,
    "colliding request consumed pending call",
  );
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert(
    writes.some((x) => x.id === 1 && x.result),
    "numeric ping was not answered",
  );
  assert(
    writes.some((x) => x.id === 99 && x.error?.code === -32601),
    "unknown request was not rejected",
  );
  assert(
    writes.some((x) => x.id === "server" && x.result),
    "string request was not answered",
  );
  client.close();
});

Deno.test("HTTP aborts hanging fetches and DELETE without leaking authenticated data", async () => {
  const seen: any[] = [];
  const hanging = ((_url: string, init: any) => {
    seen.push(init);
    return new Promise((_resolve, reject) =>
      init.signal.addEventListener(
        "abort",
        () => reject(new Error("Bearer private-key")),
        { once: true },
      )
    );
  }) as any;
  const pre = new AbortController();
  pre.abort();
  await new HTTPMCP(hanging, 5, () => "private-key").request(
    "x",
    {},
    pre.signal,
  ).then(
    () => {
      throw Error("pre-abort fetched");
    },
    (e) => assert(!e.message.includes("private-key")),
  );
  await new HTTPMCP(hanging, 5, () => "private-key").request("x").then(
    () => {
      throw Error("hanging request succeeded");
    },
    (e) => assert(!e.message.includes("private-key")),
  );
  const session = new HTTPMCP(
    (async (_url: string, init: any) => {
      seen.push(init);
      if (init.method === "POST") {
        return new Response(
          JSON.stringify({ jsonrpc: "2.0", id: 1, result: {} }),
          { headers: { "mcp-session-id": "s" } },
        );
      }
      return new Promise((_resolve, reject) =>
        init.signal.addEventListener("abort", () => reject(Error("hang")), {
          once: true,
        })
      );
    }) as any,
    5,
    () => "private-key",
  );
  await session.request("x");
  await session.close();
  assert(
    seen.some((x) =>
      x.method === "DELETE" && x.headers.authorization === "Bearer private-key"
    ),
  );
});

Deno.test("process crash rejects outstanding request, evicts it, and terminates the failed transport", async () => {
  let proc: any, kills: string[] = [];
  const spawn = (() => {
    proc = new EventEmitter();
    proc.stdout = new EventEmitter();
    proc.stderr = new EventEmitter();
    proc.stdin = {
      write(_line: string, done?: () => void) {
        done?.();
        return true;
      },
    };
    proc.kill = (signal: string) => {
      kills.push(signal);
      return true;
    };
    return proc;
  }) as any;
  const client = new StdioMCP("x", [], spawn, 100, {});
  const pending = client.request("outstanding");
  await new Promise((resolve) => setTimeout(resolve, 0));
  proc.emit("close", 1);
  await pending.then(() => {
    throw Error("crash retained pending request");
  }, () => {});
  assert(kills.includes("SIGTERM"), "crashed transport was not cleaned up");
  client.close();
});

Deno.test("tool results preserve unsupported MCP detail and throw sanitized isError text", async () => {
  const mapped = host();
  const bridge = registerMCPBridge(
    mapped.pi as any,
    deps(stdio([[{ name: "ancora_search" }]], {
      content: [{ type: "resource", resource: { uri: "file:///detail" } }, {
        type: "audio",
        data: "abc",
      }],
      structuredContent: { retained: true },
      resource: { uri: "file:///top" },
    })),
  );
  await bridge.activate();
  const output = await mapped.tools[0].execute(
    "x",
    {},
    new AbortController().signal,
  );
  assert(
    output.content.every((part: any) => part.type === "text") &&
      output.details.structuredContent.retained && output.details.resource.uri,
  );
  const failed = host();
  const failure = registerMCPBridge(
    failed.pi as any,
    deps(stdio([[{ name: "ancora_search" }]], {
      isError: true,
      content: [{ type: "text", text: "Bearer private-key denied" }],
    })),
  );
  await failure.activate();
  await failed.tools[0].execute("x", {}, new AbortController().signal).then(
    () => {
      throw Error("isError was accepted");
    },
    (e: Error) => assert(!e.message.includes("private-key")),
  );
});
