-- Run only in the isolated IAM E2E database. Benchmark on a temporary clone;
-- no application rows are inserted or changed. Keep the EXPLAIN output as evidence.
BEGIN;
CREATE TEMP TABLE dashboard_bench (LIKE learning_sessions INCLUDING DEFAULTS INCLUDING CONSTRAINTS);
CREATE INDEX dashboard_bench_user_set_created ON dashboard_bench(user_id,study_set_id,created_at DESC);
INSERT INTO dashboard_bench(user_id,study_set_id,mode,score,total,started_at,idempotency_key,created_at)
SELECT 2, 1+(g%200), 'learn', 9, 10, now(), 'bench-'||g,
       now()-(g%180)*interval '1 day'-(g%3600)*interval '1 second'
FROM generate_series(1,50000) g;
ANALYZE dashboard_bench;
SELECT 'before: latest sessions across all study sets' AS benchmark;
EXPLAIN (ANALYZE, BUFFERS)
SELECT id,study_set_id,mode,score,total,created_at FROM dashboard_bench
WHERE user_id=2 ORDER BY created_at DESC LIMIT 5;
CREATE INDEX dashboard_bench_user_created ON dashboard_bench(user_id,created_at DESC);
ANALYZE dashboard_bench;
SELECT 'after: latest sessions across all study sets' AS benchmark;
EXPLAIN (ANALYZE, BUFFERS)
SELECT id,study_set_id,mode,score,total,created_at FROM dashboard_bench
WHERE user_id=2 ORDER BY created_at DESC LIMIT 5;
SELECT 'before: daily aggregation' AS benchmark;
EXPLAIN (ANALYZE, BUFFERS)
SELECT count(*) FROM dashboard_bench WHERE created_at>=current_date-1 AND created_at<current_date;
CREATE INDEX dashboard_bench_created ON dashboard_bench(created_at);
ANALYZE dashboard_bench;
SELECT 'after: daily aggregation' AS benchmark;
EXPLAIN (ANALYZE, BUFFERS)
SELECT count(*) FROM dashboard_bench WHERE created_at>=current_date-1 AND created_at<current_date;
ROLLBACK;
