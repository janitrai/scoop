CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE SCHEMA IF NOT EXISTS news;

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_type t
		JOIN pg_namespace n ON n.oid = t.typnamespace
		WHERE n.nspname = 'news' AND t.typname = 'ingest_run_status'
	) THEN
		CREATE TYPE news.ingest_run_status AS ENUM ('running', 'completed', 'failed');
	END IF;

	IF NOT EXISTS (
		SELECT 1
		FROM pg_type t
		JOIN pg_namespace n ON n.oid = t.typnamespace
		WHERE n.nspname = 'news' AND t.typname = 'story_match_type'
	) THEN
		CREATE TYPE news.story_match_type AS ENUM (
			'seed',
			'exact_url',
			'exact_source_id',
			'exact_content_hash',
			'lexical_simhash',
			'lexical_overlap',
			'semantic',
			'manual'
		);
	END IF;

	IF NOT EXISTS (
		SELECT 1
		FROM pg_type t
		JOIN pg_namespace n ON n.oid = t.typnamespace
		WHERE n.nspname = 'news' AND t.typname = 'dedup_decision'
	) THEN
		CREATE TYPE news.dedup_decision AS ENUM ('new_story', 'auto_merge', 'gray_zone', 'manual_merge', 'manual_split');
	END IF;

	IF NOT EXISTS (
		SELECT 1
		FROM pg_type t
		JOIN pg_namespace n ON n.oid = t.typnamespace
		WHERE n.nspname = 'news' AND t.typname = 'topic_rule_type'
	) THEN
		CREATE TYPE news.topic_rule_type AS ENUM ('include', 'exclude');
	END IF;

	IF NOT EXISTS (
		SELECT 1
		FROM pg_type t
		JOIN pg_namespace n ON n.oid = t.typnamespace
		WHERE n.nspname = 'news' AND t.typname = 'digest_run_status'
	) THEN
		CREATE TYPE news.digest_run_status AS ENUM ('running', 'completed', 'failed');
	END IF;

	IF NOT EXISTS (
		SELECT 1
		FROM pg_type t
		JOIN pg_namespace n ON n.oid = t.typnamespace
		WHERE n.nspname = 'news' AND t.typname = 'digest_entry_status'
	) THEN
		CREATE TYPE news.digest_entry_status AS ENUM (
			'new',
			'seen',
			'suppressed_duplicate',
			'suppressed_manual',
			'possible_duplicate'
		);
	END IF;
END
$$;

CREATE OR REPLACE FUNCTION news.touch_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
	NEW.updated_at = now();
	RETURN NEW;
END;
$$;
