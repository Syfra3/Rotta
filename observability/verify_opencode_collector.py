"""Optional disposable Collector integration; no persistent stack or provider use."""
import json
import subprocess
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parent
IMAGE = 'otel/opentelemetry-collector-contrib:0.104.0'
CONFIG = ROOT / ('collector.claude.yaml' if '--claude' in sys.argv[1:] else 'collector.yaml')
container = subprocess.check_output([
    'docker', 'run', '-d', '--rm', '-p', '127.0.0.1::4318', '-p', '127.0.0.1::8889',
    '-v', f'{CONFIG}:/etc/otel/config.yaml:ro', IMAGE,
    '--config=/etc/otel/config.yaml',
], text=True).strip()
try:
    def port(number):
        value = subprocess.check_output(['docker', 'port', container, f'{number}/tcp'], text=True).strip()
        return int(value.rsplit(':', 1)[1])

    ingest, scrape = port(4318), port(8889)
    start = str(time.time_ns() - 1_000_000)
    now = str(time.time_ns())
    def attrs(**values):
        return [{'key': key, 'value': {'stringValue': value}} for key, value in values.items()]
    def metric(name, attributes, value=1, unit='1'):
        return {'name': name, 'unit': unit, 'sum': {'aggregationTemporality': 2, 'isMonotonic': True,
                'dataPoints': [{'attributes': attrs(**attributes), 'asDouble': value,
                                'startTimeUnixNano': start, 'timeUnixNano': now}]}}
    payload = {'resourceMetrics': [{'resource': {'attributes': attrs(**{'service.instance.id': 'secret-instance'})},
               'scopeMetrics': [{'metrics': [
                   metric('ai_cli_opencode_tokens', {'type': 'input'}, 7, '{token}'),
                   metric('ai_cli_opencode_model_tokens', {'model': 'openai/gpt-6', 'role': 'build', 'type': 'input', 'prompt': 'PRIVATE-PROMPT'}, 10, '{token}'),
                   metric('ai_cli_opencode_model_cost_usd', {'model': 'openai/gpt-6', 'role': 'build'}, .25, 'USD'),
                   metric('ai_cli_opencode_tool_calls', {'category': 'unknown', 'status': 'ok', 'file.path': 'PRIVATE-PATH'}),
                   metric('ai_cli_opencode_sessions', {'category': 'subagent'}),
                   metric('ai_cli_opencode_runs', {'status': 'error'}),
                   metric('ai_cli_opencode_compactions', {'category': 'auto'}),
                   metric('ai_cli_opencode_tool_calls', {'category': 'PRIVATE-BAD', 'status': 'ok'}),
               ]}]}]}
    body = json.dumps(payload).encode()
    for attempt in range(40):
        try:
            request = urllib.request.Request(f'http://127.0.0.1:{ingest}/v1/metrics', data=body,
                                             headers={'Content-Type': 'application/json'})
            with urllib.request.urlopen(request, timeout=1) as response:
                assert response.status == 200
            break
        except (OSError, urllib.error.URLError):
            if attempt == 39:
                raise
            time.sleep(.25)
    for attempt in range(40):
        with urllib.request.urlopen(f'http://127.0.0.1:{scrape}/metrics', timeout=2) as response:
            text = response.read().decode()
        if 'ai_cli_opencode_model_tokens_total' in text:
            break
        time.sleep(.25)
    assert 'ai_cli_opencode_model_tokens_total' in text, text
    canonical = [line for line in text.splitlines() if line.startswith('ai_cli_tokens_v1_total{') and 'type="input"' in line]
    assert len(canonical) == 1 and canonical[0].endswith(' 7'), canonical
    assert 'ai_cli_opencode_tool_calls_total' in text, text
    for name in ('ai_cli_opencode_model_cost_usd_USD_total', 'ai_cli_opencode_sessions_total',
                 'ai_cli_opencode_runs_total', 'ai_cli_opencode_compactions_total'):
        assert name in text, name
    assert 'model="openai/gpt-6"' in text and 'source="native"' in text
    for secret in ('PRIVATE-PROMPT', 'PRIVATE-PATH', 'PRIVATE-BAD', 'secret-instance'):
        assert secret not in text, secret
    print(f'disposable OpenCode metadata Collector integration ({CONFIG.name}): pass')
finally:
    subprocess.run(['docker', 'stop', container], check=False, stdout=subprocess.DEVNULL)
