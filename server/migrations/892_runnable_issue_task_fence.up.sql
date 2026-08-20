-- Create-time task admission must inspect the issue and project versions that
-- exist after any conflicting completion/update commits, not the INSERT's
-- original statement snapshot. The four-argument overload keeps the ordinary
-- ownership fence unchanged while adding that stronger contract on demand.
CREATE FUNCTION lock_task_owner_rows(
    p_agent_id uuid,
    p_issue_id uuid,
    p_runtime_id uuid,
    p_require_issue_runnable boolean
)
RETURNS boolean
LANGUAGE plpgsql
AS $$
DECLARE
    required int := (CASE WHEN p_agent_id IS NULL THEN 0 ELSE 1 END)
                  + (CASE WHEN p_issue_id IS NULL THEN 0 ELSE 1 END)
                  + (CASE WHEN p_runtime_id IS NULL THEN 0 ELSE 1 END);
    resolved int;
    distinct_workspaces int;
    locked int;
    issue_workspace_id uuid;
    candidate_project_id uuid;
    locked_project_id uuid;
    issue_status text;
    project_status text;
BEGIN
    IF NOT p_require_issue_runnable THEN
        RETURN lock_task_owner_rows(p_agent_id, p_issue_id, p_runtime_id);
    END IF;
    IF p_issue_id IS NULL THEN
        RETURN FALSE;
    END IF;

    WITH owners AS (
        SELECT a.workspace_id FROM agent a WHERE a.id = p_agent_id
        UNION ALL
        SELECT i.workspace_id FROM issue i WHERE i.id = p_issue_id
        UNION ALL
        SELECT r.workspace_id FROM agent_runtime r WHERE r.id = p_runtime_id
    )
    SELECT count(*), count(DISTINCT workspace_id)
    INTO resolved, distinct_workspaces
    FROM owners;
    IF resolved <> required THEN
        RETURN FALSE;
    END IF;

    WITH locked_workspaces AS (
        SELECT w.id
        FROM workspace w
        WHERE w.id IN (
            SELECT a.workspace_id FROM agent a WHERE a.id = p_agent_id
            UNION
            SELECT i.workspace_id FROM issue i WHERE i.id = p_issue_id
            UNION
            SELECT r.workspace_id FROM agent_runtime r WHERE r.id = p_runtime_id
        )
        ORDER BY w.id
        FOR KEY SHARE
    )
    SELECT count(*) INTO locked FROM locked_workspaces;
    IF locked <> distinct_workspaces THEN
        RETURN FALSE;
    END IF;

    LOOP
        BEGIN
            -- This VOLATILE function gets a current snapshot for each internal
            -- query. Read the candidate project, lock it before the issue, then
            -- verify the issue did not move while those locks were acquired.
            SELECT i.workspace_id, i.project_id, i.status
            INTO issue_workspace_id, candidate_project_id, issue_status
            FROM issue i
            WHERE i.id = p_issue_id;
            IF NOT FOUND OR issue_status IN ('backlog', 'done', 'cancelled') THEN
                RETURN FALSE;
            END IF;

            IF candidate_project_id IS NOT NULL THEN
                SELECT p.id, p.status
                INTO locked_project_id, project_status
                FROM project p
                WHERE p.id = candidate_project_id
                  AND p.workspace_id = issue_workspace_id
                FOR SHARE;
                IF NOT FOUND OR project_status = 'completed' THEN
                    RETURN FALSE;
                END IF;
            END IF;

            locked := 0;
            IF p_agent_id IS NOT NULL THEN
                PERFORM 1 FROM agent WHERE id = p_agent_id FOR KEY SHARE;
                IF FOUND THEN locked := locked + 1; END IF;
            END IF;

            SELECT i.project_id, i.status
            INTO locked_project_id, issue_status
            FROM issue i
            WHERE i.id = p_issue_id
            FOR SHARE;
            IF NOT FOUND THEN
                RETURN FALSE;
            END IF;
            IF locked_project_id IS DISTINCT FROM candidate_project_id THEN
                RAISE EXCEPTION USING ERRCODE = 'PT001', MESSAGE = 'issue project changed during task admission';
            END IF;
            IF issue_status IN ('backlog', 'done', 'cancelled') THEN
                RETURN FALSE;
            END IF;
            locked := locked + 1;

            IF p_runtime_id IS NOT NULL THEN
                PERFORM 1 FROM agent_runtime WHERE id = p_runtime_id FOR KEY SHARE;
                IF FOUND THEN locked := locked + 1; END IF;
            END IF;
            RETURN locked = required;
        EXCEPTION WHEN SQLSTATE 'PT001' THEN
            -- The exception subtransaction releases the candidate project and
            -- issue locks before retrying in the required project-before-issue order.
            NULL;
        END;
    END LOOP;
END;
$$;
