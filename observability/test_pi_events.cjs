const fs = require('node:fs');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const { stripTypeScriptTypes } = require('node:module');
const handlers = {}, bodies = [];
let release, fail = false, calls = 0;
const gate = new Promise(resolve => { release = resolve; });
const source = stripTypeScriptTypes(fs.readFileSync(__dirname + '/pi-telemetry.ts', 'utf8').replace(/^import type .*\n/m, '')).replace('export default function (', 'function piTelemetry(');
const context = { Map, Set, WeakSet, Object, JSON, Number, String, Symbol, Promise, AbortSignal, setTimeout,
  Date: class extends Date { static now() { return 1700000000000; } },
  fetch: async (_url, options) => { assert.equal(options.redirect, 'error'); calls++; bodies.push(JSON.parse(options.body)); if (calls === 1) await gate; if (fail) throw Error('offline'); return {ok:true}; } };
context.globalThis = context;
vm.runInNewContext(source + '\nthis.install = piTelemetry;', context);
context.install({on:(name, fn) => {handlers[name] = fn;}});
const emit = (name, event={}) => handlers[name]?.(event);
const message = (model, input=1, output=0, cost=0) => ({role:'assistant', stopReason:'stop', model, usage:{input,output,cacheRead:0,cacheWrite:0,cost:{total:cost}}});
const points = () => bodies.flatMap(b => b.resourceMetrics[0].scopeMetrics[0].metrics).map(m => ({name:m.name, point:m.sum.dataPoints[0], attrs:Object.fromEntries(m.sum.dataPoints[0].attributes.map(a=>[a.key,a.value.stringValue]))}));
const matching = (name, attrs={}) => points().filter(p=>p.name===name && Object.entries(attrs).every(([k,v])=>p.attrs[k]===v));
const latest = (name, attrs={}) => matching(name,attrs).at(-1)?.point.asDouble;
const tick = () => new Promise(resolve => setTimeout(resolve, 15));
(async () => {
  const pending = message('gpt-5.2',4,2,.25); pending.stopReason='pending';
  emit('message_end',{message:pending}); pending.stopReason='stop';
  emit('message_end',{message:pending}); emit('message_end',{message:pending});
  emit('message_end',{message:{role:'user',usage:{input:999}}});
  emit('message_end',{message:{role:'toolResult',usage:{input:999}}});
  emit('agent_start'); emit('agent_end'); emit('agent_end'); emit('session_shutdown');
  emit('session_start'); emit('agent_start');
  emit('message_end',{message:message('gpt-5.2',6,3,.5)});
  emit('tool_execution_start',{toolCallId:'a',toolName:'read'});
  emit('tool_execution_start',{toolCallId:'b',toolName:'mcp__private_secret'});
  emit('tool_execution_start',{toolCallId:'a',toolName:'read'});
  emit('tool_execution_end',{toolCallId:'unknown',toolName:'read'});
  emit('tool_execution_end',{toolCallId:'b',toolName:'mcp__private_secret',isError:true});
  emit('tool_execution_end',{toolCallId:'a',toolName:'read',isError:false});
  emit('tool_execution_end',{toolCallId:'a',toolName:'read'});
  const child = vm.runInNewContext('globalThis[Symbol.for("rotta.pi.child.telemetry")]', context);
  child({phase:'started',role:'private-role'});
  child({phase:'finished',role:'implementation',reason:'success',responseModel:'private-model',responseUsage:{input:7,output:2,cacheRead:1,cacheWrite:0,cost:.3}});
  child({phase:'finished',role:'reviewer',reason:'private-status',responseUsage:{input:888}});
  emit('agent_end'); emit('session_shutdown');
  emit('session_start'); emit('message_end',{message:message('gpt-5.2',8,4,.75)});
  for (let i=0;i<2000;i++) emit('message_end',{message:message('novel-model')});
  assert.equal(calls,1,'only one inflight request under stalled fetch');
  release(); await tick();
  assert(calls < 50,'bounded coalescing');
  assert.equal(matching('ai_cli_pi_tokens',{type:'input'})[0].point.asDouble,4,'first epoch sent before transition');
  assert.equal(latest('ai_cli_pi_tokens',{type:'input'}),2018,'all epochs counted');
  assert.equal(new Set(matching('ai_cli_pi_tokens',{type:'input'}).map(p=>p.point.startTimeUnixNano)).size,1,'one cumulative process epoch');
  assert.equal(latest('ai_cli_pi_model_tokens',{type:'input',model:'gpt-5.2'}),18);
  assert.equal(latest('ai_cli_pi_model_tokens',{type:'input',model:'novel-model'}),2000);
  assert.equal(latest('ai_cli_pi_agent_runs'),2);
  assert.equal(latest('ai_cli_pi_tool_call',{category:'mcp',status:'error'}),1);
  assert.equal(latest('ai_cli_pi_tool_call',{category:'tool',status:'ok'}),1);
  assert.equal(latest('ai_cli_pi_child_events',{role:'unknown',status:'started'}),1);
  assert.equal(latest('ai_cli_pi_child_events',{role:'reviewer',status:'child_error'}),1);
  assert.equal(latest('ai_cli_pi_child_response_tokens',{role:'implementation',type:'input',model:'private-model'}),7);
  assert.equal(latest('ai_cli_pi_child_response_estimated_cost_usd',{role:'implementation'}),.3);
  assert.equal(latest('ai_cli_pi_estimated_cost_usd'),1.5);
  for (const p of points()) assert(BigInt(p.point.timeUnixNano)>=BigInt(p.point.startTimeUnixNano));
  const times=points().map(p=>BigInt(p.point.timeUnixNano));
  assert(times.every((t,i)=>i===0 || t>times[i-1]),'strictly monotonic points at frozen clock');
  const serialized=JSON.stringify(bodies);
  assert(!serialized.includes('private_secret') && !serialized.includes('private-role') && !serialized.includes('private-status'));
  const cases = [[null,'unknown'], [123,'unknown'], ['  ','unknown'], [' x/y:v-1 ','x/y:v-1'],
    ['gpt-6-sol','gpt-6-sol'], ['gpt-6.1-sol','gpt-6.1-sol'], ['gpt-6.1-terra','gpt-6.1-terra'],
    ['a'.repeat(128),'a'.repeat(128)], ['a'.repeat(129),'unknown'], ['a b','unknown'], ['a\u0000b','unknown'], ['/bad','unknown']];
  for (const [raw, expected] of cases) {
    emit('message_end',{message:message(raw,2)});
    child({phase:'finished',role:'reviewer',reason:'success',responseModel:raw,responseUsage:{input:3}});
  }
  await tick();
  for (const expected of new Set(cases.map(([, value]) => value))) {
    const count = cases.filter(([, value]) => value === expected).length;
    assert.equal(latest('ai_cli_pi_model_tokens',{type:'input',model:expected}),count*2);
    assert.equal(latest('ai_cli_pi_child_response_tokens',{role:'reviewer',type:'input',model:expected}),count*3 + (expected === 'unknown' ? 888 : 0));
  }
  fail=true;
  for(let i=0;i<1000;i++) emit('message_end',{message:message('novel-model')});
  await tick(); const stopped=calls;
  for(let i=0;i<1000;i++) emit('message_end',{message:message('novel-model')});
  assert.equal(calls,stopped,'failed send backpressure');
  fail=false; await new Promise(resolve=>setTimeout(resolve,1600)); await tick();
  assert.equal(latest('ai_cli_pi_model_tokens',{type:'input',model:'novel-model'}),4000);
  assert.equal(latest('ai_cli_pi_tokens',{type:'input'}),4018 + cases.length * 2);
  console.log('Pi epoch, semantic callbacks, bounded recovery and privacy passed');
})().catch(e=>{console.error(e);process.exitCode=1;});
