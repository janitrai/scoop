#!/usr/bin/env python3
"""
AI News Fetcher - Gathers AI news from multiple sources and outputs a structured digest.

Usage:
    python3 fetch_news.py [--hours 24] [--max 30] [--format markdown|json]

Sources:
    - Hacker News (AI-filtered top/best stories)
    - Arxiv (cs.AI, cs.LG, cs.CL new submissions)
    - RSS feeds (major AI labs, tech publications)
"""

import argparse
import email.utils
import json
import re
import sys
import urllib.request
import urllib.error
import xml.etree.ElementTree as ET
from datetime import datetime, timedelta, timezone
from html import unescape
from concurrent.futures import ThreadPoolExecutor, as_completed

# --- Configuration ---

AI_KEYWORDS = [
    "ai", "artificial intelligence", "machine learning", "deep learning",
    "neural network", "llm", "large language model", "gpt", "claude",
    "gemini", "transformer", "diffusion", "generative", "openai",
    "anthropic", "deepmind", "meta ai", "mistral", "reasoning model",
    "agi", "alignment", "rlhf", "fine-tuning", "finetuning",
    "multimodal", "vision model", "speech model", "text-to-image",
    "text-to-video", "foundation model", "frontier model",
    "agent", "agentic", "mcp", "tool use", "function calling",
    "embedding", "rag", "retrieval augmented", "vector database",
    "hugging face", "huggingface", "pytorch", "tensorflow",
    "stable diffusion", "midjourney", "dall-e", "sora", "copilot",
    "coding agent", "codex", "devin", "cursor",
    "compute", "gpu", "tpu", "nvidia", "h100", "b200",
    "safety", "red team", "jailbreak", "hallucination",
    "benchmark", "eval", "mmlu", "humaneval",
    "open source model", "open weight", "llama", "qwen", "phi",
    "context window", "token", "inference", "training run",
]

RSS_FEEDS = {
    "OpenAI Blog": "https://openai.com/blog/rss.xml",
    # Anthropic has no RSS - scraped via web_fetch in SKILL.md workflow
    "Google AI Blog": "https://blog.google/technology/ai/rss/",
    "DeepMind Blog": "https://deepmind.google/blog/rss.xml",
    "Meta Engineering (ML)": "https://engineering.fb.com/category/ml-applications/feed/",
    "Techmeme": "https://www.techmeme.com/feed.xml",
    "The Verge AI": "https://www.theverge.com/rss/ai-artificial-intelligence/index.xml",
    "TechCrunch AI": "https://techcrunch.com/category/artificial-intelligence/feed/",
    "Ars Technica AI": "https://feeds.arstechnica.com/arstechnica/technology-lab",
    "MIT Tech Review AI": "https://www.technologyreview.com/feed/",
    "Hugging Face Blog": "https://huggingface.co/blog/feed.xml",
    "Latent Space (swyx)": "https://www.latent.space/feed",
    "Simon Willison": "https://simonwillison.net/atom/everything/",
    "Sentinel Team": "https://sentinelteam.substack.com/feed",
    "Peter Steinberger": "https://steipete.com/rss.xml",
    "Armin Ronacher": "https://lucumr.pocoo.org/feed.atom",
    "Mario Zechner": "https://marioslab.io/feed.xml",
    "SemiAnalysis": "https://www.semianalysis.com/feed",
    "Dwarkesh Patel": "https://www.dwarkesh.com/feed",
    "Interconnects (Nathan Lambert)": "https://www.interconnects.ai/feed",
    "Fabricated Knowledge (Doug O'Laughlin)": "https://www.fabricatedknowledge.com/feed",
}

ARXIV_CATEGORIES = ["cs.AI", "cs.LG", "cs.CL"]

# --- Helpers ---

def fetch_url(url, timeout=15):
    """Fetch URL content, return bytes or None on failure."""
    try:
        req = urllib.request.Request(url, headers={
            "User-Agent": "Mozilla/5.0 (compatible; AINewsBot/1.0)"
        })
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.read()
    except Exception as e:
        print(f"  [warn] Failed to fetch {url}: {e}", file=sys.stderr)
        return None


def is_ai_related(title, summary=""):
    """Check if a story is AI-related based on keywords."""
    text = f"{title} {summary}".lower()
    return any(kw in text for kw in AI_KEYWORDS)


def parse_date(date_str):
    """Parse a date string in various formats. Returns datetime (UTC) or None."""
    if not date_str or not date_str.strip():
        return None
    date_str = date_str.strip()

    # Try RFC 2822 (RSS pubDate format: "Sat, 04 Oct 2025 17:45:00 GMT")
    try:
        parsed = email.utils.parsedate_to_datetime(date_str)
        if parsed.tzinfo is None:
            parsed = parsed.replace(tzinfo=timezone.utc)
        return parsed.astimezone(timezone.utc)
    except (ValueError, TypeError):
        pass

    # Try ISO 8601 variants
    for fmt in [
        "%Y-%m-%dT%H:%M:%S%z",
        "%Y-%m-%dT%H:%M:%SZ",
        "%Y-%m-%dT%H:%M:%S.%f%z",
        "%Y-%m-%dT%H:%M:%S.%fZ",
        "%Y-%m-%d %H:%M:%S",
        "%Y-%m-%d",
    ]:
        try:
            parsed = datetime.strptime(date_str, fmt)
            if parsed.tzinfo is None:
                parsed = parsed.replace(tzinfo=timezone.utc)
            return parsed.astimezone(timezone.utc)
        except ValueError:
            continue

    # Try ISO format with fromisoformat (handles +00:00 etc.)
    try:
        parsed = datetime.fromisoformat(date_str.replace("Z", "+00:00"))
        if parsed.tzinfo is None:
            parsed = parsed.replace(tzinfo=timezone.utc)
        return parsed.astimezone(timezone.utc)
    except (ValueError, TypeError):
        pass

    return None


def parse_rss(xml_bytes, source_name, cutoff):
    """Parse RSS/Atom feed XML and return list of story dicts."""
    stories = []
    if not xml_bytes:
        return stories

    try:
        root = ET.fromstring(xml_bytes)
    except ET.ParseError:
        return stories

    # Handle both RSS and Atom namespaces
    ns = {"atom": "http://www.w3.org/2005/Atom"}

    # Try RSS 2.0 format
    items = root.findall(".//item")
    if items:
        for item in items:
            title = item.findtext("title", "").strip()
            link = item.findtext("link", "").strip()
            desc = unescape(item.findtext("description", "").strip())
            pub_date_str = item.findtext("pubDate", "")

            # Clean HTML from description
            desc = re.sub(r"<[^>]+>", "", desc)[:300]

            # Parse and filter by cutoff
            pub_date = parse_date(pub_date_str)
            if pub_date and pub_date < cutoff:
                continue  # Skip items older than cutoff

            if title and link:
                stories.append({
                    "title": title,
                    "url": link,
                    "summary": desc,
                    "source": source_name,
                    "date": pub_date.isoformat() if pub_date else "",
                })
    else:
        # Try Atom format
        entries = root.findall(".//atom:entry", ns) or root.findall(".//{http://www.w3.org/2005/Atom}entry")
        for entry in entries:
            title_el = entry.find("atom:title", ns)
            if title_el is None:
                title_el = entry.find("{http://www.w3.org/2005/Atom}title")
            link_el = entry.find("atom:link", ns)
            if link_el is None:
                link_el = entry.find("{http://www.w3.org/2005/Atom}link")
            summary_el = entry.find("atom:summary", ns)
            if summary_el is None:
                summary_el = entry.find("{http://www.w3.org/2005/Atom}summary")

            # Try multiple date fields for Atom
            date_str = ""
            for date_tag in ["atom:published", "atom:updated"]:
                date_el = entry.find(date_tag, ns)
                if date_el is None:
                    date_el = entry.find(date_tag.replace("atom:", "{http://www.w3.org/2005/Atom}"))
                if date_el is not None and date_el.text:
                    date_str = date_el.text.strip()
                    break

            title = title_el.text.strip() if title_el is not None and title_el.text else ""
            link = link_el.get("href", "") if link_el is not None else ""
            summary = summary_el.text.strip() if summary_el is not None and summary_el.text else ""
            summary = re.sub(r"<[^>]+>", "", unescape(summary))[:300]

            # Parse and filter by cutoff
            pub_date = parse_date(date_str)
            if pub_date and pub_date < cutoff:
                continue  # Skip items older than cutoff

            if title and link:
                stories.append({
                    "title": title,
                    "url": link,
                    "summary": summary,
                    "source": source_name,
                    "date": pub_date.isoformat() if pub_date else "",
                })

    return stories


# --- Source Fetchers ---

def fetch_hackernews(cutoff, max_stories=500):
    """Fetch AI-related stories from Hacker News."""
    print("  Fetching Hacker News...", file=sys.stderr)
    stories = []

    # Get top and best story IDs
    for endpoint in ["topstories", "beststories"]:
        data = fetch_url(f"https://hacker-news.firebaseio.com/v0/{endpoint}.json")
        if not data:
            continue

        ids = json.loads(data)[:max_stories]

        def fetch_item(item_id):
            item_data = fetch_url(f"https://hacker-news.firebaseio.com/v0/item/{item_id}.json")
            if not item_data:
                return None
            return json.loads(item_data)

        with ThreadPoolExecutor(max_workers=20) as pool:
            futures = {pool.submit(fetch_item, sid): sid for sid in ids}
            for future in as_completed(futures):
                item = future.result()
                if not item or item.get("type") != "story":
                    continue

                title = item.get("title", "")
                url = item.get("url", f"https://news.ycombinator.com/item?id={item['id']}")
                score = item.get("score", 0)
                ts = item.get("time", 0)

                # Filter by time
                item_time = datetime.fromtimestamp(ts, tz=timezone.utc)
                if item_time < cutoff:
                    continue

                if is_ai_related(title):
                    stories.append({
                        "title": title,
                        "url": url,
                        "summary": f"Score: {score} | Comments: {item.get('descendants', 0)}",
                        "source": "Hacker News",
                        "date": item_time.isoformat(),
                        "score": score,
                    })

    # Deduplicate by URL
    seen = set()
    unique = []
    for s in stories:
        if s["url"] not in seen:
            seen.add(s["url"])
            unique.append(s)

    # Sort by score
    unique.sort(key=lambda x: x.get("score", 0), reverse=True)
    print(f"  Found {len(unique)} AI stories on HN", file=sys.stderr)
    return unique


def fetch_arxiv(cutoff, max_results=30):
    """Fetch recent AI papers from arxiv."""
    print("  Fetching arxiv...", file=sys.stderr)
    stories = []

    cats = "+OR+".join(f"cat:{c}" for c in ARXIV_CATEGORIES)
    url = f"http://export.arxiv.org/api/query?search_query={cats}&sortBy=submittedDate&sortOrder=descending&max_results={max_results}"

    data = fetch_url(url, timeout=30)
    if not data:
        return stories

    ns = {"atom": "http://www.w3.org/2005/Atom"}
    try:
        root = ET.fromstring(data)
    except ET.ParseError:
        return stories

    for entry in root.findall("atom:entry", ns):
        title = entry.findtext("atom:title", "", ns).strip().replace("\n", " ")
        link_el = entry.find("atom:id", ns)
        link = link_el.text.strip() if link_el is not None else ""
        summary = entry.findtext("atom:summary", "", ns).strip().replace("\n", " ")[:300]

        # Get categories
        categories = [c.get("term", "") for c in entry.findall("atom:category", ns)]
        cat_str = ", ".join(categories[:3])

        stories.append({
            "title": title,
            "url": link,
            "summary": f"[{cat_str}] {summary}",
            "source": "arxiv",
            "date": entry.findtext("atom:published", "", ns),
        })

    print(f"  Found {len(stories)} papers on arxiv", file=sys.stderr)
    return stories


def fetch_rss_feeds(cutoff):
    """Fetch stories from all configured RSS feeds."""
    all_stories = []

    def fetch_one(name, url):
        print(f"  Fetching {name}...", file=sys.stderr)
        data = fetch_url(url)
        return parse_rss(data, name, cutoff)

    with ThreadPoolExecutor(max_workers=10) as pool:
        futures = {pool.submit(fetch_one, name, url): name for name, url in RSS_FEEDS.items()}
        for future in as_completed(futures):
            stories = future.result()
            # For non-AI-specific feeds, filter
            source = futures[future]
            if source in ("Ars Technica AI", "MIT Tech Review AI"):
                stories = [s for s in stories if is_ai_related(s["title"], s["summary"])]
            all_stories.extend(stories)

    print(f"  Found {len(all_stories)} stories from RSS feeds", file=sys.stderr)
    return all_stories


# --- Output Formatting ---

def format_markdown(stories, hours):
    """Format stories as a markdown digest."""
    now = datetime.now(timezone.utc)
    date_str = now.strftime("%Y-%m-%d")

    lines = [
        f"# AI News Digest — {date_str}",
        f"*Covering the last {hours} hours • Generated {now.strftime('%H:%M UTC')}*",
        "",
    ]

    # Group by source type
    groups = {
        "🔥 Top Stories (Hacker News)": [],
        "🏢 AI Lab Announcements": [],
        "✍️ Indie & Newsletters": [],
        "📰 Tech Press": [],
        "📄 Research Papers (arxiv)": [],
    }

    lab_sources = {"OpenAI Blog", "Google AI Blog", "DeepMind Blog", "Meta Engineering (ML)", "Hugging Face Blog"}
    indie_sources = {"Latent Space (swyx)", "Interconnects (Nathan Lambert)", "Fabricated Knowledge (Doug O'Laughlin)", "Simon Willison", "Sentinel Team", "Peter Steinberger", "Armin Ronacher", "Mario Zechner", "SemiAnalysis", "Dwarkesh Patel"}
    press_sources = {"Techmeme", "The Verge AI", "TechCrunch AI", "Ars Technica AI", "MIT Tech Review AI"}

    for s in stories:
        if s["source"] == "Hacker News":
            groups["🔥 Top Stories (Hacker News)"].append(s)
        elif s["source"] in lab_sources:
            groups["🏢 AI Lab Announcements"].append(s)
        elif s["source"] in indie_sources:
            groups["✍️ Indie & Newsletters"].append(s)
        elif s["source"] in press_sources:
            groups["📰 Tech Press"].append(s)
        elif s["source"] == "arxiv":
            groups["📄 Research Papers (arxiv)"].append(s)

    for heading, items in groups.items():
        if not items:
            continue
        lines.append(f"## {heading}")
        lines.append("")
        for i, s in enumerate(items[:15], 1):
            lines.append(f"**{i}. [{s['title']}]({s['url']})**")
            if s.get("summary"):
                lines.append(f"   {s['summary'][:200]}")
            lines.append(f"   *Source: {s['source']}*")
            lines.append("")
        lines.append("---")
        lines.append("")

    total = sum(len(v) for v in groups.values())
    lines.append(f"*{total} stories collected from {len(RSS_FEEDS) + 2} sources*")

    return "\n".join(lines)


def format_json(stories, hours):
    """Format stories as JSON."""
    return json.dumps({
        "generated": datetime.now(timezone.utc).isoformat(),
        "hours": hours,
        "total": len(stories),
        "stories": stories,
    }, indent=2)


# --- Main ---

def main():
    parser = argparse.ArgumentParser(description="Fetch AI news digest")
    parser.add_argument("--hours", type=int, default=24, help="Look back N hours (default: 24)")
    parser.add_argument("--max", type=int, default=30, help="Max stories per source (default: 30)")
    parser.add_argument("--format", choices=["markdown", "json"], default="markdown", help="Output format")
    args = parser.parse_args()

    cutoff = datetime.now(timezone.utc) - timedelta(hours=args.hours)
    print(f"Fetching AI news from the last {args.hours} hours...", file=sys.stderr)

    all_stories = []

    # Fetch from all sources
    hn_stories = fetch_hackernews(cutoff, max_stories=args.max * 10)
    all_stories.extend(hn_stories[:args.max])

    arxiv_stories = fetch_arxiv(cutoff, max_results=args.max)
    all_stories.extend(arxiv_stories[:args.max])

    rss_stories = fetch_rss_feeds(cutoff)
    all_stories.extend(rss_stories)

    # Deduplicate by URL
    seen = set()
    unique = []
    for s in all_stories:
        url = s["url"].rstrip("/")
        if url not in seen:
            seen.add(url)
            unique.append(s)

    print(f"\nTotal unique stories: {len(unique)}", file=sys.stderr)

    # Output
    if args.format == "json":
        print(format_json(unique, args.hours))
    else:
        print(format_markdown(unique, args.hours))


if __name__ == "__main__":
    main()
