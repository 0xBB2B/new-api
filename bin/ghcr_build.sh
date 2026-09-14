#!/bin/bash
set -euo pipefail

usage() {
  cat <<'USAGE'
用法: bin/ghcr_build.sh [branch] [--watch]
  branch  要构建的分支，缺省为当前分支；是当前分支时会先推送到 origin
  --watch 触发后持续跟进运行状态直到结束
USAGE
}

WATCH=false
BRANCH=""
for arg in "$@"; do
  case "$arg" in
    --watch) WATCH=true ;;
    -h|--help) usage; exit 0 ;;
    -*) echo "未知选项: $arg" >&2; usage >&2; exit 1 ;;
    *) BRANCH="$arg" ;;
  esac
done
BRANCH=${BRANCH:-$(git branch --show-current)}
[ -n "$BRANCH" ] || { echo "无法确定分支，请显式传入分支名" >&2; exit 1; }

REPO=$(git remote get-url origin | sed -E 's#.*github\.com[:/]##; s#\.git$##')
OWNER=${REPO%%/*}
WORKFLOW=docker-image-ghcr.yml

GH_TOKEN=$(gh auth token --user "$OWNER") || { echo "gh 未登录账号 $OWNER" >&2; exit 1; }
export OWNER GH_TOKEN

if [ "$BRANCH" = "$(git branch --show-current)" ]; then
  # shellcheck disable=SC2016
  git -c credential.helper= \
    -c 'credential.helper=!f(){ echo username=$OWNER; echo password=$GH_TOKEN; }; f' \
    push -u origin "$BRANCH"
elif ! git ls-remote --exit-code --heads origin "$BRANCH" >/dev/null; then
  echo "origin 上不存在分支 $BRANCH，请先推送" >&2
  exit 1
fi

latest_run_id() {
  gh run list --repo "$REPO" --workflow "$WORKFLOW" --event workflow_dispatch --limit 1 --json databaseId -q '.[0].databaseId // 0'
}

BEFORE=$(latest_run_id)
gh workflow run "$WORKFLOW" --repo "$REPO" -f "branch=$BRANCH"
RUN_ID=$BEFORE
for _ in $(seq 1 12); do
  sleep 5
  RUN_ID=$(latest_run_id)
  [ "$RUN_ID" -gt "$BEFORE" ] && break
done
[ "$RUN_ID" -gt "$BEFORE" ] || { echo "60 秒内未见新的运行记录，请到 Actions 页面确认" >&2; exit 1; }
echo "run: https://github.com/$REPO/actions/runs/$RUN_ID"

if [ "$WATCH" = true ]; then
  gh run watch "$RUN_ID" --repo "$REPO" --exit-status
fi
