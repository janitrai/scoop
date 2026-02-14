#!/usr/bin/env python3
"""
China News Fetcher - Gathers China news from multiple sources.

Usage:
    python3 fetch_news.py [--hours 24] [--max 30] [--format markdown|json]

Sources:
    - YouTube (Kim's Observation)
    - Reddit r/China, r/worldnews (China-filtered)
    - RSS feeds (Reuters, BBC, Guardian)
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

CHINA_KEYWORDS = [
    "china", "chinese", "beijing", "shanghai", "xi jinping", "ccp", "cpc",
    "taiwan", "taipei", "pla", "people's liberation army",
    "hong kong", "macau", "tibet", "xinjiang", "uyghur",
    "south china sea", "belt and road", "bri",
    "huawei", "tencent", "alibaba", "bytedance", "tiktok",
    "renminbi", "rmb", "yuan", "pboc",
    "us-china", "sino-american", "trade war", "tariff",
    "one china", "strait", "reunification",
    "zhongnanhai", "politburo", "standing committee",
    "evergrande", "property crisis", "deflation",
]

YOUTUBE_FEEDS = {
    "王剑每日观察 Kim's Observation": "https://www.youtube.com/feeds/videos.xml?channel_id=UC8UCbiPrm2zN9nZHKdTevZA",
}

RSS_FEEDS = {
    "BBC China": "https://feeds.bbci.co.uk/news/world/asia/china/rss.xml",
    "The Guardian China": "https://www.theguardian.com/world/china/rss",
    "ChinaTalk": "https://www.chinatalk.media/feed",
}

REDDIT_SUBS = {
    "r/China": "https://www.reddit.com/r/China/hot.json?limit=30",
    "r/worldnews (China)": "https://www.reddit.com/r/worldnews/search.json?q=china+OR+chinese+OR+beijing+OR+xi+jinping+OR+taiwan&sort=relevance&t=day&limit=20",
}

# --- Helpers ---

def fetch_url(url, timeout=15):
    try:
        req = urllib.request.Request(url, headers={
            "User-Agent": "Mozilla/5.0 (compatible; ChinaNewsBot/1.0)"
        })
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.read()
    except Exception as e:
        print(f"  [warn] Failed to fetch {url}: {e}", file=sys.stderr)
        return None


def is_china_related(title, summary=""):
    text = f"{title} {summary}".lower()
    return any(kw in text for kw in CHINA_KEYWORDS)


def parse_rss(xml_bytes, source_name):
    stories = []
    if not xml_bytes:
        return stories
    try:
        root = ET.fromstring(xml_bytes)
    except ET.ParseError:
        return stories

    ns = {"atom": "http://www.w3.org/2005/Atom"}

    # RSS 2.0
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
                    "title": title, "url": link, "summary": desc,
                    "source": source_name, "date": pub_date,
                })
    else:
        # Atom
        for ns_uri in ["http://www.w3.org/2005/Atom"]:
            entries = root.findall(f".//{{{ns_uri}}}entry")
            for entry in entries:
                title = entry.findtext(f"{{{ns_uri}}}title", "").strip()
                link_el = entry.find(f"{{{ns_uri}}}link")
                link = link_el.get("href", "") if link_el is not None else ""
                summary = entry.findtext(f"{{{ns_uri}}}summary", "").strip()
                summary = re.sub(r"<[^>]+>", "", unescape(summary))[:300]
                if title and link:
                    stories.append({
                        "title": title, "url": link, "summary": summary,
                        "source": source_name, "date": "",
                    })
    return stories


def parse_youtube_feed(xml_bytes, source_name):
    stories = []
    if not xml_bytes:
        return stories
    try:
        root = ET.fromstring(xml_bytes)
    except ET.ParseError:
        return stories

    ns = {
        "atom": "http://www.w3.org/2005/Atom",
        "yt": "http://www.youtube.com/xml/schemas/2015",
        "media": "http://search.yahoo.com/mrss/",
    }

    for entry in root.findall("atom:entry", ns):
        title = entry.findtext("atom:title", "", ns).strip()
        video_id = entry.findtext("yt:videoId", "", ns).strip()
        published = entry.findtext("atom:published", "", ns).strip()
        desc_el = entry.find("media:group/media:description", ns)
        desc = desc_el.text.strip()[:300] if desc_el is not None and desc_el.text else ""

        if title and video_id:
            stories.append({
                "title": title,
                "url": f"https://www.youtube.com/watch?v={video_id}",
                "summary": desc,
                "source": source_name,
                "date": published,
                "type": "video",
            })
    return stories


# --- Source Fetchers ---

def fetch_youtube(cutoff):
    print("  Fetching YouTube channels...", file=sys.stderr)
    all_stories = []
    for name, url in YOUTUBE_FEEDS.items():
        data = fetch_url(url)
        stories = parse_youtube_feed(data, name)
        all_stories.extend(stories)
    print(f"  Found {len(all_stories)} videos", file=sys.stderr)
    return all_stories


def fetch_reddit(cutoff):
    print("  Fetching Reddit...", file=sys.stderr)
    all_stories = []
    for name, url in REDDIT_SUBS.items():
        data = fetch_url(url)
        if not data:
            continue
        try:
            js = json.loads(data)
            posts = js.get("data", {}).get("children", [])
        except (json.JSONDecodeError, AttributeError):
            continue

        for post in posts:
            d = post.get("data", {})
            title = d.get("title", "")
            permalink = d.get("permalink", "")
            url_link = d.get("url", "")
            score = d.get("score", 0)
            created = d.get("created_utc", 0)

            post_time = datetime.fromtimestamp(created, tz=timezone.utc)
            if post_time < cutoff:
                continue

            if "worldnews" in name.lower() or is_china_related(title):
                all_stories.append({
                    "title": title,
                    "url": f"https://reddit.com{permalink}" if permalink else url_link,
                    "summary": f"Score: {score} | Comments: {d.get('num_comments', 0)}",
                    "source": name,
                    "date": post_time.isoformat(),
                    "score": score,
                })

    all_stories.sort(key=lambda x: x.get("score", 0), reverse=True)
    print(f"  Found {len(all_stories)} China stories on Reddit", file=sys.stderr)
    return all_stories


def fetch_rss_feeds(cutoff):
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
    now = datetime.now(timezone.utc)
    date_str = now.strftime("%Y-%m-%d")

    lines = [
        f"# China News Digest — {date_str}",
        f"*Covering the last {hours} hours • Generated {now.strftime('%H:%M UTC')}*",
        "",
    ]

    groups = {
        "🎥 Commentary (YouTube)": [],
        "🇨🇳 News": [],
        "💬 Reddit Discussion": [],
    }

    yt_sources = set(YOUTUBE_FEEDS.keys())
    reddit_sources = set(REDDIT_SUBS.keys())

    for s in stories:
        if s["source"] in yt_sources:
            groups["🎥 Commentary (YouTube)"].append(s)
        elif s["source"] in reddit_sources:
            groups["💬 Reddit Discussion"].append(s)
        else:
            groups["🇨🇳 News"].append(s)

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
    lines.append(f"*{total} stories collected*")
    return "\n".join(lines)


def format_json(stories, hours):
    return json.dumps({
        "generated": datetime.now(timezone.utc).isoformat(),
        "hours": hours,
        "total": len(stories),
        "stories": stories,
    }, indent=2)


# --- Main ---

def main():
    parser = argparse.ArgumentParser(description="Fetch China news digest")
    parser.add_argument("--hours", type=int, default=24, help="Look back N hours (default: 24)")
    parser.add_argument("--max", type=int, default=30, help="Max stories per source (default: 30)")
    parser.add_argument("--format", choices=["markdown", "json"], default="markdown")
    args = parser.parse_args()

    cutoff = datetime.now(timezone.utc) - timedelta(hours=args.hours)
    print(f"Fetching China news from the last {args.hours} hours...", file=sys.stderr)

    all_stories = []
    all_stories.extend(fetch_youtube(cutoff))
    all_stories.extend(fetch_reddit(cutoff))
    all_stories.extend(fetch_rss_feeds(cutoff))

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
