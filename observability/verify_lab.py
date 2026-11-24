#!/usr/bin/env python3
"""Disposable, credential-free source acceptance. Only cached Docker images.

PYTHONDONTWRITEBYTECODE=1 python3 observability/verify_lab.py --output <evidence.json>
All data is synthetic; no host ports, private paths, downloads or persistent volumes.
"""
import argparse
import importlib.util
import json
import re
import shutil
import subprocess
import tempfile
import time
import urllib.parse
import urllib.request
import uuid
from datetime import datetime, timedelta, timezone
from html.parser import HTMLParser
from pathlib import Path

ROOT = Path(__file__).resolve().parent
IMAGES = {'collector': 'otel/opentelemetry-collector-contrib:0.104.0',
          'prometheus': 'prom/prometheus:v2.53.0', 'loki': 'grafana/loki:3.1.0',
          'tempo': 'grafana/tempo:2.5.0', 'grafana': 'grafana/grafana:13.3.0',
          'client': 'node:26-bookworm-slim'}


def docker(*args, input=None, check=True):
    result = subprocess.run(['docker', *args], input=input, text=True, capture_output=True,
                            check=False, timeout=120)
    if check and result.returncode:
        raise RuntimeError(f'docker {args[:2]}: {result.stderr[-1600:]}')
    return result


def client(container, script, payload=None):
    result = docker('exec', '-i', container, 'node', '-e', script,
                    input=json.dumps(payload) if payload is not None else '', check=False)
    if result.returncode:
        raise RuntimeError(result.stderr[-2000:] + result.stdout[-1000:])
    return json.loads(result.stdout)


FETCH = """const fs=require('node:fs');(async()=>{let p=JSON.parse(fs.readFileSync(0,'utf8'));let r=await fetch(p.url,{method:p.body?'POST':'GET',headers:{'content-type':'application/json'},body:p.body?JSON.stringify(p.body):undefined});console.log(JSON.stringify({status:r.status,body:await r.text()}));})().catch(e=>{console.error(e);process.exit(1)})"""


def fetch(container, url, body=None):
    return client(container, FETCH, {'url': url, 'body': body})


def wait(container, url, timeout=90):
    until = time.monotonic() + timeout
    while time.monotonic() < until:
        try:
            if fetch(container, url)['status'] == 200:
                return
        except RuntimeError:
            pass
        time.sleep(2)
    raise RuntimeError('readiness timeout: ' + url)


def expand(expression, dashboard):
    values = {v['name']: v.get('current', {}).get('value', v.get('query', ''))
              for v in dashboard.get('templating', {}).get('list', [])}
    values.update(__rate_interval='1m', __interval='1m', __range='1h', __range_s='3600')
    def replace(match):
        key = match.group(1) or match.group(2)
        value = values.get(key)
        if value is None:
            raise AssertionError('unresolved dashboard variable: ' + key)
        if value == '$__all':
            value = '.*'
        if isinstance(value, list):
            value = '|'.join(value)
        return str(value)
    # Token-based expansion avoids the old $__range -> 1h_s prefix bug.
    return re.sub(r'\$\{([A-Za-z_][\w]*)(?::\w+)?\}|\$([A-Za-z_][\w]*)', replace, expression)


def attrs(values):
    return [{'key': key, 'value': {'stringValue': str(value)}} for key, value in values.items()]


def fixture_metrics(stage, start):
    """Use actual adapter metric names; fixtures never touch the persistent stack."""
    metrics = []
    now = time.time_ns()
    def add(name, value, dimensions=None, unit=''):
        metrics.append({'name': name, 'unit': unit, 'sum': {'aggregationTemporality': 2,
                        'isMonotonic': True, 'dataPoints': [{'startTimeUnixNano': str(start),
                        'timeUnixNano': str(now), 'asDouble': value,
                        'attributes': attrs(dimensions or {})}]}})
    for kind, first, last in [('input', 12, 32), ('output', 4, 8), ('cacheRead', 3, 9), ('cacheCreation', 1, 2)]:
        add('claude_code.token.usage', (first, last)[stage],
            {'type': kind, 'model': 'claude-sonnet-4-5', 'query_source': 'main'}, '{token}')
    add('claude_code.cost.usage', (.125, .375)[stage], {'model': 'claude-sonnet-4-5', 'query_source': 'main'}, 'USD')
    add('claude_code.code_edit_tool.decision', (0, 1)[stage], {'decision':'accept', 'source':'config', 'tool_name':'Edit'})
    for host in ('pi', 'opencode'):
        for kind, first, last in [('input', 10, 30), ('output', 3, 7), ('cache_read', 2, 8), ('cache_write', 1, 2)]:
            add('ai_cli_'+host+'_tokens', (first, last)[stage], {'type': kind}, '{token}')
        add('ai_cli_'+host+'_estimated_cost_usd', (.1, .4)[stage], unit='USD')
    for kind, first, last in [('input', 10, 30), ('output', 3, 7), ('cache_read', 2, 8), ('cache_write', 1, 2)]:
        add('ai_cli_pi_model_tokens', (first, last)[stage], {'model': 'gpt-5.2', 'type': kind}, '{token}')
    add('ai_cli_pi_model_estimated_cost_usd', (.1, .4)[stage], {'model': 'gpt-5.2'}, 'USD')
    add('ai_cli_pi_agent_runs', (1, 2)[stage])
    add('ai_cli_pi_tool_call', (1, 3)[stage], {'category': 'tool', 'status': 'ok'})
    # Same model, labels and epoch in a second process: privacy scrubbing must
    # not collapse its cumulative counter into the first producer's last value.
    second = {'name':'claude_code.token.usage','unit':'tokens','sum':{'aggregationTemporality':2,
              'isMonotonic':True,'dataPoints':[{'startTimeUnixNano':str(start),'timeUnixNano':str(now),
              'asDouble':(2,5)[stage],'attributes':attrs({'type':'input','model':'claude-sonnet-4-5','query_source':'main'})}]}}
    return {'resourceMetrics': [{'resource': {'attributes': attrs({'service.name': 'claude-code'})},
            'scopeMetrics': [{'metrics': metrics}]},
            {'resource':{'attributes':attrs({'service.name':'claude-code','service.instance.id':'NATIVE_PRIVATE_METRIC_INSTANCE'})},
             'scopeMetrics':[{'metrics':[second]}]}]}


def native_api_costs():
    """Official monitoring-usage-shaped events; raw identities are synthetic."""
    now = time.time_ns()
    resource = {'attributes':attrs({'service.name':'claude-code','session.id':'NATIVE_PRIVATE_SESSION',
                                   'user.email':'NATIVE_PRIVATE_EMAIL'})}
    logs = []
    for i, cost in enumerate((.125, .25)):
        fields = attrs({'event.name':'api_request','model':'claude-sonnet-4-5',
                        'query_source':'repl_main_thread','user.id':'NATIVE_PRIVATE_USER',
                        'user.email':'NATIVE_PRIVATE_EMAIL','request_id':'NATIVE_PRIVATE_REQUEST_'+str(i),
                        'prompt.id':'NATIVE_PRIVATE_PROMPT'})
        if i == 0:
            fields += attrs({'session.id':'NATIVE_PRIVATE_SESSION'})
        fields += [{'key':'cost_usd','value':{'doubleValue':cost}},
                   {'key':'duration_ms','value':{'intValue':'120'}},
                   {'key':'event.sequence','value':{'intValue':str(i+1)}}]
        logs.append({'timeUnixNano':str(now-(20-i)*10**9),
                     'body':{'stringValue':'NATIVE_PRIVATE_BODY'}, 'attributes':fields})
    return {'resourceLogs':[{'resource':resource,'scopeLogs':[{'scope':{'name':'NATIVE_PRIVATE_SCOPE'},'logRecords':logs}]}]}


def transcript_rows():
    rows = []
    base = datetime.now(timezone.utc) - timedelta(seconds=40)
    for i, (sid, mid, tokens, tool) in enumerate([('first', 'm1', 25000, 'Bash'),
                                                ('first', 'm2', 1000, 'Edit'),
                                                ('second', 'm3', 9000, 'Read')]):
        stamp = lambda offset: (base + timedelta(seconds=i*5+offset)).isoformat()
        prompt = mid+'prompt'
        rows.append({'type': 'user', 'timestamp': stamp(0), 'sessionId': sid, 'uuid': prompt,
                     'promptId': prompt, 'message': {'content': 'SYNTHETIC_PRIVATE_PROMPT'}})
        rows.append({'type': 'assistant', 'timestamp': stamp(1), 'sessionId': sid, 'uuid': mid,
                     'promptId': prompt, 'message': {'id': mid, 'model': 'claude-sonnet-4-5',
                     'usage': {'input_tokens': tokens, 'output_tokens': 2}, 'content': [
                         {'type': 'tool_use', 'id': mid+'tool', 'name': tool, 'input':
                          {'command': 'git commit -m "SYNTHETIC_PRIVATE_COMMAND"'} if tool == 'Bash' else
                          {'file_path': '/private/SYNTHETIC_PRIVATE_PATH'}}]}})
        if tool == 'Bash':
            rows[-1]['message']['content'].append({'type':'tool_use','id':mid+'parallel','name':'Read',
                                                  'input':{'file_path':'/private/SYNTHETIC_PRIVATE_PATH'}})
        row = {'type': 'user', 'timestamp': stamp(3), 'sessionId': sid, 'uuid': mid+'result',
               'promptId': prompt, 'message': {'content': [{'type': 'tool_result', 'tool_use_id': mid+'tool',
                                                           'content': 'SYNTHETIC_PRIVATE_OUTPUT'}]}}
        if tool == 'Bash':
            row['message']['content'].append({'type':'tool_result','tool_use_id':mid+'parallel',
                                             'content':'SYNTHETIC_PRIVATE_OUTPUT'})
        if tool == 'Edit':
            row['permissionDecision'] = {'decision': 'accept', 'source': 'config'}
        rows.append(row)
    rows.append({'type': 'system', 'timestamp': (base+timedelta(seconds=18)).isoformat(),
                 'sessionId': 'first', 'uuid': 'retry1', 'subtype': 'api_error', 'retryAttempt': 1})
    return rows


def query_target(node, panel, target, dashboard):
    expression = expand(target.get('expr', target.get('query', '')), dashboard)
    source = target.get('datasource', panel.get('datasource', {}))
    source_type = source.get('type') if isinstance(source, dict) else source
    if source_type == 'tempo':
        endpoint = 'http://tempo:3200/api/search?'
        params = {'q': expression}
    elif source_type == 'loki' or 'service_name=' in expression:
        endpoint = 'http://loki:3100/loki/api/v1/query_range?'
        params = {'query': expression, 'start': str(time.time_ns()-3600*10**9), 'end': str(time.time_ns()), 'step': '60'}
    else:
        if target.get('range', False) and not target.get('instant', False):
            endpoint = 'http://prometheus:9090/api/v1/query_range?'
            params = {'query': expression, 'start': str(time.time()-3600), 'end': str(time.time()), 'step': '60'}
        else:
            endpoint = 'http://prometheus:9090/api/v1/query?'
            params = {'query': expression}
    response = fetch(node, endpoint+urllib.parse.urlencode(params))
    try:
        parsed = json.loads(response['body'])
    except ValueError:
        parsed = {}
    result = parsed.get('traces', []) if source_type == 'tempo' else parsed.get('data', {}).get('result', [])
    status = 'error' if response['status'] != 200 or parsed.get('status') == 'error' else ('nonempty' if result else 'empty')
    return {'panel': panel['id'], 'ref': target.get('refId'), 'status': status,
            'query': expression, 'response': parsed if status == 'error' else None}


def render_dashboards(container, root, output):
    binary = shutil.which('chromium')
    if not binary:
        return {'status': 'unavailable', 'reason': 'chromium not installed'}
    # Revalidate the owned container's actual address; no shared browser profile,
    # published port, proxy, provider endpoint or private HOME is involved.
    info = json.loads(docker('inspect', container).stdout)[0]
    networks = info['NetworkSettings']['Networks']
    assert len(networks) == 1 and info['Name'].startswith('/rotta-lab-')
    address = next(iter(networks.values()))['IPAddress']
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    with opener.open('http://'+address+':3000/api/health', timeout=5) as response:
        health = json.load(response)
    assert health.get('version') == '13.3.0' and health.get('database') == 'ok', 'browser target is not the owned Grafana'
    results = {}
    for uid, title in [('claude-code-telemetry-lab', 'Cost in range'),
                       ('claude-code-members-sessions', 'Cost by member'),
                       ('claude-code-budget', 'Month to date'),
                       ('ai-cli-host-companions', 'Cache read share')]:
        profile = root/('browser-'+uid)
        screenshot = output/('browser-'+uid+'.png')
        cmd = [binary, '--headless=new', '--no-sandbox', '--disable-gpu', '--no-first-run',
               '--disable-background-networking', '--disable-component-update', '--no-proxy-server',
               '--host-resolver-rules=MAP * ~NOTFOUND, EXCLUDE '+address, '--window-size=1600,1200',
               '--virtual-time-budget=15000', '--user-data-dir='+str(profile),
               '--screenshot='+str(screenshot), '--dump-dom',
               'http://'+address+':3000/d/'+uid+'?kiosk']
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=70)
        dom = result.stdout
        class VisibleText(HTMLParser):
            def __init__(self):
                super().__init__()
                self.hidden = 0
                self.parts = []
            def handle_starttag(self, tag, _attrs):
                if tag in ('script','style'):
                    self.hidden += 1
            def handle_endtag(self, tag):
                if tag in ('script','style'):
                    self.hidden = max(0,self.hidden-1)
            def handle_data(self, data):
                if not self.hidden:
                    self.parts.append(data)
        visible = VisibleText()
        visible.feed(dom)
        text = ' '.join(visible.parts)
        (output/('browser-'+uid+'.html')).write_text(dom)
        assert result.returncode == 0 and title in dom and screenshot.is_file(), (uid, 'browser did not render expected dashboard')
        assert all(error not in dom for error in ('Dashboard not found','Error loading dashboard','Data source not found','Datasource not found','Panel plugin not found'))
        if uid in ('claude-code-telemetry-lab','claude-code-budget'):
            assert re.search(r'\$\s*0[.,]3[78]',text), (uid,'dashboard shell rendered but fixture currency values did not')
        if uid == 'ai-cli-host-companions':
            assert re.search(r'\$\s*0[.,]4',text), (uid,'host cost counters did not render their observed total')
        if uid == 'claude-code-budget':
            assert 'INCOMPLETE' in text
        results[uid] = {'status': 'rendered', 'title_observed': title, 'screenshot': str(screenshot)}
    return results


def main(output=None, browser=False, debug=False):
    for image in IMAGES.values():
        docker('image', 'inspect', image)
    tag = 'rotta-lab-' + uuid.uuid4().hex[:12]
    containers, network_created = [], False
    evidence = {'synthetic_only': True, 'checks': {}}
    with tempfile.TemporaryDirectory(prefix='rotta-lab-') as tmp:
        root = Path(tmp)
        try:
            docker('network', 'create', '--internal', tag)
            network_created = True
            def start(name, args=(), mounts=(), env=()):
                container = tag+'-'+name
                cmd = ['run', '-d', '--pull=never', '--network', tag, '--name', container,
                       '--network-alias', name, '--security-opt', 'no-new-privileges',
                       '--cap-drop=ALL', '--memory=512m']
                for src, dest in mounts:
                    cmd += ['--mount', f'type=bind,src={src},dst={dest},readonly']
                for key, value in env:
                    cmd += ['--env', f'{key}={value}']
                # Track even a failed start so finally cleans any created container.
                containers.append(container)
                docker(*(cmd+[IMAGES[name], *args]))
                return container
            start('loki', ('-config.file=/etc/loki/config.yaml',), [(ROOT/'loki.yaml', '/etc/loki/config.yaml')])
            start('tempo', ('-config.file=/etc/tempo/config.yaml',), [(ROOT/'tempo.yaml', '/etc/tempo/config.yaml')])
            collector_config = ROOT/'collector.claude.yaml'
            if debug:
                collector_config = root/'collector-debug.yaml'
                collector_config.write_text((ROOT/'collector.claude.yaml').read_text().replace(
                    'service:\n  pipelines:', 'service:\n  telemetry:\n    logs:\n      level: debug\n  pipelines:'))
            start('collector', ('--config=/etc/otel/config.yaml',), [(collector_config, '/etc/otel/config.yaml')])
            start('prometheus', ('--config.file=/etc/prometheus/prometheus.yml',), [(ROOT/'prometheus.yaml', '/etc/prometheus/prometheus.yml')])
            start('grafana', mounts=[(ROOT/'grafana/dashboards', '/etc/grafana/provisioning/dashboards'),
                                     (ROOT/'grafana/datasources', '/etc/grafana/provisioning/datasources')],
                  env=[('GF_AUTH_ANONYMOUS_ENABLED', 'true'), ('GF_AUTH_ANONYMOUS_ORG_ROLE', 'Viewer'),
                       ('GF_ANALYTICS_REPORTING_ENABLED', 'false'), ('GF_ANALYTICS_CHECK_FOR_UPDATES', 'false'),
                       ('GF_ANALYTICS_CHECK_FOR_PLUGIN_UPDATES', 'false'), ('GF_PLUGINS_PLUGIN_ADMIN_ENABLED', 'false')])
            node = start('client', ('sleep', '600'))
            for url in ('http://loki:3100/ready', 'http://tempo:3200/ready', 'http://collector:8889/metrics',
                        'http://prometheus:9090/-/ready', 'http://grafana:3000/api/health'):
                wait(node, url)
            docker('cp', str(ROOT/'probe_claude_pipeline.cjs'), node+':/privacy.cjs')
            evidence['privacy_probe'] = docker('exec', node, 'node', '/privacy.cjs').stdout
            spec = importlib.util.spec_from_file_location('lab_exporter', ROOT/'claude_exporter.py')
            exporter = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(exporter)
            project = root/'project'
            project.mkdir()
            (project/'events.jsonl').write_text(''.join(json.dumps(row)+'\n' for row in transcript_rows()))
            bodies, failed = [], False
            def send(signal, body):
                nonlocal failed
                value = json.loads(body)
                assert 'SYNTHETIC_PRIVATE_' not in body, 'exporter leaked fixture content'
                bodies.append((signal, value))
                reply = fetch(node, 'http://collector:4318/v1/'+signal, value)
                if reply['status'] != 200:
                    raise RuntimeError(f'{signal} rejected: {reply}')
                if signal == 'metrics' and not failed:
                    # Lost acknowledgement after the backend accepted the counter.
                    failed = True
                    raise OSError('injected acknowledgement loss')
            state = root/'state'
            try:
                exporter.run(project, state, 'http://127.0.0.1:14318', send=send)
                raise AssertionError('failure injection did not execute')
            except OSError:
                pass
            exporter.run(project, state, 'http://127.0.0.1:14318', send=send)
            count = len(bodies)
            exporter.run(project, state, 'http://127.0.0.1:14318', send=send)
            assert len(bodies) == count, 'restart replay produced duplicate bodies'
            evidence['sent_api_records'] = [item for signal, body in bodies if signal == 'logs'
                for resource in body.get('resourceLogs', []) for scope in resource.get('scopeLogs', [])
                for item in scope.get('logRecords', []) if any(a['key'] == 'event.name' and
                a['value'].get('stringValue') == 'api_request' for a in item['attributes'])]
            evidence.update(captured_bodies=count, acknowledgement_loss_recovered=failed,
                            unchanged_restart_sent_nothing=True)
            native_reply = fetch(node, 'http://collector:4318/v1/logs', native_api_costs())
            assert native_reply['status'] == 200, native_reply
            start_ns = time.time_ns()-10**9
            for stage in (0, 1):
                reply = fetch(node, 'http://collector:4318/v1/metrics', fixture_metrics(stage, start_ns))
                assert reply['status'] == 200, reply
                time.sleep(20)  # two real Prometheus scrapes, not a zero-rate assertion
            cost_query = urllib.parse.urlencode({'query':'sum by (session_id) (sum_over_time({service_name="claude-code"} | event_name="api_cost" | unwrap cost_usd [1h]))',
                                                'start':str(time.time_ns()-3600*10**9),'end':str(time.time_ns())})
            until = time.monotonic()+30
            while True:
                native_cost = fetch(node,'http://loki:3100/loki/api/v1/query_range?'+cost_query)
                assert 'NATIVE_PRIVATE_' not in native_cost['body'], 'native identity/body escaped sanitizer'
                data = json.loads(native_cost['body']).get('data',{}).get('result',[])
                if data or time.monotonic() >= until:
                    break
                time.sleep(2)
            assert len(data) == 1 and float(data[0]['values'][-1][1]) == .375, native_cost
            assert len(data[0]['metric']['session_id']) == 64, 'native session ID was not hashed'
            evidence['native_session_cost'] = .375
            log_query = urllib.parse.urlencode({'query': '{service_name="claude-code"} | event_name="api_request"',
                                               'start': str(time.time_ns()-3600*10**9), 'end': str(time.time_ns())})
            expected_ids = {a['value']['stringValue'] for item in evidence['sent_api_records']
                            for a in item['attributes'] if a['key'] == 'event.id'}
            until = time.monotonic()+30
            while True:
                evidence['api_logs'] = fetch(node, 'http://loki:3100/loki/api/v1/query_range?'+log_query)
                streams = json.loads(evidence['api_logs']['body']).get('data', {}).get('result', [])
                observed_ids = {item['stream'].get('event_id') for item in streams}
                if expected_ids <= observed_ids or time.monotonic() >= until:
                    break
                time.sleep(2)
            assert expected_ids <= observed_ids, 'same-timestamp log events were lost/deduplicated'
            evidence['api_event_ids_observed'] = len(expected_ids)
            # Two tool results intentionally share one timestamp. Their salted event
            # identities in the sanitized line must prevent Loki collapsing them.
            result_query = urllib.parse.urlencode({'query':'{service_name="claude-code"} | event_name="tool_result"',
                                                  'start':str(time.time_ns()-3600*10**9), 'end':str(time.time_ns())})
            until = time.monotonic()+30
            while True:
                result_logs = json.loads(fetch(node,'http://loki:3100/loki/api/v1/query_range?'+result_query)['body'])
                tool_ids = {item['stream'].get('event_id') for item in result_logs.get('data',{}).get('result',[])}
                if len(tool_ids-{None}) == 4 or time.monotonic() >= until:
                    break
                time.sleep(2)
            assert len(tool_ids-{None}) == 4, 'parallel tool-result events collapsed'
            evidence['parallel_tool_result_events'] = 4
            dashboards = ('claude-code.json', 'claude-code-members.json', 'claude-code-budget.json', 'host-companions.json')
            for filename in dashboards:
                dash = json.loads((ROOT/'grafana/dashboards'/filename).read_text())
                uid = dash['uid']
                assert fetch(node, 'http://grafana:3000/api/dashboards/uid/'+uid)['status'] == 200
                outcomes = [query_target(node, panel, target, dash) for panel in dash['panels']
                            for target in panel.get('targets', []) if target.get('expr') or target.get('query')]
                evidence['checks'][uid] = {'provisioned': True, 'outcomes': outcomes}
                assert not [o for o in outcomes if o['status'] == 'error'], (uid, [o for o in outcomes if o['status'] == 'error'])
                required = {1, 2, 4, 5, 7, 8, 30, 34, 38} if filename == 'claude-code.json' else (
                           {50, 51, 53, 56, 57, 58, 14, 15} if filename == 'claude-code-members.json' else (
                           {23, 24, 25, 26, 27} if filename == 'claude-code-budget.json' else set(range(1, 9))))
                missing = required-{o['panel'] for o in outcomes if o['status'] == 'nonempty'}
                until = time.monotonic()+30
                # OTLP acceptance and Loki indexed query visibility are asynchronous.
                # Poll known fixture-supported panels only; never turn absent data
                # into zero or treat deterministic query errors as transient success.
                while missing and time.monotonic() < until:
                    time.sleep(2)
                    outcomes = [query_target(node, panel, target, dash) for panel in dash['panels']
                                for target in panel.get('targets', []) if target.get('expr') or target.get('query')]
                    assert not [o for o in outcomes if o['status'] == 'error']
                    missing = required-{o['panel'] for o in outcomes if o['status'] == 'nonempty'}
                evidence['checks'][uid]['outcomes'] = outcomes
                assert not missing, (filename, 'required fixture-supported panels empty', missing)
            team = json.loads((ROOT/'grafana/dashboards/claude-code.json').read_text())
            peak_expression = expand(next(p for p in team['panels'] if p['id']==30)['targets'][0]['expr'], team)
            peak = fetch(node,'http://loki:3100/loki/api/v1/query?'+urllib.parse.urlencode({'query':peak_expression}))
            peak_values = json.loads(peak['body']).get('data',{}).get('result',[])
            assert len(peak_values)==1 and float(peak_values[0]['value'][1])==.125, peak
            evidence['checks']['context_peak'] = {'expected':.125,'actual':float(peak_values[0]['value'][1])}
            members = json.loads((ROOT/'grafana/dashboards/claude-code-members.json').read_text())
            overview = next(p for p in members['panels'] if p['id']==54)
            queries = [{**target,'expr':expand(target['expr'], members),'intervalMs':60000,'maxDataPoints':1000}
                       for target in overview['targets']]
            mixed = fetch(node,'http://grafana:3000/api/ds/query',
                          {'from':str(int((time.time()-3600)*1000)),'to':str(int(time.time()*1000)),'queries':queries})
            assert mixed['status']==200, mixed
            mixed_results = json.loads(mixed['body']).get('results',{})
            assert set(mixed_results)=={'A','B','C','D'} and all(not v.get('error') for v in mixed_results.values()), mixed
            assert mixed_results['C'].get('frames'), 'mixed context query has no data frame'
            evidence['mixed_member_overview'] = 'four datasource queries accepted; context frame observed'
            edit_ratio = expand(next(p for p in team['panels'] if p['id'] == 3)['targets'][0]['expr'], team)
            host = json.loads((ROOT/'grafana/dashboards/host-companions.json').read_text())
            host_expr = {p['id']:expand(p['targets'][0]['expr'], host) for p in host['panels'] if p.get('targets')}
            for label, expression, expected in [('host_cost_total', 'sum('+host_expr[1]+')', .8),
                                                 ('host_cache_percent_sum', 'sum('+host_expr[2]+')', 40),
                                                 ('host_completed_tools', 'sum('+host_expr[6]+')', 3),
                                                 ('host_completed_runs', host_expr[7], 2),
                                                 ('cost_per_accepted_edit', edit_ratio, .375),
                                                 ('sessions', 'sum(claude_code_session_count_total)', 2),
                                                 ('tools', 'sum(claude_code_tool_count_total)', 4),
                                                 ('context_drop', 'sum(claude_code_context_drop_count_total)', 1),
                                                 ('native_input', 'sum(claude_code_token_usage_tokens_total{type="input"})', 45),
                                                 ('canonical_native_input', 'sum(ai_cli_tokens_v1_total{client="claude",source="native",type="input"})', 45),
                                                 ('native_edit_decisions', 'sum(claude_code_code_edit_tool_decision_total{data_source="native",decision="accept"})', 1)]:
                response = fetch(node, 'http://prometheus:9090/api/v1/query?'+urllib.parse.urlencode({'query': expression}))
                values = json.loads(response['body'])['data']['result']
                assert len(values) == 1 and float(values[0]['value'][1]) == expected, (label, values)
                evidence['checks'][label] = {'expected': expected, 'actual': float(values[0]['value'][1])}
            if browser:
                browser_output = output.parent/(output.stem+'-browser') if output else root/'browser-evidence'
                browser_output.mkdir(parents=True, exist_ok=True)
                evidence['browser'] = render_dashboards(tag+'-grafana', root, browser_output)
            evidence['status'] = 'passed'
        except Exception as exc:
            evidence.update(status='failed', error=str(exc))
            raise
        finally:
            if debug and output and tag+'-collector' in containers:
                logs = docker('logs', tag+'-collector', check=False)
                output.parent.mkdir(parents=True, exist_ok=True)
                output.with_suffix('.collector.log').write_text((logs.stdout+logs.stderr)[-4*1024*1024:])
            for container in reversed(containers):
                docker('rm', '-f', container, check=False)
            if network_created:
                docker('network', 'rm', tag, check=False)
            remaining = [x for x in containers if docker('inspect', x, check=False).returncode == 0]
            evidence['cleanup'] = {'remaining_containers': remaining,
                                   'network_removed': docker('network', 'inspect', tag, check=False).returncode != 0}
            if output:
                output.parent.mkdir(parents=True, exist_ok=True)
                output.write_text(json.dumps(evidence, indent=2)+'\n')
            print(json.dumps(evidence, indent=2))
            assert not remaining and evidence['cleanup']['network_removed'], 'owned fixture cleanup failed'


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path)
    parser.add_argument('--browser', action='store_true', help='render owned Grafana with an empty temporary Chromium profile')
    parser.add_argument('--debug', action='store_true', help='record only synthetic fixture collector diagnostics')
    args = parser.parse_args()
    main(args.output, args.browser, args.debug)
