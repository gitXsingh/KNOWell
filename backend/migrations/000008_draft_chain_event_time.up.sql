ALTER TABLE ai_drafts
  ADD COLUMN previous_draft_id uuid REFERENCES ai_drafts(id) ON DELETE SET NULL,
  ADD COLUMN next_draft_id uuid REFERENCES ai_drafts(id) ON DELETE SET NULL;

ALTER TABLE knowledge_items
  ADD COLUMN event_occurred_at timestamptz;

CREATE INDEX IF NOT EXISTS ai_drafts_project_id_created_at_idx ON ai_drafts (project_id, created_at);
CREATE INDEX IF NOT EXISTS knowledge_items_event_occurred_at_idx ON knowledge_items (event_occurred_at);
