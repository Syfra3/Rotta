#!/usr/bin/env python3
"""Read-only provisioned layout/query check; empty native vectors are unverified."""
import json
import urllib.parse
import urllib.request
from pathlib import Path

BASE = 'http://127.0.0.1:13000'
SOURCE = Path(__file__).parent / 'grafana/dashboards/samples.json'

class CheckError(ValueError):
    pass


def get(path):
    with urllib.request.build_opener(urllib.request.ProxyHandler({})).open(BASE + path, timeout=3) as response:
        if response.status != 200:
            raise CheckError(f'HTTP {response.status}')
        return json.load(response)


def all_panels(panels):
    for panel in panels:
        yield panel
        yield from all_panels(panel.get('panels', []))


def layout_tree(panels):
    return [(p.get('id'), p.get('type'), p.get('gridPos'), p.get('collapsed'),
             layout_tree(p.get('panels', []))) for p in panels]


def validate_dashboard(dashboard):
    expected = json.loads(SOURCE.read_text())
    if dashboard.get('uid') != 'ai-cli-samples' or dashboard.get('templating', {}).get('list', [{}])[0].get('allValue') != 'claude|opencode|copilot|pi':
        raise CheckError('UID or selector mismatch')
    panels = dashboard.get('panels', [])
    panels = list(all_panels(panels))
    reference_panels = list(all_panels(expected['panels']))
    if len(panels) != 24 or layout_tree(dashboard.get('panels', [])) != layout_tree(expected['panels']):
        raise CheckError('layout mismatch')
    mcp = next(p for p in panels if p['id'] == 18)
    expected_mcp = 'sum by (client,status) (rate(ai_cli_pi_tool_call_total{job="collector",source="native",category="mcp",client=~"${agent:regex}"}[5m]))'
    if len(mcp.get('targets', [])) != 1 or mcp['targets'][0].get('expr') != expected_mcp:
        raise CheckError('panel 18 MCP native rate query mismatch')
    for p, reference in zip(panels, reference_panels):
        if p.get('targets', []) != reference.get('targets', []):
            raise CheckError(f'panel {p["id"]} unsupported metric/query')
        if reference['type'] not in ('row', 'text') and not reference.get('targets') and 'unavailable' not in p.get('title', '').lower():
            raise CheckError(f'panel {p["id"]} unsupported signal not labeled unavailable')


def check():
    dashboard = get('/api/dashboards/uid/ai-cli-samples')['dashboard']
    validate_dashboard(dashboard)
    checks = 0
    for panel in all_panels(dashboard['panels']):
        for target in panel.get('targets', []):
            for selector in ('claude|opencode|copilot|pi', 'pi', 'claude|pi'):
                expr = target['expr'].replace('${agent:regex}', selector).replace('$__range', '1h')
                result = get('/api/datasources/proxy/uid/local-prometheus/api/v1/query?' + urllib.parse.urlencode({'query': expr}))
                if result.get('status') != 'success' or result.get('data', {}).get('resultType') != 'vector':
                    raise CheckError(f'panel {panel["id"]}: query failed')
                checks += 1
    print(f'{checks} native queries parsed; empty native results remain unverified')

def poll(marker, timeout=15):
    # Refuse to report a fresh fixture without checking its exact run marker.
    if marker is not None:
        for client in ('claude', 'opencode', 'copilot', 'pi'):
            expr = f'ai_cli_fixture_run_marker{{job="collector",source="fixture",client="{client}"}}'
            result = get('/api/datasources/proxy/uid/local-prometheus/api/v1/query?' + urllib.parse.urlencode({'query': expr}))
            rows = result.get('data', {}).get('result', [])
            if len(rows) != 1 or float(rows[0]['value'][1]) != marker:
                raise CheckError('missing or stale fixture marker')
    import time
    deadline = time.monotonic() + timeout
    while True:
        try:
            check()
            return
        except (CheckError, OSError, KeyError, ValueError):
            if time.monotonic() >= deadline:
                raise
            time.sleep(0.4)

if __name__ == '__main__':
    check()
