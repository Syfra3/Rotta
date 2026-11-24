import assert from 'node:assert/strict';
import definition, { createTelemetry } from './opencode-telemetry.mjs';
const bodies = [];
const handler = createTelemetry(async body => { bodies.push(JSON.stringify(body)); });
const handle = (event) => handler({ event });
const info = (id, extra = {}) => ({ role: 'assistant', id, sessionID: 'PRIVATE-SESSION', time: { created: 1, completed: 2 }, tokens: { input: 3, output: 0, cache: { read: 0, write: 0 } }, cost: 0, content: 'PRIVATE-CONTENT', ...extra });
const event = (i) => ({ type: 'message.updated', properties: { info: i, private: 'PRIVATE-PROMPT' } });
await handle(event(info('partial', { time: { created: 1 } })));
await handle(event(info('missing', { tokens: undefined })));
await handle(event(info('negative', { tokens: { input: -1, output: 1, cache: { read: 0, write: 0 } } })));
await handle(event(info('badtime', { time: { created: 3, completed: 2 } })));
await handle(event(info('user', { role: 'user' })));
assert.equal(bodies.length, 0);
await handle(event(info('one')));
await handle(event(info('one')));
await handle(event(info('one', { time: { created: 1 } })));
assert.equal(bodies.length, 1);
const metrics = JSON.parse(bodies[0]).resourceMetrics[0].scopeMetrics[0].metrics;
assert.equal(metrics.length, 2);
for (const metric of metrics) for (const point of metric.sum.dataPoints) {
  const start = BigInt(point.startTimeUnixNano);
  const end = BigInt(point.timeUnixNano);
  assert.ok(start < end, 'nonempty cumulative interval');
  assert.equal(metric.sum.aggregationTemporality, 2);
}
assert.deepEqual(metrics[0].sum.dataPoints.map(p => p.asDouble), [3, 0, 0, 0]);
for (const canary of ['PRIVATE-SESSION', 'PRIVATE-CONTENT', 'PRIVATE-PROMPT', 'partial', 'one']) assert.ok(!bodies[0].includes(canary));
for (let i = 0; i < 1100; i++) await handle(event(info(String(i))));
assert.equal(bodies.length, 1101);
// The oldest key was evicted: replay is explicitly allowed to recount.
await handle(event(info('one')));
assert.equal(bodies.length, 1102);
// A new plugin instance has independent dedup state; replay after restart is explicitly best-effort.
const second = [];
await createTelemetry(async body => second.push(JSON.stringify(body)))({ event: event(info('one')) });
assert.equal(second.length, 1);
for (const malformed of [NaN, Infinity, -1, 1.5, '3']) {
  await handle(event(info(`bad-${String(malformed)}`, { tokens: { input: malformed, output: 0, cache: { read: 0, write: 0 } } })));
  if (malformed !== 1.5) await handle(event(info(`cost-${String(malformed)}`, { cost: malformed })));
}
await handle(event(info('zero', { tokens: { input: 0, output: 0, cache: { read: 0, write: 0 } }, cost: undefined })));
assert.equal(bodies.length, 1103);
assert.equal(JSON.parse(bodies.at(-1)).resourceMetrics[0].scopeMetrics[0].metrics.length, 2); // cumulative known cost remains present
const series = [];
const cumulative = createTelemetry(async body => { series.push(body.resourceMetrics[0].scopeMetrics[0].metrics); });
const usage = (id, input, output, read, write, price) => event(info(id, {
  tokens: { input, output, cache: { read, write } }, cost: price,
}));
await Promise.all([
  cumulative({ event: usage('a', 17, 5, 3, 2, .125) }),
  cumulative({ event: usage('b', 7, 4, 1, 0, .25) }),
]);
assert.equal(series.length, 2);
assert.deepEqual(series[0][0].sum.dataPoints.map(p => p.asDouble), [17, 5, 3, 2]);
assert.deepEqual(series[1][0].sum.dataPoints.map(p => p.asDouble), [24, 9, 4, 2]);
assert.equal(series[1][1].sum.dataPoints[0].asDouble, .375);
assert.ok(BigInt(series[1][0].sum.dataPoints[0].timeUnixNano) > BigInt(series[0][0].sum.dataPoints[0].timeUnixNano));
assert.equal(series[0][0].sum.dataPoints[0].startTimeUnixNano, series[1][0].sum.dataPoints[0].startTimeUnixNano);
await cumulative({ event: usage('b', 7, 4, 1, 0, .25) });
assert.equal(series.length, 2);
// An unresolved sender cannot retain one promise/body per completed event.
let release;
const gate = new Promise(resolve => { release = resolve; });
const stalledBodies = [];
let calls = 0;
const stalled = createTelemetry(async body => {
  stalledBodies.push(JSON.stringify(body));
  if (++calls === 1) await gate;
});
const first = stalled({ event: usage('stall-0', 17, 5, 3, 2, .125) });
for (let i = 1; i <= 5000; i++) await stalled({ event: usage(`stall-${i}`, 1, 0, 0, 0, 0) });
assert.equal(calls, 1);
assert.equal(stalledBodies.length, 1);
// A repeated recent event remains deduplicated despite coalescing.
await stalled({ event: usage('stall-5000', 1, 0, 0, 0, 0) });
release();
await first;
assert.equal(stalledBodies.length, 2);
const latest = stalledBodies.map(text => JSON.parse(text).resourceMetrics[0].scopeMetrics[0].metrics);
assert.deepEqual(latest[1][0].sum.dataPoints.map(p => p.asDouble), [5017, 5, 3, 2]);
assert.equal(latest[1][1].sum.dataPoints[0].asDouble, .125);
assert.ok(BigInt(latest[1][0].sum.dataPoints[0].timeUnixNano) > BigInt(latest[0][0].sum.dataPoints[0].timeUnixNano));
for (const text of stalledBodies) for (const canary of ['PRIVATE-SESSION', 'PRIVATE-CONTENT', 'PRIVATE-PROMPT']) assert.ok(!text.includes(canary));
const rejecting = createTelemetry(() => Promise.reject(new Error('offline')), undefined, () => {});
await rejecting({ event: usage('reject', 1, 0, 0, 0, 0) });
let retryCallback;
let attempts = 0;
const retried = [];
const retrying = createTelemetry(async body => {
  if (++attempts === 1) throw new Error('offline');
  retried.push(body);
}, callback => { retryCallback = callback; }, () => {});
await retrying({ event: usage('retry-a', 1, 0, 0, 0, 0) });
await retrying({ event: usage('retry-b', 2, 0, 0, 0, 0) });
assert.equal(attempts, 1);
await retryCallback();
assert.equal(retried[0].resourceMetrics[0].scopeMetrics[0].metrics[0].sum.dataPoints[0].asDouble, 3);
let disposedRetry;
let disposedAttempts = 0;
const disposable = createTelemetry(async () => { disposedAttempts++; throw new Error('offline'); }, callback => { disposedRetry = callback; }, () => {});
await disposable({ event: usage('dispose', 1, 0, 0, 0, 0) });
disposable.dispose();
await disposedRetry();
await disposable({ event: usage('after-dispose', 1, 0, 0, 0, 0) });
assert.equal(disposedAttempts, 1);
// Exercise the actual v2 definition, stream options, bridge and cleanup.
assert.equal(definition.id, 'rotta.telemetry');
assert.equal(typeof definition.setup, 'function');
let options;
let posted;
const previousFetch = globalThis.fetch;
globalThis.fetch = async (_url, request) => { posted = JSON.parse(request.body); return { ok: true }; };
let finished;
const done = new Promise(resolve => { finished = resolve; });
const dispose = definition.setup({ location: { directory: '/project' }, event: { subscribe: (arg) => {
  options = arg;
  return (async function* () {
    yield { type: 'session.step.ended', created: 100, location: { directory: '/project' }, data: {
      sessionID: 'SECRET', assistantMessageID: 'SECRET-MESSAGE',
      tokens: { input: 17, output: 5, reasoning: 0, cache: { read: 3, write: 2 } }, cost: .375,
    } };
    finished();
  })();
} } });
await done;
assert.ok(options.types.includes('session.step.ended') && options.types.includes('session.step.failed'));
assert.deepEqual(posted.resourceMetrics[0].scopeMetrics[0].metrics[0].sum.dataPoints.map(p => p.asDouble), [17, 5, 3, 2]);
assert.ok(!JSON.stringify(posted).includes('SECRET'));
dispose();
assert.ok(options.signal.aborted);
globalThis.fetch = previousFetch;
// Distinct location instances must not export the same server-wide event.
const locationBodies = [];
globalThis.fetch = async (_url, request) => { locationBodies.push(JSON.parse(request.body)); return { ok: true }; };
const makeApi = directory => ({ location: { directory }, event: { subscribe: () => (async function* () {
  yield { type: 'session.step.ended', created: 100, location: { directory: '/project/a' }, data: {
    sessionID: 's', assistantMessageID: 'm', tokens: { input: 1, output: 2, cache: { read: 0, write: 0 } }, cost: 0,
  } };
  yield { type: 'session.step.ended', created: 100, data: {
    sessionID: 's', assistantMessageID: 'missing-location', tokens: { input: 1, output: 2, cache: { read: 0, write: 0 } }, cost: 0,
  } };
})() } });
const stopA = definition.setup(makeApi('/project/a'));
const stopB = definition.setup(makeApi('/project/b'));
await new Promise(resolve => setTimeout(resolve, 10));
assert.equal(locationBodies.length, 1, 'only the matching location exports; unscoped events fail closed');
stopA(); stopB();
globalThis.fetch = previousFetch;
// Failed steps can report usage, but duplicate message IDs must not be recounted.
const failedBodies = [];
globalThis.fetch = async (_url, request) => { failedBodies.push(JSON.parse(request.body)); return { ok: true }; };
let finishFailed;
const failedDone = new Promise(resolve => { finishFailed = resolve; });
const stopFailed = definition.setup({ location: { directory: '/project' }, event: { subscribe: () => (async function* () {
  const data = { sessionID: 's', assistantMessageID: 'm', tokens: { input: 5, output: 2, cache: { read: 0, write: 0 } }, cost: 0 };
  yield { type: 'session.step.failed', created: 100, location: { directory: '/project' }, data };
  yield { type: 'session.step.ended', created: 101, location: { directory: '/project' }, data };
  finishFailed();
})() } });
await failedDone;
assert.equal(failedBodies.length, 1);
assert.equal(failedBodies[0].resourceMetrics[0].scopeMetrics[0].metrics[0].sum.dataPoints[0].asDouble, 5);
stopFailed();
globalThis.fetch = previousFetch;
console.log('fake OpenCode events, v2 stream bridge and serialized HTTP privacy: pass');
