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

## Scheduled Daily Digest

This skill is designed to be triggered by a daily cron job. The cron job runs the fetch, the agent curates and posts the digest to the configured Discord channel.
