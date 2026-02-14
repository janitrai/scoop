#!/usr/bin/env python3
"""
World News Fetcher - Gathers world news from multiple sources and outputs a structured digest.

Usage:
    python3 fetch_news.py [--hours 24] [--max 30] [--format markdown|json]

Sources:
    - Reddit r/worldnews (top posts)
    - Sentinel Team (global risks weekly brief)
"""

import argparse
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

RSS_FEEDS = {
    "Sentinel Team": "https://sentinelteam.substack.com/feed",
    "Kyla Scanlon": "https://kylascanlon.substack.com/feed",
}

# --- Helpers ---

def fetch_url(url, timeout=15):
    """Fetch URL content, return bytes or None on failure."""
    try:
        req = urllib.request.Request(url, headers={
            "User-Agent": "Mozilla/5.0 (compatible; WorldNewsBot/1.0)"
        })
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.read()
    except Exception as e:
        print(f"  [warn] Failed to fetch {url}: {e}", file=sys.stderr)
        return None


def parse_rss(xml_bytes, source_name):
    """Parse RSS/Atom feed XML and return list of story dicts."""
    stories = []
    if not xml_bytes:
        return stories

    try:
        root = ET.fromstring(xml_bytes)
    except ET.ParseError:
        return stories

    ns = {"atom": "http://www.w3.org/2005/Atom"}

    # Try RSS 2.0 format
    items = root.findall(".//item")
    if items:
        for item in items:
            title = item.findtext("title", "").strip()
            link = item.findtext("link", "").strip()
            desc = unescape(item.findtext("description", "").strip())
            pub_date = item.findtext("pubDate", "")
            desc = re.sub(r"<[^>]+>", "", desc)[:300]

            if title and link:
                stories.append({
                    "title": title,
                    "url": link,
                    "summary": desc,
                    "source": source_name,
                    "date": pub_date,
                })
    else:
        # Try Atom format
        entries = root.findall("atom:entry", ns) or root.findall("{http://www.w3.org/2005/Atom}entry")
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

            title = title_el.text.strip() if title_el is not None and title_el.text else ""
            link = link_el.get("href", "") if link_el is not None else ""
            summary = summary_el.text.strip() if summary_el is not None and summary_el.text else ""
            summary = re.sub(r"<[^>]+>", "", unescape(summary))[:300]

            if title and link:
                stories.append({
                    "title": title,
                    "url": link,
                    "summary": summary,
                    "source": source_name,
                    "date": "",
                })

    return stories


# --- Source Fetchers ---

def fetch_reddit_worldnews(cutoff, max_stories=50):
    """Fetch top posts from r/worldnews via JSON API."""
    print("  Fetching r/worldnews...", file=sys.stderr)
    stories = []

    for sort in ["hot", "top"]:
        params = "?t=day&limit=50" if sort == "top" else "?limit=50"
        url = f"https://www.reddit.com/r/worldnews/{sort}.json{params}"
        data = fetch_url(url)
        if not data:
            continue

        try:
            listing = json.loads(data)
        except json.JSONDecodeError:
            continue

        for child in listing.get("data", {}).get("children", []):
            post = child.get("data", {})
            title = post.get("title", "")
            permalink = post.get("permalink", "")
            link_url = post.get("url", "")
            score = post.get("score", 0)
            num_comments = post.get("num_comments", 0)
            created = post.get("created_utc", 0)

            post_time = datetime.fromtimestamp(created, tz=timezone.utc)
            if post_time < cutoff:
                continue

            # Use the external link, not reddit permalink
            url = link_url if link_url and not link_url.startswith("https://www.reddit.com") else f"https://www.reddit.com{permalink}"

            stories.append({
                "title": title,
                "url": url,
                "summary": f"Score: {score} | Comments: {num_comments}",
                "source": "r/worldnews",
                "date": post_time.isoformat(),
                "score": score,
                "comments_url": f"https://www.reddit.com{permalink}",
            })

    # Deduplicate by URL
    seen = set()
    unique = []
    for s in stories:
        if s["url"] not in seen:
            seen.add(s["url"])
            unique.append(s)

    unique.sort(key=lambda x: x.get("score", 0), reverse=True)
    print(f"  Found {len(unique)} posts on r/worldnews", file=sys.stderr)
    return unique


def fetch_rss_feeds(cutoff):
    """Fetch stories from all configured RSS feeds."""
    all_stories = []

    def fetch_one(name, url):
        print(f"  Fetching {name}...", file=sys.stderr)
        data = fetch_url(url)
        return parse_rss(data, name)

    with ThreadPoolExecutor(max_workers=5) as pool:
        futures = {pool.submit(fetch_one, name, url): name for name, url in RSS_FEEDS.items()}
        for future in as_completed(futures):
            stories = future.result()
            all_stories.extend(stories)

    print(f"  Found {len(all_stories)} stories from RSS feeds", file=sys.stderr)
    return all_stories


# --- Output Formatting ---

def format_markdown(stories, hours):
    """Format stories as a markdown digest."""
    now = datetime.now(timezone.utc)
    date_str = now.strftime("%Y-%m-%d")

    lines = [
        f"# World News Digest — {date_str}",
        f"*Covering the last {hours} hours • Generated {now.strftime('%H:%M UTC')}*",
        "",
    ]

    groups = {
        "🌍 Top World News (Reddit)": [],
        "⚠️ Global Risks (Sentinel)": [],
        "💰 Economics & Markets": [],
    }

    for s in stories:
        if s["source"] == "r/worldnews":
            groups["🌍 Top World News (Reddit)"].append(s)
        elif s["source"] == "Sentinel Team":
            groups["⚠️ Global Risks (Sentinel)"].append(s)
        elif s["source"] == "Kyla Scanlon":
            groups["💰 Economics & Markets"].append(s)

    for heading, items in groups.items():
        if not items:
            continue
        lines.append(f"## {heading}")
        lines.append("")
        for i, s in enumerate(items[:20], 1):
            lines.append(f"**{i}. [{s['title']}]({s['url']})**")
            if s.get("summary"):
                lines.append(f"   {s['summary'][:200]}")
            if s.get("comments_url"):
                lines.append(f"   [💬 Discussion]({s['comments_url']})")
            lines.append(f"   *Source: {s['source']}*")
            lines.append("")
        lines.append("---")
        lines.append("")

    total = sum(len(v) for v in groups.values())
    lines.append(f"*{total} stories collected from {len(RSS_FEEDS) + 1} sources*")

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
    parser = argparse.ArgumentParser(description="Fetch world news digest")
    parser.add_argument("--hours", type=int, default=24, help="Look back N hours (default: 24)")
    parser.add_argument("--max", type=int, default=30, help="Max stories per source (default: 30)")
    parser.add_argument("--format", choices=["markdown", "json"], default="markdown", help="Output format")
    args = parser.parse_args()

    cutoff = datetime.now(timezone.utc) - timedelta(hours=args.hours)
    print(f"Fetching world news from the last {args.hours} hours...", file=sys.stderr)

    all_stories = []

    reddit_stories = fetch_reddit_worldnews(cutoff, max_stories=args.max * 2)
    all_stories.extend(reddit_stories[:args.max])

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

    if args.format == "json":
        print(format_json(unique, args.hours))
    else:
        print(format_markdown(unique, args.hours))


if __name__ == "__main__":
    main()
