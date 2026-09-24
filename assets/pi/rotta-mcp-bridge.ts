// Managed MCP bridge. It is loaded explicitly by the parent and isolated Pi children.
import { spawn as nodeSpawn } from "node:child_process";
import * as fs from "node:fs";
import * as path from "node:path";
import { StringDecoder } from "node:string_decoder";
import { Type } from "typebox";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

const LIMIT = 64 * 1024, PAGES = 32, PENDING = 64, TIMEOUT = 20_000;
const PROTOCOL = "2025-06-18";
const SERVICES = ["ancora", "vela", "context7"] as const;
type Service = typeof SERVICES[number];
type Role =
  | "parent"
  | "implementation"
  | "reviewer"
  | "exploration"
  | "operations";
type Tool = {
  name: string;
  description?: string;
  inputSchema?: Record<string, unknown>;
};
type Config = { version: 1; services: Record<Service, { enabled: boolean }> };
export type BridgeDependencies = {
  spawn?: typeof nodeSpawn;
  fetch?: typeof fetch;
  home?: () => string;
  cwd?: () => string;
  role?: Role;
  timeoutMs?: number;
  readFile?: (file: string) => string;
  exists?: (file: string) => boolean;
  env?: (key: string) => string | undefined;
};

const allow: Record<Role, Record<Service, Set<string>>> = {
  parent: {
    ancora: new Set([
      "save",
      "search",
      "context",
      "summarize",
      "start",
      "end",
      "get",
      "suggest_topic",
      "capture",
      "save_prompt",
      "update",
    ]),
    vela: new Set(),
    context7: new Set(["resolve-library-id", "query-docs"]),
  },
  implementation: {
    ancora: new Set([
      "save",
      "summarize",
      "start",
      "end",
      "search",
      "context",
      "get",
    ]),
    vela: new Set(),
    context7: new Set(["resolve-library-id", "query-docs"]),
  },
  reviewer: {
    ancora: new Set([
      "save",
      "summarize",
      "start",
      "end",
      "search",
      "context",
      "get",
    ]),
    vela: new Set(),
    context7: new Set(["resolve-library-id", "query-docs"]),
  },
  exploration: {
    ancora: new Set([
      "save",
      "summarize",
      "start",
      "end",
      "search",
      "context",
      "get",
    ]),
    vela: new Set([
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
    ]),
    context7: new Set(["resolve-library-id", "query-docs"]),
  },
  operations: {
    ancora: new Set([
      "save",
      "summarize",
      "start",
      "end",
      "search",
      "context",
      "get",
    ]),
    vela: new Set(),
    context7: new Set(),
  },
};
function clean(value: unknown) {
  return String(value).replace(/Bearer\s+[^\s"']+/gi, "Bearer [redacted]")
    .replace(
      /(api[_-]?key|token|authorization)\s*[:=]\s*[^\s,"}]+/gi,
      "$1=[redacted]",
    );
}
function text(value: string, cap = LIMIT) {
  let out = "";
  for (const c of value) {
    if (Buffer.byteLength(out + c) > cap) break;
    out += c;
  }
  return out;
}
function err(value: unknown) {
  return new Error(text(clean(value), 2048));
}
function rpc(id: number, method: string, params?: unknown) {
  return {
    jsonrpc: "2.0",
    id,
    method,
    ...(params === undefined ? {} : { params }),
  };
}
function config(deps: BridgeDependencies): Config | undefined {
  const home = deps.home?.() ?? process.env.HOME ?? "";
  try {
    const file = path.join(home, ".pi/agent/rotta-next/mcp.json");
    const raw = deps.readFile
      ? deps.readFile(file)
      : fs.readFileSync(file, "utf8");
    const value = JSON.parse(raw);
    if (
      value?.version !== 1 || !value.services ||
      Object.keys(value.services).length !== 3 || !SERVICES.every((s) =>
        Object.keys(value.services).includes(s) &&
        Object.keys(value.services[s] ?? {}).length === 1 &&
        typeof value.services[s]?.enabled === "boolean"
      )
    ) {
      return undefined;
    }
    return value;
  } catch {
    return undefined;
  }
}
function localBinary(
  command: string,
  env: (key: string) => string | undefined,
  exists: (f: string) => boolean,
) {
  return (env("PATH") ?? "").split(path.delimiter).some((dir) =>
    !!dir && exists(path.join(dir, command))
  );
}
function graph(deps: BridgeDependencies) {
  const candidate = path.join(
    deps.cwd?.() ?? process.cwd(),
    ".vela",
    "graph.json",
  );
  return (deps.exists ?? fs.existsSync)(candidate) ? candidate : undefined;
}
function original(service: Service, name: string) {
  return name.startsWith(`${service}_`) ? name.slice(service.length + 1) : name;
}
function exposed(service: Service, name: string) {
  return `rotta_${service}_${name.replace(/[^A-Za-z0-9_]/g, "_")}`;
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
function truncateAnsiLine(line: string, width: number, suffix = "") {
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
function compactComponent(line: string) {
  return {
    render: (width: number) => [truncateAnsiLine(line, width, "…")],
    handleMouse(event: { button?: string; type?: string }) {
      if (event.button === "left" && event.type === "click") {
        void (globalThis as any)[Symbol.for("rotta.pi.openLatestDetail")]?.();
        return { handled: true };
      }
      return undefined;
    },
    invalidate() {},
  };
}
export function renderManagedCall(service: string, action: string, theme: any) {
  return compactComponent(`${theme.fg("toolTitle", service)} ${theme.fg("accent", action)} ${theme.fg("warning", "● running")}`);
}
export function renderManagedResult(
  service: string,
  action: string,
  result: { content?: unknown; details?: Record<string, unknown> },
  options: { isPartial?: boolean },
  theme: any,
  context: { isError?: boolean } = {},
) {
  const running = options.isPartial;
  const failed = context.isError === true;
  const state = running ? theme.fg("warning", "● running") : failed
    ? theme.fg("error", "✗ failed")
    : theme.fg("success", "✓ complete");
  const content = Array.isArray(result.content)
    ? result.content.filter((part: any) => part?.type === "text" && typeof part.text === "string").map((part: any) => part.text).join(" ")
    : "";
  const rawReason = typeof result.details?.reason === "string"
    ? result.details.reason
    : failed ? content : "";
  const reason = rawReason
    ? ` ${theme.fg("dim", truncateAnsiLine(clean(rawReason).replace(/\s+/g, " ").trim(), 120, "…"))}`
    : "";
  return compactComponent(`${theme.fg("toolTitle", service)} ${theme.fg("accent", action)} ${state}${reason} ${theme.fg("dim", "ctrl+shift+o details")}`);
}
function toolResult(value: any) {
  if (value?.isError === true) {
    throw err(JSON.stringify(value.content ?? value));
  }
  const content = Array.isArray(value?.content)
    ? value.content.map((part: any) => {
      if (part?.type === "text" && typeof part.text === "string") {
        return { type: "text", text: text(part.text) };
      }
      if (
        part?.type === "image" && typeof part.data === "string" &&
        typeof part.mimeType === "string"
      ) {
        return {
          type: "image",
          data: text(part.data),
          mimeType: text(part.mimeType, 256),
        };
      }
      // Pi does not accept arbitrary MCP blocks; retain their data in details and give a bounded summary.
      return {
        type: "text",
        text: text(
          `[MCP ${typeof part?.type === "string" ? part.type : "content"}] ${
            JSON.stringify(part)
          }`,
        ),
      };
    })
    : [{ type: "text", text: text(JSON.stringify(value)) }];
  return {
    content,
    details: {
      structuredContent: value?.structuredContent,
      resource: value?.resource,
      content: value?.content,
    },
  };
}
async function boundedBody(response: Response, limit = LIMIT * 2) {
  if (!response.body) return "";
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let bytes = 0, output = "";
  for (;;) {
    const next = await reader.read();
    if (next.done) return output + decoder.decode();
    bytes += next.value.byteLength;
    if (bytes > limit) {
      await reader.cancel();
      throw err("MCP HTTP body exceeded limit");
    }
    output += decoder.decode(next.value, { stream: true });
  }
}

type Pending = {
  resolve: (v: any) => void;
  reject: (e: Error) => void;
  timer: ReturnType<typeof setTimeout>;
  abort?: () => void;
};
export class StdioMCP {
  private next = 1;
  private proc: any;
  private pending = new Map<number, Pending>();
  private writes: Promise<void> = Promise.resolve();
  private stderr = "";
  constructor(
    private command: string,
    private args: string[],
    private spawn = nodeSpawn,
    private timeout = TIMEOUT,
    private env: Record<string, string> = {},
  ) {}
  async start() {
    if (this.proc) return;
    const proc = this.proc = this.spawn(this.command, this.args, {
      shell: false,
      stdio: ["pipe", "pipe", "pipe"],
      env: this.env,
    });
    let buffer = "";
    const decoder = new StringDecoder("utf8"),
      errDecoder = new StringDecoder("utf8");
    proc.stdout?.on("data", (chunk: Buffer) => {
      buffer += decoder.write(chunk);
      if (Buffer.byteLength(buffer) > LIMIT * 2) {
        return this.close(err("MCP stdout exceeded limit"));
      }
      for (;;) {
        const at = buffer.indexOf("\n");
        if (at < 0) break;
        const line = buffer.slice(0, at);
        buffer = buffer.slice(at + 1);
        try {
          const msg = JSON.parse(line);
          if (msg?.jsonrpc !== "2.0") {
            throw Error();
          }
          // JSON-RPC requests have precedence over responses: a server may
          // legitimately use the same numeric id as our outstanding call.
          if (typeof msg.method === "string") {
            if (msg.id === undefined) continue; // notification
            if (typeof msg.id !== "number" && typeof msg.id !== "string") {
              throw Error();
            }
            void this.write(
              JSON.stringify({
                jsonrpc: "2.0",
                id: msg.id,
                ...(msg.method === "ping"
                  ? { result: {} }
                  : { error: { code: -32601, message: "Method not found" } }),
              }) + "\n",
            );
            continue;
          }
          if (
            typeof msg.id !== "number" ||
            (("result" in msg) === ("error" in msg))
          ) throw Error();
          const p = this.pending.get(msg.id);
          if (p) {
            this.done(msg.id);
            "error" in msg
              ? p.reject(err(JSON.stringify(msg.error)))
              : p.resolve(msg.result);
          }
        } catch {
          this.close(err("Malformed MCP stdout"));
          return;
        }
      }
    });
    proc.stderr?.on("data", (chunk: Buffer) => {
      this.stderr = text(this.stderr + errDecoder.write(chunk));
    });
    proc.on("error", (e: Error) => this.close(err(e.message)));
    proc.on(
      "close",
      () => this.close(err(this.stderr || "MCP process closed")),
    );
  }
  private done(id: number) {
    const p = this.pending.get(id);
    if (!p) return;
    this.pending.delete(id);
    clearTimeout(p.timer);
    p.abort?.();
  }
  async request(
    method: string,
    params?: unknown,
    signal?: AbortSignal,
  ): Promise<any> {
    if (signal?.aborted) throw err("MCP request aborted");
    await this.start();
    if (!this.proc?.stdin) throw err("MCP process unavailable");
    if (this.pending.size >= PENDING) throw err("MCP pending request limit");
    const id = this.next++;
    return await new Promise((resolve, reject) => {
      const abort = () => {
        this.done(id);
        reject(err("MCP request aborted"));
      };
      const item: Pending = {
        resolve,
        reject,
        timer: setTimeout(() => {
          this.done(id);
          this.close(err("MCP request timed out"));
          reject(err("MCP request timed out"));
        }, this.timeout),
        abort: () => signal?.removeEventListener("abort", abort),
      };
      this.pending.set(id, item);
      signal?.addEventListener("abort", abort, { once: true });
      this.write(JSON.stringify(rpc(id, method, params)) + "\n").catch((e) => {
        this.done(id);
        reject(err(e));
      });
    });
  }
  private write(line: string) {
    this.writes = this.writes.then(() =>
      new Promise<void>((resolve, reject) => {
        if (
          !this.proc?.stdin?.write(
            line,
            (e?: Error) => e ? reject(e) : resolve(),
          )
        ) resolve();
      })
    );
    return this.writes;
  }
  async notify(method: string, params?: unknown) {
    await this.write(
      JSON.stringify({
        jsonrpc: "2.0",
        method,
        ...(params === undefined ? {} : { params }),
      }) + "\n",
    );
  }
  close(reason = err("MCP transport closed")) {
    const proc = this.proc;
    this.proc = undefined;
    for (const [id, p] of this.pending) {
      this.done(id);
      p.reject(reason);
    }
    if (proc) {
      try {
        proc.stdin?.end?.();
        proc.kill("SIGTERM");
        const escalation = setTimeout(
          () => proc.exitCode == null && proc.kill("SIGKILL"),
          1000,
        );
        proc.once?.("close", () => clearTimeout(escalation));
      } catch { /* best effort */ }
    }
  }
}

function sse(raw: string, id: number) {
  let event = "", data: string[] = [];
  const messages: any[] = [];
  const emit = () => {
    if (!data.length) return;
    const payload = data.join("\n");
    if (Buffer.byteLength(payload) > LIMIT) {
      throw err("SSE frame exceeded limit");
    }
    let value: any;
    try {
      value = JSON.parse(payload);
    } catch {
      throw err("Malformed MCP SSE");
    }
    if (value?.id === id) messages.push(value);
    event = "";
    data = [];
  };
  for (const line of raw.split(/\r?\n/)) {
    if (!line) {
      emit();
      continue;
    }
    if (line.startsWith(":")) continue;
    if (line.startsWith("id:") || line.startsWith("retry:")) continue;
    if (line.startsWith("event:")) event = line.slice(6).trim();
    else if (line.startsWith("data:")) data.push(line.slice(5).trimStart());
    else throw err("Malformed MCP SSE");
  }
  emit();
  const message = messages.find((m) => m.id === id);
  if (!message) {
    throw err(`MCP SSE ${event || "stream"} closed without response`);
  }
  return message;
}
export class HTTPMCP {
  private next = 1;
  private session = "";
  constructor(
    private fetcher: typeof fetch,
    private timeout = TIMEOUT,
    private token = () => process.env.CONTEXT7_API_KEY,
  ) {}
  async request(method: string, params?: unknown, signal?: AbortSignal) {
    if (signal?.aborted) throw err("MCP request aborted");
    const controller = new AbortController();
    const abort = () => controller.abort();
    signal?.addEventListener("abort", abort, { once: true });
    const timer = setTimeout(abort, this.timeout);
    const id = this.next++;
    try {
      const token = this.token();
      const response = await this.fetcher("https://mcp.context7.com/mcp", {
        method: "POST",
        redirect: "error",
        signal: controller.signal,
        headers: {
          "content-type": "application/json",
          accept: "application/json, text/event-stream",
          "mcp-protocol-version": PROTOCOL,
          ...(this.session ? { "mcp-session-id": this.session } : {}),
          ...(token ? { authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify(rpc(id, method, params)),
      });
      const session = response.headers.get("mcp-session-id");
      if (session && /^[\w.-]{1,512}$/.test(session)) this.session = session;
      if (!response.ok) {
        throw err(
          `Context7 HTTP ${response.status}: ${
            text(await boundedBody(response, 2048), 2048)
          }`,
        );
      }
      const type = response.headers.get("content-type") ?? "";
      const body = await boundedBody(response);
      const message = type.includes("text/event-stream")
        ? sse(body, id)
        : (() => {
          try {
            return JSON.parse(body);
          } catch {
            throw err("Malformed MCP JSON response");
          }
        })();
      if (message?.jsonrpc !== "2.0" || message.id !== id) {
        throw err("MCP response id mismatch");
      }
      if (message.error) throw err(JSON.stringify(message.error));
      return message.result;
    } catch (e) {
      throw err(e instanceof Error ? e.message : e);
    } finally {
      clearTimeout(timer);
      signal?.removeEventListener("abort", abort);
    }
  }
  async notify(method: string, params?: unknown) {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), this.timeout);
    try {
      const response = await this.fetcher("https://mcp.context7.com/mcp", {
        method: "POST",
        redirect: "error",
        signal: controller.signal,
        headers: {
          "content-type": "application/json",
          accept: "application/json, text/event-stream",
          "mcp-protocol-version": PROTOCOL,
          ...(this.session ? { "mcp-session-id": this.session } : {}),
          ...(this.token() ? { authorization: `Bearer ${this.token()}` } : {}),
        },
        body: JSON.stringify({
          jsonrpc: "2.0",
          method,
          ...(params === undefined ? {} : { params }),
        }),
      });
      if (response.status !== 202) {
        throw err(`Context7 notification HTTP ${response.status}`);
      }
    } finally {
      clearTimeout(timer);
    }
  }
  async close() {
    if (this.session) {
      const controller = new AbortController();
      const timer = setTimeout(() => controller.abort(), this.timeout);
      try {
        await this.fetcher("https://mcp.context7.com/mcp", {
          method: "DELETE",
          redirect: "error",
          signal: controller.signal,
          headers: {
            "mcp-session-id": this.session,
            "mcp-protocol-version": PROTOCOL,
            ...(this.token()
              ? { authorization: `Bearer ${this.token()}` }
              : {}),
          },
        });
      } catch {
        /* optional session close */
      } finally {
        clearTimeout(timer);
      }
      this.session = "";
    }
  }
}

async function initialise(client: any, signal?: AbortSignal) {
  const init = await client.request("initialize", {
    protocolVersion: PROTOCOL,
    capabilities: {},
    clientInfo: { name: "rotta-pi", version: "1" },
  }, signal);
  if (
    !init?.protocolVersion || init.protocolVersion !== PROTOCOL ||
    typeof init.capabilities !== "object"
  ) {
    throw err("MCP initialize response missing protocol version");
  }
  await client.notify("notifications/initialized");
}
async function list(client: any, signal?: AbortSignal) {
  const items: Tool[] = [], seen = new Set<string>();
  let cursor: string | undefined;
  for (let page = 0; page < PAGES; page++) {
    const value = await client.request(
      "tools/list",
      cursor ? { cursor } : {},
      signal,
    );
    if (!Array.isArray(value?.tools)) {
      throw err("MCP tools/list schema invalid");
    }
    for (const tool of value.tools) {
      if (
        typeof tool?.name === "string" && tool.name.length <= 256 &&
        (!tool.inputSchema || typeof tool.inputSchema === "object")
      ) items.push(tool);
    }
    const next = value.nextCursor;
    if (!next) return items;
    if (typeof next !== "string" || seen.has(next)) {
      throw err("MCP tools/list cursor cycle");
    }
    seen.add(next);
    cursor = next;
  }
  throw err("MCP tools/list page limit");
}

export function registerMCPBridge(
  pi: ExtensionAPI,
  deps: BridgeDependencies = {},
) {
  // Only the parent extension omits the child environment; the child launcher sets it.
  const env = deps.env ?? ((key) => process.env[key]);
  const requested = deps.role ?? (env("ROTTA_CHILD_ROLE") || "parent");
  const knownRole = requested in allow;
  const role: Role = (knownRole ? requested : "operations") as Role;
  const permitted = knownRole ? allow[role] : {
    ancora: new Set<string>(),
    vela: new Set<string>(),
    context7: new Set<string>(),
  };
  const statuses: Record<Service, { state: string; reason?: string }> = Object
    .fromEntries(
      SERVICES.map((s) => [s, { state: "configured-pending-health" }]),
    ) as any;
  const clients = new Map<Service, any>();
  const registered = new Set<string>();
  let active: Promise<void> | undefined, velaCalls = 0;
  const current = () => config(deps);
  const enabled = (s: Service) =>
    !!current()?.services[s].enabled && permitted[s].size > 0;
  const clientFor = async (service: Service, signal?: AbortSignal) => {
    let client = clients.get(service);
    if (client) return client;
    const minimal = {
      PATH: env("PATH") ?? "",
      HOME: env("HOME") ?? deps.home?.() ?? "",
      XDG_CONFIG_HOME: env("XDG_CONFIG_HOME") ?? "",
    };
    if (
      service !== "context7" &&
      !localBinary(
        service === "ancora" ? "ancora" : "vela",
        env,
        deps.exists ?? fs.existsSync,
      )
    ) throw err(`${service} unavailable: binary missing`);
    if (service === "ancora") {
      client = new StdioMCP(
        "ancora",
        ["mcp", "--tools=agent"],
        deps.spawn ?? nodeSpawn,
        deps.timeoutMs ?? TIMEOUT,
        minimal,
      );
    } else if (service === "vela") {
      const file = graph(deps);
      if (!file) throw err("Vela unavailable: canonical graph is missing");
      client = new StdioMCP(
        "vela",
        ["serve", "--graph", file],
        deps.spawn ?? nodeSpawn,
        deps.timeoutMs ?? TIMEOUT,
        minimal,
      );
    } else {client = new HTTPMCP(
        deps.fetch ?? fetch,
        deps.timeoutMs ?? TIMEOUT,
        () => env("CONTEXT7_API_KEY"),
      );}
    clients.set(service, client);
    try {
      await client.start?.();
      await initialise(client, signal);
      return client;
    } catch (error) {
      // A failed initialize must finish bounded cleanup before an explicit
      // later action can create another session.
      try {
        await client.close?.();
      } catch { /* retain original failure */ }
      clients.delete(service);
      throw error;
    }
  };
  const activate = async (signal?: AbortSignal) => {
    if (active) return active;
    active = (async () => {
      await Promise.all(SERVICES.map(async (service) => {
        if (!current()) {
          statuses[service] = {
            state: "unavailable",
            reason: "invalid or missing managed config",
          };
          return;
        }
        if (!current()!.services[service].enabled) {
          statuses[service] = { state: "disabled" };
          return;
        }
        if (!permitted[service].size) {
          statuses[service] = {
            state: "disabled",
            reason: "role not permitted",
          };
          return;
        }
        try {
          const client = await clientFor(service, signal);
          for (const remote of await list(client, signal)) {
            const name = original(service, remote.name),
              mapped = exposed(service, name);
            if (!permitted[service].has(name) || registered.has(mapped)) {
              continue;
            }
            registered.add(mapped);
            pi.registerTool({
              name: mapped,
              label: `${service}: ${name}`,
              description: remote.description ?? `${service} MCP tool`,
              parameters: Type.Unsafe(
                remote.inputSchema ??
                  { type: "object", additionalProperties: true },
              ),
              renderCall: (_args: unknown, theme: any) =>
                renderManagedCall(service, name, theme),
              renderResult: (result: any, options: any, theme: any, context: any) =>
                renderManagedResult(service, name, result, options, theme, context),
              async execute(
                _id: string,
                params: unknown,
                callSignal: AbortSignal,
              ) {
                if (!enabled(service)) {
                  throw err(`${service} disabled or no longer permitted`);
                }
                if (service === "vela" && ++velaCalls > 2) {
                  throw err("Vela call denied: capsule limit is two");
                }
                try {
                  return toolResult(
                    await (await clientFor(service, callSignal)).request(
                      "tools/call",
                      { name: remote.name, arguments: params },
                      callSignal,
                    ),
                  );
                } catch (e) {
                  await clients.get(service)?.close?.();
                  clients.delete(service);
                  statuses[service] = {
                    state: "unavailable",
                    reason: clean(e instanceof Error ? e.message : e),
                  };
                  throw err(e);
                }
              },
            });
          }
          statuses[service] = {
            state: "healthy",
            reason: "initialize and tools/list observed",
          };
        } catch (e) {
          statuses[service] = {
            state: "unavailable",
            reason: clean(e instanceof Error ? e.message : e),
          };
          const failed = clients.get(service);
          // List/abort failures have the same close-before-evict ownership
          // as initialize and tool calls; never leave a background session.
          try {
            await failed?.close?.();
          } catch { /* original error wins */ }
          clients.delete(service);
        }
      }));
    })();
    try {
      await active;
    } finally {
      active = undefined;
    }
  };
  const close = async () => {
    await Promise.all([...clients.values()].map((client) => client.close?.()));
    clients.clear();
    velaCalls = 0;
    active = undefined;
  };
  pi.on("before_agent_start", async (event: any) => {
    await activate();
    return event;
  });
  pi.on("session_shutdown", close as any);
  return { activate, close, status: () => structuredClone(statuses) };
}
export default registerMCPBridge;
