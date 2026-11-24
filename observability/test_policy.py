#!/usr/bin/env python3
"""Offline negative fixtures for policy and dashboard; not collector execution."""
import json
from pathlib import Path
from fixture import payload

root = Path(__file__).parent
compose = (root / 'compose.yaml').read_text()
for setting in ('GF_ANALYTICS_REPORTING_ENABLED', 'GF_ANALYTICS_CHECK_FOR_UPDATES',
                'GF_ANALYTICS_CHECK_FOR_PLUGIN_UPDATES', 'GF_PLUGINS_PLUGIN_ADMIN_ENABLED'):
    assert f'{setting}: "false"' in compose
assert 'GF_PLUGINS_PUBLIC_KEY_RETRIEVAL_DISABLED: "true"' in compose
assert 'internal: true' not in compose
assert '127.0.0.1:14318:4318' in compose and '127.0.0.1:13000:3000' in compose
config = (root / 'collector.yaml').read_text()
assert 'exporters: [prometheus]' in config and 'endpoint: 0.0.0.0:8889' in config
assert 'url: http://prometheus:9090' in (root / 'grafana/datasources/default.yaml').read_text()
assert 'resource_to_telemetry_conversion:\n      enabled: false' in config
copilot = {'gen_ai.client.inference.usage.' + suffix for suffix in
           ('input_tokens', 'output_tokens', 'cache_read.input_tokens', 'cache_write.input_tokens')} | {
           'github.copilot.tool.call.count', 'github.copilot.code.lines_added',
           'github.copilot.code.lines_removed'}
native = {'claude_code.token.usage', 'claude_code.cost.usage', 'ai_cli_pi_agent_runs', 'ai_cli_pi_tokens', 'ai_cli_pi_estimated_cost_usd', 'ai_cli_pi_model_tokens', 'ai_cli_pi_model_estimated_cost_usd', 'ai_cli_pi_tool_call'} | copilot
fixture = {'ai_cli_fixture_canonical_tokens', 'ai_cli_fixture_run_marker', 'ai_cli_sample_tokens', 'ai_cli_fixture_claude_tokens',
           'ai_cli_fixture_claude_cost_usd', 'ai_cli_fixture_pi_marker', 'ai_cli_fixture_copilot_tokens'}
allow = next(line for line in config.splitlines() if 'metric.name != "ai_cli_sample_tokens"' in line)
assert all(f'metric.name != "{name}"' in allow for name in native | fixture)
assert 'resource.attributes["ai_cli_source"] == "fixture"' in config
assert 'keep_keys(attributes, [])' in config
assert 'keep_keys(attributes, ["client", "type", "category", "status", "model", "role", "query_source", "counter.id"])' in config
assert 'set(attributes["counter.id"], SHA256(' in config
assert 'metric_expiration: 5m' in config
assert 'set(attributes["source"], "native")' in config
assert 'set(attributes["source"], "fixture")' in config

sent = payload()['resourceMetrics'][0]
metrics = {m['name']: m for m in sent['scopeMetrics'][0]['metrics']}
assert set(metrics) == fixture and not set(metrics) & native
assert any(a['key'] == 'ai_cli_source' and a['value']['stringValue'] == 'fixture'
           for a in sent['resource']['attributes'])
assert any(a['key'] == 'user.account_uuid' for a in sent['resource']['attributes'])
points = metrics['ai_cli_sample_tokens']['gauge']['dataPoints']
assert {p['attributes'][0]['value']['stringValue']: p['asInt'] for p in points} == {
    'claude': '12', 'opencode': '0', 'copilot': '7', 'pi': '3'}
for name in fixture - {'ai_cli_sample_tokens', 'ai_cli_fixture_run_marker'}:
    point = (metrics[name].get('sum') or metrics[name].get('gauge'))['dataPoints'][0]
    assert set(a['key'] for a in point['attributes']) & {
        'session.id', 'model', 'user.account_uuid', 'file.path', 'prompt'}

# Static semantic-label projection only: this is NOT receiver execution or a
# simulation of its counter-epoch fingerprint. verify_lab.py tests actual output.
# Collector requires canonical valid Pi model IDs; the adapter normalizes invalid input.
def projected(name, attributes, resource=None):
    resource = resource or {}
    if name not in native | fixture or (name in {'claude_code.token.usage', 'ai_cli_fixture_claude_tokens'} and
        attributes.get('type') not in {'input', 'output', 'cacheRead', 'cacheCreation'}):
        return None
    if name in native and resource.get('ai_cli_source') == 'fixture':
        return None
    if name in {'ai_cli_pi_model_tokens', 'ai_cli_pi_model_estimated_cost_usd'}:
        import re
        model = attributes.get('model')
        if not isinstance(model, str) or not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}', model, re.ASCII):
            return None
    if name == 'ai_cli_pi_model_tokens' and attributes.get('type') not in {'input', 'output', 'cache_read', 'cache_write'}:
        return None
    client = ('claude' if name in {'claude_code.token.usage', 'claude_code.cost.usage',
                                  'ai_cli_fixture_claude_tokens', 'ai_cli_fixture_claude_cost_usd'}
              else 'copilot' if name in copilot | {'ai_cli_fixture_copilot_tokens'}
              else 'pi' if name in {'ai_cli_pi_agent_runs', 'ai_cli_pi_tokens', 'ai_cli_pi_estimated_cost_usd', 'ai_cli_pi_model_tokens', 'ai_cli_pi_model_estimated_cost_usd', 'ai_cli_pi_tool_call', 'ai_cli_fixture_pi_marker'}
              else attributes.get('client') if attributes.get('client') in
                   {'claude', 'opencode', 'copilot', 'pi'} else 'unknown')
    result = {'client': client, 'source': 'fixture' if name in fixture else 'native'}
    if name in {'claude_code.token.usage', 'ai_cli_fixture_claude_tokens', 'ai_cli_pi_model_tokens'}:
        result['type'] = attributes['type']
    if name in {'ai_cli_pi_model_tokens', 'ai_cli_pi_model_estimated_cost_usd'}:
        result['model'] = attributes['model']
    return result

private = {'session.id': 'secret', 'model': 'private', 'prompt': 'secret',
           'file.path': '/secret', 'client': 'spoofed', 'source': 'fixture'}
assert projected('claude_code.token.usage', {**private, 'type': 'cacheRead'},
                 {'user.account_uuid': 'secret'}) == {'client': 'claude', 'source': 'native', 'type': 'cacheRead'}
assert projected('claude_code.cost.usage', private) == {'client': 'claude', 'source': 'native'}
assert projected('ai_cli_pi_agent_runs', private) == {'client': 'pi', 'source': 'native'}
assert projected('ai_cli_pi_model_tokens', {**private, 'type': 'input'}) == {'client': 'pi', 'source': 'native', 'type': 'input', 'model': 'private'}
assert projected('ai_cli_pi_model_tokens', {'type': 'input'}) is None
assert projected('ai_cli_pi_model_estimated_cost_usd', {'model': 'custom/provider-v2'}) == {'client': 'pi', 'source': 'native', 'model': 'custom/provider-v2'}
assert '^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$' in config
for value in ('a'*129, '/bad', 'bad value', 'bad\x00value'):
    assert projected('ai_cli_pi_model_estimated_cost_usd', {'model': value}) is None
assert projected('ai_cli_pi_model_estimated_cost_usd', {'model': 'a'*128})['model'] == 'a'*128
assert projected('ai_cli_pi_model_tokens', {'model': 'gpt-5.2', 'type': 'input', 'prompt': 'secret', 'source': 'fixture'}) == {
    'client': 'pi', 'source': 'native', 'type': 'input', 'model': 'gpt-5.2'}
assert projected('ai_cli_pi_model_estimated_cost_usd', {'model': 'unknown', 'file.path': '/secret'}) == {
    'client': 'pi', 'source': 'native', 'model': 'unknown'}
assert projected('ai_cli_pi_model_estimated_cost_usd', {'model': 'unknown'}, {'ai_cli_source': 'fixture'}) is None
for metric in copilot:
    assert projected(metric, private, {'user.account_uuid': 'secret'}) == {'client': 'copilot', 'source': 'native'}
    assert projected(metric, private, {'ai_cli_source': 'fixture'}) is None
    assert f'metric.name == "{metric}"' in config
assert projected('ai_cli_fixture_copilot_tokens', private) == {'client': 'copilot', 'source': 'fixture'}
# Canonical v1 preserves source and type; the shared dashboard uses this schema.
assert 'set(name, "ai_cli_tokens_v1")' in config
assert 'name == "ai_cli_pi_tokens"' in config
assert 'name == "ai_cli_pi_estimated_cost_usd"' in config
assert 'set(name, "ai_cli_estimated_cost_usd_v1")' in config
assert 'set(unit, "{token}") where name == "ai_cli_tokens_v1"' in config
for kind, native_name in [('input', 'input_tokens'), ('output', 'output_tokens'),
                          ('cache_read', 'cache_read.input_tokens'), ('cache_write', 'cache_write.input_tokens')]:
    assert f'set(attributes["type"], "{kind}") where metric.name == "gen_ai.client.inference.usage.{native_name}"' in config
assert 'attributes["type"] == "cacheRead"' in config and 'attributes["type"] == "cacheCreation"' in config
assert 'ai_cli_pi_agent_runs" or name == "ai_cli_fixture_canonical_tokens"' not in config
canonical = metrics['ai_cli_fixture_canonical_tokens']['sum']['dataPoints']
assert {next(a['value']['stringValue'] for a in p['attributes'] if a['key'] == 'client'): p['asDouble'] for p in canonical} == {'claude': 12.0, 'opencode': 0.0, 'copilot': 7.0, 'pi': 3.0}
assert len({p['startTimeUnixNano'] for p in canonical}) == 1
assert projected('gen_ai.client.inference.usage.reasoning.output_tokens', private) is None
assert projected('github.copilot.tool.call.duration', private) is None
assert projected('ai_cli_sample_tokens', {'client': 'opencode', **{k: v for k, v in private.items() if k != 'client'}}) == {'client': 'opencode', 'source': 'fixture'}
assert projected('ai_cli_sample_tokens', private) == {'client': 'unknown', 'source': 'fixture'}
assert projected('ai_cli_fixture_run_marker', {'client': 'pi'}) == {'client': 'pi', 'source': 'fixture'}
assert projected('claude_code.token.usage', {**private, 'type': 'unexpected'}) is None
assert projected('claude_code.api_request', private) is None
assert projected('ai_cli_fixture_claude_tokens', {'type': 'unbounded'}) is None
assert 'metric.name == "ai_cli_fixture_claude_tokens" and attributes["type"] != "input"' in config
assert projected('claude_code.token.usage', {'type': 'input'}, {'ai_cli_source': 'fixture'}) is None
assert projected('ai_cli_pi_agent_runs', {}, {'ai_cli_source': 'fixture'}) is None

dashboard = json.loads((root / 'grafana/dashboards/samples.json').read_text())
assert dashboard['uid'] == 'ai-cli-samples'
selector, = dashboard['templating']['list']
assert selector['name'] == 'agent' and selector['type'] == 'custom'
assert selector['multi'] and selector['includeAll']
assert selector['allValue'] == 'claude|opencode|copilot|pi'
assert selector['current']['value'] == '$__all'
assert all(label in selector['query'] for label in
           ('Claude Code : claude', 'OpenCode (native opt-in; synthetic example separate) : opencode',
            'GitHub Copilot CLI : copilot', 'Pi : pi'))
# Simulate Grafana regex interpolation for All, one and multiple selections;
# every query has the same bounded selector, including freshness probes.
import re
from check_dashboard import all_panels
panels = list(all_panels(dashboard['panels']))
queries = [target['expr'] for panel in panels for target in panel.get('targets', [])]
assert queries and len({panel['id'] for panel in panels}) == len(panels)
assert all('client=~"${agent:regex}"' in expr and 'job="collector"' in expr for expr in queries)
for selected, expected in [(('claude', 'opencode', 'copilot', 'pi'), {'claude', 'opencode', 'copilot', 'pi'}),
                           (('pi',), {'pi'}), (('claude', 'copilot'), {'claude', 'copilot'})]:
    matcher = re.compile('|'.join(selected))
    assert {agent for agent in ('claude', 'opencode', 'copilot', 'pi')
            if matcher.fullmatch(agent)} == expected
assert all(' or ' not in expr for panel in panels if panel['id'] != 19 for expr in (target['expr'] for target in panel.get('targets', [])))
assert next(panel for panel in panels if panel['id'] == 19)['targets'][0]['expr'].count(' or on (client,category) ') == 1
assert all(('source="fixture"' if 'ai_cli_sample_tokens{' in expr or ('ai_cli_tokens_v1_total{' in expr and 'source="fixture"' in expr) else 'source="native"') in expr
           for expr in queries)
assert not any('opencode' in expr or 'source=~' in expr for expr in queries)
assert any('increase(' in expr for expr in queries)
allowed_names = {'ai_cli_sample_tokens', 'ai_cli_tokens_v1_total',
                 'ai_cli_estimated_cost_usd_v1_USD_total', 'ai_cli_pi_tool_call_total',
                 'ai_cli_pi_model_tokens_total', 'ai_cli_pi_model_estimated_cost_usd_USD_total',
                 'ai_cli_pi_child_response_tokens_total', 'ai_cli_pi_child_response_estimated_cost_usd_USD_total',
                 'ai_cli_pi_child_events_total'}
assert not any(p['type'] == 'row' and any(agent in p['title'] for agent in ('Claude', 'Copilot', 'OpenCode', 'Pi |')) for p in panels)
assert not any(p['type'] == 'row' and 'Pi event marker' in p['title'] for p in panels)
assert len([p for p in panels if p['type'] == 'row']) == 5
assert all(next(p for p in panels if p['id'] == i)['targets'][0]['range'] is True
           and next(p for p in panels if p['id'] == i)['targets'][0]['instant'] is False
           for i in (1, 2))
assert 'not billing' in next(p for p in panels if p['id'] == 1)['description']
assert 'cache_read' in next(p for p in panels if p['id'] == 2)['targets'][0]['expr']
assert 'rate(' in next(p for p in panels if p['id'] == 7)['targets'][0]['expr']
assert all('client="pi"' in next(p for p in panels if p['id'] == i)['targets'][0]['expr'] for i in (5, 8))
assert all('claude_code_' not in expr and 'gen_ai_' not in expr for expr in queries)
for expr in queries:
    names = re.findall(r'([a-zA-Z_:][a-zA-Z0-9_:]*)\{', expr)
    assert names and set(names) <= allowed_names, expr
    assert 'source="fixture"' in expr or 'source="native"' in expr
assert all('unavailable' in p['title'].lower() for p in panels if p['id'] in (3, 4, 9, 10, 11, 12, 13, 14, 15, 17))
from install import IMAGES
assert 'image: grafana/grafana:13.3.0' in compose
assert 'grafana/grafana:13.3.0' in IMAGES
assert 'grafana/grafana-oss:' not in compose and not any('grafana-oss:' in image for image in IMAGES)
# Static provisioning checks; Grafana 13 runtime import remains unverified without an image.
provider = (root / 'grafana/dashboards/provider.yaml').read_text()
assert 'apiVersion: 1' in provider and 'type: file' in provider
assert 'path: /etc/grafana/provisioning/dashboards' in provider
assert './grafana:/etc/grafana/provisioning:ro' in compose
assert dashboard['schemaVersion'] == 40 and dashboard['uid'] == 'ai-cli-samples'
assert all(panel.get('datasource', {}).get('uid') == 'local-prometheus'
           for panel in panels if panel.get('targets'))
print('offline policy: filter, supported metrics, synthetic/native split and empty-state guidance passed (static, not Grafana runtime)')
