ALTER TABLE ai_drafts
  ADD COLUMN decision_body text NOT NULL DEFAULT '',
  ADD COLUMN agents_md text NOT NULL DEFAULT '';

ALTER TABLE knowledge_items
  ADD COLUMN decision_body text NOT NULL DEFAULT '',
  ADD COLUMN agents_md text NOT NULL DEFAULT '';
