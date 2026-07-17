-- Missing indexes for performance
CREATE INDEX IF NOT EXISTS knowledge_items_commit_id_idx ON knowledge_items (commit_id);
CREATE INDEX IF NOT EXISTS knowledge_items_pull_request_id_idx ON knowledge_items (pull_request_id);
CREATE INDEX IF NOT EXISTS webhook_events_status_received_idx ON webhook_events (processing_status, received_at);

-- Auto-update search_vector on knowledge_items insert/update
CREATE OR REPLACE FUNCTION update_knowledge_search_vector()
RETURNS trigger AS $$
BEGIN
  NEW.search_vector := to_tsvector('english',
    coalesce(NEW.title, '') || ' ' ||
    coalesce(NEW.summary, '') || ' ' ||
    coalesce(NEW.body, '')
  );
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_knowledge_search_vector ON knowledge_items;
CREATE TRIGGER trg_knowledge_search_vector
  BEFORE INSERT OR UPDATE ON knowledge_items
  FOR EACH ROW
  EXECUTE FUNCTION update_knowledge_search_vector();

-- Missing foreign key constraints for data integrity
ALTER TABLE join_requests
  DROP CONSTRAINT IF EXISTS join_requests_workspace_id_fkey,
  ADD CONSTRAINT join_requests_workspace_id_fkey
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE;

ALTER TABLE join_requests
  DROP CONSTRAINT IF EXISTS join_requests_user_id_fkey,
  ADD CONSTRAINT join_requests_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE join_requests
  DROP CONSTRAINT IF EXISTS join_requests_reviewed_by_user_id_fkey,
  ADD CONSTRAINT join_requests_reviewed_by_user_id_fkey
    FOREIGN KEY (reviewed_by_user_id) REFERENCES users(id) ON DELETE SET NULL;
