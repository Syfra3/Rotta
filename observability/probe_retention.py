"""Disposable Loki 3.1 retention admission/query probe; endpoint must be isolated."""
import json
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

base = sys.argv[1].rstrip('/')
now = time.time_ns()
labels = '{job="retention-r123"}'

def request(path, body=None):
    req = urllib.request.Request(base + path, data=body, headers={'Content-Type': 'application/json'}, method='POST' if body else 'GET')
    try:
        with urllib.request.urlopen(req, timeout=10) as response:
            return response.status, json.load(response) if path.startswith('/loki/api/v1/query') else response.read().decode()
    except urllib.error.HTTPError as exc:
        return exc.code, exc.read().decode()

for name, age in [('current', 0), ('expired', 13 * 3600)]:
    payload = {'streams': [{'stream': {'job': 'retention-r123'}, 'values': [[str(now - age * 1_000_000_000), name]]}]}
    print(name, request('/loki/api/v1/push', json.dumps(payload).encode()))
for name, start in [('recent', now - 3600 * 1_000_000_000), ('wide', now - 24 * 3600 * 1_000_000_000)]:
    params = urllib.parse.urlencode({'query': labels, 'start': start, 'end': now + 1_000_000_000})
    print(name, request('/loki/api/v1/query_range?' + params))
