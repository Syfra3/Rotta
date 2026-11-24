// Synthetic only. Run inside an owned --internal network with collector, Loki, Tempo and Prometheus aliases.
const assert = require('node:assert/strict');
const canary = 'PRIVATE_CANARY_r126';
const ns = BigInt(Date.now()) * 1000000n;
const t = ns.toString();
const attrs = (o) => Object.entries(o).map(([key,value]) => ({key, value: typeof value === 'number' ? {doubleValue: value} : {stringValue:String(value)}}));
const resource = {attributes:attrs({'service.name':'claude-code','secret':canary})};
const scope = {name:canary, version:canary, attributes:attrs({secret:canary})};
const sender = async (path, payload) => {
  const r = await fetch('http://collector:4318/v1/'+path,{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify(payload)});
  assert.equal(r.status,200,`${path}: ${await r.text()}`);
};
const query = async (url) => {const r=await fetch(url); const text=await r.text(); assert.equal(r.status,200,`${url}: ${text}`); return JSON.parse(text);};
(async () => {
  for (const [i,total] of [3,8].entries()) {
    await sender('metrics',{resourceMetrics:[{resource,scopeMetrics:[{scope,metrics:[{name:'claude_code.token.usage',description:canary,unit:canary,sum:{aggregationTemporality:2,isMonotonic:true,dataPoints:[{asInt:String(total),startTimeUnixNano:(ns-10000000000n).toString(),timeUnixNano:(ns+BigInt(i)*1000000n).toString(),attributes:attrs({type:'input',model:'sonnet',query_source:'main','session.id':canary})}]}}]}]}]});
  }
  const base={'event.name':'api_request','data_source':'transcript','session.id':'0123456789abcdef','user.id':'0123456789abcdef','prompt.id':'1123456789abcdef',model:'sonnet',tool_name:'Read',branch_id:'01234567',context_tokens:'123',context_pct:'0.5',tool_parameters:'{"path_hash": "0123456789abcdef"}',source:'config',command_class:'git',git_action:'commit',context_drop:'false',attempt:'1',retry_in_ms:'100'};
  let logSequence=0n;
  // Loki deduplicates equal stream/timestamp/body even if structured metadata
  // differs. Distinct input events need distinct timestamps to test admission.
  const log=(fields,body='claude_code.metadata')=>({timeUnixNano:(ns+(++logSequence)*1000000n).toString(),body:{stringValue:body},severityText:canary,attributes:attrs({...base,...fields})});
  await sender('logs',{resourceLogs:[{resource,scopeLogs:[{scope,logRecords:[log({},'valid-r126')]}]}]});
  await sender('logs',{resourceLogs:[{resource,scopeLogs:[{scope,logRecords:[log({context_tokens:123,context_pct:0.5,duration_ms:12,attempt:1},'numeric-r126')]}]}]});
  for(const key of ['prompt.id','event.id','tool_parameters','context_tokens','context_pct','duration_ms','input_tokens','output_tokens','cache_read_tokens','cache_creation_tokens','context_window','context_previous_tokens','attempt','max_retries','retry_in_ms','command_class','git_action','context_drop','source','model','tool_name','branch_class']) {
    await sender('logs',{resourceLogs:[{resource,scopeLogs:[{scope,logRecords:[log({[key]:canary},canary)]}]}]});
  }
  await sender('traces',{resourceSpans:[{resource,scopeSpans:[{scope,spans:[{traceId:'1123456789abcdef0123456789abcdef',spanId:'0123456789abcdef',name:'claude_code.interaction',traceState:canary,startTimeUnixNano:t,endTimeUnixNano:(ns+1000000n).toString(),attributes:attrs({'session.id':'0123456789abcdef','prompt.id':'1123456789abcdef',model:'sonnet',tool_name:'Read',secret:canary}),status:{code:2,message:canary}}]}]}]});
  const nested=log({},canary);
  nested.attributes.find(a=>a.key==='context_tokens').value={kvlistValue:{values:attrs({secret:canary})}};
  await sender('logs',{resourceLogs:[{resource,scopeLogs:[{scope,logRecords:[nested]}]}]});
  const badSpan=(id,extra)=>({traceId:'1123456789abcdef0123456789abcdef',spanId:id,name:'claude_code.tool',startTimeUnixNano:t,endTimeUnixNano:(ns+1000000n).toString(),...extra});
  await sender('traces',{resourceSpans:[{resource,scopeSpans:[{scope,spans:[
    badSpan('2123456789abcdef',{events:[{timeUnixNano:t,name:canary,attributes:attrs({secret:canary})}]}),
    badSpan('3123456789abcdef',{links:[{traceId:'2123456789abcdef0123456789abcdef',spanId:'4123456789abcdef',traceState:canary,attributes:attrs({secret:canary})}]})
  ]}]}]});
  await sender('metrics',{resourceMetrics:[{resource,scopeMetrics:[{scope,metrics:[{name:'claude_code.token.usage',sum:{aggregationTemporality:2,isMonotonic:true,dataPoints:[{
    asInt:'777',startTimeUnixNano:(ns-10000000000n).toString(),timeUnixNano:(ns+10000000n).toString(),attributes:attrs({type:'input',model:'sonnet',query_source:'main'}),
    exemplars:[{asInt:'777',timeUnixNano:t,filteredAttributes:attrs({secret:canary})}]
  }]}}]}]}]});
  for(const bad of [-1,'NaN','Infinity',1e16]) {
    await sender('metrics',{resourceMetrics:[{resource,scopeMetrics:[{scope,metrics:[{name:'claude_code.token.usage',sum:{aggregationTemporality:2,isMonotonic:true,dataPoints:[{asDouble:bad,startTimeUnixNano:(ns-10000000000n).toString(),timeUnixNano:(ns+20000000n).toString(),attributes:attrs({type:'input',model:'sonnet',query_source:'main'})}]}}]}]}]});
  }
  for(const model of [canary,'gpt-5.2']) {
    await sender('metrics',{resourceMetrics:[{resource,scopeMetrics:[{scope,metrics:[{name:'ai_cli_pi_model_tokens',description:canary,unit:canary,sum:{aggregationTemporality:2,isMonotonic:true,dataPoints:[{asInt:'11',startTimeUnixNano:(ns-10000000000n).toString(),timeUnixNano:t,attributes:attrs({type:'input',model,secret:canary})}]}}]}]}]});
  }
  await new Promise(r=>setTimeout(r,19000));
  const metrics=await (await fetch('http://collector:8889/metrics')).text();
  const loki=await query('http://loki:3100/loki/api/v1/query_range?query='+encodeURIComponent('{service_name="claude-code"}')+'&start='+(ns-60000000000n)+'&end='+(ns+60000000000n));
  const tempo=await query('http://tempo:3200/api/traces/1123456789abcdef0123456789abcdef');
  const prom=await query('http://prometheus:9090/api/v1/query?query='+encodeURIComponent('claude_code_token_usage_tokens_total'));
  for (const [name,v] of Object.entries({metrics,loki,tempo,prom})) {const s=typeof v==='string'?v:JSON.stringify(v); assert.ok(!s.includes(canary),`${name} leaked canary`); console.log(name,'bytes',s.length);}
  assert.ok(metrics.includes('model="sonnet"') && metrics.includes('query_source="main"') && metrics.includes('data_source="native"'), 'native reference dimensions missing');
  assert.equal(prom.data.result.length,1,'native reference series missing');
  assert.equal(Number(prom.data.result[0].value[1]),8,'exemplar-bearing point must not overwrite safe cumulative total');
  const records=loki.data.result.flatMap(x=>x.values);
  assert.equal(records.length,2,'expected valid string and numeric logs only; malformed bodies must not be stored');
  assert.ok(records.every(v=>v[1]==='claude_code.metadata:api_request'), 'sanitized bodies not stored');
  assert.ok(JSON.stringify(loki).includes('path_hash'), 'valid structured metadata missing');
  console.log('prom native',JSON.stringify(prom.data.result));
  console.log('loki streams',loki.data.result.length,'tempo batches',tempo.batches?.length);
})().catch(e=>{console.error(e);process.exitCode=1});
