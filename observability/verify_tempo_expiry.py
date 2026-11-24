"""Actual cached Tempo 2.5 aged trace/block expiry in an owned disposable network.

PYTHONDONTWRITEBYTECODE=1 python3 observability/verify_tempo_expiry.py
"""
import base64
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import time
import uuid

from verify_lab import IMAGES, docker, fetch, client

ROOT = Path(__file__).resolve().parent


def files(root):
    return {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in root.rglob('*') if p.is_file()}


def main():
    for name in ('tempo', 'client'):
        docker('image', 'inspect', IMAGES[name])
    tag = 'rotta-tempo-exp-' + uuid.uuid4().hex[:10]
    tempo, node = tag + '-tempo', tag + '-node'
    report = {'fixture': tag, 'status': 'failed', 'cleanup': False, 'attempts': []}
    with tempfile.TemporaryDirectory(prefix='rotta-tempo-exp-') as directory:
        root = Path(directory)
        root.chmod(0o711)
        data = root / 'data'
        data.mkdir(mode=0o777)
        data.chmod(0o777)
        # Tempo's local backend creates these directories as its unprivileged UID.
        for sub in ('blocks', 'wal'):
            (data / sub).mkdir(mode=0o777)
            (data / sub).chmod(0o777)
        base = (ROOT / 'tempo.yaml').read_text()
        assert 'block_retention: 12h' in base
        assert 'max_block_duration: 10m' in base
        # 2h aged record: queryable during the first pass, then expired under
        # a one-hour test retention without changing the host clock.
        fixture = base.replace('block_retention: 12h', 'block_retention: 1h').replace(
            'max_block_duration: 10m', 'max_block_duration: 1s').replace(
            'complete_block_timeout: 10m', 'complete_block_timeout: 1s').replace(
            'blocklist_poll: 1m', 'blocklist_poll: 1s').replace(
            'compacted_block_retention: 10m', 'compacted_block_retention: 1s').replace(
            'compaction_cycle: 1m', 'compaction_cycle: 1s')
        before = fixture.replace('block_retention: 1h', 'block_retention: 12h')
        config = root / 'tempo.yaml'
        config.write_text(before)
        config.chmod(0o644)
        report['config_sha256'] = hashlib.sha256(fixture.encode()).hexdigest()
        started = []
        network = False
        try:
            docker('network', 'create', '--internal', tag)
            network = True
            for name, image, args in ((tempo, IMAGES['tempo'], ['-config.file=/etc/tempo/config.yaml']),
                                      (node, IMAGES['client'], ['sleep', '600'])):
                command = ['run', '-d', '--pull=never', '--network', tag, '--name', name,
                           '--security-opt', 'no-new-privileges', '--cap-drop=ALL']
                if name == tempo:
                    command += ['--mount', f'type=bind,src={config},dst=/etc/tempo/config.yaml,readonly',
                                '--mount', f'type=bind,src={data},dst=/var/tempo']
                started.append(name)
                docker(*(command + [image, *args]))
            def request(path, body=None):
                return fetch(node, 'http://' + tempo + ':3200' + path, body)
            def ready():
                end = time.monotonic() + 80
                while time.monotonic() < end:
                    try:
                        if request('/ready')['status'] == 200:
                            return
                    except Exception:
                        pass
                    time.sleep(1)
                raise RuntimeError('Tempo readiness timeout: ' + docker('logs', tempo).stdout[-1600:])
            ready()
            status = request('/status/config')
            report['effective_config'] = {'status': status['status'], 'sha256': hashlib.sha256(status['body'].encode()).hexdigest(), 'relevant': [line.strip() for line in status['body'].splitlines() if any(key in line for key in ('block_retention:', 'compaction_window:', 'blocklist_poll:', 'complete_block_timeout:', 'max_block_duration:', 'compacted_block_retention:'))][:30]}
            assert status['status'] == 200, status
            trace_id = uuid.uuid4().hex
            span_id = uuid.uuid4().hex[:16]
            stamp = time.time_ns() - 2 * 3600 * 10**9
            payload = {'resourceSpans': [{'resource': {'attributes': [{'key': 'service.name', 'value': {'stringValue': 'expiry-fixture'}}]},
                'scopeSpans': [{'spans': [{'traceId': trace_id, 'spanId': span_id, 'name': 'aged-synthetic',
                    'startTimeUnixNano': str(stamp), 'endTimeUnixNano': str(stamp + 1000000)}]}]}]}
            # Distributor OTLP endpoint differs from query endpoint.
            pushed = fetch(node, 'http://' + tempo + ':4318/v1/traces', payload)
            assert pushed['status'] == 200, pushed
            report['push'] = pushed
            def trace():
                return request('/api/traces/' + trace_id)
            end = time.monotonic() + 40
            while time.monotonic() < end:
                observed = trace()
                if observed['status'] == 200 and base64.b64encode(bytes.fromhex(span_id)).decode() in observed['body']:
                    break
                time.sleep(1)
            else:
                raise RuntimeError('trace never queryable: ' + str(observed)[:500])
            report['before_query'] = observed
            end = time.monotonic() + 95
            while time.monotonic() < end:
                prior = files(data / 'blocks')
                if any(path.endswith('/data.parquet') for path in prior):
                    break
                time.sleep(2)
            else:
                raise RuntimeError('persisted parquet block not observed: ' + str(prior))
            report['before_blocks'] = prior
            assert trace()['status'] == 200, 'trace disappeared before retention transition'
            docker('stop', '--time', '20', tempo)
            config.write_text(fixture)
            docker('start', tempo)
            ready()
            expiry_status = request('/status/config')
            assert expiry_status['status'] == 200
            report['expiry_effective_config'] = {'status': 200, 'sha256': hashlib.sha256(expiry_status['body'].encode()).hexdigest(), 'relevant': [line.strip() for line in expiry_status['body'].splitlines() if any(key in line for key in ('block_retention:', 'compaction_window:', 'blocklist_poll:', 'complete_block_timeout:', 'max_block_duration:', 'compacted_block_retention:'))][:30]}
            end = time.monotonic() + 120
            while time.monotonic() < end:
                remaining = files(data / 'blocks')
                after = trace()
                if not any(path in remaining for path in prior if path.endswith('/data.parquet')) and after['status'] == 404:
                    report['after_blocks'] = remaining
                    report['after_query'] = after
                    report['status'] = 'passed'
                    break
                time.sleep(5)
            else:
                report['attempts'].append({'remaining_blocks': remaining, 'after_query': after,
                                           'tempo_logs': docker('logs', tempo).stdout[-4000:]})
                raise RuntimeError('real compactor failed to delete known parquet block and return 404 within 120s')
        finally:
            for name in reversed(started):
                docker('rm', '-f', name, check=False)
            if network:
                docker('network', 'rm', tag, check=False)
            # Root-owned files belong only to this test's temporary directory.
            docker('run', '--rm', '--pull=never', '--network', 'none', '--user', '0',
                   '--mount', f'type=bind,src={data},dst=/owned', IMAGES['client'], 'node', '-e',
                   "const fs=require('fs');function fix(p){for(const e of fs.readdirSync(p,{withFileTypes:true})){let q=p+'/'+e.name;if(e.isDirectory())fix(q);fs.chmodSync(q,e.isDirectory()?0o777:0o666)}}fix('/owned')", check=False)
            report['cleanup'] = all(docker('inspect', n, check=False).returncode != 0 for n in started) and docker('network', 'inspect', tag, check=False).returncode != 0
            print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
