package db

import (
	"context"
	"fmt"
	"time"
)

// StatsCollectionCount stores per-collection pipeline counts.
type StatsCollectionCount struct {
	Collection string `json:"collection"`
	Articles   int64  `json:"articles"`
	Stories    int64  `json:"stories"`
	Embeddings int64  `json:"embeddings"`
}

// StatsTotals stores totals across collections.
type StatsTotals struct {
	Articles   int64 `json:"articles"`
	Stories    int64 `json:"stories"`
	Embeddings int64 `json:"embeddings"`
}

// PipelineThroughput stores throughput/pending counters.
type PipelineThroughput struct {
	ArticlesIngestedToday int64 `json:"articles_ingested_today"`
	StoriesCreatedToday   int64 `json:"stories_created_today"`
	PendingNotEmbedded    int64 `json:"pending_not_embedded"`
	PendingNotDeduped     int64 `json:"pending_not_deduped"`
}

// PipelineStats is the read model returned by the stats command.
type PipelineStats struct {
	Day         string                 `json:"day"`
	Collections []StatsCollectionCount `json:"collections"`
	Totals      StatsTotals            `json:"totals"`
	Throughput  PipelineThroughput     `json:"throughput"`
}

// QueryPipelineStats returns per-collection and total counts plus daily throughput.
func (p *Pool) QueryPipelineStats(ctx context.Context, dayStart, dayEnd time.Time) (*PipelineStats, error) {
	startUTC := dayStart.UTC()
	endUTC := dayEnd.UTC()
	if !startUTC.Before(endUTC) {
		return nil, fmt.Errorf("dayStart must be before dayEnd")
	}

	stats := &PipelineStats{
		Day:         startUTC.Format("2006-01-02"),
		Collections: make([]StatsCollectionCount, 0, 16),
	}

	const countsQuery = `
WITH article_counts AS (
	SELECT a.collection, COUNT(*)::BIGINT AS articles
	FROM news.articles a
	WHERE a.deleted_at IS NULL
	GROUP BY a.collection
),
story_counts AS (
	SELECT s.collection, COUNT(*)::BIGINT AS stories
	FROM news.stories s
	WHERE s.deleted_at IS NULL
	GROUP BY s.collection
),
embedding_counts AS (
	SELECT a.collection, COUNT(*)::BIGINT AS embeddings
	FROM news.article_embeddings ae
	JOIN news.articles a
		ON a.article_id = ae.article_id
	WHERE a.deleted_at IS NULL
	GROUP BY a.collection
)
SELECT
	COALESCE(a.collection, s.collection, e.collection) AS collection,
	COALESCE(a.articles, 0) AS articles,
	COALESCE(s.stories, 0) AS stories,
	COALESCE(e.embeddings, 0) AS embeddings
FROM article_counts a
FULL OUTER JOIN story_counts s
	ON s.collection = a.collection
FULL OUTER JOIN embedding_counts e
	ON e.collection = COALESCE(a.collection, s.collection)
ORDER BY 1
`

	rows, err := p.Query(ctx, countsQuery)
	if err != nil {
		return nil, fmt.Errorf("query stats collection counts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var row StatsCollectionCount
		if err := rows.Scan(&row.Collection, &row.Articles, &row.Stories, &row.Embeddings); err != nil {
			return nil, fmt.Errorf("scan stats collection row: %w", err)
		}
		stats.Collections = append(stats.Collections, row)
		stats.Totals.Articles += row.Articles
		stats.Totals.Stories += row.Stories
		stats.Totals.Embeddings += row.Embeddings
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stats collection rows: %w", err)
	}

	const throughputQuery = `
SELECT
	(SELECT COUNT(*) FROM news.articles a WHERE a.created_at >= $1 AND a.created_at < $2 AND a.deleted_at IS NULL) AS articles_ingested_today,
	(SELECT COUNT(*) FROM news.stories s WHERE s.created_at >= $1 AND s.created_at < $2 AND s.deleted_at IS NULL) AS stories_created_today,
	(SELECT COUNT(*) FROM news.articles a WHERE a.deleted_at IS NULL AND NOT EXISTS (SELECT 1 FROM news.article_embeddings ae WHERE ae.article_id = a.article_id)) AS pending_not_embedded,
	(SELECT COUNT(*) FROM news.articles a WHERE a.deleted_at IS NULL AND NOT EXISTS (SELECT 1 FROM news.story_articles sa WHERE sa.article_id = a.article_id)) AS pending_not_deduped
`

	if err := p.QueryRow(ctx, throughputQuery, startUTC, endUTC).Scan(
		&stats.Throughput.ArticlesIngestedToday,
		&stats.Throughput.StoriesCreatedToday,
		&stats.Throughput.PendingNotEmbedded,
		&stats.Throughput.PendingNotDeduped,
	); err != nil {
		return nil, fmt.Errorf("query stats throughput: %w", err)
	}

	return stats, nil
}
