import importlib.util
from pathlib import Path
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('consumer_probe', Path(__file__).with_name('verify_lab.py'))
p = importlib.util.module_from_spec(spec)
spec.loader.exec_module(p)


class Harness(unittest.TestCase):
    def test_template_tokens_do_not_replace_prefixes(self):
        dashboard = {'templating':{'list':[{'name':'topn','current':{'value':'10'}},
                                         {'name':'budget','current':{'value':'1200'}},
                                         {'name':'data_source','current':{'value':'.*'}}]}}
        self.assertEqual(p.expand('$__range_s / $budget; $topn; $__range; ${data_source:regex}', dashboard),
                         '3600 / 1200; 10; 1h; .*')
        with self.assertRaisesRegex(AssertionError, 'unresolved'):
            p.expand('$unknown', dashboard)

    def test_range_queries_use_real_range_not_empty_shifted_instant(self):
        with patch.object(p, 'fetch', return_value={'status':200,'body':'{"status":"success","data":{"result":[]}}'}) as fetch:
            panel = {'id':1,'datasource':{'type':'prometheus'}}
            result = p.query_target('owned-node', panel, {'expr':'metric','range':True,'instant':False}, {})
            self.assertIn('/api/v1/query_range?', fetch.call_args.args[1])
            self.assertEqual(result['status'], 'empty')
            p.query_target('owned-node', panel, {'expr':'metric','range':False,'instant':True}, {})
            self.assertIn('/api/v1/query?', fetch.call_args.args[1])


if __name__ == '__main__':
    unittest.main()
