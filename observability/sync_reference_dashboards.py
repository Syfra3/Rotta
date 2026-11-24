#!/usr/bin/env python3
"""Regenerate complete local dashboard artifacts from pinned pristine references.

Only datasource compatibility, privacy/coverage text and missing-value behavior
are adapted. Charts, layout, expressions (except the Tempo compatibility query),
units and navigation stay reference-owned. No Downloads dependency at runtime.
"""
import copy
import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parent
NAMES = ('claude-code.json', 'claude-code-members.json', 'claude-code-budget.json')
NOTICE = ' Local sanitized metadata; missing data is not zero. Monthly values incomplete beyond seven-day retention.'
SOURCE_DESCRIPTION = ('Local wiring: native supplies usage/cost; transcript supplies sanitized session/tool/context '
                      'metadata without duplicate usage counters. All combines these disjoint signals. '
                      'Synthetic values are test-only. Monthly coverage is incomplete with seven-day metrics retention.')
LEGACY_TRACE = '{ name = "claude_code.interaction" && (span.data_source =~ "$data_source" || (span.data_source = nil && "" =~ "$data_source")) }'
LOCAL_TRACE = '{ name = "claude_code.interaction" && span.data_source =~ "$data_source" }'
BUDGET_WARNING = 'Budget — INCOMPLETE: seven-day data, not full-month totals'


def local_dashboard(reference):
    dashboard = copy.deepcopy(reference)
    def adapt(value):
        if isinstance(value, dict):
            for key, item in list(value.items()):
                if key == 'uid' and item == 'prometheus':
                    value[key] = 'local-prometheus'
                elif key == 'noValue' and item in ('0', 0):
                    value[key] = 'Not observed'
                elif key == 'query' and item == LEGACY_TRACE:
                    value[key] = LOCAL_TRACE.replace('$data_source', '${data_source:raw}')
                elif key == 'expr' and isinstance(item, str):
                    # Native edit counters are authoritative; transcript explicit
                    # decision metadata must not double-increment the same chart.
                    item = item.replace('claude_code_code_edit_tool_decision_total{',
                                        'claude_code_code_edit_tool_decision_total{data_source="native",')
                    if 'cost_usd' in item:
                        item = item.replace('event_name = "api_request"', 'event_name =~ "api_request|api_cost"')
                    value[key] = item.replace('$data_source', '${data_source:raw}')
                else:
                    adapt(item)
            if value.get('id') == 'noValue' and value.get('value') in ('0', 0):
                value['value'] = 'Not observed'
        elif isinstance(value, list):
            for item in value:
                adapt(item)
    adapt(dashboard)
    dashboard['description'] = reference.get('description', '') + ' INCOMPLETE local coverage: see SOURCE_SUPPORT.md and RETENTION.md.'
    source = dashboard['templating']['list'][0]
    assert source['name'] == 'data_source'
    source.update(description=SOURCE_DESCRIPTION,
                  query='transcript : transcript, real-otel : native|^$, all : .*, synthetic : synthetic, transcript-fixture : transcript-fixture',
                  current={'selected': True, 'text': 'all', 'value': '.*'},
                  options=[{'selected': text == 'all', 'text': text, 'value': value}
                           for text, value in [('transcript','transcript'), ('real-otel','native|^$'),
                                               ('all','.*'), ('synthetic','synthetic'),
                                               ('transcript-fixture','transcript-fixture')]])
    uid = dashboard['uid']
    for panel in dashboard['panels']:
        if panel['type'] == 'row':
            if uid == 'claude-code-budget' and panel['id'] == 106:
                panel['title'] = BUDGET_WARNING
            continue
        panel['description'] = panel.get('description', '') + NOTICE
        def context_peak(target, group):
            # A Prometheus gauge exported after a one-shot history scan retains
            # only its last request. Query stored request logs for a real range max.
            profile = ' | label_format user="local-user"' if group == 'user' else ''
            target.update(datasource={'type':'loki','uid':'loki'}, queryType='instant',
                          expr='max by ('+group+') (max_over_time({service_name="claude-code"} | event_name="api_request" | data_source=~"${data_source:raw}" | event_id!="" | context_pct!=""'+profile+' | unwrap context_pct [$__range]))')
        if uid == 'claude-code-telemetry-lab' and panel['id'] == 30:
            panel['datasource'] = {'type':'loki','uid':'loki'}
            context_peak(panel['targets'][0], 'model')
        if uid == 'claude-code-members-sessions' and panel['id'] == 53:
            panel['datasource'] = {'type':'loki','uid':'loki'}
            context_peak(panel['targets'][0], 'user')
        if uid == 'claude-code-members-sessions' and panel['id'] == 54:
            panel['datasource'] = {'type':'mixed','uid':'-- Mixed --'}
            context_peak(next(t for t in panel['targets'] if t['refId'] == 'C'), 'user')
        if uid == 'claude-code-telemetry-lab':
            if panel['id'] in (17, 18):
                panel['description'] += ' Unavailable locally: raw MCP server/tool names are excluded.'
            if panel['id'] in (21, 22):
                panel['description'] += ' Native cost has no branch join: unlabeled spend is unattributed, not branch spend.'
            if panel['id'] in (38, 39):
                panel['description'] += ' Transcript latency is an inferred record interval, not measured provider latency.'
        if uid == 'claude-code-members-sessions' and panel['id'] == 56:
            panel['description'] += ' Native api_cost events supply per-request estimates grouped by pseudonymous session; transcript-only history has no bundled pricing estimator.'
    return dashboard


def counter_window(series):
    current = 'last_over_time('+series+'[$__range])'
    prior = '(last_over_time('+series+'[7d] offset $__range) or '+current+' * 0)'
    return '('+current+' - '+prior+' * ('+current+' >= bool '+prior+'))'


def host_dashboard(reference):
    dashboard = copy.deepcopy(reference)
    for panel in dashboard['panels']:
        for target in panel.get('targets', []):
            target['expr'] = re.sub(r'increase\(([A-Za-z_:][A-Za-z0-9_:]*\{[^{}]*\})\[\$__range\]\)',
                                    lambda match: counter_window(match.group(1)), target['expr'])
    return dashboard


def main():
    for name in NAMES:
        reference = json.loads((ROOT/'reference-dashboards'/name).read_text())
        # Generated artifacts are complete rewrites, never changes to originals.
        (ROOT/'grafana/dashboards'/name).write_text(json.dumps(local_dashboard(reference), indent=2)+'\n')
    host = json.loads((ROOT/'host-companions.source.json').read_text())
    (ROOT/'grafana/dashboards/host-companions.json').write_text(json.dumps(host_dashboard(host), indent=2)+'\n')


if __name__ == '__main__':
    main()
