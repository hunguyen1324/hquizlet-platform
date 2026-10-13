"""Additional API acceptance flows on the disposable IAM E2E stack."""
import json
import os
import time
import urllib.error
import urllib.request

gateway = "http://127.0.0.1:18080"
admin = os.environ["IAM_QA_ADMIN_TOKEN"]
learner = os.environ["IAM_QA_LEARNER_TOKEN"]
class_id = os.environ["IAM_QA_CLASS_ID"]
learner_id = int(os.environ["IAM_QA_LEARNER_ID"])

def call(path, token, body=None, method=None, expected=200, password=None):
    headers = {"Authorization": "Bearer " + token, "Content-Type": "application/json"}
    if password is not None:
        headers["X-Admin-Password"] = password
    req = urllib.request.Request(gateway + path, data=json.dumps(body).encode() if body is not None else None,
                                 headers=headers, method=method or ("POST" if body is not None else "GET"))
    try:
        with urllib.request.urlopen(req, timeout=10) as response:
            status, data = response.status, response.read()
    except urllib.error.HTTPError as error:
        status, data = error.code, error.read()
    assert status == expected, f"{req.method} {path}: expected {expected}, received {status}: {data[:300]!r}"
    return json.loads(data) if data else None

roles = call("/v1/admin/roles", admin)["roles"]
assert all(role.get("policy", {}).get("Statements") for role in roles), "Role policies unavailable"

personal = call("/v1/study-sets", learner, {"title": "Learner editable set", "visibility": "public", "contentType": "flashcard"}, expected=201)
call(f"/v1/study-sets/{personal['id']}", admin, {"title": "Admin reviewed set", "visibility": "public"}, method="PUT")
class_info = call(f"/v1/classes/{class_id}", admin)
call(f"/v1/classes/{class_info['inviteCode']}/join", learner, {})
quiz = call("/v1/study-sets", admin, {"title": "IAM scored quiz", "visibility": "public", "contentType": "quiz"}, expected=201)
quiz_id = quiz["id"]
call(f"/v1/study-sets/{quiz_id}/quiz-questions", admin, {"questions": [{"questionType": "written", "questionText": "Say hello", "correctAnswer": "hello", "position": 0}]}, method="PUT")
call(f"/v1/classes/{class_id}/study-sets", admin, {"studySetId": quiz_id})
before = call("/v1/me/study-stats", learner)["sessionsTotal"]
session = call(f"/v1/study-sets/{quiz_id}/play", learner, {"mode": "practice", "layout": "default", "instant": False}, expected=201)
path = f"/v1/study-sets/{quiz_id}/play/{session['id']}"
call(path + "/answer", learner, {"index": 0, "answer": "hello"})
result = call(path + "/submit", learner, {})
assert result["submitted"] and result["score"] == 1, result
call(path + "/submit", learner, {})
assert call("/v1/me/study-stats", learner)["sessionsTotal"] == before + 1, "Repeated quiz submission duplicated learning progress"
for attempt in range(30):
    dashboard = call(f"/v1/classes/{class_id}/dashboard", admin)
    if any(event["eventType"] == "quiz.completed" for event in dashboard["recentActivity"]):
        break
    time.sleep(1)
else:
    raise AssertionError("Quiz completion event did not reach class activity")
assigned = next(row for row in dashboard["assignedSets"] if row["studySetId"] == quiz_id)
assert assigned["sessions"] == 1 and assigned["avgScorePct"] == 100, assigned

# Read-only support cannot adjust balances. Fresh reauthentication must not revive
# a cached administrator identity after the underlying grant was revoked.
support = call("/v1/admin/assignments", admin, {"userId": learner_id, "roleKey": "PlatformSupport", "scopeType": "platform"}, expected=201)
login = {"email": "iam-outsider@e2e.local", "password": "Password123!"}
support_token = call("/v1/auth/login", "", login)["token"]
call("/v1/admin/wallet/credit", support_token, {"userId": learner_id, "amountVnd": 1000, "note": "must reject"}, password="Password123!", expected=403)
call(f"/v1/admin/assignments/{support['id']}", admin, method="DELETE")
grant = call("/v1/admin/assignments", admin, {"userId": learner_id, "roleKey": "PlatformAdmin", "scopeType": "platform"}, password="Password123!", expected=201)
cached_admin = call("/v1/auth/login", "", login)["token"]
call("/v1/admin/users", cached_admin)
other_class = call("/v1/classes", cached_admin, {"name": "Other owner class"}, expected=201)
call(f"/v1/admin/assignments/{grant['id']}", admin, method="DELETE")
call("/v1/admin/users", cached_admin, expected=403)
call("/v1/admin/wallet/credit", cached_admin, {"userId": learner_id, "amountVnd": 1000, "note": "revoked must reject"}, password="Password123!", expected=403)
call(f"/v1/classes/{other_class['id']}/study-sets", admin, {"studySetId": quiz_id})
call(f"/v1/classes/{other_class['id']}/study-sets", admin)
call(f"/v1/classes/{other_class['id']}/study-sets/{quiz_id}", admin, method="DELETE")
print("Protected quiz score, event deduplication, cross-owner admin write, support and immediate revocation OK")
