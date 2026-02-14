# World News Sources Reference

## Source Categories

### Reddit r/worldnews (Community)
- **API**: `https://www.reddit.com/r/worldnews/{sort}.json`
- **Sorts**: `hot`, `top` (with `?t=day` for daily top)
- **Strength**: Community-curated, high volume, real-time breaking news
- **Note**: Score and comment count indicate story importance

### Sentinel Team (Global Risks)
- **Feed**: `https://sentinelteam.substack.com/feed`
- **Website**: [sentinel-team.org](https://sentinel-team.org)
- **Weekly brief**: [xrisk.fyi](https://xrisk.fyi)
- **Strength**: Expert foresight team focused on global catastrophic risks
- **Frequency**: Weekly roundup + occasional special reports
- **Note**: Covers geopolitical, biosecurity, AI safety, and existential risk angles

### Kyla Scanlon (Economics / Markets)
- **Feed**: `https://kylascanlon.substack.com/feed`
- **Website**: [kylascanlon.com](https://kylascanlon.com)
- **Strength**: Macro economics, markets, Fed policy, economic sentiment. Coined "vibecession"
- **Frequency**: Multiple times per week
- **Note**: Covers the gap between economic data and public perception. Writes for NYT, WSJ, Foreign Policy

## Adding New Sources

To add a new RSS source, edit the `RSS_FEEDS` dict in `scripts/fetch_news.py`.
For Reddit subreddits, modify the `fetch_reddit_worldnews` function.

## Source Priority for Digest

When curating the digest, prioritize:
1. **Breaking news** - Major geopolitical events, conflicts, disasters
2. **High-impact policy** - International agreements, sanctions, elections
3. **Sentinel alerts** - Global risk assessments and early warnings
4. **Trending stories** - High-score Reddit posts indicating public interest
