#!/bin/bash
# Seed test patterns in GlowBlaster
# Usage: ./seed_test_patterns.sh <base_url> <session_cookie>
# Example: ./seed_test_patterns.sh https://your-app.com "session=abc123..."

BASE_URL="${1:-http://localhost:3000}"
COOKIE="${2:-}"

if [ -z "$COOKIE" ]; then
    echo "Usage: $0 <base_url> <session_cookie>"
    echo "Example: $0 https://your-app.com 'session=abc123...'"
    exit 1
fi

echo "Seeding test patterns to $BASE_URL..."
echo

# All Red
echo "Creating: All Red"
curl -s -X POST "$BASE_URL/api/glowblaster/patterns" \
    -H "Content-Type: application/json" \
    -H "Cookie: $COOKIE" \
    -d '{
        "name": "All Red",
        "lcl": "effect: solid\nname: \"All Red\"\n\nappearance:\n  color: red\n  brightness: bright\n\ntiming:\n  speed: medium"
    }' | jq -r 'if .success then "  OK" else "  Failed: " + .error end'

# All Blue
echo "Creating: All Blue"
curl -s -X POST "$BASE_URL/api/glowblaster/patterns" \
    -H "Content-Type: application/json" \
    -H "Cookie: $COOKIE" \
    -d '{
        "name": "All Blue",
        "lcl": "effect: solid\nname: \"All Blue\"\n\nappearance:\n  color: blue\n  brightness: bright\n\ntiming:\n  speed: medium"
    }' | jq -r 'if .success then "  OK" else "  Failed: " + .error end'

# All Green
echo "Creating: All Green"
curl -s -X POST "$BASE_URL/api/glowblaster/patterns" \
    -H "Content-Type: application/json" \
    -H "Cookie: $COOKIE" \
    -d '{
        "name": "All Green",
        "lcl": "effect: solid\nname: \"All Green\"\n\nappearance:\n  color: green\n  brightness: bright\n\ntiming:\n  speed: medium"
    }' | jq -r 'if .success then "  OK" else "  Failed: " + .error end'

# All White
echo "Creating: All White"
curl -s -X POST "$BASE_URL/api/glowblaster/patterns" \
    -H "Content-Type: application/json" \
    -H "Cookie: $COOKIE" \
    -d '{
        "name": "All White",
        "lcl": "effect: solid\nname: \"All White\"\n\nappearance:\n  color: white\n  brightness: bright\n\ntiming:\n  speed: medium"
    }' | jq -r 'if .success then "  OK" else "  Failed: " + .error end'

# Red, White, Blue
echo "Creating: Red, White, Blue"
curl -s -X POST "$BASE_URL/api/glowblaster/patterns" \
    -H "Content-Type: application/json" \
    -H "Cookie: $COOKIE" \
    -d '{
        "name": "Red, White, Blue",
        "lcl": "effect: chase\nname: \"Red, White, Blue\"\n\nbehavior:\n  head_size: medium\n  tail_length: medium\n  tail_style: fade\n  count: triple\n\nappearance:\n  colors:\n    - red\n    - white\n    - blue\n  brightness: bright\n\ntiming:\n  speed: medium\n\nspatial:\n  direction: forward"
    }' | jq -r 'if .success then "  OK" else "  Failed: " + .error end'

echo
echo "Done!"
