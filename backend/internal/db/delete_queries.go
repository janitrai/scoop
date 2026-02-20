package db

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type SoftDeleteCollectionResult struct {
	RawArrivals int64
	Articles    int64
	Stories     int64
}

type SoftDeleteBeforeResult struct {
	RawArrivals int64
	Articles    int64
	Stories     int64
}

func (p *Pool) SoftDeleteStory(ctx context.Context, storyUUID string, now time.Time) (int64, error) {
	trimmedUUID := strings.TrimSpace(storyUUID)
	if trimmedUUID == "" {
		return 0, fmt.Errorf("story UUID is required")
	}

	tx, err := p.BeginTx(ctx, TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const q = `
UPDATE news.stories
SET
	deleted_at = $2,
	updated_at = $2
WHERE story_uuid = $1::uuid
  AND deleted_at IS NULL
`
	tag, err := tx.Exec(ctx, q, trimmedUUID, now.UTC())
	if err != nil {
		return 0, fmt.Errorf("soft delete story: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit transaction: %w", err)
	}

	return tag.RowsAffected(), nil
}

func (p *Pool) SoftDeleteArticle(ctx context.Context, articleUUID string, now time.Time) (int64, error) {
	trimmedUUID := strings.TrimSpace(articleUUID)
	if trimmedUUID == "" {
		return 0, fmt.Errorf("article UUID is required")
	}

	tx, err := p.BeginTx(ctx, TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const q = `
UPDATE news.articles
SET
	deleted_at = $2,
	updated_at = $2
WHERE article_uuid = $1::uuid
  AND deleted_at IS NULL
`
	tag, err := tx.Exec(ctx, q, trimmedUUID, now.UTC())
	if err != nil {
		return 0, fmt.Errorf("soft delete article: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit transaction: %w", err)
	}

	return tag.RowsAffected(), nil
}

func (p *Pool) SoftDeleteCollection(ctx context.Context, collection string, now time.Time) (SoftDeleteCollectionResult, error) {
	normalizedCollection := strings.TrimSpace(strings.ToLower(collection))
	if normalizedCollection == "" {
		return SoftDeleteCollectionResult{}, fmt.Errorf("collection is required")
	}

	tx, err := p.BeginTx(ctx, TxOptions{})
	if err != nil {
		return SoftDeleteCollectionResult{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var result SoftDeleteCollectionResult

	const rawArrivalsQuery = `
UPDATE news.raw_arrivals
SET deleted_at = $2
WHERE collection = $1
  AND deleted_at IS NULL
`
	if tag, err := tx.Exec(ctx, rawArrivalsQuery, normalizedCollection, now.UTC()); err != nil {
		return SoftDeleteCollectionResult{}, fmt.Errorf("soft delete raw_arrivals collection=%q: %w", normalizedCollection, err)
	} else {
		result.RawArrivals = tag.RowsAffected()
	}

	const articlesQuery = `
UPDATE news.articles
SET
	deleted_at = $2,
	updated_at = $2
WHERE collection = $1
  AND deleted_at IS NULL
`
	if tag, err := tx.Exec(ctx, articlesQuery, normalizedCollection, now.UTC()); err != nil {
		return SoftDeleteCollectionResult{}, fmt.Errorf("soft delete articles collection=%q: %w", normalizedCollection, err)
	} else {
		result.Articles = tag.RowsAffected()
	}

	const storiesQuery = `
UPDATE news.stories
SET
	deleted_at = $2,
	updated_at = $2
WHERE collection = $1
  AND deleted_at IS NULL
`
	if tag, err := tx.Exec(ctx, storiesQuery, normalizedCollection, now.UTC()); err != nil {
		return SoftDeleteCollectionResult{}, fmt.Errorf("soft delete stories collection=%q: %w", normalizedCollection, err)
	} else {
		result.Stories = tag.RowsAffected()
	}

	if err := tx.Commit(ctx); err != nil {
		return SoftDeleteCollectionResult{}, fmt.Errorf("commit transaction: %w", err)
	}

	return result, nil
}

func (p *Pool) SoftDeleteBefore(ctx context.Context, before time.Time, now time.Time) (SoftDeleteBeforeResult, error) {
	beforeUTC := before.UTC()
	if beforeUTC.IsZero() {
		return SoftDeleteBeforeResult{}, fmt.Errorf("before time is required")
	}

	tx, err := p.BeginTx(ctx, TxOptions{})
	if err != nil {
		return SoftDeleteBeforeResult{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var result SoftDeleteBeforeResult

	const rawArrivalsQuery = `
UPDATE news.raw_arrivals
SET deleted_at = $2
WHERE fetched_at < $1
  AND deleted_at IS NULL
`
	if tag, err := tx.Exec(ctx, rawArrivalsQuery, beforeUTC, now.UTC()); err != nil {
		return SoftDeleteBeforeResult{}, fmt.Errorf("soft delete raw_arrivals before=%s: %w", beforeUTC.Format(time.RFC3339), err)
	} else {
		result.RawArrivals = tag.RowsAffected()
	}

	const articlesQuery = `
UPDATE news.articles
SET
	deleted_at = $2,
	updated_at = $2
WHERE created_at < $1
  AND deleted_at IS NULL
`
	if tag, err := tx.Exec(ctx, articlesQuery, beforeUTC, now.UTC()); err != nil {
		return SoftDeleteBeforeResult{}, fmt.Errorf("soft delete articles before=%s: %w", beforeUTC.Format(time.RFC3339), err)
	} else {
		result.Articles = tag.RowsAffected()
	}

	const storiesQuery = `
UPDATE news.stories
SET
	deleted_at = $2,
	updated_at = $2
WHERE last_seen_at < $1
  AND deleted_at IS NULL
`
	if tag, err := tx.Exec(ctx, storiesQuery, beforeUTC, now.UTC()); err != nil {
		return SoftDeleteBeforeResult{}, fmt.Errorf("soft delete stories before=%s: %w", beforeUTC.Format(time.RFC3339), err)
	} else {
		result.Stories = tag.RowsAffected()
	}

	if err := tx.Commit(ctx); err != nil {
		return SoftDeleteBeforeResult{}, fmt.Errorf("commit transaction: %w", err)
	}

	return result, nil
}

func (p *Pool) RestoreStory(ctx context.Context, storyUUID string, now time.Time) (int64, error) {
	trimmedUUID := strings.TrimSpace(storyUUID)
	if trimmedUUID == "" {
		return 0, fmt.Errorf("story UUID is required")
	}

	tx, err := p.BeginTx(ctx, TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const q = `
UPDATE news.stories
SET
	deleted_at = NULL,
	updated_at = $2
WHERE story_uuid = $1::uuid
  AND deleted_at IS NOT NULL
`
	tag, err := tx.Exec(ctx, q, trimmedUUID, now.UTC())
	if err != nil {
		return 0, fmt.Errorf("restore story: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit transaction: %w", err)
	}

	return tag.RowsAffected(), nil
}

func (p *Pool) RestoreArticle(ctx context.Context, articleUUID string, now time.Time) (int64, error) {
	trimmedUUID := strings.TrimSpace(articleUUID)
	if trimmedUUID == "" {
		return 0, fmt.Errorf("article UUID is required")
	}

	tx, err := p.BeginTx(ctx, TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const q = `
UPDATE news.articles
SET
	deleted_at = NULL,
	updated_at = $2
WHERE article_uuid = $1::uuid
  AND deleted_at IS NOT NULL
`
	tag, err := tx.Exec(ctx, q, trimmedUUID, now.UTC())
	if err != nil {
		return 0, fmt.Errorf("restore article: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit transaction: %w", err)
	}

	return tag.RowsAffected(), nil
}
