#!/usr/bin/env python3
"""Synthetic OTLP/HTTP metrics only; never reads CLI sessions or credentials."""
import json
import secrets
import sys
import time
import urllib.request


class RejectRedirects(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, msg, headers, newurl):
        raise ValueError('local HTTP redirect refused')


LOCAL_HTTP = urllib.request.build_opener(urllib.request.ProxyHandler({}), RejectRedirects())

CLIENTS = ("claude", "opencode", "copilot", "pi")
PRIVATE = {"session.id", "user.account_uuid", "file.path", "model", "prompt"}


def attrs(**values):
    return [{"key": key, "value": {"stringValue": value}} for key, value in values.items()]


def payload(marker=None):
    now = time.time_ns()
    points = [{"attributes": attrs(client=client), "asInt": str(marker * 100 + value if marker is not None else value), "timeUnixNano": str(now)}
              for client, value in zip(CLIENTS, (12, 0, 7, 3))]
    def counter(name, unit, value, labels):
        return {"name": name, "unit": unit, "sum": {"aggregationTemporality": 2,
                "isMonotonic": True, "dataPoints": [{"attributes": attrs(**labels),
                "asDouble": value, "startTimeUnixNano": str(now - 60_000_000_000),
                "timeUnixNano": str(now)}]}}

    # Distinct cumulative series; zero is an intentional synthetic example, not native absence.
    # Synthetic per-run counter incarnation: marker encodes the seed, not actual usage.
    # 40-bit marker * 100 + value stays exactly representable in binary64.
    canonical = counter("ai_cli_fixture_canonical_tokens", "{token}", 12.0,
                        {"client": "claude", "type": "input", "session.id": "synthetic-private"})
    canonical["sum"]["dataPoints"] = [
        {**canonical["sum"]["dataPoints"][0], "attributes": attrs(client=client, type="input", **{"session.id": "synthetic-private"}),
         "asDouble": float(marker * 100 + value if marker is not None else value)}
        for client, value in zip(CLIENTS, (12, 0, 7, 3))]
    metrics = [canonical,
        {"name": "ai_cli_sample_tokens", "gauge": {"dataPoints": points}},
        {"name": "ai_cli_fixture_run_marker", "gauge": {"dataPoints": [
            {"attributes": attrs(client=client), "asInt": str(marker if marker is not None else 1), "timeUnixNano": str(now)}
            for client in CLIENTS]}},
        counter("ai_cli_fixture_claude_tokens", "tokens", 42.0,
                {"type": "cacheRead", "session.id": "synthetic-private", "model": "synthetic-private"}),
        counter("ai_cli_fixture_copilot_tokens", "{token}", 17.0,
                {"model": "synthetic-private", "session.id": "synthetic-private"}),
        counter("ai_cli_fixture_claude_cost_usd", "USD", 0.012,
                {"user.account_uuid": "synthetic-private", "prompt": "synthetic-private"}),
        {"name": "ai_cli_fixture_pi_marker", "gauge": {"dataPoints": [
            {"attributes": attrs(**{"file.path": "/synthetic/private", "session.id": "synthetic-private"}),
             "asInt": "1", "timeUnixNano": str(now)}]}},
    ]
    return {"resourceMetrics": [{"resource": {"attributes": attrs(**{"user.account_uuid": "synthetic-private", "ai_cli_source": "fixture"})},
            "scopeMetrics": [{"metrics": metrics}]}]}


def check():
    data = payload()["resourceMetrics"][0]
    metrics = {m["name"]: m for m in data["scopeMetrics"][0]["metrics"]}
    assert set(metrics) == {"ai_cli_fixture_canonical_tokens", "ai_cli_fixture_run_marker", "ai_cli_sample_tokens", "ai_cli_fixture_claude_tokens",
                            "ai_cli_fixture_claude_cost_usd", "ai_cli_fixture_pi_marker", "ai_cli_fixture_copilot_tokens"}
    canonical_points = metrics["ai_cli_fixture_canonical_tokens"]["sum"]["dataPoints"]
    assert len(canonical_points) == 4
    assert {next(a["value"]["stringValue"] for a in p["attributes"] if a["key"] == "client"): p["asDouble"] for p in canonical_points} == dict(zip(CLIENTS, (12.0, 0.0, 7.0, 3.0)))
    points = metrics["ai_cli_sample_tokens"]["gauge"]["dataPoints"]
    assert len(points) == 4
    assert {p["attributes"][0]["value"]["stringValue"] for p in points} == set(CLIENTS)
    assert next(p for p in points if p["attributes"][0]["value"]["stringValue"] == "opencode")["asInt"] == "0"
    assert all(len(p["attributes"]) == 1 for p in points)
    for name, unit in (("ai_cli_fixture_claude_tokens", "tokens"), ("ai_cli_fixture_claude_cost_usd", "USD"), ("ai_cli_fixture_copilot_tokens", "{token}")):
        metric = metrics[name]
        assert metric["unit"] == unit and metric["sum"]["isMonotonic"]
        assert metric["sum"]["aggregationTemporality"] == 2
        point = metric["sum"]["dataPoints"][0]
        assert int(point["startTimeUnixNano"]) < int(point["timeUnixNano"])
        assert PRIVATE.intersection(a["key"] for a in point["attributes"])
    assert any(a["key"] == "type" and a["value"]["stringValue"] == "cacheRead"
               for a in metrics["ai_cli_fixture_claude_tokens"]["sum"]["dataPoints"][0]["attributes"])
    assert PRIVATE.intersection(a["key"] for a in data["resource"]["attributes"])
    assert PRIVATE.intersection(a["key"] for a in metrics["ai_cli_fixture_pi_marker"]["gauge"]["dataPoints"][0]["attributes"])
    print("fixture: four attributed samples, explicit zero, distinct Claude/Copilot fixture counters and Pi marker with synthetic private labels (offline only)")


if __name__ == "__main__":
    check()
    if sys.argv[1:] == ["--send"]:
        marker = secrets.randbelow(2**40 - 1) + 1
        request = urllib.request.Request("http://127.0.0.1:14318/v1/metrics",
                                         json.dumps(payload(marker)).encode(),
                                         {"Content-Type": "application/json"})
        with LOCAL_HTTP.open(request, timeout=3) as response:
            print("collector HTTP", response.status)
        print("fixture marker", marker)
    elif sys.argv[1:]:
        sys.exit("usage: python3 fixture.py [--send]")
