DROP INDEX IF EXISTS ai_drafts_project_id_created_at_idx;
DROP INDEX IF EXISTS knowledge_items_event_occurred_at_idx;

ALTER TABLE knowledge_items DROP COLUMN IF EXISTS event_occurred_at;

ALTER TABLE ai_drafts
  DROP COLUMN IF EXISTS next_draft_id,
  DROP COLUMN IF EXISTS previous_draft_id;
