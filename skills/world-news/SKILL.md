---
name: world-news
description: Gather and compile world news from multiple sources (Reddit r/worldnews, Sentinel Team global risks) into a curated daily digest. Use when asked for world news, global news digest, what's happening in the world, geopolitical updates, or when a scheduled daily news job triggers.
---

# World News

Fetch world news from multiple sources and compile a curated digest for High Command.

## Quick Start

1. Run the fetch script to gather raw stories:
   ```bash
   python3 scripts/fetch_news.py --hours 24 --format markdown
   ```
2. Review the raw output
3. Curate: pick the top 10-15 most important stories
4. Write a digest with concise summaries and links
5. Deliver to the configured channel

## Digest Format

Structure the digest as:

```
# World News Digest — YYYY-MM-DD

## 🌍 Headlines
Top 3-5 stories that matter most today. Each gets 1-2 sentence summary.

## ⚠️ Global Risks
Sentinel Team alerts, risk assessments, and early warnings.

## 🔗 Quick Links
Remaining notable stories as bullet points with links.
```

## Curation Guidelines

- **Prioritize**: Armed conflicts, major policy changes, elections, natural disasters, international agreements
- **Deprioritize**: Celebrity news, sports, local crime stories, opinion pieces
- **Summarize**: Write neutral, factual summaries — no editorializing
- **Deduplicate**: Same event from multiple sources → pick best source
- **Context**: Add brief context for ongoing situations (e.g. "Day 3 of...")
- **Timestamps**: Include the publication date/time for each story (e.g. "Feb 8", "2h ago"). For commentary/analysis, note when the underlying event happened if different from publication date

## Sources

See `references/sources.md` for full source list and configuration.

Core sources: Reddit r/worldnews (community-curated), Sentinel Team (global risks foresight).

## Script Reference

```
python3 scripts/fetch_news.py [--hours N] [--max N] [--format markdown|json]

  --hours   Lookback window in hours (default: 24)
  --max     Max stories per source (default: 30)
  --format  Output format: markdown or json
```

## Scoop Pipeline Integration

This skill integrates with the **Scoop** NEWSINT pipeline for ingestion, embedding, and semantic dedup.

### Prerequisites
- PostgreSQL with `news` schema running (DB creds in `~/scoop/backend/.env`)
- Embedding service running on port 8844 (Qwen3-Embedding-8B):
  ```bash
  cd ~/scoop/embedding-service && /home/bob/janitr/scripts/.venv/bin/python3 main.py --backend transformers --port 8844 --server
  ```
- Scoop binary built: `cd ~/scoop/backend && go build -o /tmp/scoop ./cmd/scoop/`

### Ingest + Dedup Pipeline

```bash
# 1. Fetch raw stories as JSON
cd ~/scoop/skills/world-news
python3 scripts/fetch_news.py --hours 26 --max 50 --format json > /tmp/world-news-raw.json

# 2. Convert to canonical schema and ingest each item (collection = "world_news")
#    Items without a parseable date MUST be skipped (never default to today)

# 3. Run the full pipeline (normalize → embed → dedup)
cd ~/scoop/backend && source .env
/tmp/scoop run-once

# 4. Get today's digest stories (today + yesterday for cross-day dedup context)
/tmp/scoop digest --collection world_news --format json
# Returns: {date, collection, today: [...stories], yesterday: [...stories]}
```

### Scoop CLI Reference
```
# Write-side (pipeline)
/tmp/scoop ingest --payload-file <path.json>    # Ingest one item
/tmp/scoop run-once                              # Full pipeline cycle (normalize+embed+dedup)

# Read-side (querying)
/tmp/scoop digest --collection <name> [--date YYYY-MM-DD] [--format json|table]
/tmp/scoop stories [--collection <name>] [--from YYYY-MM-DD] [--to YYYY-MM-DD] [--limit N] [--format json|table]
/tmp/scoop story <uuid> [--format json|table]    # Detail view with merged articles
/tmp/scoop stats [--format json|table]           # Per-collection counts + pipeline throughput
/tmp/scoop collections [--format json|table]     # List collections with counts
/tmp/scoop search --query <text> [--collection <name>] [--limit N] [--format json|table]
/tmp/scoop articles [--collection <name>] [--from YYYY-MM-DD] [--to YYYY-MM-DD] [--limit N] [--format json|table]
```

### Important
- **Never fake dates**: if a story has no parseable publication date, skip it
- **Embedding service must be running**: embed step will fail without it
- The scoop dedup uses cosine similarity ≥0.935 for auto-merge, 0.89-0.935 for gray zone
- Collection for world news is `world_news`

## Scheduled Daily Digest

This skill is triggered by a daily cron job. The workflow:
1. Fetch from all sources via `fetch_news.py`
2. Ingest into scoop pipeline for semantic dedup
3. Get digest stories: `/tmp/scoop digest --collection world_news --format json`
4. Curate top 10-15 unique stories from today, excluding themes from yesterday
5. Post digest to Discord #world-news channel
