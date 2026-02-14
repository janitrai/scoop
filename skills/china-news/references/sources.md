# China News Sources Reference

## YouTube Commentary

### 王剑每日观察 Kim's Observation
- **Channel**: [@wongkim728](https://youtube.com/@wongkim728)
- **Channel ID**: `UC8UCbiPrm2zN9nZHKdTevZA`
- **RSS Feed**: `https://www.youtube.com/feeds/videos.xml?channel_id=UC8UCbiPrm2zN9nZHKdTevZA`
- **Language**: Chinese (Mandarin), titles often bilingual
- **Focus**: Daily China political and economic commentary, news analysis
- **Frequency**: Daily uploads, often multiple per day
- **Note**: Commentary/opinion, not straight news. Summarize key arguments from titles and descriptions

## Reddit

### r/China
- **API**: `https://www.reddit.com/r/China/{sort}.json`
- **Sorts**: `hot`, `top` (with `?t=day` for daily top)
- **Strength**: English-language community discussion, expat perspectives
- **Note**: Score and comment count indicate story importance

### r/worldnews (China-filtered)
- **API**: `https://www.reddit.com/r/worldnews/search.json?q=china+OR+chinese+OR+beijing+OR+xi+jinping+OR+taiwan&sort=relevance&t=day`
- **Strength**: Global perspective on China-related stories

## News Outlets (RSS)

| Source | Feed URL | Notes |
|--------|----------|-------|
| SCMP China | — | No RSS; use `web_fetch("https://www.scmp.com/news/china")` |
| Nikkei Asia China | — | No RSS; use `web_fetch("https://asia.nikkei.com/Politics/China")` |
| Reuters China | `https://www.reuters.com/arc/outboundfeeds/v4/latest/tag:china/?outputType=xml` | RSS ✅ |
| BBC China | `https://feeds.bbci.co.uk/news/world/asia/china/rss.xml` | RSS ✅ |
| Al Jazeera China | `https://www.aljazeera.com/xml/rss/all.xml` | Filter for China keywords |
| The Guardian China | `https://www.theguardian.com/world/china/rss` | RSS ✅ |

### ChinaTalk (Newsletter / Podcast)
- **Feed**: `https://www.chinatalk.media/feed`
- **Website**: [chinatalk.media](https://www.chinatalk.media)
- **Strength**: Deep coverage of technology, China, and US policy. Original analysis + interviews with policymakers
- **Frequency**: Multiple times per week
- **Note**: High-quality long-form analysis, not breaking news

## Adding New Sources

To add a new RSS source, edit the `RSS_FEEDS` dict in `scripts/fetch_news.py`.
To add a new YouTube channel, add its RSS feed URL (format: `https://www.youtube.com/feeds/videos.xml?channel_id=CHANNEL_ID`).

## Source Priority for Digest

When curating the digest, prioritize:
1. **Breaking news** — Major political events, military developments, Taiwan
2. **Economic data** — GDP, trade figures, property market, tech sector
3. **US-China relations** — Tariffs, sanctions, diplomacy
4. **Kim's Observation** — Daily commentary provides good pulse on Chinese discourse
5. **Reddit discussion** — Community-validated important stories
