-- "This machine is a test host": the explicit designation a device round
-- (android_device / ios_device) needs before it may be bound to the phones
-- the machine's device hub reports. Reporting is unconditional so the runtime
-- page can show the hub before the switch is flipped. Fork-local (900+).
ALTER TABLE agent_runtime
ADD COLUMN IF NOT EXISTS test_host_enabled BOOLEAN NOT NULL DEFAULT FALSE;
