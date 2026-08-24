-- A design run's companion task records where it came from.
--
-- The design launcher can open a task card next to a run ("同步创建任务").
-- That card and a task the user picked from the project are the same column on
-- design_document, so nothing could tell them apart afterwards — and they must
-- be treated differently: the companion card IS the design work and may be
-- advanced when its own agent starts, while a task the user linked is usually
-- the implementation work the design feeds into, and moving it would claim
-- that implementation had started.
--
-- origin_id carries the design_document.id, which also gives the issue side a
-- way back to the document without a second column.
ALTER TABLE issue DROP CONSTRAINT IF EXISTS issue_origin_type_check;
ALTER TABLE issue ADD CONSTRAINT issue_origin_type_check
    CHECK (origin_type IN ('autopilot', 'quick_create', 'lark_chat', 'slack_chat', 'agent_create', 'dingtalk_chat', 'wecom_chat', 'telegram_chat', 'design_document'));
