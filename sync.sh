#!/bin/bash
#
# VisionLoop Git Sync Script
# Usage: ./sync.sh [status|commit|push|sync]
#

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Remote name
REMOTE_NAME="origin"

# Branch name
BRANCH_NAME="$(git rev-parse --abbrev-ref HEAD)"

show_status() {
    echo -e "${YELLOW}=== Git Status ===${NC}"
    git status --short
    echo ""

    echo -e "${YELLOW}=== Uncommitted Changes ===${NC}"
    CHANGES=$(git diff --stat | tail -n +2 | grep -v "^$" | wc -l)
    if [ "$CHANGES" -gt 0 ]; then
        git diff --stat | tail -n +2
        echo ""
        echo -e "${GREEN}Changes detected ($CHANGES files)${NC}"
    else
        echo -e "${GREEN}No uncommitted changes${NC}"
    fi
    echo ""

    echo -e "${YELLOW}=== Untracked Files ===${NC}"
    git ls-files --others --exclude-standard
    echo ""

    echo -e "${YELLOW}=== Recent Commits ===${NC}"
    git log --oneline -5
}

do_commit() {
    local MSG="${1:-Auto sync $(date '+%Y-%m-%d %H:%M:%S')}"

    echo -e "${YELLOW}=== Git Commit ===${NC}"

    # Add all changes
    echo "Staging all changes..."
    git add -A

    # Check if there are staged changes
    if git diff --cached --quiet; then
        echo -e "${GREEN}No changes to commit${NC}"
        return 0
    fi

    echo "Committing with message: $MSG"
    git commit -m "$MSG"

    echo -e "${GREEN}Commit successful${NC}"
}

do_push() {
    echo -e "${YELLOW}=== Git Push ===${NC}"

    # Check if there are commits to push
    local UPSTREAM="refs/remotes/${REMOTE_NAME}/${BRANCH_NAME}"
    local LOCAL_AHEAD=0

    if git rev-parse "@" >/dev/null 2>&1 && git rev-parse "${UPSTREAM}" >/dev/null 2>&1; then
        LOCAL_AHEAD=$(git rev-list --count "${UPSTREAM}..@")
    elif ! git rev-parse "${UPSTREAM}" >/dev/null 2>&1; then
        # Remote branch doesn't exist, will push to create
        LOCAL_AHEAD=1
    fi

    if [ "$LOCAL_AHEAD" -eq 0 ]; then
        echo -e "${GREEN}Nothing to push, branch is up to date${NC}"
        return 0
    fi

    echo "Pushing to ${REMOTE_NAME}/${BRANCH_NAME}..."
    git push "${REMOTE_NAME}" "${BRANCH_NAME}"

    echo -e "${GREEN}Push successful${NC}"
}

do_sync() {
    local MSG="${1:-Auto sync $(date '+%Y-%m-%d %H:%M:%S')}"

    echo -e "${YELLOW}=== Git Sync ===${NC}"

    show_status

    do_commit "$MSG"
    do_push
}

show_help() {
    echo "VisionLoop Git Sync Script"
    echo ""
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  status          Show git status and recent commits"
    echo "  commit [msg]    Commit all changes (auto message if not provided)"
    echo "  push            Push commits to remote"
    echo "  sync [msg]      Commit and push in one step (default)"
    echo "  help            Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 status"
    echo "  $0 commit \"Fix encoding bug\""
    echo "  $0 sync \"Update MP4 mux\""
}

# Main
case "${1:-sync}" in
    status)
        show_status
        ;;
    commit)
        do_commit "${2:-}"
        ;;
    push)
        do_push
        ;;
    sync)
        do_sync "${2:-}"
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        echo -e "${RED}Unknown command: $1${NC}"
        show_help
        exit 1
        ;;
esac
