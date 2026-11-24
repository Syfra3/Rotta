"""Offline source-boundary layout/privacy checks; not a Grafana or Pi runtime smoke."""
import json
import subprocess
from pathlib import Path

root = Path(__file__).parent
ours = json.loads((root / 'grafana/dashboards/samples.json').read_text())
from check_dashboard import all_panels
panels = list(all_panels(ours['panels']))
assert ours['uid'] == 'ai-cli-samples'
assert len(panels) == 24 and len({p['id'] for p in panels}) == 24
assert all('unavailable' in p['title'].lower() for p in panels if p['id'] in (3, 4, 9, 10, 11, 12, 13, 14, 15, 17))
# Historical reference layout intentionally differs from the Pi-first presentation.
print('legacy positional layout superseded; IDs and unavailable signals retained')
reliability = next(p for p in ours['panels'] if p['id'] == 19)
expr = reliability['targets'][0]['expr']
assert 'status="error"' in expr and expr.count('increase(ai_cli_pi_tool_call_total') == 3 and 'or on (client,category) (0 *' in expr
assert 'source="native"' in expr and 'client=~"${agent:regex}"' in expr
assert reliability['fieldConfig']['defaults']['unit'] == 'percent'
selector, = ours['templating']['list']
assert selector['multi'] and selector['includeAll'] and selector['allValue'] == 'claude|opencode|copilot|pi'
source = (root / 'pi-telemetry.ts').read_text()
assert 'JSON.stringify(event)' not in source and 'sessionManager' not in source
assert 'message_end' in source and 'tool_execution_end' in source
subprocess.run(['node', str(root / 'test_pi_events.cjs')], check=True)
print('fake Pi events and outgoing payload verified (not latest-lab AC1)')
