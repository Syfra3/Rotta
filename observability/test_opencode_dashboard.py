"""Offline checks for the OpenCode metadata dashboard's source contract."""
import json
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parent


class OpenCodeDashboard(unittest.TestCase):
    def test_supported_metrics_and_no_content(self):
        dashboard = json.loads((ROOT / 'grafana/dashboards/opencode.json').read_text())
        self.assertEqual(dashboard['uid'], 'ai-cli-opencode')
        panels = {p['id']: p for p in dashboard['panels']}
        expected = {
            2: 'ai_cli_opencode_model_tokens_total',
            3: 'ai_cli_opencode_model_cost_usd_USD_total',
            4: 'ai_cli_opencode_tool_calls_total',
            5: 'ai_cli_opencode_runs_total',
            6: 'ai_cli_opencode_sessions_total',
            7: 'ai_cli_opencode_compactions_total',
        }
        for panel_id, name in expected.items():
            query = panels[panel_id]['targets'][0]['expr']
            self.assertIn(name, query)
            self.assertIn('client="opencode"', query)
            self.assertIn('source="native"', query)
            self.assertIn('[$__range]', query)
        self.assertIn('not billing', panels[3]['title'])
        self.assertIn('No data is not zero', panels[1]['options']['content'])


if __name__ == '__main__':
    unittest.main()
