---
name: ai-news
description: Gather and compile AI news from multiple sources (Hacker News, arxiv, AI lab blogs, tech press) into a curated daily digest. Use when asked for AI news, AI digest, tech news roundup, what's happening in AI, or when a scheduled daily news job triggers.
---

# AI News

Fetch AI news from multiple sources and compile a curated digest for High Command.

## Quick Start

1. Run the fetch script to gather raw stories:
   ```bash
   python3 scripts/fetch_news.py --hours 24 --format markdown
   ```
2. Supplement with web_fetch for sources without RSS:
   - `web_fetch("https://www.anthropic.com/research")` — Anthropic research/news
   - `web_fetch("https://ai.meta.com/blog/")` — Meta AI blog
3. Review the raw output
4. Curate: pick the top 10-15 most important stories
5. Write a digest with one-liner summaries and links
6. Deliver to the configured channel

## Digest Format

Structure the digest as:

```
# AI News Digest — YYYY-MM-DD

## 🔥 Headlines
Top 3-5 stories that matter most today. Each gets 1-2 sentence summary.

## 🏢 Lab Updates
Announcements from OpenAI, Anthropic, Google, Meta, etc.

## 📰 Industry
Notable coverage from tech press.

## 📄 Research
2-3 interesting papers, explained simply.

## 🔗 Quick Links
Remaining stories as bullet points with links.
```

## Curation Guidelines

- **Prioritize**: Model releases, major product launches, policy/regulation, fundraising, open source releases
- **Deprioritize**: Repetitive coverage of the same story (pick best source), opinion pieces, listicles
- **Summarize**: Write summaries in plain language, not marketing speak
- **Deduplicate**: Same story from multiple sources → pick the best one, note others covered it
- **Context**: Add brief context when a story is part of a larger trend
- **Timestamps**: Include the publication date/time for each story (e.g. "Feb 8", "2h ago"). For commentary/analysis pieces, note when the underlying event happened if different from publication date — a video published today may be discussing last week's news

## Sources

See `references/sources.md` for full source list and configuration.

Core sources: Hacker News, arxiv (cs.AI/cs.LG/cs.CL), OpenAI blog, Anthropic blog, Google AI blog, DeepMind blog, Meta AI blog, The Verge AI, TechCrunch AI.

## Script Reference

```
python3 scripts/fetch_news.py [--hours N] [--max N] [--format markdown|json]

  --hours   Lookback window in hours (default: 24)
  --max     Max stories per source (default: 30)
  --format  Output format: markdown or json
```

The script fetches concurrently from all sources, filters for AI relevance, deduplicates, and outputs a structured digest.

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

After fetching raw stories, ingest them into scoop for semantic dedup:

```bash
# 1. Fetch raw stories as JSON
cd ~/scoop/skills/ai-news
python3 scripts/fetch_news.py --hours 24 --max 50 --format json > /tmp/ai-news-raw.json

# 2. Convert to canonical schema and ingest each item
#    Each item needs: payload_version, source, source_item_id, title, canonical_url,
#    published_at (RFC3339), source_domain, source_metadata.collection = "ai_news",
#    source_metadata.job_name, source_metadata.job_run_id, source_metadata.scraped_at
#    Items without a parseable date MUST be skipped (never default to today)

# 3. Run the full pipeline (normalize → embed → dedup)
cd ~/scoop/backend && source .env
/tmp/scoop run-once

# 4. Get today's digest stories (today + yesterday for cross-day dedup context)
/tmp/scoop digest --collection ai_news --format json
# Returns: {date, collection, today: [...stories], yesterday: [...stories]}
```

### Scoop CLI Reference
```
# Write-side (pipeline)
/tmp/scoop ingest --payload-file <path.json>    # Ingest one item
/tmp/scoop normalize                             # Raw arrivals → articles
/tmp/scoop embed                                 # Generate embeddings (requires embedding service)
/tmp/scoop dedup                                 # Semantic dedup → stories
/tmp/scoop run-once                              # Full pipeline cycle (normalize+embed+dedup)
/tmp/scoop serve                                 # HTTP API on port 8090

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
- Collection for AI news is `ai_news`

## Scheduled Daily Digest

This skill is triggered by a daily cron job. The workflow:
1. Fetch from all sources via `fetch_news.py`
2. Ingest into scoop pipeline for semantic dedup
3. Curate top 10-15 unique stories from pipeline output
4. Post digest to Discord #ai-news channel
