import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

// Opt-in, metrics only. Valid model IDs are exported; event content is not.
const endpoint = "http://127.0.0.1:14318/v1/metrics";
const categories = ["input", "output", "cache_read", "cache_write"] as const;
function modelLabel(value: unknown): string {
  if (typeof value !== "string") return "unknown";
  const trimmed = value.trim();
  return /^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$(?![\s\S])/.test(trimmed) ? trimmed : "unknown";
}
type Series = { value: number; start: string; attributes: Record<string, string> };
export default function (pi: ExtensionAPI) {
  const series = new Map<string, Series>();
  const started = new Map<string, number>();
  const completed = new Set<string>();
  const seenMessages = new WeakSet<object>();
  const childHook = Symbol.for("rotta.pi.child.telemetry");
  const roles = new Set(["exploration", "implementation", "reviewer", "operations"]);
  const boundedRole = (role: unknown) => typeof role === "string" && roles.has(role) ? role : "unknown";
  // Child usage is observed in the isolated Pi process; parent toolResult usage is not a parent message.
  (globalThis as any)[childHook] = (event: { phase: string; role: unknown; reason?: string; responseUsage?: any; responseModel?: unknown }) => {
    const role = boundedRole(event.role);
    if (event.phase === "started") {
      add("ai_cli_pi_child_events", 1, { role, status: "started", query_source: "subagent" });
      return;
    }
    if (event.phase !== "finished") return;
    const status = (["success", "timeout", "cancelled", "start_failed", "invalid_result", "child_error"] as string[])
      .includes(event.reason ?? "") ? event.reason! : "child_error";
    add("ai_cli_pi_child_events", 1, { role, status, query_source: "subagent" });
    if (status !== "success" && status !== "child_error") return;
    // Only the final assistant response is observed, never the child session total.
    const usage = event.responseUsage;
    if (!usage || typeof usage !== "object") return;
    const model = modelLabel(event.responseModel);
    for (const [field, type] of [["input", "input"], ["output", "output"], ["cacheRead", "cache_read"], ["cacheWrite", "cache_write"]] as const) {
      if (typeof usage[field] === "number") add("ai_cli_pi_child_response_tokens", usage[field], { role, type, model, query_source: "subagent" }, "{token}");
    }
    if (typeof usage.cost === "number") add("ai_cli_pi_child_response_estimated_cost_usd", usage.cost,
      { role, query_source: "subagent" }, "USD");
  };
  let runStarted: number | undefined;
  let active = false;
  // Process-lifetime cumulative counters keep session transitions from replacing
  // unsent points while a fetch is stalled. Distinct model IDs can grow series count.
  // At most one request and one coalesced latest value per series.
  const dirty = new Map<string, { name: string; unit: string; point: Series }>();
  let inflight = false;
  let retry: ReturnType<typeof setTimeout> | undefined;
  function schedule() {
    if (inflight || retry || !dirty.size) return;
    void drain();
  }
  async function drain() {
    if (inflight) return;
    inflight = true;
    while (dirty.size) {
      const [key, snapshot] = dirty.entries().next().value!;
      dirty.delete(key);
      const now = BigInt(Date.now()) * 1_000_000n;
      const streamStart = BigInt(snapshot.point.start);
      lastPointTime = [now, streamStart, lastPointTime + 1n].reduce((max, value) => value > max ? value : max);
      const body = { resourceMetrics: [{ scopeMetrics: [{ metrics: [{ name: snapshot.name, unit: snapshot.unit,
        sum: { aggregationTemporality: 2, isMonotonic: true, dataPoints: [{ asDouble: snapshot.point.value,
          startTimeUnixNano: snapshot.point.start, timeUnixNano: String(lastPointTime),
          attributes: Object.entries(snapshot.point.attributes).map(([key, value]) => ({ key, value: { stringValue: value } })) }] } }] }] }] };
      try {
        const response = await fetch(endpoint, { method: "POST", redirect: "error", headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body), signal: AbortSignal.timeout(1500) });
        if (!response.ok) throw new Error("collector rejected metrics");
      } catch {
        // Keep the newer cumulative point, never an obsolete snapshot.
        if (!dirty.has(key)) dirty.set(key, snapshot);
        retry = setTimeout(() => { retry = undefined; schedule(); }, 1500);
        break;
      }
    }
    inflight = false;
  }
  let lastStart = 0n;
  let lastPointTime = 0n;
  function nextStart() {
    // Nanoseconds exceed Number's safe integer range at current epoch timestamps.
    const now = BigInt(Date.now()) * 1_000_000n;
    lastStart = now > lastStart ? now : lastStart + 1n;
    return String(lastStart);
  }
  const start = nextStart();
  function add(name: string, amount: number, attributes: Record<string, string> = {}, unit = "1") {
    if (!Number.isFinite(amount) || amount < 0) return;
    const key = JSON.stringify([name, attributes]);
    const previous = series.get(key);
    const value = (previous?.value ?? 0) + amount;
    if (!Number.isFinite(value)) return;
    const current = { value, start, attributes };
    series.set(key, current);
    dirty.set(key, { name, unit, point: current });
    schedule();
  }
  pi.on("agent_start", () => { if (!active) { active = true; runStarted = Date.now(); } });
  pi.on("agent_end", () => {
    if (!active) return;
    active = false;
    add("ai_cli_pi_agent_runs", 1);
    if (runStarted !== undefined) add("ai_cli_pi_run_duration_ms", Date.now() - runStarted, {}, "ms");
    runStarted = undefined;
  });
  pi.on("message_end", (event) => {
    const message = event.message;
    if (message.role !== "assistant" || message.stopReason === "pending" || seenMessages.has(message)) return;
    seenMessages.add(message);
    const usage = message.usage;
    const model = modelLabel(message.model);
    for (const [index, key] of categories.entries()) {
      const amount = [usage.input, usage.output, usage.cacheRead, usage.cacheWrite][index];
      add("ai_cli_pi_tokens", amount, { type: key }, "{token}");
      add("ai_cli_pi_model_tokens", amount, { type: key, model }, "{token}");
    }
    add("ai_cli_pi_estimated_cost_usd", usage.cost?.total);
    add("ai_cli_pi_model_estimated_cost_usd", usage.cost?.total, { model }, "USD");
  });
  pi.on("tool_execution_start", (event) => {
    if (started.has(event.toolCallId) || completed.has(event.toolCallId)) return;
    started.set(event.toolCallId, Date.now());
  });
  pi.on("tool_execution_end", (event) => {
    const at = started.get(event.toolCallId);
    if (at === undefined) return;
    started.delete(event.toolCallId);
    completed.add(event.toolCallId);
    const category = event.toolName.startsWith("mcp__") ? "mcp" : "tool";
    const status = event.isError ? "error" : "ok";
    add("ai_cli_pi_tool_call", 1, { category, status });
    add("ai_cli_pi_tool_duration_ms", Date.now() - at, { category, status }, "ms");
  });
  pi.on("session_shutdown", () => {
    if (active) { active = false; add("ai_cli_pi_agent_runs", 1); }
    started.clear(); completed.clear(); runStarted = undefined;
    // Preserve cumulative counters and their start across sessions: a dirty point
    // from an earlier session may still be awaiting the single in-flight sender.
  });
}
