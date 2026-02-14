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

## Raw News Archive

Every fetch saves raw JSON to `~/playground/ai-news-raw/YYYY-MM-DD.json`. This is the unprocessed data — all stories from all sources, dated.

```bash
# Save raw fetch to archive
python3 scripts/fetch_news.py --hours 24 --max 20 --format markdown > ~/playground/ai-news-raw/$(date +%Y-%m-%d).md
```

This archive feeds into `STATE_OF_AI.md` — but **only the Chairman edits that document**. Never autonomously add to STATE_OF_AI.md.

## Scheduled Daily Digest

This skill is designed to be triggered by a daily cron job. The cron job runs the fetch, saves raw JSON to the archive, curates a digest, and posts it to the configured Discord channel.
