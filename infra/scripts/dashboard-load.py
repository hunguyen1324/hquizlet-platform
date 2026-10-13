"""Lightweight dashboard acceptance check; only use against an isolated test stack.

IAM_LOAD_TOKEN is a test user's token. Avoid printing it or passing it in argv.
"""
import concurrent.futures
import json
import os
import time
import urllib.request

gateway = os.environ.get("IAM_LOAD_GATEWAY", "http://127.0.0.1:18080").rstrip("/")
token = os.environ["IAM_LOAD_TOKEN"]
class_id = os.environ["IAM_LOAD_CLASS_ID"]
paths = ["/v1/me/study-stats", f"/v1/classes/{class_id}/dashboard", "/v1/admin/overview?range=90d", "/v1/admin/study-stats?range=90d"]

def request(path):
    start = time.perf_counter()
    req = urllib.request.Request(gateway + path, headers={"Authorization": "Bearer " + token})
    with urllib.request.urlopen(req, timeout=10) as response:
        assert response.status == 200
        json.load(response)
    return (time.perf_counter() - start) * 1000

with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
    timings = sorted(pool.map(request, paths * 12))
result = {"requests": len(timings), "concurrency": 4, "p50Ms": round(timings[len(timings)//2], 1), "p95Ms": round(timings[int(len(timings)*.95)], 1), "maxMs": round(max(timings), 1)}
print(json.dumps(result))
assert result["p95Ms"] < 1500, "Dashboard p95 exceeded the 1.5 second acceptance target"
