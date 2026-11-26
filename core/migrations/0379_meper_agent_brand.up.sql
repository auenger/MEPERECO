-- Rename only default onboarding assistants. Preserve IDs, runtime assignments,
-- user customizations, and historical conversation text.
WITH renamed AS (
    UPDATE agent a
       SET name = 'MEPER Agent',
           instructions = replace(a.instructions, 'Sili Agent', 'MEPER Agent'),
           updated_at = now()
     WHERE a.name = 'Sili Agent'
       AND a.visibility = 'workspace'
       AND a.archived_at IS NULL
       AND (a.instructions LIKE 'You are Sili Agent, the built-in AI assistant for this MEPERStudio workspace.%'
         OR a.instructions LIKE '你是 Sili Agent，这个 MEPERStudio workspace 内置的 AI 助手。%')
       AND NOT EXISTS (
           SELECT 1 FROM agent b WHERE b.workspace_id = a.workspace_id AND b.name = 'MEPER Agent'
       )
    RETURNING a.id
)
UPDATE chat_session
   SET title = '👋 MEPER Agent', updated_at = now()
 WHERE agent_id IN (SELECT id FROM renamed)
   AND is_agent_intro
   AND title = '👋 Sili Agent';
