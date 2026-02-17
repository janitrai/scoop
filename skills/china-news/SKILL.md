---
name: china-news
description: Gather and compile China news from multiple sources (YouTube commentators, Reddit, SCMP, Nikkei Asia) into a curated daily digest. Use when asked for China news, China digest, what's happening in China, Chinese politics/economy updates, or when a scheduled daily news job triggers.
---

# China News

Fetch China-focused news from multiple sources and compile a curated digest for High Command.

## Quick Start

1. Run the fetch script to gather raw stories:
   ```bash
   python3 scripts/fetch_news.py --hours 24 --format markdown
   ```
2. Supplement with web_fetch for sources without RSS:
   - `web_fetch("https://www.scmp.com/news/china")` — South China Morning Post
   - `web_fetch("https://asia.nikkei.com/Politics/China")` — Nikkei Asia China
3. Review the raw output
4. Curate: pick the top 10-15 most important stories
5. Write a digest with concise summaries and links
6. Deliver to the configured channel

## Digest Format

```
# China News Digest — YYYY-MM-DD

## 🇨🇳 Headlines
Top 3-5 stories that matter most today. Each gets 1-2 sentence summary.

## 🎥 Commentary
Key takeaways from YouTube commentators (Kim's Observation, etc.)

## 💰 Economy & Trade
Economic data, trade war developments, market moves.

## 🏛️ Politics & Policy
CCP decisions, Xi Jinping, governance, regulations.

## 🌏 Foreign Relations
US-China, Taiwan, Belt & Road, diplomacy.

## 🔗 Quick Links
Remaining stories as bullet points with links.
```

## Curation Guidelines

- **Prioritize**: US-China tensions, Taiwan, economic data, CCP policy, military, tech restrictions, trade
- **Deprioritize**: Celebrity gossip, entertainment, lifestyle pieces
- **Summarize**: Write neutral, factual summaries. Present multiple perspectives when available
- **Deduplicate**: Same story from multiple sources → pick best source
- **Context**: Add brief context for ongoing situations
- **Timestamps**: Include the publication date/time for each story (e.g. "Feb 8", "2h ago"). For commentary/analysis (especially YouTube videos), note when the underlying event happened if different from publication date — a video published today may be discussing last week's news
- **YouTube videos**: Summarize the key points/arguments from the video title and description. Note these are commentary/opinion, not straight news. Always distinguish "published date" from "event date"

## Sources

See `references/sources.md` for full source list and configuration.

Core sources: 王剑每日观察 Kim's Observation (YouTube), Reddit r/China, SCMP, Nikkei Asia, Reuters.

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
cd ~/scoop/skills/china-news
python3 scripts/fetch_news.py --hours 26 --max 50 --format json > /tmp/china-news-raw.json

# 2. Supplement with web_fetch for SCMP and Nikkei Asia (no RSS)

# 3. Convert to canonical schema and ingest each item (collection = "china_news")
#    Items without a parseable date MUST be skipped (never default to today)

# 4. Run the full pipeline (normalize → embed → dedup)
cd ~/scoop/backend && source .env
/tmp/scoop run-once

# 5. Get today's digest stories (today + yesterday for cross-day dedup context)
/tmp/scoop digest --collection china_news --format json
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
/tmp/scoop stats [--format json|table]           # Per-collection counts + pipeline throughput
/tmp/scoop search --query <text> [--collection <name>] [--limit N] [--format json|table]
```

### Important
- **Never fake dates**: if a story has no parseable publication date, skip it
- **Embedding service must be running**: embed step will fail without it
- Collection for China news is `china_news`

## Scheduled Daily Digest

This skill is triggered by a daily cron job. The workflow:
1. Fetch from all sources via `fetch_news.py` + web_fetch for SCMP/Nikkei
2. Ingest into scoop pipeline for semantic dedup
3. Get digest stories: `/tmp/scoop digest --collection china_news --format json`
4. Curate top 10-15 unique stories from today, excluding themes from yesterday
5. Post digest to Discord #china-news channel
