#!/usr/bin/env python3
"""Opt-in local telemetry installer; never reads provider credentials."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shlex
import shutil
import socket
import subprocess
import sys
import tempfile
import time
import urllib.parse
import urllib.request

sys.path.insert(0, str(Path(__file__).resolve().parent))
from check_dashboard import check as check_dashboard, poll as poll_dashboard


class RejectRedirects(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, msg, headers, newurl):
        raise ValueError('local HTTP redirect refused')


LOCAL_HTTP = urllib.request.build_opener(urllib.request.ProxyHandler({}), RejectRedirects())

ROOT = Path(__file__).resolve().parent.parent
EXT = str(ROOT / 'observability/pi-telemetry.ts')
COMPOSE = ROOT / 'observability/compose.yaml'
PROJECT = 'rotta-ai-telemetry'
DASHBOARD_URL = 'http://127.0.0.1:13000/d/ai-cli-samples'
MARK = '# rotta-ai-telemetry managed launcher v1'
IMAGES = ('otel/opentelemetry-collector-contrib:0.104.0', 'prom/prometheus:v2.53.0', 'grafana/grafana:13.3.0')


def safe(path):
    for part in (Path(path), *Path(path).parents):
        if part.is_symlink():
            raise ValueError('symlink path conflict')
    if Path(path).exists() and Path(path).stat().st_uid != os.getuid():
        raise ValueError('foreign owner conflict')


def run(*args):
    return subprocess.run(args, check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)


def compose(*args):
    return run('docker', 'compose', '-f', str(COMPOSE), '-p', PROJECT, *args)


def existing_stack_on_ports():
    """Verify every required service is a distinct canonical running container."""
    expected = {13000: ('grafana', '3000/tcp'), 14318: ('collector', '4318/tcp')}
    containers = {}
    for port, (service, internal) in expected.items():
        ids = run('docker', 'ps', '--filter', f'publish={port}', '--format', '{{.ID}}').stdout.splitlines()
        if len(ids) != 1:
            raise ValueError('ambiguous telemetry port ownership')
        details = json.loads(run('docker', 'inspect', ids[0]).stdout)
        if not isinstance(details, list) or len(details) != 1:
            raise ValueError('ambiguous telemetry container')
        item = details[0]
        labels = item['Config']['Labels']
        bindings = item['NetworkSettings']['Ports'][internal]
        if (item['State']['Running'] is not True or
            labels.get('com.docker.compose.project') != PROJECT or
            labels.get('com.docker.compose.service') != service or
            labels.get('com.docker.compose.project.working_dir') != str(COMPOSE.parent) or
            labels.get('com.docker.compose.project.config_files') != str(COMPOSE) or
            not isinstance(bindings, list) or len(bindings) != 1 or
            bindings[0] != {'HostIp': '127.0.0.1', 'HostPort': str(port)} or
            ids[0] in containers):
            raise ValueError('foreign or ambiguous telemetry port occupant')
        containers[ids[0]] = service
    ids = run('docker', 'ps', '-a', '--filter', f'label=com.docker.compose.project={PROJECT}',
              '--filter', 'label=com.docker.compose.service=prometheus',
              '--format', '{{.ID}}').stdout.splitlines()
    if len(ids) != 1 or ids[0] in containers:
        raise ValueError('missing or ambiguous telemetry prometheus')
    details = json.loads(run('docker', 'inspect', ids[0]).stdout)
    if not isinstance(details, list) or len(details) != 1:
        raise ValueError('ambiguous telemetry prometheus container')
    item = details[0]
    labels = item['Config']['Labels']
    if (item['State']['Running'] is not True or
        labels.get('com.docker.compose.project') != PROJECT or
        labels.get('com.docker.compose.service') != 'prometheus' or
        labels.get('com.docker.compose.project.working_dir') != str(COMPOSE.parent) or
        labels.get('com.docker.compose.project.config_files') != str(COMPOSE)):
        raise ValueError('foreign or stopped telemetry prometheus')
    return True


def atomic(path, data, mode):
    fd, tmp = tempfile.mkstemp(prefix='.rotta-', dir=path.parent)
    try:
        os.fchmod(fd, mode)
        with os.fdopen(fd, 'wb') as out:
            out.write(data)
            out.flush()
            os.fsync(out.fileno())
        os.replace(tmp, path)
        directory = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        if os.path.exists(tmp):
            os.unlink(tmp)


def launcher_bytes():
    return ('#!/bin/sh\n' + MARK + '\nexec ' + shlex.quote(shutil.which('python3') or sys.executable) + ' ' + shlex.quote(str(ROOT / 'observability/install.py')) + ' "$@"\n').encode()


def verify_fixture_query(timeout=12, interval=0.5):
    url = 'http://127.0.0.1:13000/api/datasources/proxy/uid/local-prometheus/api/v1/query?' + urllib.parse.urlencode({'query': 'ai_cli_sample_tokens{source="fixture"}'})
    deadline = time.monotonic() + timeout
    while True:
        try:
            with LOCAL_HTTP.open(url, timeout=min(3, max(0.1, deadline - time.monotonic()))) as response:
                data = json.load(response)
            if data.get('status') != 'success' or data.get('data', {}).get('resultType') != 'vector':
                raise ValueError('malformed fixture query')
            samples = data['data']['result']
            if samples:
                expected = {'claude': 12, 'opencode': 0, 'copilot': 7, 'pi': 3}
                seen = {}
                for sample in samples:
                    labels = sample['metric']
                    if not isinstance(labels, dict) or set(labels) != {'__name__', 'client', 'source', 'job', 'instance'} or any(labels.get(key) != value for key, value in {'__name__': 'ai_cli_sample_tokens', 'source': 'fixture', 'job': 'collector', 'instance': 'collector:8889'}.items()) or labels['client'] not in expected or labels['client'] in seen:
                        raise ValueError('unexpected or private fixture labels')
                    value = sample['value']
                    if not isinstance(value, list) or len(value) != 2 or float(value[1]) != expected[labels['client']]:
                        raise ValueError('incorrect fixture sample')
                    seen[labels['client']] = value[1]
                if set(seen) == set(expected):
                    return
                raise ValueError('incomplete fixture samples')
        except (OSError, TimeoutError):
            pass  # Bounded scrape propagation or transient local HTTP failure.
        if time.monotonic() >= deadline:
            raise ValueError('fixture samples absent after scrape deadline')
        time.sleep(min(interval, max(0, deadline - time.monotonic())))


def opencode_config_root(home):
    config = os.environ.get('XDG_CONFIG_HOME')
    root = Path(config) if config else home / '.config'
    if not root.is_absolute():
        raise ValueError('XDG_CONFIG_HOME must be absolute')
    return root / 'opencode'


def opencode_integration(action, config_root):
    """Manage only our autoloaded plugin; leave Pi and the shared stack alone on removal."""
    source = ROOT / 'observability/opencode-telemetry.mjs'
    directory = config_root / 'plugins/rotta-telemetry'
    plugin = directory / 'server.js'
    legacy = directory.parent / 'opencode-telemetry.mjs'
    receipt = config_root / '.rotta-telemetry-owned.json'
    journal = config_root / '.rotta-telemetry-journal'
    backup = config_root / '.rotta-telemetry-plugin.bak'
    for path in (source, directory, plugin, legacy, receipt, journal, backup):
        safe(path)
    if journal.exists():
        raise ValueError('interrupted OpenCode telemetry transaction; inspect journal and files')
    if backup.exists():
        raise ValueError('OpenCode telemetry backup requires manual inspection')
    if legacy.exists():
        raise ValueError('legacy OpenCode telemetry artifact requires manual inspection')
    if directory.exists() and (not directory.is_dir() or set(directory.iterdir()) != {plugin}):
        raise ValueError('unowned or modified OpenCode telemetry directory')
    owned = False
    if receipt.exists():
        data = json.loads(receipt.read_text())
        if not isinstance(data, dict) or set(data) != {'sha256'} or not plugin.is_file():
            raise ValueError('partial OpenCode ownership state')
        owned = data['sha256'] == hashlib.sha256(plugin.read_bytes()).hexdigest()
        if not owned:
            raise ValueError('modified OpenCode telemetry plugin')
    elif directory.exists():
        raise ValueError('unowned OpenCode telemetry plugin directory')
    if action == 'uninstall':
        if not owned:
            print('not installed')
            return
        atomic(journal, b'uninstall\n', 0o600)
        plugin.unlink()
        directory.rmdir()
        receipt.unlink()
        journal.unlink()
        print('OpenCode telemetry uninstalled; shared stack and volumes retained')
        return
    if owned:
        if plugin.read_bytes() != source.read_bytes():
            existing_stack_on_ports()
            previous = plugin.read_bytes()
            payload = source.read_bytes()
            old_receipt = receipt.read_bytes()
            atomic(backup, previous, 0o600)
            atomic(journal, json.dumps({'action': 'upgrade', 'old_sha256': hashlib.sha256(previous).hexdigest(),
                                        'new_sha256': hashlib.sha256(payload).hexdigest()}).encode(), 0o600)
            try:
                atomic(plugin, payload, 0o600)
                atomic(receipt, json.dumps({'sha256': hashlib.sha256(payload).hexdigest()}).encode(), 0o600)
            except Exception:
                # Leave journal and backup for explicit recovery, including if rollback fails.
                atomic(plugin, previous, 0o600)
                atomic(receipt, old_receipt, 0o600)
                raise
            journal.unlink()
            backup.unlink()
            print('OpenCode telemetry upgraded; verify plugin reload and collector compatibility')
            return
        existing_stack_on_ports()
        print('already installed')
        return
    if not shutil.which('docker'):
        raise ValueError('docker binary required')
    if json.loads(compose('config', '--format', 'json').stdout).get('name') != PROJECT:
        raise ValueError('compose project mismatch')
    for image in IMAGES:
        run('docker', 'image', 'inspect', image)
    occupied = []
    for port in (13000, 14318):
        with socket.socket() as sock:
            try:
                sock.bind(('127.0.0.1', port))
            except OSError:
                occupied.append(port)
    running = False
    if occupied:
        if len(occupied) != 2:
            raise ValueError('partial telemetry port conflict')
        running = existing_stack_on_ports()
    directory.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    payload = source.read_bytes()
    atomic(journal, b'install\n', 0o600)
    created = False
    try:
        directory.mkdir(mode=0o700)
        created = True
        atomic(plugin, payload, 0o600)
        atomic(receipt, json.dumps({'sha256': hashlib.sha256(payload).hexdigest()}).encode(), 0o600)
        if not running:
            compose('up', '-d', '--no-build', '--pull', 'never')
        if not existing_stack_on_ports():
            raise ValueError('stack identity not verified')
    except Exception:
        if created:
            plugin.unlink(missing_ok=True)
            directory.rmdir()
        receipt.unlink(missing_ok=True)
        # Retain journal for explicit recovery; never stop a possibly pre-existing stack.
        raise
    journal.unlink()
    print('OpenCode telemetry installed; restart OpenCode to load it')
    print(DASHBOARD_URL)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('action', choices=('install', 'uninstall', 'status', 'up', 'down', 'logs', 'smoke', 'pi', 'check-dashboard'))
    parser.add_argument('--dry-run', action='store_true')
    parser.add_argument('--host', choices=('pi', 'opencode'), default='pi')
    args, rest = parser.parse_known_args()
    if rest and args.action != 'pi':
        parser.error('unexpected arguments')
    home = Path.home()
    settings = home / '.pi/agent/settings.json'
    launcher = home / '.local/bin/rotta-telemetry'
    receipt = home / '.pi/agent/.rotta-telemetry-owned'
    backup = home / '.pi/agent/.rotta-telemetry-settings.bak'
    journal = home / '.pi/agent/.rotta-telemetry-journal'
    if args.dry_run:
        print(f'dry-run: {args.action} {PROJECT}; no effects')
        return
    config_root = None
    if (args.host == 'opencode' and args.action in ('install', 'uninstall', 'status')) or (args.host == 'pi' and args.action == 'uninstall'):
        config_root = opencode_config_root(home)
    for path in (home, COMPOSE, ROOT / 'observability/pi-telemetry.ts'):
        safe(path)
    if args.action in ('status', 'up', 'down', 'logs', 'smoke', 'check-dashboard'):
        if args.action == 'check-dashboard':
            check_dashboard()
        elif args.action == 'status':
            if args.host == 'opencode':
                directory = config_root / 'plugins/rotta-telemetry'
                plugin = directory / 'server.js'
                legacy = directory.parent / 'opencode-telemetry.mjs'
                receipt = config_root / '.rotta-telemetry-owned.json'
                journal = config_root / '.rotta-telemetry-journal'
                backup = config_root / '.rotta-telemetry-plugin.bak'
                for path in (directory, plugin, legacy, receipt, journal, backup):
                    safe(path)
                if journal.exists() or backup.exists():
                    print('OpenCode telemetry: interrupted transaction')
                elif receipt.exists() or directory.exists() or legacy.exists():
                    try:
                        data = json.loads(receipt.read_text())
                        owned = (not legacy.exists() and directory.is_dir() and
                                 set(directory.iterdir()) == {plugin} and set(data) == {'sha256'} and plugin.is_file() and
                                 data['sha256'] == hashlib.sha256(plugin.read_bytes()).hexdigest())
                    except (ValueError, OSError, TypeError):
                        owned = False
                    print('OpenCode telemetry: ' + ('installed' if owned else 'ownership conflict'))
                else:
                    print('OpenCode telemetry: not installed')
            compose('ps')
            try:
                if existing_stack_on_ports():
                    print(DASHBOARD_URL)
            except (ValueError, OSError, subprocess.CalledProcessError, KeyError, TypeError, IndexError):
                pass  # Status remains read-only; an unverified stack gets no URL.
        elif args.action == 'up':
            compose('up', '-d', '--no-build', '--pull', 'never')
            try:
                if existing_stack_on_ports():
                    print(DASHBOARD_URL)
            except (ValueError, OSError, subprocess.CalledProcessError, KeyError, TypeError, IndexError):
                pass  # Successful up is not proof that the canonical stack is running.
        elif args.action == 'down':
            compose('down')
        elif args.action == 'logs':
            compose('logs', '--tail', '100')
        else:
            sent = run(sys.executable, str(ROOT / 'observability/fixture.py'), '--send')
            marker = int(sent.stdout.split('fixture marker ')[-1].strip())
            for url in ('http://127.0.0.1:13000/api/health', 'http://127.0.0.1:13000/api/datasources'):
                with LOCAL_HTTP.open(url, timeout=3) as response:
                    if response.status != 200:
                        raise ValueError('health check failed')
            poll_dashboard(marker)
            try:
                if existing_stack_on_ports():
                    print(DASHBOARD_URL)
            except (ValueError, OSError, subprocess.CalledProcessError, KeyError, TypeError, IndexError):
                pass  # HTTP success alone does not establish canonical stack ownership.
        return
    if args.host == 'opencode' and args.action in ('install', 'uninstall'):
        opencode_integration(args.action, config_root)
        return
    if args.action == 'pi':
        # User-wide registration already loads this extension; do not load it twice.
        if settings.exists():
            safe(settings)
            try:
                registered = EXT in json.loads(settings.read_text()).get('extensions', [])
            except (ValueError, AttributeError, TypeError):
                registered = False
        else:
            registered = False
        os.execvp('pi', ['pi', *([] if registered else ['--extension', EXT]), *rest])
    for path in (settings.parent, settings, launcher.parent, launcher, receipt, backup, journal):
        safe(path)
    if journal.exists():
        raise ValueError('interrupted transaction; inspect backup and journal before recovery')
    value = {'extensions': []}
    original = settings.read_bytes() if settings.exists() else None
    if original is not None:
        value = json.loads(original)
        if not isinstance(value, dict):
            raise ValueError('settings must be an object')
    entries = value.get('extensions', [])
    if not isinstance(entries, list) or any(not isinstance(x, str) for x in entries):
        raise ValueError('extensions must be a list of strings')
    owned = receipt.exists() and receipt.is_file() and receipt.read_text() == EXT + '\n'
    if receipt.exists() and not owned:
        raise ValueError('foreign ownership receipt')
    if launcher.exists() and (not owned or not launcher.is_file() or launcher.read_bytes() != launcher_bytes()):
        raise ValueError('foreign or modified launcher conflict')
    if EXT in entries and not owned:
        raise ValueError('extension already registered without ownership receipt')
    if owned and (EXT not in entries or not launcher.exists()):
        raise ValueError('partial ownership state; inspect backup and recover manually')
    if args.action == 'uninstall' and not owned:
        if launcher.exists():
            raise ValueError('unowned launcher')
        print('not installed')
        return
    if args.action == 'install':
        if owned:
            try:
                existing_stack_on_ports()
            except (ValueError, OSError, subprocess.CalledProcessError, KeyError, TypeError, IndexError) as exc:
                raise ValueError('partial telemetry state: owned files exist but stack identity/health is unverified; inspect docker compose ps, backup and journal before retry') from exc
            print('already installed')
            return
        if not shutil.which('docker') or not shutil.which('pi'):
            raise ValueError('docker and pi binaries required')
        if json.loads(compose('config', '--format', 'json').stdout).get('name') != PROJECT:
            raise ValueError('compose project mismatch')
        for image in IMAGES:
            run('docker', 'image', 'inspect', image)
        occupied = []
        for port in (13000, 14318):
            with socket.socket() as sock:
                try:
                    sock.bind(('127.0.0.1', port))
                except OSError:
                    occupied.append(port)
        stack_running = False
        if occupied:
            if len(occupied) != 2:
                raise ValueError('partial telemetry port conflict')
            stack_running = existing_stack_on_ports()
    if backup.exists():
        raise ValueError('existing backup; inspect and recover manually')
    settings.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    launcher.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    # Durable original and intent precede every mutable step. Backup remains on failure.
    atomic(backup, original if original is not None else b'', 0o600)
    atomic(journal, json.dumps({'action': args.action, 'settings_existed': original is not None}).encode(), 0o600)
    try:
        if args.action == 'install':
            value['extensions'] = entries + [EXT]
            atomic(settings, (json.dumps(value, indent=2) + '\n').encode(), 0o600)
            atomic(launcher, launcher_bytes(), 0o700)
            atomic(receipt, (EXT + '\n').encode(), 0o600)
            if not stack_running:
                compose('up', '-d', '--no-build', '--pull', 'never')
            try:
                if not existing_stack_on_ports():
                    raise ValueError('stack identity not verified')
            except (ValueError, OSError, subprocess.CalledProcessError, KeyError, TypeError, IndexError) as exc:
                raise ValueError('partial telemetry state: stack identity/health unverified after setup; inspect docker compose ps, backup and journal before retry') from exc
            print('installed; use rotta-telemetry status or uninstall')
            print(DASHBOARD_URL)
        else:
            # The OpenCode integration has independent ownership of the shared stack.
            oc_receipt = config_root / '.rotta-telemetry-owned.json'
            safe(oc_receipt)
            if not oc_receipt.exists():
                compose('down')
            value['extensions'] = [x for x in entries if x != EXT]
            atomic(settings, (json.dumps(value, indent=2) + '\n').encode(), 0o600)
            launcher.unlink()
            receipt.unlink()
            print('uninstalled; volumes retained')
    except Exception:
        # Never down a stack on failed install: it may have existed before us.
        if original is None:
            settings.unlink(missing_ok=True)
        else:
            atomic(settings, original, 0o600)
        if args.action == 'install':
            launcher.unlink(missing_ok=True)
            receipt.unlink(missing_ok=True)
        raise
    finally:
        # Keep the journal and backup on failure for human recovery.
        if sys.exc_info()[0] is None:
            journal.unlink()
            backup.unlink()


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError, subprocess.CalledProcessError, json.JSONDecodeError) as exc:
        print('telemetry: operation failed; inspect ownership, backup and journal before retry', file=sys.stderr)
        sys.exit(1)
