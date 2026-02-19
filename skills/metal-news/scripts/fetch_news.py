#!/usr/bin/env python3
"""
Metal News Fetcher — Gathers nonferrous metals news from CNIA (cnmn.com.cn) and Reddit.

Usage:
    python3 fetch_news.py [--hours 48] [--max 200] [--format json]

Sources:
    - cnmn.com.cn (中国有色网 — CNIA official media outlet)
    - Reddit r/Copper, r/aluminum, r/mining, r/commodities
"""

import argparse
import hashlib
import json
import re
import sys
import urllib.request
import urllib.error
from datetime import datetime, timedelta, timezone
from html import unescape
from concurrent.futures import ThreadPoolExecutor, as_completed


# --- CNIA Sections ---

CNMN_SECTIONS = {
    "要闻 (Headlines)": "https://www.cnmn.com.cn/ShowNewsList.aspx?id=13",
    "铜 (Copper)": "https://www.cnmn.com.cn/metal.aspx?id=1",
    "铝 (Aluminum)": "https://www.cnmn.com.cn/metal.aspx?id=23",
    "铅锌 (Lead/Zinc)": "https://www.cnmn.com.cn/metal.aspx?id=35",
    "镍钴 (Nickel/Cobalt)": "https://www.cnmn.com.cn/metal.aspx?id=14",
    "锡锑 (Tin/Antimony)": "https://www.cnmn.com.cn/metal.aspx?id=22",
    "贵金属 (Precious)": "https://www.cnmn.com.cn/metal.aspx?id=87",
    "市场行情 (Market)": "https://www.cnmn.com.cn/NewsMarket.aspx",
}

REDDIT_SUBREDDITS = [
    "Copper",
    "aluminum",
    "mining",
    "commodities",
]

METAL_KEYWORDS = [
    "copper", "aluminum", "aluminium", "zinc", "lead", "nickel",
    "cobalt", "tin", "antimony", "tungsten", "molybdenum", "titanium",
    "rare earth", "lithium", "gold", "silver", "platinum", "palladium",
    "smelter", "smelting", "refinery", "mining", "ore", "concentrate",
    "lme", "shfe", "comex", "metal price", "metal market",
    "nonferrous", "non-ferrous", "base metal",
    "铜", "铝", "铅", "锌", "镍", "钴", "锡", "锑", "钨",
    "钼", "钛", "稀土", "锂", "贵金属", "有色金属", "冶炼",
]


def fetch_url(url, timeout=15):
    try:
        req = urllib.request.Request(url, headers={
            "User-Agent": "Mozilla/5.0 (compatible; MetalNewsBot/1.0)"
        })
        return urllib.request.urlopen(req, timeout=timeout).read()
    except Exception as e:
        print(f"  [warn] fetch failed {url}: {e}", file=sys.stderr)
        return None


def make_item_id(source, unique_part):
    h = hashlib.md5(f"{source}:{unique_part}".encode()).hexdigest()[:16]
    return h


# --- CNIA Scraper ---

def scrape_cnmn_section(section_name, url):
    """Scrape article links + titles from a CNIA section page."""
    data = fetch_url(url)
    if not data:
        return []
    text = data.decode("utf-8", errors="replace")
    # Match: href="/ShowNews1.aspx?id=XXXXX">Title</a>
    pattern = r'href="(/ShowNews1\.aspx\?id=(\d+))"[^>]*>\s*([^<]{3,})'
    matches = re.findall(pattern, text)
    items = []
    seen_ids = set()
    for href, article_id, title in matches:
        title = unescape(title.strip().replace("&nbsp;", " "))
        if not title or article_id in seen_ids:
            continue
        seen_ids.add(article_id)
        full_url = f"https://www.cnmn.com.cn{href}"
        items.append({
            "source": "cnmn.com.cn",
            "source_item_id": make_item_id("cnmn", article_id),
            "title": title,
            "url": full_url,
            "published_at": datetime.now(timezone.utc).isoformat(),
            "section": section_name,
        })
    return items


def fetch_cnmn(max_per_section=30):
    """Fetch articles from all CNIA sections in parallel."""
    all_items = []
    seen_ids = set()
    with ThreadPoolExecutor(max_workers=6) as pool:
        futures = {
            pool.submit(scrape_cnmn_section, name, url): name
            for name, url in CNMN_SECTIONS.items()
        }
        for future in as_completed(futures):
            name = futures[future]
            try:
                items = future.result()
                for item in items[:max_per_section]:
                    if item["source_item_id"] not in seen_ids:
                        seen_ids.add(item["source_item_id"])
                        all_items.append(item)
                print(f"  [cnmn] {name}: {len(items)} articles", file=sys.stderr)
            except Exception as e:
                print(f"  [warn] {name} failed: {e}", file=sys.stderr)
    return all_items


# --- Reddit Scraper ---

def fetch_reddit(subreddits, hours=48, max_per_sub=25):
    cutoff = datetime.now(timezone.utc) - timedelta(hours=hours)
    all_items = []
    for sub in subreddits:
        url = f"https://www.reddit.com/r/{sub}/new.json?limit=50"
        data = fetch_url(url)
        if not data:
            continue
        try:
            listing = json.loads(data)
            posts = listing.get("data", {}).get("children", [])
        except (json.JSONDecodeError, KeyError):
            continue
        count = 0
        for post in posts:
            d = post.get("data", {})
            created = datetime.fromtimestamp(d.get("created_utc", 0), tz=timezone.utc)
            if created < cutoff:
                continue
            title = d.get("title", "").strip()
            if not title:
                continue
            post_url = d.get("url", "")
            permalink = f"https://www.reddit.com{d.get('permalink', '')}"
            all_items.append({
                "source": "reddit",
                "source_item_id": make_item_id("reddit", d.get("id", "")),
                "title": title,
                "url": post_url if post_url and not post_url.startswith("https://www.reddit.com") else permalink,
                "published_at": created.isoformat(),
                "score": d.get("score", 0),
                "num_comments": d.get("num_comments", 0),
                "subreddit": sub,
            })
            count += 1
            if count >= max_per_sub:
                break
        print(f"  [reddit] r/{sub}: {count} posts", file=sys.stderr)
    return all_items


# --- Main ---

def main():
    parser = argparse.ArgumentParser(description="Fetch metal news from CNIA and Reddit")
    parser.add_argument("--hours", type=int, default=48, help="Lookback window for Reddit (default: 48)")
    parser.add_argument("--max", type=int, default=200, help="Max total items (default: 200)")
    parser.add_argument("--format", choices=["json", "markdown"], default="json", help="Output format")
    args = parser.parse_args()

    print(f"Fetching metal news (Reddit lookback: {args.hours}h)...", file=sys.stderr)

    # Fetch all sources
    cnmn_items = fetch_cnmn()
    reddit_items = fetch_reddit(REDDIT_SUBREDDITS, hours=args.hours)

    all_items = cnmn_items + reddit_items

    # Deduplicate by source_item_id
    seen = set()
    unique = []
    for item in all_items:
        sid = item["source_item_id"]
        if sid not in seen:
            seen.add(sid)
            unique.append(item)

    # Trim to max
    unique = unique[: args.max]

    print(f"Total: {len(unique)} items ({len(cnmn_items)} CNIA, {len(reddit_items)} Reddit)", file=sys.stderr)

    if args.format == "json":
        output = {
            "collection": "metal_news",
            "fetched_at": datetime.now(timezone.utc).isoformat(),
            "item_count": len(unique),
            "items": unique,
        }
        json.dump(output, sys.stdout, ensure_ascii=False, indent=2)
        sys.stdout.write("\n")
    else:
        print(f"# Metal News Digest ({datetime.now(timezone.utc).strftime('%Y-%m-%d')})\n")
        print(f"**{len(unique)} stories** from CNIA and Reddit\n")
        for item in unique:
            src = item["source"]
            title = item["title"]
            url = item["url"]
            section = item.get("section", "")
            prefix = f"🇨🇳 [{section}]" if src == "cnmn.com.cn" else "💬"
            print(f"- {prefix} [{title}]({url})")


if __name__ == "__main__":
    main()
