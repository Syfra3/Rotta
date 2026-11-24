"""Owned, cached-image-only Loki aged-data retention trial; no host listeners or volumes."""
import json
import pathlib
import subprocess
import tempfile
import time
import uuid

ROOT = pathlib.Path(__file__).resolve().parent
IMAGE = 'grafana/loki:3.1.0'
NODE = 'node:26-bookworm-slim'

def docker(*args, check=True):
    return subprocess.run(['docker', *args], text=True, capture_output=True, check=check, timeout=120)

def request(name, path, payload=None):
    code = '''const http=require('http');let data=process.argv[3];let r=http.request({host:process.argv[1],port:3100,path:process.argv[2],method:data==='-'?'GET':'POST',headers:{'Content-Type':'application/json'}},s=>{let b='';s.on('data',x=>b+=x);s.on('end',()=>console.log(JSON.stringify({status:s.statusCode,body:b.slice(0,3000)})))});r.on('error',e=>{console.error(e.message);process.exitCode=2});r.end(data==='-'?undefined:data)'''
    result = docker('exec', name, 'node', '-e', code, name.split('-')[0] + '-loki', path, payload or '-')
    return json.loads(result.stdout)

def files(path):
    return sorted(str(p.relative_to(path)) for p in path.rglob('*') if p.is_file())

def main():
    for image in (IMAGE, NODE):
        docker('image', 'inspect', image)
    tag = 'ret' + uuid.uuid4().hex[:10]
    net, loki, client = tag, tag + '-loki', tag + '-client'
    report = {'fixture': tag, 'attempts': [], 'cleanup': False}
    with tempfile.TemporaryDirectory(prefix='rotta-retention-') as directory:
        store = pathlib.Path(directory)
        config = (ROOT / 'loki.yaml').read_text().replace('retention_period: 12h', 'retention_period: 1h').replace('reject_old_samples: true', 'reject_old_samples: false').replace('reject_old_samples_max_age: 12h', 'reject_old_samples_max_age: 0s').replace('max_query_lookback: 12h', 'max_query_lookback: 0s').replace('compaction_interval: 10m', 'compaction_interval: 1s').replace('apply_retention_interval: 10m', 'apply_retention_interval: 1s').replace('retention_delete_delay: 10m', 'retention_delete_delay: 1s')
        config = config.replace('max_chunk_age: 2h', 'max_chunk_age: 1s').replace('chunk_idle_period: 10m', 'chunk_idle_period: 1s').replace('flush_check_period: 1m', 'flush_check_period: 1s').replace('checkpoint_duration: 5m', 'checkpoint_duration: 1s').replace('cache_ttl: 1h', 'cache_ttl: 1s').replace('resync_interval: 1m', 'resync_interval: 1s')
        # Establish stored/queryable input with retention disabled first, then
        # restart with the exact same fixture store and retention enabled.
        (store / 'config.yaml').write_text(config.replace('retention_enabled: true', 'retention_enabled: false'))
        (store / 'data').mkdir()
        (store / 'data').chmod(0o777)
        try:
            docker('network', 'create', '--internal', net)
            docker('run', '-d', '--pull=never', '--network', net, '--name', loki, '-v', str(store) + '/config.yaml:/etc/loki/local-config.yaml:ro', '-v', str(store) + '/data:/loki', IMAGE, '-config.file=/etc/loki/local-config.yaml')
            docker('run', '-d', '--pull=never', '--network', net, '--name', client, NODE, 'sleep', '360')
            for _ in range(75):
                try:
                    if request(client, '/ready')['status'] == 200:
                        break
                except Exception:
                    pass
                time.sleep(1)
            else:
                raise RuntimeError('Loki readiness timed out: ' + (docker('logs', loki).stdout + docker('logs', loki).stderr)[-2000:])
            stamp = time.time_ns() - 2 * 3600 * 1_000_000_000
            stream = {'streams': [{'stream': {'job': tag}, 'values': [[str(stamp), tag]]}]}
            pushed = request(client, '/loki/api/v1/push', json.dumps(stream))
            report['attempts'].append({'push': pushed})
            if pushed['status'] != 204:
                raise RuntimeError('aged ingest rejected')
            query = '/loki/api/v1/query_range?query=%7Bjob%3D%22' + tag + '%22%7D&start=' + str(stamp - 1000000000) + '&end=' + str(stamp + 1000000000)
            for _ in range(15):
                before = request(client, query)
                if tag in before['body']:
                    break
                time.sleep(1)
            report['attempts'].append({'before_query': before, 'before_files': files(store / 'data')})
            if tag not in before['body']:
                raise RuntimeError('aged sample not visible before compaction')
            flush = request(client, '/flush', '{}')
            report['attempts'].append({'flush': flush})
            deadline = time.monotonic() + 20
            while time.monotonic() < deadline:
                chunks = [p for p in files(store / 'data') if p.startswith('chunks/fake/')]
                if chunks:
                    break
                time.sleep(1)
            if not chunks:
                raise RuntimeError('no physical chunk produced; expiry cannot be tested')
            report['attempts'].append({'persisted_chunks': chunks})
            # A graceful restart seals/ships the active TSDB index immediately,
            # instead of waiting for its normal rotation interval in this short test.
            docker('stop', '--time', '30', loki)
            report['attempts'].append({'sealed_files': files(store / 'data')})
            # Production uses equal admission/read/retention windows. The initial
            # fixture alone allowed an expired record to prove it existed; restore
            # the matching read window while the real compactor deletes its chunk.
            (store / 'config.yaml').write_text(config.replace('max_query_lookback: 0s', 'max_query_lookback: 1h'))
            docker('start', loki)
            for _ in range(75):
                try:
                    if request(client, '/ready')['status'] == 200:
                        break
                except Exception:
                    pass
                time.sleep(1)
            else:
                raise RuntimeError('Loki restart readiness timed out')
            for _ in range(90):
                time.sleep(1)
                after = request(client, query)
                remaining_chunks = [p for p in chunks if (store / 'data' / p).exists()]
                if tag not in after['body'] and not remaining_chunks:
                    report['attempts'].append({'after_query': after, 'after_files': files(store / 'data'),
                                               'physical_chunks_removed': True})
                    report['status'] = 'passed'
                    break
            else:
                report['attempts'].append({'timeout_query': after, 'after_files': files(store / 'data'),
                                           'remaining_chunks': remaining_chunks})
                report['status'] = 'failed'
                raise RuntimeError('expiry deadline: aged chunk/query still present')
        finally:
            for name in (client, loki):
                docker('rm', '-f', name, check=False)
            docker('network', 'rm', net, check=False)
            docker('run', '--rm', '--pull=never', '--network', 'none', '--user', '0', '-v', str(store / 'data') + ':/owned', NODE, 'node', '-e', "const fs=require('fs');function fix(p){for(const e of fs.readdirSync(p,{withFileTypes:true})){let q=p+'/'+e.name;if(e.isDirectory())fix(q);fs.chmodSync(q,e.isDirectory()?0o777:0o666)}}fix('/owned')", str(__import__('os').getuid()), str(__import__('os').getgid()), check=False)
            report['cleanup'] = all(docker('inspect', name, check=False).returncode != 0 for name in (client, loki)) and docker('network', 'inspect', net, check=False).returncode != 0
            print(json.dumps(report, indent=2))

if __name__ == '__main__':
    main()
