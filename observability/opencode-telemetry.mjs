// Optional OpenCode plugin; copied to the global autoload directory only on opt-in.
// Process-local cumulative counters reset on restart; evicted IDs or replay after restart
// can be recounted. Delivery is best effort and retries only while running.
// At most one in-flight body and one latest cumulative pending body are retained.
// Intermediate snapshots are coalesced under backpressure; a stalled sender may
// delay delivery indefinitely, but accepted totals and dedup continue advancing.
const endpoint = 'http://127.0.0.1:14318/v1/metrics';
const MAX = 1024;
const valid = (n) => typeof n === 'number' && Number.isSafeInteger(n) && n >= 0;
const fields = ['input', 'output', 'cache_read', 'cache_write'];

// Only validated dimensions leave this process. Correlation IDs and event
// content stay in bounded in-memory maps and are never metric attributes.
export function createMetadataTelemetry(publish) {
  const series = new Map();
  const started = new Map();
  const tools = new Map();
  const seen = new Set();
  const completedSteps = new Set();
  const start = (BigInt(Date.now()) * 1000000n).toString();
  let last = BigInt(start);
  const bounded = (map, key, value) => {
    map.set(key, value);
    if (map.size > MAX) map.delete(map.keys().next().value);
  };
  const remember = (set, key) => {
    set.add(key);
    if (set.size > MAX) set.delete(set.values().next().value);
  };
  const label = value => typeof value === 'string' && /^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$(?![\s\S])/.test(value) ? value : 'unknown';
  const add = (name, amount, attributes, unit = '1') => {
    if (typeof amount !== 'number' || !Number.isFinite(amount) || amount < 0) return;
    const key = JSON.stringify([name, attributes]);
    const value = (series.get(key)?.value ?? 0) + amount;
    if (!Number.isFinite(value) || (!series.has(key) && series.size >= MAX)) return;
    series.set(key, { name, value, attributes, unit });
  };
  const snapshot = () => {
    const now = BigInt(Date.now()) * 1000000n;
    last = (now > last ? now : last) + 1n;
    return [...series.values()].map(({ name, value, attributes, unit }) => ({ name, unit, sum: {
      aggregationTemporality: 2, isMonotonic: true, dataPoints: [{
        attributes: Object.entries(attributes).map(([key, val]) => ({ key, value: { stringValue: val } })),
        asDouble: value, startTimeUnixNano: start, timeUnixNano: last.toString(),
      }],
    } }));
  };
  return event => {
    const d = event?.data;
    if (!d || typeof event.id !== 'string' || !event.id || event.id.length > 256 || seen.has(event.id) ||
        typeof d.sessionID !== 'string' || !d.sessionID || d.sessionID.length > 256) return;
    remember(seen, event.id);
    const key = typeof d.assistantMessageID === 'string' && d.assistantMessageID.length <= 256
      ? d.sessionID + '\0' + d.assistantMessageID : null;
    let changed = false;
    if (event.type === 'session.step.started' && key) {
      const provider = label(d.model?.providerID), model = label(d.model?.id);
      const joined = provider !== 'unknown' && model !== 'unknown' ? label(provider + '/' + model) : 'unknown';
      const role = ['build', 'plan', 'exploration', 'implementation', 'reviewer', 'operations', 'orchestrator'].includes(d.agent) ? d.agent : 'unknown';
      bounded(started, key, { model: joined, role });
    } else if ((event.type === 'session.step.ended' || event.type === 'session.step.failed') && key && !completedSteps.has(key)) {
      const usage = d.tokens, info = started.get(key);
      if (info && usage && valid(usage.input) && valid(usage.output) && valid(usage.cache?.read) && valid(usage.cache?.write)) {
        started.delete(key);
        remember(completedSteps, key);
        [usage.input, usage.output, usage.cache.read, usage.cache.write].forEach((n, i) => add('ai_cli_opencode_model_tokens', n, { ...info, type: fields[i] }, '{token}'));
        if (typeof d.cost === 'number' && Number.isFinite(d.cost) && d.cost >= 0) add('ai_cli_opencode_model_cost_usd', d.cost, info, 'USD');
        changed = true;
      }
    } else if (event.type === 'session.tool.input.started' && key && typeof d.id === 'string' && d.id && d.id.length <= 256) {
      // MCP direct names are <server>_<tool>, and Code Mode multiplexes calls.
      // Only well-known builtins are safely classified; all others are unknown.
      const builtins = ['read', 'write', 'edit', 'patch', 'glob', 'grep', 'shell', 'question', 'skill', 'webfetch', 'websearch'];
      bounded(tools, key + '\0' + d.id, builtins.includes(d.name) ? 'tool' : 'unknown');
    } else if ((event.type === 'session.tool.success' || event.type === 'session.tool.failed') && key && typeof d.id === 'string' && d.id.length <= 256) {
      const toolKey = key + '\0' + d.id;
      const category = tools.get(toolKey);
      if (category) { tools.delete(toolKey); add('ai_cli_opencode_tool_calls', 1, { category, status: event.type.endsWith('success') ? 'ok' : 'error' }); changed = true; }
    } else if (event.type === 'session.created') {
      add('ai_cli_opencode_sessions', 1, { category: typeof d.parentID === 'string' && d.parentID ? 'subagent' : 'main' }); changed = true;
    } else if (['session.execution.succeeded', 'session.execution.failed', 'session.execution.interrupted'].includes(event.type)) {
      const status = event.type.endsWith('succeeded') ? 'ok' : event.type.endsWith('failed') ? 'error' : 'interrupted';
      add('ai_cli_opencode_runs', 1, { status }); changed = true;
    } else if (event.type === 'session.compaction.ended') {
      add('ai_cli_opencode_compactions', 1, { category: ['auto', 'manual'].includes(d.reason) ? d.reason : 'unknown' }); changed = true;
    }
    if (changed) publish(snapshot());
  };
}

function createMetadataSender() {
  let pending = null, sending = false, retry, disposed = false, warned = false;
  const instance = crypto.randomUUID();
  const drain = async () => {
    if (sending || disposed) return;
    sending = true;
    while (pending && !disposed) {
      const body = pending;
      pending = null;
      try {
        const response = await fetch(endpoint, { method: 'POST', redirect: 'error',
          headers: { 'content-type': 'application/json' }, body: JSON.stringify(body), signal: AbortSignal.timeout(2000) });
        if (!response.ok) throw new Error('collector rejected metrics');
        if (warned) { console.warn('Rotta OpenCode telemetry metadata delivery recovered'); warned = false; }
      } catch {
        if (!disposed) {
          if (!warned) { console.warn('Rotta OpenCode telemetry metadata delivery failed; retrying'); warned = true; }
          pending ??= body;
          retry = setTimeout(() => { retry = undefined; void drain(); }, 1500);
          retry.unref?.();
        }
        break;
      }
    }
    sending = false;
  };
  return {
    publish(metrics) {
      if (disposed) return;
      pending = { resourceMetrics: [{ resource: { attributes: [{ key: 'service.instance.id', value: { stringValue: instance } }] }, scopeMetrics: [{ metrics }] }] };
      if (!retry) void drain();
    },
    dispose() { disposed = true; pending = null; clearTimeout(retry); },
  };
}

export function createTelemetry(send = async (body) => {
  const response = await fetch(endpoint, {
    method: 'POST', redirect: 'error', headers: { 'content-type': 'application/json' }, body: JSON.stringify(body),
    signal: AbortSignal.timeout(2000),
  });
  if (!response.ok) throw new Error('collector rejected metrics');
}, scheduleRetry = callback => { const timer = setTimeout(callback, 1500); timer.unref?.(); return timer; },
report = message => console.warn(message)) {
  const seen = new Set();
  const totals = Object.fromEntries(fields.map(field => [field, 0]));
  let cost = 0;
  let hasCost = false;
  const start = BigInt(Date.now()) * 1000000n;
  let last = start;
  let sending = false;
  let retrying = false;
  let retryTimer;
  let disposed = false;
  let warned = false;
  let pending = null;
  const drain = async () => {
    while (!disposed && pending !== null) {
      const body = pending;
      pending = null;
      try {
        await send(body);
        if (warned) { report('Rotta OpenCode telemetry usage delivery recovered'); warned = false; }
      } catch {
        if (disposed) break;
        if (!warned) { report('Rotta OpenCode telemetry usage delivery failed; retrying'); warned = true; }
        if (pending === null) pending = body;
        sending = false;
        retrying = true;
        retryTimer = scheduleRetry(() => {
          if (disposed) return;
          retrying = false; sending = true; return drain();
        });
        return;
      }
    }
    sending = false;
  };
  const metric = (name, unit, points, now) => ({ name, unit, sum: {
    aggregationTemporality: 2, isMonotonic: true,
    dataPoints: points.map(([type, value]) => ({
      attributes: type ? [{ key: 'type', value: { stringValue: type } }] : [],
      asDouble: value, startTimeUnixNano: start.toString(), timeUnixNano: now.toString(),
    })),
  } });
  const handle = ({ event }) => {
    if (disposed) return Promise.resolve();
    if (event?.type !== 'message.updated') return Promise.resolve();
    const info = event.properties?.info;
    if (info?.role !== 'assistant' || typeof info.id !== 'string' || !info.id || info.id.length > 256 ||
        typeof info.sessionID !== 'string' || !info.sessionID || info.sessionID.length > 256 ||
        !Number.isFinite(info.time?.completed) || !Number.isFinite(info.time?.created) ||
        info.time.completed < info.time.created) return Promise.resolve();
    const key = info.sessionID + '\0' + info.id;
    if (seen.has(key)) return Promise.resolve();
    const tokens = info.tokens;
    if (!tokens || !valid(tokens.input) || !valid(tokens.output) ||
        !tokens.cache || !valid(tokens.cache.read) || !valid(tokens.cache.write)) return Promise.resolve();
    if (info.cost !== undefined && (typeof info.cost !== 'number' || !Number.isFinite(info.cost) || info.cost < 0)) return Promise.resolve();
    const values = [tokens.input, tokens.output, tokens.cache.read, tokens.cache.write];
    if (fields.some((field, i) => !Number.isSafeInteger(totals[field] + values[i])) ||
        (info.cost !== undefined && !Number.isFinite(cost + info.cost))) return Promise.resolve();
    seen.add(key);
    if (seen.size > MAX) seen.delete(seen.values().next().value);
    fields.forEach((field, i) => { totals[field] += values[i]; });
    if (info.cost !== undefined) { cost += info.cost; hasCost = true; }
    // Snapshot at acceptance time, not when an earlier asynchronous POST finishes.
    const snapshot = { ...totals };
    const snapshotCost = cost;
    const snapshotHasCost = hasCost;
    const now = BigInt(Date.now()) * 1000000n;
    last = (now > last ? now : last) + 1n;
    const timestamp = last;
    const metrics = [metric('ai_cli_opencode_tokens', '{token}',
      fields.map(field => [field, snapshot[field]]), timestamp)];
    if (snapshotHasCost) metrics.push(metric('ai_cli_opencode_estimated_cost_usd', 'USD', [[null, snapshotCost]], timestamp));
    const body = { resourceMetrics: [{ resource: { attributes: [] }, scopeMetrics: [{ metrics }] }] };
    pending = body;
    if (!sending && !retrying) {
      sending = true;
      // drain catches sender failures, including synchronous throws.
      return drain();
    }
    return Promise.resolve();
  };
  handle.dispose = () => { disposed = true; pending = null; clearTimeout(retryTimer); };
  return handle;
}

// v2.2.7 exposes an async event stream. A step-ended event represents a
// finalized assistant message; its data has the same usage fields as the
// portable handler's legacy fixture shape. The stream is scoped by OpenCode.
export default {
  id: 'rotta.telemetry',
  setup(api) {
    const abort = new AbortController();
    const handle = createTelemetry();
    const sender = createMetadataSender();
    const metadata = createMetadataTelemetry(metrics => sender.publish(metrics));
    const types = ['session.step.started', 'session.step.ended', 'session.step.failed',
      'session.tool.input.started', 'session.tool.success', 'session.tool.failed', 'session.created',
      'session.execution.succeeded', 'session.execution.failed', 'session.execution.interrupted', 'session.compaction.ended'];
    const stream = api.event.subscribe({ types, signal: abort.signal });
    void (async () => {
      try {
        for await (const event of stream) {
          // The server stream spans locations, while plugin state is per location.
          // Ignore unscoped events rather than counting them once per instance.
          if (!api.location?.directory || event.location?.directory !== api.location.directory ||
              event.location?.workspaceID !== api.location.workspaceID) continue;
          metadata(event);
          if (event.type !== 'session.step.ended' && event.type !== 'session.step.failed') continue;
          const data = event.data;
          void handle({ event: { type: 'message.updated', properties: { info: {
            role: 'assistant', id: data?.assistantMessageID, sessionID: data?.sessionID,
            time: { created: event.created, completed: event.created },
            tokens: data?.tokens, cost: data?.cost,
          } } } });
        }
      } catch {
        if (!abort.signal.aborted) console.warn('Rotta OpenCode telemetry event stream stopped');
      }
    })();
    return () => { abort.abort(); handle.dispose(); sender.dispose(); };
  },
};
