-- Per-round cap on how many case tasks run at once (M4 "release one as one
-- completes"). NULL = no cap: every case is dispatched immediately and the
-- daemon's task slots plus the hub's lease waits throttle. Fork-local (900+).
ALTER TABLE test_run
ADD COLUMN IF NOT EXISTS parallelism INTEGER;
