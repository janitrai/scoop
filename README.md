# scoop

Public monorepo for the news stack.

## Contents

- `news-pipeline/`: Go backend for ingest, normalize, embed, dedup, and API/web viewer
- `embedding-service/`: Python embedding HTTP service (`/embed`, `/v1/embeddings`, `/health`)
- `ai-news-raw/`: raw news collection artifacts
- `skills/ai-news`, `skills/world-news`, `skills/china-news`: scraping/agent skill assets

## License

MIT (`LICENSE`)
