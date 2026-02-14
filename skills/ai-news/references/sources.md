# AI News Sources Reference

## Source Categories

### Hacker News (Community)
- **API**: `https://hacker-news.firebaseio.com/v0/`
- **Endpoints**: `topstories.json`, `beststories.json`, `newstories.json`
- **Filtering**: Keyword-based AI filtering on titles
- **Strength**: Community-curated, high signal-to-noise for trending topics
- **Note**: Score and comment count indicate community interest level

### arxiv (Research)
- **API**: `http://export.arxiv.org/api/query`
- **Categories**: cs.AI (Artificial Intelligence), cs.LG (Machine Learning), cs.CL (Computation and Language)
- **Strength**: Primary source for new research papers
- **Note**: Papers are often technical; summarize key contributions for digest

### AI Lab Blogs (Primary Sources)
| Lab | Feed URL | Notes |
|-----|----------|-------|
| OpenAI | `https://openai.com/blog/rss.xml` | RSS ✅ |
| Anthropic | — | No RSS; use `web_fetch("https://www.anthropic.com/research")` |
| Google AI | `https://blog.google/technology/ai/rss/` | RSS ✅ |
| DeepMind | `https://deepmind.google/blog/rss.xml` | RSS ✅ |
| Meta AI | — | No RSS; use `web_fetch("https://ai.meta.com/blog/")` |
| Meta Engineering | `https://engineering.fb.com/category/ml-applications/feed/` | RSS ✅ (ML category) |
| Hugging Face | `https://huggingface.co/blog/feed.xml` | RSS ✅ |

### Indie & Newsletters
| Author | Feed URL |
|--------|----------|
| swyx (Latent Space) | `https://www.latent.space/feed` |
| Nathan Lambert (Interconnects) | `https://www.interconnects.ai/feed` |
| Doug O'Laughlin (Fabricated Knowledge) | `https://www.fabricatedknowledge.com/feed` |
| Simon Willison | `https://simonwillison.net/atom/everything/` |
| Sentinel Team | `https://sentinelteam.substack.com/feed` |
| Peter Steinberger | `https://steipete.com/rss.xml` |
| Armin Ronacher | `https://lucumr.pocoo.org/feed.atom` |
| Mario Zechner | `https://marioslab.io/feed.xml` |
| SemiAnalysis | `https://www.semianalysis.com/feed` |
| Dwarkesh Patel | `https://www.dwarkesh.com/feed` |

### X/Twitter Accounts to Follow
| Person | X Handle | Blog |
|--------|----------|------|
| Peter Steinberger | [@steipete](https://x.com/steipete) | [steipete.com](https://steipete.com) |
| Armin Ronacher | [@maboroshi](https://x.com/maboroshi) | [lucumr.pocoo.org](https://lucumr.pocoo.org) |
| Mario Zechner | [@badaborshi](https://x.com/badlogicgames) | [marioslab.io](https://marioslab.io) |

### Tech Press (Secondary Sources)
| Publication | Feed URL |
|-------------|----------|
| Techmeme | `https://www.techmeme.com/feed.xml` |
| The Verge AI | `https://www.theverge.com/rss/ai-artificial-intelligence/index.xml` |
| TechCrunch AI | `https://techcrunch.com/category/artificial-intelligence/feed/` |
| Ars Technica | `https://feeds.arstechnica.com/arstechnica/technology-lab` |
| MIT Tech Review | `https://www.technologyreview.com/feed/` |

### X/Twitter Lists (requires API token)
| List | URL |
|------|-----|
| AI High Signal | `https://x.com/i/lists/1585430245762441216` |
| Fellow Builders (Onur's) | `https://x.com/i/lists/2005934341768061086` |

**Status**: Saved for when X API bearer token is configured. Use `GET /2/lists/:id/members` to pull handles, then `GET /2/tweets/search/recent` for their posts.

## Adding New Sources

To add a new RSS source, edit the `RSS_FEEDS` dict in `scripts/fetch_news.py`.
To add a new keyword for AI filtering, edit the `AI_KEYWORDS` list.

## Source Priority for Digest

When curating the digest, prioritize:
1. **AI lab announcements** - Product launches, model releases (highest impact)
2. **HN top stories** - Community-validated important news
3. **Tech press** - Industry analysis and coverage
4. **arxiv papers** - Notable research (pick 3-5 most interesting)
