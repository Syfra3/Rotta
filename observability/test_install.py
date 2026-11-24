"""Hermetic installer checks; fake binaries never contact a daemon or provider."""
import json
import socket
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest import mock
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import install
import fixture

SCRIPT = Path(__file__).resolve().parent / 'install.py'
EXT = str(SCRIPT.parent / 'pi-telemetry.ts')


class Installer(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.home = Path(self.temp.name)
        self.bin = self.home / 'fakebin'
        self.bin.mkdir()
        for name, body in [('docker', '#!/bin/sh\nif [ "$1 $2" = "compose -f" ]; then\n case "$*" in *"config --format json"*) echo \'{"name":"rotta-ai-telemetry"}\';; esac; fi\ncase "$*" in *"up -d"*) [ -z "$FAIL_UP" ];; esac\n'), ('pi', '#!/bin/sh\nexit 0\n')]:
            path = self.bin / name
            path.write_text(body)
            path.chmod(0o700)
        self.env = dict(os.environ, HOME=str(self.home), PATH=str(self.bin) + ':' + os.environ['PATH'],
                        XDG_CONFIG_HOME=str(self.home / '.config'), XDG_DATA_HOME=str(self.home / '.local/share'),
                        XDG_CACHE_HOME=str(self.home / '.cache'), XDG_STATE_HOME=str(self.home / '.local/state'),
                        XDG_RUNTIME_DIR=str(self.home / 'runtime'))
        self.settings = self.home / '.pi/agent/settings.json'
        self.launcher = self.home / '.local/bin/rotta-telemetry'
        self.log = self.home / 'calls'
        self.env['CALL_LOG'] = str(self.log)
        docker = self.bin / 'docker'
        docker.write_text('#!/bin/sh\nprintf "%s\\n" "$*" >> "$CALL_LOG"\ncase "$*" in *"config --format json"*) echo \'{"name":"rotta-ai-telemetry"}\';; esac\ncase "$*" in *"image inspect"*) [ -z "$FAIL_IMAGE" ] || exit 1;; esac\ncase "$*" in *"up -d"*) [ -z "$FAIL_UP" ] || exit 1;; esac\ncase "$*" in *"down"*) [ -z "$FAIL_DOWN" ] || exit 1;; esac\n')
        docker.write_text('''#!/bin/sh
printf "%s\\n" "$*" >> "$CALL_LOG"
case "$*" in
 *"config --format json"*) echo '{"name":"rotta-ai-telemetry"}';;
 *"image inspect"*) [ -z "$FAIL_IMAGE" ] || exit 1;;
 *"ps -a --filter label=com.docker.compose.project=rotta-ai-telemetry --filter label=com.docker.compose.service=prometheus"*)
   if [ -z "$FAKE_PROMETHEUS_MISSING" ]; then echo prometheus-id; fi;;
 *"ps --filter publish="*)
   if [ -n "$FAKE_STOPPED" ]; then exit 0; fi
   if [ -n "$FAKE_AMBIGUOUS" ]; then printf 'other-id\\n'; fi
   case "$*" in *"publish=13000"*) echo grafana-id;; *"publish=14318"*) echo collector-id;; esac;;
 *"inspect grafana-id"*|*"inspect collector-id"*|*"inspect prometheus-id"*)
   case "$*" in *"grafana-id"*) service=grafana; internal=3000; port=13000;;
                    *"collector-id"*) service=collector; internal=4318; port=14318;;
                    *) service=prometheus; internal=9090; port=0;; esac
   SERVICE="$service" INTERNAL="$internal" PORT="$port" python3 -c 'import json,os; from pathlib import Path; root=Path(os.environ["COMPOSE_PATH"]); service=os.environ["SERVICE"]; port=os.environ["PORT"]; labels={"com.docker.compose.project":os.environ.get("FAKE_PROJECT","rotta-ai-telemetry"),"com.docker.compose.service":service,"com.docker.compose.project.working_dir":str(root.parent),"com.docker.compose.project.config_files":str(root)}; print(json.dumps([{"Config":{"Labels":labels},"State":{"Running":not (service=="prometheus" and bool(os.environ.get("FAKE_PROMETHEUS_STOPPED")))},"NetworkSettings":{"Ports":{os.environ["INTERNAL"]+"/tcp":[{"HostIp":"127.0.0.1","HostPort":os.environ.get("FAKE_PORT",port)}]}}}]))';;
 *"up -d"*) [ -z "$FAIL_UP" ] || exit 1;;
 *"down"*) [ -z "$FAIL_DOWN" ] || exit 1;;
esac
''')
        docker.chmod(0o700)
        self.env['COMPOSE_PATH'] = str(SCRIPT.parent / 'compose.yaml')
        pi = self.bin / 'pi'
        pi.write_text('#!/bin/sh\nprintf "pi %s\\n" "$*" >> "$CALL_LOG"\n')
        pi.chmod(0o700)

    def call(self, *args, ok=True):
        code = ('import socket,runpy,sys,os; original=socket.socket.bind; '
                'defect=lambda sock,address: (_ for _ in ()).throw(OSError("occupied")) '
                'if address[0]=="127.0.0.1" and str(address[1]) in os.environ.get("OCCUPIED_PORTS", "").split(",") '
                'else None if address[0]=="127.0.0.1" and address[1] in (13000,14318) '
                'else original(sock,address); socket.socket.bind=defect; '
                'sys.argv=[os.environ["INSTALL_SCRIPT"],*sys.argv[1:]]; runpy.run_path(sys.argv[0],run_name="__main__")')
        result = subprocess.run([sys.executable, '-c', code, *args], env=dict(self.env, INSTALL_SCRIPT=str(SCRIPT)), capture_output=True, text=True)
        self.assertEqual(result.returncode == 0, ok, result.stderr)
        return result

    def test_opencode_lifecycle_conflicts_and_failure(self):
        plugin = self.home / '.config/opencode/plugins/rotta-telemetry/server.js'
        other = plugin.parent.parent / 'other.mjs'
        other.parent.mkdir(parents=True)
        other.write_text('unrelated')
        self.settings.parent.mkdir(parents=True)
        self.settings.write_bytes(b'{"theme":"keep"}\n')
        original = self.settings.read_bytes()
        self.env['OCCUPIED_PORTS'] = '13000,14318'
        self.assertIn('not installed', self.call('status', '--host', 'opencode').stdout)
        self.call('install', '--host', 'opencode')
        self.assertIn('OpenCode telemetry: installed', self.call('status', '--host', 'opencode').stdout)
        content = plugin.read_bytes()
        self.assertEqual(content, (SCRIPT.parent / 'opencode-telemetry.mjs').read_bytes())
        self.call('install', '--host', 'opencode')
        self.assertEqual(plugin.read_bytes(), content)
        self.assertEqual(self.settings.read_bytes(), original)
        self.assertFalse((self.settings.parent / '.rotta-telemetry-owned').exists())
        plugin.write_text('modified')
        self.call('uninstall', '--host', 'opencode', ok=False)
        self.call('install', '--host', 'opencode', ok=False)
        self.assertEqual(plugin.read_text(), 'modified')
        plugin.write_bytes(content)
        self.log.write_text('')
        self.call('uninstall', '--host', 'opencode')
        self.call('uninstall', '--host', 'opencode')
        self.assertFalse(plugin.exists())
        self.assertEqual(other.read_text(), 'unrelated')
        self.assertEqual(self.settings.read_bytes(), original)
        self.assertNotIn('down', self.log.read_text())
        plugin.parent.mkdir()
        plugin.write_text('foreign')
        self.call('install', '--host', 'opencode', ok=False)
        self.assertEqual(plugin.read_text(), 'foreign')
        plugin.unlink()
        plugin.parent.rmdir()
        self.env['FAKE_PROMETHEUS_STOPPED'] = '1'
        self.call('install', '--host', 'opencode', ok=False)
        self.assertFalse(plugin.exists())
        self.assertNotIn('down', self.log.read_text())
        self.assertEqual(self.settings.read_bytes(), original)

    def test_opencode_owned_plugin_upgrade_and_modified_conflict(self):
        root = self.home / 'source'
        source = root / 'observability/opencode-telemetry.mjs'
        source.parent.mkdir(parents=True)
        source.write_bytes(b'new plugin')
        config = self.home / 'config/opencode'
        plugin = config / 'plugins/rotta-telemetry/server.js'
        plugin.parent.mkdir(parents=True)
        plugin.write_bytes(b'old plugin')
        receipt = config / '.rotta-telemetry-owned.json'
        import hashlib
        receipt.write_text(json.dumps({'sha256': hashlib.sha256(b'old plugin').hexdigest()}))
        with mock.patch.object(install, 'ROOT', root), mock.patch.object(install, 'existing_stack_on_ports', return_value=True):
            install.opencode_integration('install', config)
            self.assertEqual(plugin.read_bytes(), b'new plugin')
            self.assertEqual(json.loads(receipt.read_text())['sha256'], hashlib.sha256(b'new plugin').hexdigest())
            self.assertFalse((config / '.rotta-telemetry-journal').exists())
            self.assertFalse((config / '.rotta-telemetry-plugin.bak').exists())
            plugin.write_bytes(b'user edit')
            with self.assertRaisesRegex(ValueError, 'modified'):
                install.opencode_integration('install', config)
            self.assertEqual(plugin.read_bytes(), b'user edit')

    def test_opencode_upgrade_failure_preserves_recovery_evidence(self):
        import hashlib
        root = self.home / 'source'
        source = root / 'observability/opencode-telemetry.mjs'
        source.parent.mkdir(parents=True)
        source.write_bytes(b'new plugin')
        config = self.home / 'config/opencode'
        plugin = config / 'plugins/rotta-telemetry/server.js'
        plugin.parent.mkdir(parents=True)
        plugin.write_bytes(b'old plugin')
        receipt = config / '.rotta-telemetry-owned.json'
        old_receipt = json.dumps({'sha256': hashlib.sha256(b'old plugin').hexdigest()}).encode()
        receipt.write_bytes(old_receipt)
        real_atomic = install.atomic
        failed = False
        def flaky(path, data, mode):
            nonlocal failed
            if path == receipt and not failed:
                failed = True
                raise OSError('injected receipt write failure')
            return real_atomic(path, data, mode)
        with mock.patch.object(install, 'ROOT', root), mock.patch.object(install, 'existing_stack_on_ports', return_value=True), mock.patch.object(install, 'atomic', side_effect=flaky):
            with self.assertRaisesRegex(OSError, 'injected'):
                install.opencode_integration('install', config)
            self.assertEqual(plugin.read_bytes(), b'old plugin')
            self.assertEqual(receipt.read_bytes(), old_receipt)
            self.assertEqual((config / '.rotta-telemetry-plugin.bak').read_bytes(), b'old plugin')
            self.assertTrue((config / '.rotta-telemetry-journal').exists())
            with self.assertRaisesRegex(ValueError, 'interrupted'):
                install.opencode_integration('install', config)
    def test_opencode_external_xdg_lifecycle_and_pi_preservation(self):
        with tempfile.TemporaryDirectory(prefix='rotta xdg outside home ') as external:
            self.env['XDG_CONFIG_HOME'] = external
            root = Path(external) / 'opencode'
            plugin = root / 'plugins/rotta-telemetry/server.js'
            sentinel = self.home / '.config/opencode/sentinel'
            sentinel.parent.mkdir(parents=True)
            sentinel.write_text('unchanged')
            self.settings.parent.mkdir(parents=True)
            self.settings.write_text('{"theme":"keep"}\n')
            settings = self.settings.read_bytes()
            self.env['OCCUPIED_PORTS'] = '13000,14318'
            self.call('install', '--host', 'opencode')
            payload = plugin.read_bytes()
            self.assertEqual(payload, (SCRIPT.parent / 'opencode-telemetry.mjs').read_bytes())
            self.assertIn('installed', self.call('status', '--host', 'opencode').stdout)
            self.call('install', '--host', 'opencode')
            self.assertEqual(plugin.read_bytes(), payload)
            plugin.write_text('conflict')
            self.assertIn('ownership conflict', self.call('status', '--host', 'opencode').stdout)
            self.call('uninstall', '--host', 'opencode', ok=False)
            plugin.write_bytes(payload)
            self.assertEqual(self.settings.read_bytes(), settings)
            self.call('install')
            self.log.write_text('')
            self.call('uninstall')
            self.assertNotIn(' down', self.log.read_text())
            self.assertTrue(plugin.exists())
            self.call('uninstall', '--host', 'opencode')
            self.call('uninstall', '--host', 'opencode')
            self.assertFalse(plugin.exists())
            self.assertEqual(sentinel.read_text(), 'unchanged')
            self.assertFalse((sentinel.parent / '.rotta-telemetry-owned.json').exists())

    def test_opencode_external_xdg_legacy_and_journal_conflicts(self):
        with tempfile.TemporaryDirectory(prefix='rotta xdg ') as external:
            self.env['XDG_CONFIG_HOME'] = external
            root = Path(external) / 'opencode'
            plugins = root / 'plugins'
            plugins.mkdir(parents=True)
            legacy = plugins / 'opencode-telemetry.mjs'
            legacy.write_text('legacy')
            self.assertIn('ownership conflict', self.call('status', '--host', 'opencode').stdout)
            self.call('install', '--host', 'opencode', ok=False)
            self.assertEqual(legacy.read_text(), 'legacy')
            legacy.unlink()
            journal = root / '.rotta-telemetry-journal'
            journal.write_text('interrupted')
            self.assertIn('interrupted', self.call('status', '--host', 'opencode').stdout)
            self.call('uninstall', '--host', 'opencode', ok=False)
            self.assertEqual(journal.read_text(), 'interrupted')

    def test_opencode_relative_xdg_rejected_before_mutation(self):
        self.env['XDG_CONFIG_HOME'] = 'relative config'
        for action in ('install', 'uninstall', 'status'):
            self.call(action, '--host', 'opencode', ok=False)
        self.call('uninstall', ok=False)
        self.assertFalse(self.log.exists())
        self.assertFalse((self.home / '.config').exists())
        self.assertFalse(self.settings.exists())

    def test_opencode_empty_xdg_uses_default(self):
        self.env['XDG_CONFIG_HOME'] = ''
        self.env['OCCUPIED_PORTS'] = '13000,14318'
        self.call('install', '--host', 'opencode')
        self.assertTrue((self.home / '.config/opencode/plugins/rotta-telemetry/server.js').exists())
        self.call('uninstall', '--host', 'opencode')

    def test_opencode_directory_ownership_and_legacy_conflicts(self):
        root = self.home / '.config/opencode/plugins'
        root.mkdir(parents=True)
        legacy = root / 'opencode-telemetry.mjs'
        legacy.write_text('legacy')
        self.call('install', '--host', 'opencode', ok=False)
        self.assertEqual(legacy.read_text(), 'legacy')
        legacy.unlink()
        self.env['OCCUPIED_PORTS'] = '13000,14318'
        self.call('install', '--host', 'opencode')
        extra = root / 'rotta-telemetry/foreign.js'
        extra.write_text('foreign')
        self.assertIn('ownership conflict', self.call('status', '--host', 'opencode').stdout)
        self.call('uninstall', '--host', 'opencode', ok=False)
        self.call('install', '--host', 'opencode', ok=False)
        self.assertEqual(extra.read_text(), 'foreign')
        extra.unlink()
        self.call('uninstall', '--host', 'opencode')
        self.assertFalse((root / 'rotta-telemetry').exists())

    def test_pi_uninstall_keeps_stack_for_opencode(self):
        self.call('install')
        self.env['OCCUPIED_PORTS'] = '13000,14318'
        self.call('install', '--host', 'opencode')
        self.log.write_text('')
        self.call('uninstall')
        self.assertNotIn(' down', self.log.read_text())
        self.assertTrue((self.home / '.config/opencode/plugins/rotta-telemetry/server.js').exists())
        self.call('uninstall', '--host', 'opencode')

    def test_opencode_write_failure_preserves_existing_stack(self):
        original_atomic = install.atomic
        def fail_receipt(path, data, mode):
            if path.name == '.rotta-telemetry-owned.json':
                raise PermissionError('injected')
            return original_atomic(path, data, mode)
        class Occupied:
            def __enter__(self): return self
            def __exit__(self, *args): pass
            def bind(self, address): raise OSError('occupied')
        with mock.patch.object(install, 'atomic', side_effect=fail_receipt), \
             mock.patch.object(install.shutil, 'which', return_value='/fake/docker'), \
             mock.patch.object(install, 'compose', return_value=type('Result', (), {'stdout': '{"name":"rotta-ai-telemetry"}'})()) as compose, \
             mock.patch.object(install, 'run'), \
             mock.patch.object(install.socket, 'socket', return_value=Occupied()), \
             mock.patch.object(install, 'existing_stack_on_ports', return_value=True):
            with self.assertRaises(PermissionError):
                install.opencode_integration('install', self.home / '.config/opencode')
            self.assertEqual([c.args for c in compose.call_args_list], [('config', '--format', 'json')])
        self.assertFalse((self.home / '.config/opencode/plugins/rotta-telemetry/server.js').exists())
        self.assertTrue((self.home / '.config/opencode/.rotta-telemetry-journal').exists())
        self.assertFalse(self.settings.parent.exists())

    def test_opencode_failure_after_start_keeps_stack(self):
        self.env['FAKE_STOPPED'] = '1'
        self.call('install', '--host', 'opencode', ok=False)
        self.assertFalse((self.home / '.config/opencode/plugins/rotta-telemetry/server.js').exists())
        self.assertTrue((self.home / '.config/opencode/.rotta-telemetry-journal').exists())
        self.assertFalse(self.settings.parent.exists())
        self.assertNotIn(' down', self.log.read_text())

    def test_dashboard_url_only_after_success(self):
        url = install.DASHBOARD_URL
        self.assertIn(url, self.call('up').stdout)
        self.assertIn(url, self.call('install').stdout)
        self.assertIn('already installed', self.call('install').stdout)
        self.assertIn(url, self.call('status').stdout)
        self.env['FAKE_STOPPED'] = '1'
        self.assertNotIn(url, self.call('up').stdout)
        self.assertNotIn(url, self.call('status').stdout)
        del self.env['FAKE_STOPPED']
        self.env['FAKE_PROJECT'] = 'foreign'
        self.assertNotIn(url, self.call('status').stdout)
        del self.env['FAKE_PROJECT']
        for action in ('down', 'uninstall'):
            self.assertNotIn(url, self.call(action).stdout)
        for action in ('up', 'install', 'status', 'smoke'):
            self.assertNotIn(url, self.call(action, '--dry-run').stdout)
        self.env['FAIL_UP'] = '1'
        self.assertNotIn(url, self.call('up', ok=False).stdout)
        self.assertNotIn(url, self.call('install', ok=False).stdout)

    def test_smoke_url_requires_verified_query(self):
        url = install.DASHBOARD_URL
        class Response:
            status = 200
            def __enter__(self):
                return self
            def __exit__(self, *args):
                pass
        with mock.patch.object(sys, 'argv', ['install.py', 'smoke']), \
             mock.patch.object(install.Path, 'home', return_value=self.home), \
             mock.patch.object(install, 'run', return_value=type('Result', (), {'stdout': 'fixture marker 123\n'})()), \
             mock.patch.object(install.LOCAL_HTTP, 'open', return_value=Response()), \
             mock.patch.object(install, 'poll_dashboard') as query, \
             mock.patch.object(install, 'existing_stack_on_ports', return_value=True) as stack, \
             mock.patch('builtins.print') as output:
            install.main()
            output.assert_called_once_with(url)
            output.reset_mock()
            stack.side_effect = ValueError('foreign container')
            install.main()
            output.assert_not_called()
            stack.side_effect = None
            output.reset_mock()
            query.side_effect = ValueError('fixture absent')
            with self.assertRaisesRegex(ValueError, 'fixture absent'):
                install.main()
            output.assert_not_called()

    def test_unverified_stack_is_partial_not_installed(self):
        self.env['FAKE_STOPPED'] = '1'
        result = self.call('install', ok=False)
        self.assertNotIn('installed;', result.stdout)
        self.assertFalse(self.settings.exists())
        self.assertTrue((self.settings.parent / '.rotta-telemetry-journal').exists())
        self.call('install', ok=False)

    def test_prometheus_required_for_success_and_owned_repeat(self):
        for failure in ('FAKE_PROMETHEUS_MISSING', 'FAKE_PROMETHEUS_STOPPED'):
            self.env[failure] = '1'
            self.assertNotIn(install.DASHBOARD_URL, self.call('status').stdout)
            self.assertNotIn(install.DASHBOARD_URL, self.call('up').stdout)
            result = self.call('install', ok=False)
            self.assertNotIn('installed;', result.stdout)
            self.assertFalse(self.settings.exists())
            self.assertTrue((self.settings.parent / '.rotta-telemetry-journal').exists())
            (self.settings.parent / '.rotta-telemetry-journal').unlink()
            (self.settings.parent / '.rotta-telemetry-settings.bak').unlink()
            del self.env[failure]
        self.call('install')
        self.env['FAKE_PROMETHEUS_STOPPED'] = '1'
        self.assertNotIn('already installed', self.call('install', ok=False).stdout)

    def test_foreign_runtime_after_up_is_partial(self):
        self.env['FAKE_PROJECT'] = 'foreign'
        self.assertNotIn(install.DASHBOARD_URL, self.call('up').stdout)
        result = self.call('install', ok=False)
        self.assertNotIn('installed;', result.stdout)
        self.assertFalse(self.settings.exists())

    def test_owned_install_requires_current_stack_identity(self):
        self.call('install')
        self.env['FAKE_PROJECT'] = 'foreign'
        result = self.call('install', ok=False)
        self.assertNotIn('already installed', result.stdout)
        self.assertTrue(self.settings.exists())

    def test_marker_only_and_modified_owned_launcher(self):
        self.launcher.parent.mkdir(parents=True)
        foreign = '#!/bin/sh\n' + install.MARK + '\nexec /foreign "$@"\n'
        self.launcher.write_text(foreign)
        self.call('install', ok=False)
        self.assertEqual(self.launcher.read_text(), foreign)
        self.assertFalse(self.settings.exists())
        self.launcher.unlink()
        self.call('install')
        self.launcher.write_text(foreign)
        self.log.write_text('')
        for action in ('install', 'uninstall'):
            self.call(action, ok=False)
        self.assertEqual(self.launcher.read_text(), foreign)
        self.assertEqual(self.log.read_text(), '')

    def test_fixture_send_local_only(self):
        received = []
        state = {'status': 200}

        class Handler(BaseHTTPRequestHandler):
            def do_POST(self):
                received.append((self.path, json.loads(self.rfile.read(int(self.headers['Content-Length'])))))
                self.send_response(state['status'])
                if state['status'] != 200:
                    self.send_header('Location', 'http://example.invalid/private')
                self.end_headers()

            def log_message(self, *args):
                pass

        server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        self.addCleanup(thread.join)
        self.addCleanup(server.server_close)
        self.addCleanup(server.shutdown)
        script = Path(fixture.__file__).resolve()
        for status in (200, 307, 308):
            state['status'] = status
            env = dict(self.env, HTTP_PROXY='http://127.0.0.1:1', HTTPS_PROXY='http://127.0.0.1:1',
                       ALL_PROXY='http://127.0.0.1:1', NO_PROXY='')
            # Run the actual CLI entry point with its collector address redirected only in-process.
            result = subprocess.run([sys.executable, '-c',
                'import runpy, urllib.request; original = urllib.request.Request; '
                'urllib.request.Request = lambda url, *a, **kw: original(' + repr(f'http://127.0.0.1:{server.server_port}/v1/metrics') + ', *a, **kw); '
                'import sys; sys.argv = [' + repr(str(script)) + ', "--send"]; runpy.run_path(' + repr(str(script)) + ', run_name="__main__")'],
                env=env, capture_output=True, text=True, timeout=5)
            self.assertEqual(result.returncode == 0, status == 200, result.stderr)
            self.assertEqual('collector HTTP 200' in result.stdout, status == 200)
            if status != 200:
                self.assertIn('redirect refused', result.stderr)
        self.assertEqual([path for path, _ in received], ['/v1/metrics'] * 3)
        self.assertTrue(all(item['resourceMetrics'] for _, item in received))

    def test_smoke_query_empty_valid_and_private(self):
        expected = {'claude': 12, 'opencode': 0, 'copilot': 7, 'pi': 3}
        state = {'samples': []}
        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                if state.get('redirect'):
                    self.send_response(302)
                    self.send_header('Location', 'http://example.invalid/private')
                    self.end_headers()
                    return
                self.send_response(200)
                self.send_header('Content-Type', 'application/json')
                self.end_headers()
                self.wfile.write(json.dumps({'status': 'success', 'data': {'resultType': 'vector', 'result': state['samples']}}).encode())
            def log_message(self, *args):
                pass
        server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        self.addCleanup(thread.join)
        self.addCleanup(server.server_close)
        self.addCleanup(server.shutdown)
        local_url = f'http://127.0.0.1:{server.server_port}/'
        opener = install.urllib.request.build_opener(install.urllib.request.ProxyHandler({}), install.RejectRedirects())
        original = opener.open
        with mock.patch.object(install, 'LOCAL_HTTP', opener):
            with mock.patch.object(opener, 'open', side_effect=lambda url, **kwargs: original(local_url, **kwargs)):
                with self.assertRaisesRegex(ValueError, 'absent'):
                    install.verify_fixture_query(timeout=0.05, interval=0.01)
                state['samples'] = [{'metric': {'__name__': 'ai_cli_sample_tokens', 'source': 'fixture', 'client': k, 'job': 'collector', 'instance': 'collector:8889'}, 'value': [1, str(v)]} for k, v in expected.items()]
                install.verify_fixture_query(timeout=1)
                state['samples'][0]['metric']['session.id'] = 'private'
                with self.assertRaisesRegex(ValueError, 'private'):
                    install.verify_fixture_query(timeout=0.05)
                del state['samples'][0]['metric']['session.id']
                del state['samples'][0]['metric']['job']
                with self.assertRaisesRegex(ValueError, 'private'):
                    install.verify_fixture_query(timeout=0.05)
                state['redirect'] = True
                with mock.patch.object(install.urllib.request, 'urlopen', side_effect=AssertionError('external request')):
                    with self.assertRaisesRegex(ValueError, 'redirect refused'):
                        install.verify_fixture_query(timeout=0.05)

    def test_redirect_rejected_before_external_request(self):
        requests = []
        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                requests.append(self.path)
                self.send_response(302)
                self.send_header('Location', 'http://example.invalid/private')
                self.end_headers()
            def log_message(self, *args):
                pass
        server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        self.addCleanup(thread.join)
        self.addCleanup(server.server_close)
        self.addCleanup(server.shutdown)
        for path in ('/api/health', '/api/v1/query?query=ai_cli_sample_tokens'):
            with self.assertRaisesRegex(ValueError, 'redirect refused'):
                install.LOCAL_HTTP.open(f'http://127.0.0.1:{server.server_port}{path}', timeout=1)
        self.assertEqual(len(requests), 2)

    def test_cycle_and_preserve(self):
        self.settings.parent.mkdir(parents=True)
        self.settings.write_text(json.dumps({'theme': 'custom', 'extensions': ['other']}))
        self.call('install')
        self.assertEqual(json.loads(self.settings.read_text())['extensions'], ['other', EXT])
        self.assertEqual(self.settings.stat().st_mode & 0o777, 0o600)
        before = self.settings.read_bytes()
        self.call('install')
        self.assertEqual(before, self.settings.read_bytes())
        self.call('uninstall')
        self.assertEqual(json.loads(self.settings.read_text()), {'theme': 'custom', 'extensions': ['other']})
        self.assertFalse(self.launcher.exists())
        self.call('uninstall')

    def test_owned_repeat_with_occupied_ports(self):
        self.call('install')
        before = (self.settings.read_bytes(), self.launcher.read_bytes(), self.log.read_text())
        self.env['OCCUPIED_PORTS'] = '13000,14318'
        self.call('install')
        self.assertEqual(before[:2], (self.settings.read_bytes(), self.launcher.read_bytes()))
        self.assertNotIn('up -d', self.log.read_text()[len(before[2]):])

    def test_fresh_install_with_running_canonical_stack(self):
        self.env['OCCUPIED_PORTS'] = '13000,14318'
        self.call('install')
        self.assertEqual(json.loads(self.settings.read_text())['extensions'], [EXT])
        calls = self.log.read_text()
        self.assertIn('inspect grafana-id', calls)
        self.assertIn('inspect collector-id', calls)
        self.assertNotIn('up -d', calls)
        self.call('install')
        self.assertNotIn('up -d', self.log.read_text()[len(calls):])

    def test_running_foreign_project_or_port_rejected(self):
        for key, value in (('FAKE_PROJECT', 'foreign'), ('FAKE_PORT', '13001')):
            self.env['OCCUPIED_PORTS'] = '13000,14318'
            self.env[key] = value
            self.call('install', ok=False)
            del self.env[key]
            self.assertFalse(self.settings.exists())
            self.assertFalse((self.settings.parent / '.rotta-telemetry-journal').exists())

    def test_ambiguous_occupant_rejected(self):
        self.env['OCCUPIED_PORTS'] = '13000,14318'
        self.env['FAKE_AMBIGUOUS'] = '1'
        self.call('install', ok=False)
        self.assertFalse(self.settings.exists())
        self.assertNotIn('up -d', self.log.read_text())

    def test_only_one_owned_port_is_rejected(self):
        self.env['OCCUPIED_PORTS'] = '13000'
        self.call('install', ok=False)
        self.assertFalse(self.settings.exists())
        self.assertNotIn('up -d', self.log.read_text())

    def test_injected_write_failures_preserve_foreign_files(self):
        self.settings.parent.mkdir(parents=True)
        self.settings.write_text('{"theme":"foreign","extensions":["other"]}\n')
        self.launcher.parent.mkdir(parents=True)
        foreign = self.launcher.parent / 'foreign-file'
        foreign.write_text('untouched')
        original = self.settings.read_bytes()
        for target in ('backup', 'settings'):
            code = ("import sys; import install; original=install.atomic; "
                    "defect=lambda path,data,mode: "
                    "(_ for _ in ()).throw(PermissionError('denied')) "
                    f"if path.name == {('.rotta-telemetry-settings.bak' if target == 'backup' else 'settings.json')!r} "
                    "else original(path,data,mode); "
                    "install.atomic=defect; sys.argv=['install.py','install']; install.main()")
            result = subprocess.run([sys.executable, '-c', code], cwd=SCRIPT.parent,
                                    env=self.env, capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(self.settings.read_bytes(), original)
            self.assertEqual(foreign.read_text(), 'untouched')
            self.assertFalse(self.launcher.exists())
            self.assertNotIn('foreign', result.stderr)
            if target == 'settings':
                self.assertTrue((self.settings.parent / '.rotta-telemetry-journal').exists())
                (self.settings.parent / '.rotta-telemetry-journal').unlink()
                (self.settings.parent / '.rotta-telemetry-settings.bak').unlink()

    def test_launcher_from_other_cwd(self):
        self.call('install')
        self.log.write_text('')
        result = subprocess.run([str(self.launcher), 'status'], cwd=self.home, env=self.env,
                                capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('-f ' + str(SCRIPT.parent / 'compose.yaml') + ' -p rotta-ai-telemetry ps', self.log.read_text())

    def test_uninstall_compose_plan_retains_volumes(self):
        self.call('install')
        self.log.write_text('')
        self.call('uninstall')
        downs = [line for line in self.log.read_text().splitlines() if ' down' in line]
        self.assertEqual(len(downs), 1)
        self.assertEqual(downs[0].split()[-1], 'down')
        self.assertNotIn(' -v', downs[0])
        self.assertNotIn('--volumes', downs[0])

    def test_dry_run(self):
        self.call('install', '--dry-run')
        self.assertFalse(self.settings.parent.exists())
        self.assertFalse(self.launcher.exists())

    def test_malformed_foreign_and_symlink(self):
        self.settings.parent.mkdir(parents=True)
        self.settings.write_text('{bad')
        self.call('install', ok=False)
        self.assertEqual(self.settings.read_text(), '{bad')
        self.settings.write_text('{}')
        self.launcher.parent.mkdir(parents=True)
        self.launcher.write_text('foreign')
        self.call('install', ok=False)
        self.assertEqual(self.launcher.read_text(), 'foreign')
        self.launcher.unlink()
        self.launcher.symlink_to(self.settings)
        self.call('install', ok=False)

    def test_all_dry_runs_do_nothing(self):
        for action in ('install', 'uninstall', 'status', 'up', 'down', 'logs', 'smoke', 'pi'):
            self.call(action, '--dry-run')
        self.assertFalse(self.log.exists())
        self.assertFalse(self.settings.parent.exists())

    def test_missing_image_and_port_conflict(self):
        self.env['FAIL_IMAGE'] = '1'
        self.call('install', ok=False)
        self.assertFalse(self.settings.exists())
        del self.env['FAIL_IMAGE']
        self.env['OCCUPIED_PORTS'] = '13000'
        self.call('install', ok=False)
        self.assertFalse(self.settings.exists())

    def test_runtime_paths_and_duplicate_pi(self):
        self.settings.parent.mkdir(parents=True)
        self.settings.symlink_to(self.home / 'foreign')
        for action in ('status', 'logs', 'up', 'down'):
            self.call(action)
        self.settings.unlink()
        self.call('pi', '--version')
        self.assertIn('--extension ' + EXT, self.log.read_text())
        self.call('install')
        self.log.write_text('')
        self.call('pi', '--version')
        self.assertNotIn('--extension', self.log.read_text())
        self.call('status')
        self.assertIn(' -p rotta-ai-telemetry ps', self.log.read_text())

    def test_interrupted_and_uninstall_failure(self):
        self.call('install')
        original = self.settings.read_bytes()
        self.env['FAIL_DOWN'] = '1'
        self.call('uninstall', ok=False)
        self.assertEqual(self.settings.read_bytes(), original)
        self.assertTrue(self.launcher.exists())
        self.assertTrue((self.settings.parent / '.rotta-telemetry-journal').exists())
        self.call('install', ok=False)
        self.assertTrue((self.settings.parent / '.rotta-telemetry-settings.bak').exists())

    def test_error_privacy_and_quoted_launcher(self):
        self.settings.parent.mkdir(parents=True)
        self.settings.write_text('{secret-password')
        result = self.call('install', ok=False)
        self.assertNotIn('secret-password', result.stderr)
        self.settings.unlink()
        self.call('install')
        self.assertIn('exec ', self.launcher.read_text())

    def test_failed_start_restores(self):
        self.settings.parent.mkdir(parents=True)
        self.settings.write_text('{"theme":"original"}\n')
        self.env['FAIL_UP'] = '1'
        self.call('install', ok=False)
        self.assertEqual(self.settings.read_text(), '{"theme":"original"}\n')
        self.assertFalse(self.launcher.exists())
        self.assertTrue((self.settings.parent / '.rotta-telemetry-settings.bak').exists())
        self.assertNotIn(' down', self.log.read_text())


if __name__ == '__main__':
    unittest.main()
