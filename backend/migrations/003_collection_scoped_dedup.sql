BEGIN;

SET search_path = news, public;

ALTER TABLE news.raw_arrivals
	ADD COLUMN IF NOT EXISTS collection TEXT;

UPDATE news.raw_arrivals
SET collection = NULLIF(BTRIM(raw_payload #>> '{source_metadata,collection}'), '')
WHERE collection IS NULL;

UPDATE news.raw_arrivals
SET collection = 'unknown'
WHERE collection IS NULL;

ALTER TABLE news.raw_arrivals
	ALTER COLUMN collection SET NOT NULL;

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'raw_arrivals_collection_nonempty'
			AND conrelid = 'news.raw_arrivals'::regclass
	) THEN
		ALTER TABLE news.raw_arrivals
			ADD CONSTRAINT raw_arrivals_collection_nonempty
			CHECK (length(trim(collection)) > 0);
	END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_raw_arrivals_collection_fetched
	ON news.raw_arrivals (collection, fetched_at DESC);

ALTER TABLE news.documents
	ADD COLUMN IF NOT EXISTS collection TEXT;

UPDATE news.documents d
SET collection = ra.collection
FROM news.raw_arrivals ra
WHERE d.raw_arrival_id = ra.raw_arrival_id
	AND d.collection IS NULL;

UPDATE news.documents
SET collection = 'unknown'
WHERE collection IS NULL;

ALTER TABLE news.documents
	ALTER COLUMN collection SET NOT NULL;

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'documents_collection_nonempty'
			AND conrelid = 'news.documents'::regclass
	) THEN
		ALTER TABLE news.documents
			ADD CONSTRAINT documents_collection_nonempty
			CHECK (length(trim(collection)) > 0);
	END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_documents_collection_source_item
	ON news.documents (collection, source, source_item_id);

CREATE INDEX IF NOT EXISTS idx_documents_collection_canonical_url_hash
	ON news.documents (collection, canonical_url_hash)
	WHERE canonical_url_hash IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_documents_collection_content_hash
	ON news.documents (collection, content_hash);

CREATE INDEX IF NOT EXISTS idx_documents_collection_created_at
	ON news.documents (collection, created_at DESC);

ALTER TABLE news.stories
	ADD COLUMN IF NOT EXISTS collection TEXT;

UPDATE news.stories s
SET collection = d.collection
FROM news.documents d
WHERE s.collection IS NULL
	AND s.representative_document_id = d.document_id;

UPDATE news.stories s
SET collection = x.collection
FROM (
	SELECT sm.story_id, MIN(d.collection) AS collection
	FROM news.story_members sm
	JOIN news.documents d ON d.document_id = sm.document_id
	GROUP BY sm.story_id
) x
WHERE s.collection IS NULL
	AND s.story_id = x.story_id;

UPDATE news.stories
SET collection = 'unknown'
WHERE collection IS NULL;

ALTER TABLE news.stories
	ALTER COLUMN collection SET NOT NULL;

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'stories_collection_nonempty'
			AND conrelid = 'news.stories'::regclass
	) THEN
		ALTER TABLE news.stories
			ADD CONSTRAINT stories_collection_nonempty
			CHECK (length(trim(collection)) > 0);
	END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_stories_collection_last_seen
	ON news.stories (collection, last_seen_at DESC);

CREATE INDEX IF NOT EXISTS idx_stories_collection_canonical_url_hash
	ON news.stories (collection, canonical_url_hash)
	WHERE canonical_url_hash IS NOT NULL;

COMMENT ON COLUMN news.raw_arrivals.collection IS 'Logical collection label (e.g. ai_news/world_news/china_news) from source_metadata.collection.';
COMMENT ON COLUMN news.documents.collection IS 'Normalized collection label carried from raw_arrivals.';
COMMENT ON COLUMN news.stories.collection IS 'Collection boundary for dedup; stories are only merged within the same collection.';

COMMIT;
