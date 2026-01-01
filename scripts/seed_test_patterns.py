#!/usr/bin/env python3
"""
Seed script to create test patterns in GlowBlaster.
Run this after logging into the web app (uses session cookie).

Usage:
  python scripts/seed_test_patterns.py --base-url https://your-app-url.com --cookie "session=..."
"""

import argparse
import requests
import json

TEST_PATTERNS = [
    {
        "name": "All Red",
        "lcl": """effect: solid
name: "All Red"

appearance:
  color: red
  brightness: bright

timing:
  speed: medium"""
    },
    {
        "name": "All Blue",
        "lcl": """effect: solid
name: "All Blue"

appearance:
  color: blue
  brightness: bright

timing:
  speed: medium"""
    },
    {
        "name": "All Green",
        "lcl": """effect: solid
name: "All Green"

appearance:
  color: green
  brightness: bright

timing:
  speed: medium"""
    },
    {
        "name": "All White",
        "lcl": """effect: solid
name: "All White"

appearance:
  color: white
  brightness: bright

timing:
  speed: medium"""
    },
    {
        "name": "Red, White, Blue",
        "lcl": """effect: chase
name: "Red, White, Blue"

behavior:
  head_size: medium
  tail_length: medium
  tail_style: fade
  count: triple

appearance:
  colors:
    - red
    - white
    - blue
  brightness: bright

timing:
  speed: medium

spatial:
  direction: forward"""
    }
]


def create_pattern(base_url: str, cookie: str, pattern: dict) -> bool:
    """Create a single pattern via the API."""
    url = f"{base_url}/api/glowblaster/patterns"
    headers = {
        "Content-Type": "application/json",
        "Cookie": cookie
    }
    payload = {
        "name": pattern["name"],
        "lcl": pattern["lcl"]
    }

    try:
        resp = requests.post(url, headers=headers, json=payload)
        data = resp.json()

        if data.get("success"):
            print(f"  Created: {pattern['name']}")
            return True
        else:
            print(f"  Failed: {pattern['name']} - {data.get('error', 'Unknown error')}")
            return False
    except Exception as e:
        print(f"  Error: {pattern['name']} - {e}")
        return False


def main():
    parser = argparse.ArgumentParser(description="Seed test patterns in GlowBlaster")
    parser.add_argument("--base-url", required=True, help="Base URL of the application")
    parser.add_argument("--cookie", required=True, help="Session cookie value")
    args = parser.parse_args()

    print(f"Seeding {len(TEST_PATTERNS)} test patterns...")
    print()

    success_count = 0
    for pattern in TEST_PATTERNS:
        if create_pattern(args.base_url, args.cookie, pattern):
            success_count += 1

    print()
    print(f"Complete: {success_count}/{len(TEST_PATTERNS)} patterns created")


if __name__ == "__main__":
    main()
