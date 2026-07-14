UPDATE ai_drafts
SET decision_body = CASE
  WHEN reason IS NOT NULL AND reason != '' THEN
    '## Decision: ' || suggested_title || E'\n\n' ||
    '**Summary:** ' || summary || E'\n\n' ||
    '**Reason:** ' || reason
  ELSE '## Decision: ' || suggested_title || E'\n\n' || summary
  END,
  agents_md = CASE
  WHEN reason IS NOT NULL AND reason != '' THEN
    '- **' || suggested_title || '**: ' || summary || E'\n  - Importance: ' || importance || E'/4\n  - Reason: ' || reason
  ELSE '- **' || suggested_title || '**: ' || summary || E'\n  - Importance: ' || importance || '/4'
  END
WHERE decision_body = '' AND agents_md = '';

UPDATE knowledge_items
SET decision_body = CASE
  WHEN body IS NOT NULL AND body != '' AND body != summary THEN
    '## Decision: ' || title || E'\n\n' ||
    '**Summary:** ' || summary || E'\n\n' ||
    '**Details:** ' || body
  ELSE '## Decision: ' || title || E'\n\n' || summary
  END,
  agents_md = CASE
  WHEN body IS NOT NULL AND body != '' AND body != summary THEN
    '- **' || title || '**: ' || summary || E'\n  - Importance: ' || importance || E'/4\n  - Details: ' || body
  ELSE '- **' || title || '**: ' || summary || E'\n  - Importance: ' || importance || '/4'
  END
WHERE decision_body = '' AND agents_md = '';
