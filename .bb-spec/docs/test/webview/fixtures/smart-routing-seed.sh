#!/usr/bin/env bash
# 给智能路由网页验收灌测试数据。可重复执行：已有的账号、渠道、密钥跳过，选项和日志每次覆盖。
# 会重启后端容器一次：一期格式的旧配置只在启动时迁移，要先写进数据库再重启。
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
CHANNEL_TYPE_OPENAI=1
CHANNEL_TYPE_MINIMAX=35
CHANNEL_TYPE_CODEX=57

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

root_login() {
  local login
  login=$(curl -sS -X POST "$BASE_URL/api/user/login" -H 'Content-Type: application/json' \
    --data-binary "{\"username\":\"${ROOT_USER}\",\"password\":\"${ROOT_PASS}\"}")
  require_success "$login" "root 登录"
  TOKEN=$(printf '%s' "$login" | json_get data.access_token)
  ROOT_ID=$(printf '%s' "$login" | json_get data.user.id)
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
root_login

echo "== 2. 分组与倍率"
# team-a / team-b / team-c 只给智能路由的分组顺序用：凑满 5 个分组，以及放 MiniMax、Codex + OpenAI 的渠道组合
put_option GroupRatio '{"default":1,"subscription":0.5,"api":1.2,"team-a":1,"team-b":1,"team-c":1}'
# auto 让密钥抽屉能选「自动分组」，用来核对密钥里的分组顺序组件没被改样子
put_option UserUsableGroups '{"default":"默认分组","subscription":"订阅","api":"按量","auto":"自动分组"}'

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
model.update({"gpt-5.4-mini":0.125,"gpt-5.5":1,"gpt-5.6-sol":1.25,"gpt-5.6-luna":0.05,"gpt-5.6-terra":1.5,"gpt-5.6-ghost":1,
              "gpt-4.1":1,"glm-5.1":0.5,"claude-sonnet-4-5":1.5,"gemini-2.5-pro":0.625,"deepseek-v4-pro":0.3,"o3-pro":10,
              "claude-opus-4-6":2.5,"gemini-3.8-flash":0.15,"gpt-5.2":0.875})
completion.update({"gpt-5.4-mini":8,"gpt-5.5":8,"gpt-5.6-sol":8,"gpt-5.6-luna":5,"gpt-5.6-terra":8,"gpt-5.6-ghost":8,
                   "gpt-4.1":4,"glm-5.1":4,"claude-sonnet-4-5":5,"gemini-2.5-pro":8,"deepseek-v4-pro":2,"o3-pro":4,
                   "claude-opus-4-6":5,"gemini-3.8-flash":4,"gpt-5.2":8})
model.pop("scorer-mini",None)
print(json.dumps(model)); print(json.dumps(completion))')
put_option ModelRatio "$(printf '%s\n' "$ratios" | sed -n 1p)"
put_option CompletionRatio "$(printf '%s\n' "$ratios" | sed -n 2p)"

echo "== 5. 渠道（密钥为假值）"
add_channel() {
  local name=$1 type=$2 group=$3 models=$4 key=$5 body
  if [[ -n $(psql_exec -c "select id from channels where name='$name'") ]]; then
    echo "渠道 $name 已存在，跳过"
    return
  fi
  body=$(api POST /api/channel/ "$(python3 -I -c 'import json,sys
print(json.dumps({"mode":"single","channel":{"name":sys.argv[1],"type":int(sys.argv[2]),"key":sys.argv[5],"group":sys.argv[3],"models":sys.argv[4],"status":1}}))' "$name" "$type" "$group" "$models" "$key")")
  require_success "$body" "创建渠道 $name"
}
add_channel codex-sub "$CHANNEL_TYPE_CODEX" subscription 'gpt-5.5,gpt-5.6-sol,gpt-5.6-luna,gpt-5.6-terra' "$CODEX_KEY"
add_channel codex-api "$CHANNEL_TYPE_CODEX" api 'gpt-5.4-mini,gpt-5.5,gpt-5.6-sol,gpt-5.6-luna' "$CODEX_KEY"
add_channel codex-unpriced "$CHANNEL_TYPE_CODEX" 'subscription,api' 'scorer-mini' "$CODEX_KEY"
add_channel codex-team "$CHANNEL_TYPE_CODEX" team-b 'gpt-5.5' "$CODEX_KEY"
add_channel openai-sub "$CHANNEL_TYPE_OPENAI" subscription 'gpt-5.6-sol,gpt-4.1,glm-5.1' 'sk-fake-openai-key'
add_channel openai-api "$CHANNEL_TYPE_OPENAI" api 'gpt-5.5,gpt-5.6-sol,gpt-4.1,glm-5.1,claude-sonnet-4-5,gemini-2.5-pro,deepseek-v4-pro,o3-pro' 'sk-fake-openai-key'
# 已有环境里 openai-api 会被跳过、不会补模型，所以能力表口径变更后新加的模型单独放一个渠道
add_channel openai-api-cap "$CHANNEL_TYPE_OPENAI" api 'claude-opus-4-6,gemini-3.8-flash,gpt-5.2' 'sk-fake-openai-key'
add_channel openai-team "$CHANNEL_TYPE_OPENAI" team-b 'gpt-5.5' 'sk-fake-openai-key'
add_channel minimax-team "$CHANNEL_TYPE_MINIMAX" team-a 'gpt-5.5' 'fake-minimax-key'
SUB_CHANNEL=$(psql_exec -c "select id from channels where name='codex-sub'")
API_CHANNEL=$(psql_exec -c "select id from channels where name='codex-api'")
OPENAI_API_CHANNEL=$(psql_exec -c "select id from channels where name='openai-api'")

echo "== 6. 密钥"
if [[ -z $(psql_exec -c "select id from tokens where name='cc-switch-key' and user_id=${ROOT_ID} and deleted_at is null") ]]; then
  body=$(api POST /api/token/ '{"name":"cc-switch-key","expired_time":-1,"unlimited_quota":true,"remain_quota":0,"group":""}')
  require_success "$body" "创建密钥 cc-switch-key"
else
  echo "密钥 cc-switch-key 已存在，跳过"
fi
# vm-limit-key 由 key-model-limits-virtual 用例自己新建，先删掉上次留下的
psql_exec -c "delete from tokens where name='vm-limit-key'" >/dev/null

echo "== 7. 关掉「Chat 转 Responses」策略和全局透传"
put_option global.chat_completions_to_responses_policy '{"enabled":false,"all_channels":true}'
put_option global.pass_through_request_enabled false

echo "== 8. 写入一期格式的智能路由配置，重启后端触发迁移"
# 一期格式：没有 endpoints，档位只有 effort（困难档 high，其他两档不覆盖）。保存接口会拒收这种格式，只能直接写数据库
LEGACY_ROUTES='[{"name":"auto-gpt","tiers":{"simple":{"model":"gpt-5.4-mini","groups":["api"],"effort":""},"normal":{"model":"gpt-5.5","groups":["subscription","api"],"effort":""},"hard":{"model":"gpt-5.6-sol","groups":["subscription","api"],"effort":"high"}},"default_tier":"normal","scorer":{"model":"gpt-5.6-luna","groups":["subscription","api"],"timeout_ms":2000},"advisor":{"model":"gpt-5.6-sol","groups":["subscription","api"],"after_dissatisfied":2},"downgrade_max_history_tokens":20000,"confidence_threshold":0.6,"simple_max_message_chars":2000}]'
psql_exec -v routes="$LEGACY_ROUTES" >/dev/null <<'SQL'
insert into options ("key", value) values ('smart_routing_setting.routes', :'routes')
  on conflict ("key") do update set value = excluded.value;
insert into options ("key", value) values ('smart_routing_setting.thinking_capabilities', '{}')
  on conflict ("key") do update set value = excluded.value;
SQL
"${COMPOSE[@]}" restart new-api >/dev/null
for _ in $(seq 1 90); do
  if curl -fsS "$BASE_URL/api/status" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done
curl -fsS "$BASE_URL/api/status" >/dev/null || { echo "失败：后端重启后 3 分钟内没有起来" >&2; exit 1; }
MIGRATED_ROUTES=$(psql_exec -c "select value from options where \"key\"='smart_routing_setting.routes'")
if [[ $MIGRATED_ROUTES != *'"endpoints":["openai-response"]'* || $MIGRATED_ROUTES != *'"effort_mode":"fixed"'* ]]; then
  echo "失败：启动迁移没有把一期配置改成新格式 -> $MIGRATED_ROUTES" >&2
  exit 1
fi
root_login

echo "== 9. 使用日志"
NOW=$(date +%s)
python3 -I - "$NOW" "${ROOT_ID}" "${ALICE_ID}" "${SUB_CHANNEL}" "${API_CHANNEL}" "${OPENAI_API_CHANNEL}" "${ROOT_USER}" "${ALICE_USER}" <<'PY' | psql_exec >/dev/null
import json, sys
now, root_id, alice_id, sub_ch, api_ch, openai_ch = map(int, sys.argv[1:7])
root_name, alice_name = sys.argv[7], sys.argv[8]

# 字段格式和 service/smartroute/log.go 的 MainLogFields 一致；effort 为 None 表示一期写入、没有思考力度的旧日志
def main(tier, reason, params, scorer, labels=None, cleaned=0, scored=None, effort=None, virtual="auto-gpt"):
    routing = {"role": "main", "virtual_model": virtual, "tier": tier, "reason": reason,
               "reason_params": params, "scorer": scorer}
    if scored:
        routing["scored_tier"] = scored
    if labels:
        routing["labels"] = labels
    if effort:
        routing["effort"] = effort
    if cleaned:
        routing["cleaned_reasoning"] = cleaned
    return routing

def effort(value, source, frm=None):
    data = {"value": value, "source": source}
    if frm:
        data["from"] = frm
    return data

def scorer_record(main_tier, main_model):
    return {"role": "scorer", "virtual_model": "auto-gpt", "main_tier": main_tier, "main_model": main_model}

def labels(task, reasoning, workload, risk, domain, confidence, feedback):
    return {"task_type": task, "reasoning": reasoning, "workload": workload, "risk": risk, "domain": domain,
            "confidence": confidence, "feedback": feedback}

ok = {"status": "ok", "billed": True}
rows = [
    # 秒数偏移, 用户, 用户名, 令牌, 模型, 分组, 输入, 输出, 额度, 耗时, 流式, 渠道, 请求 ID, 倍率, 补全倍率, smart_routing
    (60, root_id, root_name, "chat-key", "gpt-5.5", "subscription", 3402, 512, 3000, 4, True, sub_ch, "req_b2c9f0", 1, 8,
     main("normal", "scorer_timeout", {"timeout_ms": 2000}, {"status": "timeout", "billed": False, "timeout_ms": 2000},
          effort=effort("high", "follow", "xhigh"))),
    (90, alice_id, alice_name, "alice-key", "gpt-5.5", "api", 2100, 380, 2500, 3, True, api_ch, "req_alice01", 1, 8,
     main("normal", "rule_normal", {}, ok, labels=labels("rewrite", "low", "one_step", "low", "other", 0.82, "satisfied"),
          effort=effort("medium", "client"))),
    (91, alice_id, alice_name, "alice-key", "gpt-5.6-luna", "api", 980, 64, 80, 1, False, api_ch, "req_alice01", 0.05, 5,
     scorer_record("normal", "gpt-5.5")),
    (120, root_id, root_name, "chat-key", "gpt-5.6-sol", "subscription", 12880, 2210, 30550, 18, True, sub_ch, "req_a7310e", 1.25, 8,
     main("advisor", "advisor", {"count": 2}, ok, labels=labels("generate", "medium", "few_steps", "low", "operations", 0.78, "dissatisfied"),
          effort=effort("low", "client"))),
    (180, root_id, root_name, "codex-key", "gpt-5.6-sol", "subscription", 86420, 420, 9500, 5, True, sub_ch, "req_90a1d3", 1.25, 8,
     main("hard", "downgrade_blocked", {"history_tokens": 86420, "limit": 20000, "kept_tier": "hard", "scored_tier": "simple"}, ok,
          labels=labels("chat_qa", "low", "one_step", "low", "code", 0.91, "satisfied"), scored="simple",
          effort=effort("high", "fixed"))),
    (240, root_id, root_name, "codex-key", "gpt-5.6-sol", "subscription", 41377, 612, 6500, 6, True, sub_ch, "req_8f2c77", 1.25, 8,
     main("hard", "tool_continuation", {}, {"status": "skipped", "billed": False}, effort=effort("high", "follow"))),
    (300, root_id, root_name, "codex-key", "gpt-5.6-sol", "subscription", 38120, 1904, 41050, 14, True, sub_ch, "req_8f2c41", 1.25, 8,
     main("hard", "rule_hard", {"matched": ["risk"]}, ok, labels=labels("analyze", "medium", "few_steps", "high", "finance", 0.86, "neutral"),
          cleaned=2, effort=effort("medium", "follow"))),
    (301, root_id, root_name, "codex-key", "gpt-5.6-luna", "api", 1240, 86, 100, 1, False, api_ch, "req_8f2c41", 0.05, 5,
     scorer_record("hard", "gpt-5.6-sol")),
    (360, alice_id, alice_name, "alice-key", "gpt-5.5", "api", 5200, 300, 2400, 3, True, api_ch, "req_plain01", 1, 8, None),
    (400, root_id, root_name, "chat-key", "gpt-5.4-mini", "api", 1357, 79, 150, 2, True, api_ch, "req_eff_raise", 0.125, 8,
     main("simple", "rule_simple", {}, ok, labels=labels("lookup", "low", "one_step", "low", "code", 0.88, "satisfied"),
          effort=effort("low", "client", "minimal"))),
    (420, root_id, root_name, "chat-key", "gpt-4.1", "api", 2468, 135, 900, 2, True, openai_ch, "req_eff_nothink", 1, 4,
     main("simple", "rule_simple", {}, ok, labels=labels("chat_qa", "low", "one_step", "low", "other", 0.9, "satisfied"),
          effort=effort("", "no_thinking"), virtual="auto-chat")),
    (440, root_id, root_name, "chat-key", "gpt-5.5", "api", 3579, 246, 2800, 3, True, api_ch, "req_eff_unset", 1, 8,
     main("normal", "rule_normal", {}, ok, labels=labels("rewrite", "low", "one_step", "low", "other", 0.8, "satisfied"),
          effort=effort("", "client_unset"))),
    (460, root_id, root_name, "chat-key", "gpt-5.5", "subscription", 4680, 357, 3100, 3, True, sub_ch, "req_eff_legacy", 1, 8,
     main("normal", "rule_normal", {}, ok, labels=labels("rewrite", "low", "one_step", "low", "other", 0.83, "satisfied"))),
]

ids = ",".join(f"'{r[12]}'" for r in rows)
print("begin;")
print(f"delete from logs where request_id in ({ids});")
# 按时间先后插入，让 id 和时间同序：真实流程里打分记录先于主请求写入
for (offset, uid, uname, token, model, group, prompt, completion, quota, use_time, stream, channel, req, ratio, comp, routing) in sorted(rows, key=lambda r: -r[0]):
    path = "/v1/chat/completions" if routing and routing.get("virtual_model") == "auto-chat" else "/v1/responses"
    other = {"model_ratio": ratio, "completion_ratio": comp, "group_ratio": 0.5 if group == "subscription" else 1.2,
             "model_price": -1, "request_path": path}
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

echo "== 10. 清零登录类接口的限流计数"
# 登录、刷新会话共用每个 IP 20 分钟 20 次的限额，网页用例每次整页加载都会刷新一次会话，不清零跑不完一轮
"${COMPOSE[@]}" exec -T redis redis-cli --scan --pattern 'rateLimit:v2:ip:CT:*' | tr -d '\r' | while read -r rate_key; do
  "${COMPOSE[@]}" exec -T redis redis-cli DEL "$rate_key" >/dev/null </dev/null
done

cat <<EOF

================ 数据摘要 ================
站点：$BASE_URL
账号：
  管理员 ${ROOT_USER} / ${ROOT_PASS}（id ${ROOT_ID}），密钥 cc-switch-key
  普通用户 ${ALICE_USER} / ${ALICE_PASS}（id ${ALICE_ID}，分组 ${ALICE_GROUP}）
分组倍率：default 1、subscription 0.5、api 1.2、team-a / team-b / team-c 1
用户可选分组：default / subscription / api / auto
渠道（密钥为假值；Codex 只收 Responses，OpenAI 三种格式都收，MiniMax 不收 Responses）：
  codex-sub      Codex   subscription      gpt-5.5、gpt-5.6-sol、gpt-5.6-luna、gpt-5.6-terra
  codex-api      Codex   api               gpt-5.4-mini、gpt-5.5、gpt-5.6-sol、gpt-5.6-luna
  codex-unpriced Codex   subscription,api  scorer-mini
  codex-team     Codex   team-b            gpt-5.5
  openai-sub     OpenAI  subscription      gpt-5.6-sol、gpt-4.1、glm-5.1
  openai-api     OpenAI  api               gpt-5.5、gpt-5.6-sol、gpt-4.1、glm-5.1、claude-sonnet-4-5、gemini-2.5-pro、deepseek-v4-pro、o3-pro
  openai-api-cap OpenAI  api               claude-opus-4-6、gemini-3.8-flash、gpt-5.2
  openai-team    OpenAI  team-b            gpt-5.5
  minimax-team   MiniMax team-a            gpt-5.5
关键组合：
  gpt-5.5 在 subscription 里只有 Codex 渠道（勾 Anthropic / Chat 会被拦下）；在 team-b 里有 Codex 和 OpenAI；在 team-a 里只有 MiniMax
  gpt-5.4-mini 只有 Codex 渠道，模型列表里只支持 Responses；o3-pro 只支持 Responses、glm-5.1 只支持 Chat（都在 OpenAI 渠道上）
  gpt-5.6-terra 只在 subscription；gpt-5.6-ghost 有价格没渠道；scorer-mini 有渠道没价格
内置思考能力（只收 2026 年发布的模型，不支持思考的老模型除外）：gpt-5.4-mini、gpt-5.5、gpt-5.6-sol、gpt-4.1（不支持思考）、claude-opus-4-6、gemini-3.8-flash、deepseek-v4-pro；
  未确认：glm-5.1、o3-pro，以及 2025 年发布的 claude-sonnet-4-5、gemini-2.5-pro、gpt-5.2
智能路由配置：一期格式的 auto-gpt 已由启动迁移改成新格式（接口 Responses，困难档固定 high），模型思考能力覆盖为空
使用日志（时间为执行脚本前 1~8 分钟；括号里是思考力度）：
  req_b2c9f0      root   gpt-5.5       打分超时，普通档（high，跟随打分，由 xhigh 降档）
  req_alice01     alice  gpt-5.5       普通档（medium，不覆盖，客户端传入）
  req_alice01     alice  gpt-5.6-luna  打分记录
  req_a7310e      root   gpt-5.6-sol   交给顾问（low，客户端传入）
  req_90a1d3      root   gpt-5.6-sol   降档被拦（high，固定）
  req_8f2c77      root   gpt-5.6-sol   工具结果续跑（high，跟随打分）
  req_8f2c41      root   gpt-5.6-sol   困难档，清理 2 段思考内容（medium，跟随打分）
  req_8f2c41      root   gpt-5.6-luna  打分记录
  req_plain01     alice  gpt-5.5       普通日志，没有 smart_routing
  req_eff_raise   root   gpt-5.4-mini  简单档（low，不覆盖，由 minimal 升档）
  req_eff_nothink root   gpt-4.1       auto-chat 简单档（不支持思考）
  req_eff_unset   root   gpt-5.5       普通档（客户端未指定）
  req_eff_legacy  root   gpt-5.5       一期旧日志，没有思考力度
==========================================
EOF
