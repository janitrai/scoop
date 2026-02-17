BEGIN;

SET search_path = news, public;

DO $$
BEGIN
	IF to_regclass('news.documents') IS NOT NULL
		AND to_regclass('news.articles') IS NULL THEN
		ALTER TABLE news.documents RENAME TO articles;
	END IF;

	IF to_regclass('news.document_embeddings') IS NOT NULL
		AND to_regclass('news.article_embeddings') IS NULL THEN
		ALTER TABLE news.document_embeddings RENAME TO article_embeddings;
	END IF;

	IF to_regclass('news.story_members') IS NOT NULL
		AND to_regclass('news.story_articles') IS NULL THEN
		ALTER TABLE news.story_members RENAME TO story_articles;
	END IF;
END
$$;

DO $$
BEGIN
	IF EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = 'news'
			AND table_name = 'articles'
			AND column_name = 'document_id'
	) AND NOT EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = 'news'
			AND table_name = 'articles'
			AND column_name = 'article_id'
	) THEN
		ALTER TABLE news.articles RENAME COLUMN document_id TO article_id;
	END IF;

	IF EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = 'news'
			AND table_name = 'articles'
			AND column_name = 'document_uuid'
	) AND NOT EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = 'news'
			AND table_name = 'articles'
			AND column_name = 'article_uuid'
	) THEN
		ALTER TABLE news.articles RENAME COLUMN document_uuid TO article_uuid;
	END IF;
END
$$;

DO $$
BEGIN
	IF EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = 'news'
			AND table_name = 'article_embeddings'
			AND column_name = 'embedding_id'
	) AND NOT EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = 'news'
			AND table_name = 'article_embeddings'
			AND column_name = 'article_embedding_id'
	) THEN
		ALTER TABLE news.article_embeddings RENAME COLUMN embedding_id TO article_embedding_id;
	END IF;

	IF EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = 'news'
			AND table_name = 'article_embeddings'
			AND column_name = 'document_id'
	) AND NOT EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = 'news'
			AND table_name = 'article_embeddings'
			AND column_name = 'article_id'
	) THEN
		ALTER TABLE news.article_embeddings RENAME COLUMN document_id TO article_id;
	END IF;

	IF EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = 'news'
			AND table_name = 'article_embeddings'
			AND column_name = 'document_embedding_uuid'
	) AND NOT EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = 'news'
			AND table_name = 'article_embeddings'
			AND column_name = 'article_embedding_uuid'
	) THEN
		ALTER TABLE news.article_embeddings RENAME COLUMN document_embedding_uuid TO article_embedding_uuid;
	END IF;
END
$$;

DO $$
BEGIN
	IF EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = 'news'
			AND table_name = 'story_articles'
			AND column_name = 'document_id'
	) AND NOT EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = 'news'
			AND table_name = 'story_articles'
			AND column_name = 'article_id'
	) THEN
		ALTER TABLE news.story_articles RENAME COLUMN document_id TO article_id;
	END IF;

	IF EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = 'news'
			AND table_name = 'story_articles'
			AND column_name = 'story_member_uuid'
	) AND NOT EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = 'news'
			AND table_name = 'story_articles'
			AND column_name = 'story_article_uuid'
	) THEN
		ALTER TABLE news.story_articles RENAME COLUMN story_member_uuid TO story_article_uuid;
	END IF;
END
$$;

DO $$
BEGIN
	IF EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = 'news'
			AND table_name = 'stories'
			AND column_name = 'representative_document_id'
	) AND NOT EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = 'news'
			AND table_name = 'stories'
			AND column_name = 'representative_article_id'
	) THEN
		ALTER TABLE news.stories RENAME COLUMN representative_document_id TO representative_article_id;
	END IF;

	IF EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = 'news'
			AND table_name = 'stories'
			AND column_name = 'item_count'
	) AND NOT EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = 'news'
			AND table_name = 'stories'
			AND column_name = 'article_count'
	) THEN
		ALTER TABLE news.stories RENAME COLUMN item_count TO article_count;
	END IF;
END
$$;

DO $$
BEGIN
	IF EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = 'news'
			AND table_name = 'dedup_events'
			AND column_name = 'document_id'
	) AND NOT EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_schema = 'news'
			AND table_name = 'dedup_events'
			AND column_name = 'article_id'
	) THEN
		ALTER TABLE news.dedup_events RENAME COLUMN document_id TO article_id;
	END IF;
END
$$;

ALTER SEQUENCE IF EXISTS news.documents_document_id_seq RENAME TO articles_article_id_seq;
ALTER SEQUENCE IF EXISTS news.document_embeddings_embedding_id_seq RENAME TO article_embeddings_article_embedding_id_seq;

DO $$
BEGIN
	IF to_regclass('news.articles_article_id_seq') IS NOT NULL
		AND EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'news'
				AND table_name = 'articles'
				AND column_name = 'article_id'
		) THEN
		ALTER SEQUENCE news.articles_article_id_seq OWNED BY news.articles.article_id;
		ALTER TABLE news.articles
			ALTER COLUMN article_id SET DEFAULT nextval('news.articles_article_id_seq'::regclass);
	END IF;

	IF to_regclass('news.article_embeddings_article_embedding_id_seq') IS NOT NULL
		AND EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'news'
				AND table_name = 'article_embeddings'
				AND column_name = 'article_embedding_id'
		) THEN
		ALTER SEQUENCE news.article_embeddings_article_embedding_id_seq OWNED BY news.article_embeddings.article_embedding_id;
		ALTER TABLE news.article_embeddings
			ALTER COLUMN article_embedding_id SET DEFAULT nextval('news.article_embeddings_article_embedding_id_seq'::regclass);
	END IF;
END
$$;

ALTER INDEX IF EXISTS news.uq_documents_uuid RENAME TO uq_articles_uuid;
ALTER INDEX IF EXISTS news.uq_document_embeddings_uuid RENAME TO uq_article_embeddings_uuid;
ALTER INDEX IF EXISTS news.uq_story_members_uuid RENAME TO uq_story_articles_uuid;

ALTER INDEX IF EXISTS news.idx_documents_source_item RENAME TO idx_articles_source_item;
ALTER INDEX IF EXISTS news.idx_documents_canonical_url_hash RENAME TO idx_articles_canonical_url_hash;
ALTER INDEX IF EXISTS news.idx_documents_content_hash RENAME TO idx_articles_content_hash;
ALTER INDEX IF EXISTS news.idx_documents_source_domain_published RENAME TO idx_articles_source_domain_published;
ALTER INDEX IF EXISTS news.idx_documents_created_at RENAME TO idx_articles_created_at;
ALTER INDEX IF EXISTS news.idx_documents_title_simhash RENAME TO idx_articles_title_simhash;
ALTER INDEX IF EXISTS news.idx_documents_collection_source_item RENAME TO idx_articles_collection_source_item;
ALTER INDEX IF EXISTS news.idx_documents_collection_canonical_url_hash RENAME TO idx_articles_collection_canonical_url_hash;
ALTER INDEX IF EXISTS news.idx_documents_collection_content_hash RENAME TO idx_articles_collection_content_hash;
ALTER INDEX IF EXISTS news.idx_documents_collection_created_at RENAME TO idx_articles_collection_created_at;

ALTER INDEX IF EXISTS news.idx_document_embeddings_document RENAME TO idx_article_embeddings_article;
ALTER INDEX IF EXISTS news.idx_document_embeddings_model_time RENAME TO idx_article_embeddings_model_time;
ALTER INDEX IF EXISTS news.idx_document_embeddings_embedded_at RENAME TO idx_article_embeddings_embedded_at;
ALTER INDEX IF EXISTS news.idx_document_embeddings_hnsw RENAME TO idx_article_embeddings_hnsw;

ALTER INDEX IF EXISTS news.idx_story_members_story RENAME TO idx_story_articles_story;
ALTER INDEX IF EXISTS news.idx_story_members_match_type RENAME TO idx_story_articles_match_type;

DO $$
DECLARE
	r RECORD;
	new_name TEXT;
BEGIN
	FOR r IN
		SELECT c.conname
		FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = 'news'
			AND t.relname = 'articles'
			AND c.conname LIKE 'documents_%'
	LOOP
		new_name := regexp_replace(r.conname, '^documents_', 'articles_');
		IF NOT EXISTS (
			SELECT 1
			FROM pg_constraint c2
			WHERE c2.conrelid = 'news.articles'::regclass
				AND c2.conname = new_name
		) THEN
			EXECUTE format('ALTER TABLE news.articles RENAME CONSTRAINT %I TO %I', r.conname, new_name);
		END IF;
	END LOOP;

	FOR r IN
		SELECT c.conname
		FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = 'news'
			AND t.relname = 'article_embeddings'
			AND c.conname LIKE 'document_embeddings_%'
	LOOP
		new_name := regexp_replace(r.conname, '^document_embeddings_', 'article_embeddings_');
		IF NOT EXISTS (
			SELECT 1
			FROM pg_constraint c2
			WHERE c2.conrelid = 'news.article_embeddings'::regclass
				AND c2.conname = new_name
		) THEN
			EXECUTE format('ALTER TABLE news.article_embeddings RENAME CONSTRAINT %I TO %I', r.conname, new_name);
		END IF;
	END LOOP;

	FOR r IN
		SELECT c.conname
		FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = 'news'
			AND t.relname = 'story_articles'
			AND c.conname LIKE 'story_members_%'
	LOOP
		new_name := regexp_replace(r.conname, '^story_members_', 'story_articles_');
		IF NOT EXISTS (
			SELECT 1
			FROM pg_constraint c2
			WHERE c2.conrelid = 'news.story_articles'::regclass
				AND c2.conname = new_name
		) THEN
			EXECUTE format('ALTER TABLE news.story_articles RENAME CONSTRAINT %I TO %I', r.conname, new_name);
		END IF;
	END LOOP;
END
$$;

DO $$
DECLARE
	r RECORD;
	new_name TEXT;
BEGIN
	FOR r IN
		SELECT c.conname, t.relname AS table_name
		FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = 'news'
			AND t.relname IN ('article_embeddings', 'story_articles', 'dedup_events', 'stories')
			AND c.conname LIKE '%document_id%'
	LOOP
		new_name := replace(r.conname, 'document_id', 'article_id');
		IF NOT EXISTS (
			SELECT 1
			FROM pg_constraint c2
			JOIN pg_class t2 ON t2.oid = c2.conrelid
			JOIN pg_namespace n2 ON n2.oid = t2.relnamespace
			WHERE n2.nspname = 'news'
				AND t2.relname = r.table_name
				AND c2.conname = new_name
		) THEN
			EXECUTE format('ALTER TABLE news.%I RENAME CONSTRAINT %I TO %I', r.table_name, r.conname, new_name);
		END IF;
	END LOOP;
END
$$;

DO $$
DECLARE
	r RECORD;
	new_name TEXT;
BEGIN
	FOR r IN
		SELECT c.relname AS index_name
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'news'
			AND c.relkind = 'i'
			AND c.relname IN (
				'idx_article_embeddings_document',
				'story_articles_document_id_key',
				'dedup_events_document_id_key'
			)
	LOOP
		new_name := replace(r.index_name, 'document_id', 'article_id');
		new_name := replace(new_name, '_document', '_article');
		IF to_regclass('news.' || new_name) IS NULL THEN
			EXECUTE format('ALTER INDEX news.%I RENAME TO %I', r.index_name, new_name);
		END IF;
	END LOOP;
END
$$;

DO $$
BEGIN
	IF EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'uq_document_embeddings_doc_model'
			AND conrelid = 'news.article_embeddings'::regclass
	) AND NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'uq_article_embeddings_article_model'
			AND conrelid = 'news.article_embeddings'::regclass
	) THEN
		ALTER TABLE news.article_embeddings
			RENAME CONSTRAINT uq_document_embeddings_doc_model TO uq_article_embeddings_article_model;
	END IF;

	IF EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'stories_representative_document_id_fkey'
			AND conrelid = 'news.stories'::regclass
	) AND NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'stories_representative_article_id_fkey'
			AND conrelid = 'news.stories'::regclass
	) THEN
		ALTER TABLE news.stories
			RENAME CONSTRAINT stories_representative_document_id_fkey TO stories_representative_article_id_fkey;
	END IF;

	IF EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'stories_item_count_check'
			AND conrelid = 'news.stories'::regclass
	) AND NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'stories_article_count_check'
			AND conrelid = 'news.stories'::regclass
	) THEN
		ALTER TABLE news.stories
			RENAME CONSTRAINT stories_item_count_check TO stories_article_count_check;
	END IF;
END
$$;

COMMENT ON TABLE news.articles IS 'Normalized canonical articles derived from raw arrivals.';
COMMENT ON TABLE news.article_embeddings IS 'Embedding vectors per article and embedding model/version.';
COMMENT ON TABLE news.story_articles IS 'Membership mapping from normalized article to canonical story.';
COMMENT ON COLUMN news.stories.representative_article_id IS 'Best article used for title/url when presenting the story.';

COMMIT;
