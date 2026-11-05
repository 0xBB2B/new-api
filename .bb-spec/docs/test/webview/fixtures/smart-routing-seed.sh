#!/usr/bin/env bash
# 给智能路由网页验收灌测试数据。可重复执行：已有的账号、渠道跳过，选项和日志每次覆盖。
# 用法：smart-routing-seed.sh [BASE_URL]，BASE_URL 默认 http://localhost:3000
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
cd "$SCRIPT_DIR/../../../../.."

BASE_URL=${1:-http://localhost:3000}
ROOT_USER=root
ROOT_PASS='Test123456!'
ALICE_USER=alice
ALICE_PASS='Test123456!'
ALICE_GROUP=api

COMPOSE=(docker compose -f docker-compose.dev.yml)
CODEX_KEY='{"access_token":"fake-access-token","account_id":"fake-account"}'

psql_exec() {
  "${COMPOSE[@]}" exec -T postgres psql -U root -d new-api -v ON_ERROR_STOP=1 -qAt "$@"
}

json_get() {
  python3 -I -c 'import json,sys
data=json.load(sys.stdin)
for part in sys.argv[1].split("."):
    data=data[part]
print(data if not isinstance(data,(dict,list)) else json.dumps(data))' "$1"
}

require_success() {
  local body=$1 what=$2
  if [[ $(printf '%s' "$body" | json_get success) != "True" ]]; then
    echo "失败：${what} -> $body" >&2
    exit 1
  fi
}

api() {
  local method=$1 path=$2 data=${3:-}
  local args=(-sS -X "$method" "$BASE_URL$path" -H "Authorization: Bearer $TOKEN" -H "New-Api-User: ${ROOT_ID}" -H 'Content-Type: application/json')
  if [[ -n $data ]]; then
    args+=(--data-binary "$data")
  fi
  curl "${args[@]}"
}

put_option() {
  local key=$1 value=$2 body
  body=$(api PUT /api/option/ "$(python3 -I -c 'import json,sys;print(json.dumps({"key":sys.argv[1],"value":sys.argv[2]}))' "${key}" "$value")")
  require_success "$body" "写入选项 ${key}"
}

echo "== 1. 初始化 root 管理员"
if [[ $(curl -sS "$BASE_URL/api/setup" | json_get data.status) == "True" ]]; then
  echo "root 已存在，跳过"
else
  body=$(curl -sS -X POST "$BASE_URL/api/setup" -H 'Content-Type: application/json' \
    --data-binary "{\"username\":\"${ROOT_USER}\",\"password\":\"${ROOT_PASS}\",\"confirmPassword\":\"${ROOT_PASS}\",\"SelfUseModeEnabled\":false,\"DemoSiteEnabled\":false}")
  require_success "$body" "初始化 root"
fi

# 每个用例都会重新登录，累积的会话会撞上每个用户的会话数上限（AUTH_SESSION_LIMIT）
psql_exec -c "delete from user_sessions" >/dev/null

login=$(curl -sS -X POST "$BASE_URL/api/user/login" -H 'Content-Type: application/json' \
  --data-binary "{\"username\":\"${ROOT_USER}\",\"password\":\"${ROOT_PASS}\"}")
require_success "$login" "root 登录"
TOKEN=$(printf '%s' "$login" | json_get data.access_token)
ROOT_ID=$(printf '%s' "$login" | json_get data.user.id)

echo "== 2. 分组与倍率"
put_option GroupRatio '{"default":1,"subscription":0.5,"api":1.2}'
put_option UserUsableGroups '{"default":"默认分组","subscription":"订阅","api":"按量"}'

echo "== 3. 普通用户 ${ALICE_USER}（分组 ${ALICE_GROUP}）"
if [[ -z $(psql_exec -c "select id from users where username='${ALICE_USER}' and deleted_at is null") ]]; then
  body=$(api POST /api/user/ "{\"username\":\"${ALICE_USER}\",\"password\":\"${ALICE_PASS}\",\"display_name\":\"Alice\",\"role\":1}")
  require_success "$body" "创建 ${ALICE_USER}"
else
  echo "${ALICE_USER} 已存在，跳过创建"
fi
ALICE_ID=$(psql_exec -c "select id from users where username='${ALICE_USER}' and deleted_at is null")
psql_exec -c "update users set \"group\"='${ALICE_GROUP}' where id=${ALICE_ID}" >/dev/null
# 用户信息有 Redis 缓存，改了分组要删掉缓存才能生效
"${COMPOSE[@]}" exec -T redis redis-cli DEL "user:${ALICE_ID}" >/dev/null

echo "== 4. 模型价格"
options=$(api GET /api/option/)
require_success "$options" "读取选项"
ratios=$(printf '%s' "$options" | python3 -I -c 'import json,sys
opts={o["key"]:o["value"] for o in json.load(sys.stdin)["data"]}
model=json.loads(opts.get("ModelRatio") or "{}")
completion=json.loads(opts.get("CompletionRatio") or "{}")
model.update({"gpt-5.4-mini":0.125,"gpt-5.5":1,"gpt-5.6-sol":1.25,"gpt-5.6-luna":0.05,"gpt-5.6-terra":1.5,"gpt-5.6-ghost":1})
completion.update({"gpt-5.4-mini":8,"gpt-5.5":8,"gpt-5.6-sol":8,"gpt-5.6-luna":5,"gpt-5.6-terra":8,"gpt-5.6-ghost":8})
model.pop("scorer-mini",None)
print(json.dumps(model)); print(json.dumps(completion))')
put_option ModelRatio "$(printf '%s\n' "$ratios" | sed -n 1p)"
put_option CompletionRatio "$(printf '%s\n' "$ratios" | sed -n 2p)"

echo "== 5. 渠道（密钥为假值）"
add_channel() {
  local name=$1 group=$2 models=$3 body
  if [[ -n $(psql_exec -c "select id from channels where name='$name'") ]]; then
    echo "渠道 $name 已存在，跳过"
    return
  fi
  body=$(api POST /api/channel/ "$(python3 -I -c 'import json,sys
print(json.dumps({"mode":"single","channel":{"name":sys.argv[1],"type":57,"key":sys.argv[4],"group":sys.argv[2],"models":sys.argv[3],"status":1}}))' "$name" "$group" "$models" "$CODEX_KEY")")
  require_success "$body" "创建渠道 $name"
}
add_channel codex-sub subscription 'gpt-5.5,gpt-5.6-sol,gpt-5.6-luna,gpt-5.6-terra'
add_channel codex-api api 'gpt-5.4-mini,gpt-5.5,gpt-5.6-sol,gpt-5.6-luna'
add_channel codex-unpriced 'subscription,api' 'scorer-mini'
SUB_CHANNEL=$(psql_exec -c "select id from channels where name='codex-sub'")
API_CHANNEL=$(psql_exec -c "select id from channels where name='codex-api'")

echo "== 6. 智能路由配置置空"
put_option smart_routing_setting.routes '[]'

echo "== 7. 使用日志"
NOW=$(date +%s)
python3 -I - "$NOW" "${ROOT_ID}" "${ALICE_ID}" "${SUB_CHANNEL}" "${API_CHANNEL}" "${ROOT_USER}" "${ALICE_USER}" <<'PY' | psql_exec >/dev/null
import json, sys
now, root_id, alice_id, sub_ch, api_ch = map(int, sys.argv[1:6])
root_name, alice_name = sys.argv[6], sys.argv[7]

def main(tier, reason, params, scorer, labels=None, cleaned=0, scored=None):
    routing = {"role": "main", "virtual_model": "auto-gpt", "tier": tier, "reason": reason,
               "reason_params": params, "scorer": scorer}
    if scored:
        routing["scored_tier"] = scored
    if labels:
        routing["labels"] = labels
    if cleaned:
        routing["cleaned_reasoning"] = cleaned
    return routing

def scorer_record(main_tier, main_model):
    return {"role": "scorer", "virtual_model": "auto-gpt", "main_tier": main_tier, "main_model": main_model}

ok = {"status": "ok", "billed": True}
rows = [
    # 秒数偏移, 用户, 用户名, 令牌, 模型, 分组, 输入, 输出, 额度, 耗时, 流式, 渠道, 请求 ID, 倍率, 补全倍率, smart_routing
    (60, root_id, root_name, "chat-key", "gpt-5.5", "subscription", 3402, 512, 3000, 4, True, sub_ch, "req_b2c9f0", 1, 8,
     main("normal", "scorer_timeout", {"timeout_ms": 2000}, {"status": "timeout", "billed": False, "timeout_ms": 2000})),
    (90, alice_id, alice_name, "alice-key", "gpt-5.5", "api", 2100, 380, 2500, 3, True, api_ch, "req_alice01", 1, 8,
     main("normal", "rule_normal", {}, ok, labels={"task_type": "rewrite", "reasoning": "low", "workload": "one_step",
          "risk": "low", "domain": "other", "confidence": 0.82, "feedback": "satisfied"})),
    (91, alice_id, alice_name, "alice-key", "gpt-5.6-luna", "api", 980, 64, 80, 1, False, api_ch, "req_alice01", 0.05, 5,
     scorer_record("normal", "gpt-5.5")),
    (120, root_id, root_name, "chat-key", "gpt-5.6-sol", "subscription", 12880, 2210, 30550, 18, True, sub_ch, "req_a7310e", 1.25, 8,
     main("advisor", "advisor", {"count": 2}, ok, labels={"task_type": "generate", "reasoning": "medium", "workload": "few_steps",
          "risk": "low", "domain": "operations", "confidence": 0.78, "feedback": "dissatisfied"})),
    (180, root_id, root_name, "codex-key", "gpt-5.6-sol", "subscription", 86420, 420, 9500, 5, True, sub_ch, "req_90a1d3", 1.25, 8,
     main("hard", "downgrade_blocked", {"history_tokens": 86420, "limit": 20000, "kept_tier": "hard", "scored_tier": "simple"}, ok,
          labels={"task_type": "chat_qa", "reasoning": "low", "workload": "one_step", "risk": "low", "domain": "code",
                  "confidence": 0.91, "feedback": "satisfied"}, scored="simple")),
    (240, root_id, root_name, "codex-key", "gpt-5.6-sol", "subscription", 41377, 612, 6500, 6, True, sub_ch, "req_8f2c77", 1.25, 8,
     main("hard", "tool_continuation", {}, {"status": "skipped", "billed": False})),
    (300, root_id, root_name, "codex-key", "gpt-5.6-sol", "subscription", 38120, 1904, 41050, 14, True, sub_ch, "req_8f2c41", 1.25, 8,
     main("hard", "rule_hard", {"matched": ["risk"]}, ok, labels={"task_type": "analyze", "reasoning": "medium", "workload": "few_steps",
          "risk": "high", "domain": "finance", "confidence": 0.86, "feedback": "neutral"}, cleaned=2)),
    (301, root_id, root_name, "codex-key", "gpt-5.6-luna", "api", 1240, 86, 100, 1, False, api_ch, "req_8f2c41", 0.05, 5,
     scorer_record("hard", "gpt-5.6-sol")),
    (360, alice_id, alice_name, "alice-key", "gpt-5.5", "api", 5200, 300, 2400, 3, True, api_ch, "req_plain01", 1, 8, None),
]

ids = ",".join(f"'{r[12]}'" for r in rows)
print("begin;")
print(f"delete from logs where request_id in ({ids});")
# 按时间先后插入，让 id 和时间同序：真实流程里打分记录先于主请求写入
for (offset, uid, uname, token, model, group, prompt, completion, quota, use_time, stream, channel, req, ratio, comp, routing) in sorted(rows, key=lambda r: -r[0]):
    other = {"model_ratio": ratio, "completion_ratio": comp, "group_ratio": 0.5 if group == "subscription" else 1.2,
             "model_price": -1, "request_path": "/v1/responses"}
    if stream:
        other["frt"] = 800
    if routing:
        other["smart_routing"] = routing
    print("insert into logs (user_id, created_at, type, content, username, token_name, model_name, quota, prompt_tokens,"
          " completion_tokens, use_time, is_stream, channel_id, token_id, \"group\", ip, request_id, upstream_request_id, other)"
          f" values ({uid}, {now - offset}, 2, '', '{uname}', '{token}', '{model}', {quota}, {prompt}, {completion}, {use_time},"
          f" {'true' if stream else 'false'}, {channel}, 0, '{group}', '', '{req}', '', $j${json.dumps(other)}$j$);")
print("commit;")
PY

echo "== 8. 清零登录类接口的限流计数"
# 登录、刷新会话共用每个 IP 20 分钟 20 次的限额，网页用例每次整页加载都会刷新一次会话，不清零跑不完一轮
"${COMPOSE[@]}" exec -T redis redis-cli --scan --pattern 'rateLimit:v2:ip:CT:*' | tr -d '\r' | while read -r rate_key; do
  "${COMPOSE[@]}" exec -T redis redis-cli DEL "$rate_key" >/dev/null </dev/null
done

cat <<EOF

================ 数据摘要 ================
站点：$BASE_URL
账号：
  管理员 ${ROOT_USER} / ${ROOT_PASS}（id ${ROOT_ID}）
  普通用户 ${ALICE_USER} / ${ALICE_PASS}（id ${ALICE_ID}，分组 ${ALICE_GROUP}）
分组倍率：default 1、subscription 0.5、api 1.2（用户可选 default / subscription / api）
渠道（Codex 类型，只走 Responses，密钥为假值）：
  codex-sub（#${SUB_CHANNEL}，subscription）：gpt-5.5、gpt-5.6-sol、gpt-5.6-luna、gpt-5.6-terra
  codex-api（#${API_CHANNEL}，api）：gpt-5.4-mini、gpt-5.5、gpt-5.6-sol、gpt-5.6-luna
  codex-unpriced（subscription,api）：scorer-mini
模型：
  gpt-5.4-mini   简单档   只在 api，有价格
  gpt-5.5        普通档   subscription + api，有价格
  gpt-5.6-sol    困难档/顾问 subscription + api，有价格
  gpt-5.6-luna   打分模型 subscription + api，有价格
  gpt-5.6-terra  只在 subscription（用来触发"在分组 api 里没有可用渠道"），有价格
  gpt-5.6-ghost  所有分组都没有渠道，有价格（下拉里不应出现）
  scorer-mini    有渠道但没定价
智能路由配置 smart_routing_setting.routes = []
使用日志（虚拟模型都是 auto-gpt，时间为执行脚本前 1~6 分钟）：
  req_b2c9f0   root   gpt-5.5       打分超时，普通档（默认档）
  req_alice01  alice  gpt-5.5       普通档，rule_normal
  req_alice01  alice  gpt-5.6-luna  打分记录（属于 alice 的主请求）
  req_a7310e   root   gpt-5.6-sol   交给顾问，连续 2 条不满意
  req_90a1d3   root   gpt-5.6-sol   降档被拦，打分结果简单档
  req_8f2c77   root   gpt-5.6-sol   工具结果续跑
  req_8f2c41   root   gpt-5.6-sol   困难档，出错代价=高，清理 2 段思考内容
  req_8f2c41   root   gpt-5.6-luna  打分记录（属于上一条）
  req_plain01  alice  gpt-5.5       普通日志，没有 smart_routing
==========================================
EOF
