DROP INDEX IF EXISTS knowledge_items_commit_id_idx;
DROP INDEX IF EXISTS knowledge_items_pull_request_id_idx;
DROP INDEX IF EXISTS webhook_events_status_received_idx;

DROP TRIGGER IF EXISTS trg_knowledge_search_vector ON knowledge_items;
DROP FUNCTION IF EXISTS update_knowledge_search_vector;

ALTER TABLE join_requests DROP CONSTRAINT IF EXISTS join_requests_workspace_id_fkey;
ALTER TABLE join_requests DROP CONSTRAINT IF EXISTS join_requests_user_id_fkey;
ALTER TABLE join_requests DROP CONSTRAINT IF EXISTS join_requests_reviewed_by_user_id_fkey;
