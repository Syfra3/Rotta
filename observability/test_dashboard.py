"""Pi-first dashboard contract checks independent of Grafana availability."""
import json
import unittest
from pathlib import Path
from check_dashboard import CheckError, validate_dashboard, all_panels

ROOT = Path(__file__).parent

class DashboardTests(unittest.TestCase):
    def setUp(self):
        self.dashboard = json.loads((ROOT / 'grafana/dashboards/samples.json').read_text())
        self.panels = list(all_panels(self.dashboard['panels']))

    def test_layout_and_supported_queries(self):
        validate_dashboard(self.dashboard)
        self.assertEqual(len(self.panels), 24)
        for panel in self.panels:
            if panel['id'] in (1, 2, 5, 6, 7, 8, 16, 18, 19):
                self.assertIn('source="native"', panel['targets'][0]['expr'])
            elif panel['type'] not in ('row', 'text'):
                self.assertIn('unavailable', panel['title'].lower())
                self.assertFalse(panel.get('targets'))

    def test_pi_overview_and_collapsed_limits(self):
        visible = self.dashboard['panels']
        self.assertEqual([p['id'] for p in visible], [100, 1, 5, 6, 8, 104, 16, 19, 18, 101, 102, 103])
        for row_id in (101, 102, 103):
            row = next(p for p in visible if p['id'] == row_id)
            self.assertTrue(row['collapsed'])
            self.assertTrue(row['panels'])
        for panel in self.panels:
            if panel['type'] != 'row' and not panel.get('targets'):
                self.assertNotIn(panel, visible)
                self.assertIn('unavailable', panel['title'].lower())
                self.assertTrue(panel.get('description'))
        self.assertEqual(next(p for p in self.panels if p['id'] == 1)['options']['colorMode'], 'value')
        self.assertEqual(next(p for p in self.panels if p['id'] == 8)['options']['legend']['placement'], 'right')
        for panel in visible:
            pos = panel['gridPos']
            self.assertLessEqual(pos['x'] + pos['w'], 24)
            for other in visible:
                if panel is other:
                    continue
                b = other['gridPos']
                self.assertFalse(pos['x'] < b['x'] + b['w'] and b['x'] < pos['x'] + pos['w'] and pos['y'] < b['y'] + b['h'] and b['y'] < pos['y'] + pos['h'])

    def test_completed_tool_failure_share(self):
        panel = next(p for p in self.panels if p['id'] == 19)
        expr = panel['targets'][0]['expr']
        self.assertEqual(expr.count('increase(ai_cli_pi_tool_call_total'), 3)
        self.assertIn('or on (client,category) (0 * sum by (client,category)', expr)
        self.assertIn(' / sum by (client,category)', expr)
        self.assertIn('status="error"', expr)
        self.assertEqual(panel['fieldConfig']['defaults']['unit'], 'percent')
        self.assertNotIn('unavailable', panel['title'].lower())

    def test_model_charts(self):
        panels = {panel['id']: panel for panel in self.panels}
        for panel_id, metric, unit, grouping, position in (
            (5, 'ai_cli_pi_model_estimated_cost_usd_USD_total', 'currencyUSD', 'model', (6, 1)),
            (8, 'ai_cli_pi_model_tokens_total', 'short', 'model,type', (0, 9)),
        ):
            panel = panels[panel_id]
            expr = panel['targets'][0]['expr']
            self.assertEqual(panel['type'], 'timeseries')
            self.assertEqual((panel['gridPos']['x'], panel['gridPos']['y']), position)
            self.assertEqual(panel['fieldConfig']['defaults']['unit'], unit)
            self.assertIn(metric, expr)
            self.assertIn('sum by (' + grouping + (',query_source' if panel_id == 8 else '') + ')', expr)
            for term in ('source="native"', 'client="pi"', 'client=~"${agent:regex}"', '[$__range]'):
                self.assertIn(term, expr)
            self.assertIn('child', panel['description'].lower())
            self.assertTrue(panel['targets'][0]['legendFormat'])
        self.assertIn('ai_cli_tokens_v1_total', panels[7]['targets'][0]['expr'])
        self.assertEqual(len(panels[6]['targets']), 2)
        self.assertIn('query_source', panels[6]['targets'][0]['expr'])
        self.assertIn('ai_cli_pi_child_response_estimated_cost_usd_USD_total', panels[6]['targets'][1]['expr'])
        self.assertIn('ai_cli_pi_child_response_tokens_total', panels[8]['targets'][1]['expr'])

    def test_cost_and_cache_stat_presentation(self):
        panels = {panel['id']: panel for panel in self.panels}
        for panel_id, unit in ((1, 'currencyUSD'), (2, 'percent')):
            panel = panels[panel_id]
            self.assertEqual(panel['type'], 'stat')
            self.assertEqual(panel['fieldConfig']['defaults']['unit'], unit)
            self.assertEqual(panel['fieldConfig']['defaults']['color']['fixedColor'], 'blue')
            self.assertEqual(panel['options']['graphMode'], 'none')
            self.assertEqual(panel['options']['reduceOptions']['calcs'], ['lastNotNull'])
            self.assertTrue(panel['targets'][0]['range'])
            self.assertFalse(panel['targets'][0]['instant'])
            self.assertIn('[$__range]', panel['targets'][0]['expr'])
            self.assertIn('client=~"${agent:regex}"', panel['targets'][0]['expr'])
        self.assertEqual(panels[2]['fieldConfig']['defaults']['min'], 0)
        self.assertEqual(panels[2]['fieldConfig']['defaults']['max'], 100)
        self.assertTrue(panels[2]['targets'][0]['expr'].startswith('100 * '))
        self.assertIn('increase(ai_cli_estimated_cost_usd_v1_USD_total', panels[1]['targets'][0]['expr'])

    def test_rejects_broken_query_even_when_native_empty(self):
        next(p for p in self.panels if p['id'] == 1)['targets'][0]['expr'] = 'missing_metric{}'
        with self.assertRaises(CheckError):
            validate_dashboard(self.dashboard)

    def test_mcp_rate_query_is_native_and_agent_filtered(self):
        panel = next(p for p in self.panels if p['id'] == 18)
        expr = panel['targets'][0]['expr']
        self.assertIn('rate(ai_cli_pi_tool_call_total{', expr)
        self.assertIn('category="mcp"', expr)
        self.assertIn('client=~"${agent:regex}"', expr)
        self.assertNotIn('unavailable', panel['title'].lower())
        server = next(p for p in self.panels if p['id'] == 17)
        self.assertFalse(server.get('targets'))
        self.assertIn('unavailable', server['title'].lower())
        panel['targets'][0]['expr'] = 'sum(rate(ai_cli_pi_tool_call_total{category="tool"}[5m]))'
        with self.assertRaises(CheckError):
            validate_dashboard(self.dashboard)

    def test_rejects_unmarked_unsupported_panel(self):
        next(p for p in self.panels if p['id'] == 3)['title'] = 'Cost per edit'
        with self.assertRaises(CheckError):
            validate_dashboard(self.dashboard)

if __name__ == '__main__':
    unittest.main()
