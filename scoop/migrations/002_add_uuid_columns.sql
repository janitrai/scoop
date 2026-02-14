BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;
SET search_path = news, public;

ALTER TABLE news.ingest_runs ADD COLUMN IF NOT EXISTS ingest_run_uuid UUID;
UPDATE news.ingest_runs
SET ingest_run_uuid = gen_random_uuid()
WHERE ingest_run_uuid IS NULL;
ALTER TABLE news.ingest_runs ALTER COLUMN ingest_run_uuid SET DEFAULT gen_random_uuid();
ALTER TABLE news.ingest_runs ALTER COLUMN ingest_run_uuid SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_ingest_runs_uuid ON news.ingest_runs (ingest_run_uuid);

ALTER TABLE news.source_checkpoints ADD COLUMN IF NOT EXISTS source_checkpoint_uuid UUID;
UPDATE news.source_checkpoints
SET source_checkpoint_uuid = gen_random_uuid()
WHERE source_checkpoint_uuid IS NULL;
ALTER TABLE news.source_checkpoints ALTER COLUMN source_checkpoint_uuid SET DEFAULT gen_random_uuid();
ALTER TABLE news.source_checkpoints ALTER COLUMN source_checkpoint_uuid SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_source_checkpoints_uuid ON news.source_checkpoints (source_checkpoint_uuid);

ALTER TABLE news.raw_arrivals ADD COLUMN IF NOT EXISTS raw_arrival_uuid UUID;
UPDATE news.raw_arrivals
SET raw_arrival_uuid = gen_random_uuid()
WHERE raw_arrival_uuid IS NULL;
ALTER TABLE news.raw_arrivals ALTER COLUMN raw_arrival_uuid SET DEFAULT gen_random_uuid();
ALTER TABLE news.raw_arrivals ALTER COLUMN raw_arrival_uuid SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_raw_arrivals_uuid ON news.raw_arrivals (raw_arrival_uuid);

ALTER TABLE news.documents ADD COLUMN IF NOT EXISTS document_uuid UUID;
UPDATE news.documents
SET document_uuid = gen_random_uuid()
WHERE document_uuid IS NULL;
ALTER TABLE news.documents ALTER COLUMN document_uuid SET DEFAULT gen_random_uuid();
ALTER TABLE news.documents ALTER COLUMN document_uuid SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_documents_uuid ON news.documents (document_uuid);

ALTER TABLE news.document_embeddings ADD COLUMN IF NOT EXISTS document_embedding_uuid UUID;
UPDATE news.document_embeddings
SET document_embedding_uuid = gen_random_uuid()
WHERE document_embedding_uuid IS NULL;
ALTER TABLE news.document_embeddings ALTER COLUMN document_embedding_uuid SET DEFAULT gen_random_uuid();
ALTER TABLE news.document_embeddings ALTER COLUMN document_embedding_uuid SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_document_embeddings_uuid ON news.document_embeddings (document_embedding_uuid);

ALTER TABLE news.stories ADD COLUMN IF NOT EXISTS story_uuid UUID;
UPDATE news.stories
SET story_uuid = gen_random_uuid()
WHERE story_uuid IS NULL;
ALTER TABLE news.stories ALTER COLUMN story_uuid SET DEFAULT gen_random_uuid();
ALTER TABLE news.stories ALTER COLUMN story_uuid SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_stories_uuid ON news.stories (story_uuid);

ALTER TABLE news.story_members ADD COLUMN IF NOT EXISTS story_member_uuid UUID;
UPDATE news.story_members
SET story_member_uuid = gen_random_uuid()
WHERE story_member_uuid IS NULL;
ALTER TABLE news.story_members ALTER COLUMN story_member_uuid SET DEFAULT gen_random_uuid();
ALTER TABLE news.story_members ALTER COLUMN story_member_uuid SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_story_members_uuid ON news.story_members (story_member_uuid);

ALTER TABLE news.dedup_events ADD COLUMN IF NOT EXISTS dedup_event_uuid UUID;
UPDATE news.dedup_events
SET dedup_event_uuid = gen_random_uuid()
WHERE dedup_event_uuid IS NULL;
ALTER TABLE news.dedup_events ALTER COLUMN dedup_event_uuid SET DEFAULT gen_random_uuid();
ALTER TABLE news.dedup_events ALTER COLUMN dedup_event_uuid SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_dedup_events_uuid ON news.dedup_events (dedup_event_uuid);

ALTER TABLE news.topics ADD COLUMN IF NOT EXISTS topic_uuid UUID;
UPDATE news.topics
SET topic_uuid = gen_random_uuid()
WHERE topic_uuid IS NULL;
ALTER TABLE news.topics ALTER COLUMN topic_uuid SET DEFAULT gen_random_uuid();
ALTER TABLE news.topics ALTER COLUMN topic_uuid SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_topics_uuid ON news.topics (topic_uuid);

ALTER TABLE news.topic_source_rules ADD COLUMN IF NOT EXISTS topic_source_rule_uuid UUID;
UPDATE news.topic_source_rules
SET topic_source_rule_uuid = gen_random_uuid()
WHERE topic_source_rule_uuid IS NULL;
ALTER TABLE news.topic_source_rules ALTER COLUMN topic_source_rule_uuid SET DEFAULT gen_random_uuid();
ALTER TABLE news.topic_source_rules ALTER COLUMN topic_source_rule_uuid SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_topic_source_rules_uuid ON news.topic_source_rules (topic_source_rule_uuid);

ALTER TABLE news.topic_keyword_rules ADD COLUMN IF NOT EXISTS topic_keyword_rule_uuid UUID;
UPDATE news.topic_keyword_rules
SET topic_keyword_rule_uuid = gen_random_uuid()
WHERE topic_keyword_rule_uuid IS NULL;
ALTER TABLE news.topic_keyword_rules ALTER COLUMN topic_keyword_rule_uuid SET DEFAULT gen_random_uuid();
ALTER TABLE news.topic_keyword_rules ALTER COLUMN topic_keyword_rule_uuid SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_topic_keyword_rules_uuid ON news.topic_keyword_rules (topic_keyword_rule_uuid);

ALTER TABLE news.story_topic_state ADD COLUMN IF NOT EXISTS story_topic_state_uuid UUID;
UPDATE news.story_topic_state
SET story_topic_state_uuid = gen_random_uuid()
WHERE story_topic_state_uuid IS NULL;
ALTER TABLE news.story_topic_state ALTER COLUMN story_topic_state_uuid SET DEFAULT gen_random_uuid();
ALTER TABLE news.story_topic_state ALTER COLUMN story_topic_state_uuid SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_story_topic_state_uuid ON news.story_topic_state (story_topic_state_uuid);

ALTER TABLE news.digest_runs ADD COLUMN IF NOT EXISTS digest_run_uuid UUID;
UPDATE news.digest_runs
SET digest_run_uuid = gen_random_uuid()
WHERE digest_run_uuid IS NULL;
ALTER TABLE news.digest_runs ALTER COLUMN digest_run_uuid SET DEFAULT gen_random_uuid();
ALTER TABLE news.digest_runs ALTER COLUMN digest_run_uuid SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_digest_runs_uuid ON news.digest_runs (digest_run_uuid);

ALTER TABLE news.digest_entries ADD COLUMN IF NOT EXISTS digest_entry_uuid UUID;
UPDATE news.digest_entries
SET digest_entry_uuid = gen_random_uuid()
WHERE digest_entry_uuid IS NULL;
ALTER TABLE news.digest_entries ALTER COLUMN digest_entry_uuid SET DEFAULT gen_random_uuid();
ALTER TABLE news.digest_entries ALTER COLUMN digest_entry_uuid SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_digest_entries_uuid ON news.digest_entries (digest_entry_uuid);

COMMIT;
