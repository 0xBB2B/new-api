---
name: sync-upstream
description: >-
  一键把上游仓库（upstream = QuantumNous/new-api）默认分支的最新代码合并进本地
  main，验证通过后用 0xBB2B 账号推送到 origin/main。用于：同步上游、合并上游、
  拉上游最新代码、sync upstream、merge upstream/main、更新 fork。执行同步前必须
  先加载本 skill，按步骤走，不要临场自拟流程。
---

# 同步上游到 main

本仓库是 fork：`origin` = `0xBB2B/new-api`（自己的 fork），`upstream` = `QuantumNous/new-api`（上游）。fork 的 main 上有上游没有的自有功能提交，所以同步只能用 merge，把上游合进来。

调用本 skill 就等于授权这次推送 origin/main，不用再问。遇到下面「停下」的情况时，说清楚停在哪、为什么停、现在仓库处于什么状态，然后等用户决定。

## 硬规则

1. **只在临时分支 `chore/sync-upstream` 上 merge，不在 main 上 merge。** 工作流 hook 会拦 main 上的 `git commit`，但不拦 `git merge`。在 main 上 merge 一旦冲突，收尾的 commit 会被拦，main 就卡在合并了一半的状态。
2. **合并结果必须是双父 merge commit。** 禁止 squash、rebase、cherry-pick 上游提交，禁止 GitHub 网页的 Sync fork / Discard commits。历史被压平后，下次同步会把合过的上游提交全部重放，冒出大量假冲突。
3. **禁止 force push。** 推送被拒就停下。
4. **GitHub 写操作只用 0xBB2B 账号**，每次都显式取令牌，不依赖 gh 当前激活的账号。

## 步骤

每次 Bash 调用都是新 shell，变量不会带到下一次调用。`TARGET`、`OLD_MAIN`、`UP_SHA` 求出来后记下具体值，后面的命令里直接写值；第 8 步的推送命令要放在同一次调用里跑完。

### 1. 前置检查（任一不满足就停下）

```bash
git status --porcelain --untracked-files=no   # 必须为空：有未提交改动就停（未跟踪文件不影响合并）
git rev-parse -q --verify MERGE_HEAD   # 必须无输出：有正在进行的 merge 就停
git worktree list                      # main 不能被其他 worktree 占用，否则后面切不到 main
git rev-parse -q --verify refs/heads/chore/sync-upstream
```

如果 `chore/sync-upstream` 已存在：
- 它已经合进 main（`git merge-base --is-ancestor chore/sync-upstream main` 成功）：`git branch -d chore/sync-upstream` 删掉，继续。
- 它没合进 main：停下，用 `git log --oneline main..chore/sync-upstream` 给用户看上面挂着什么，问用户是先把它合进 main 还是丢掉。

### 2. 拉取并确定目标

```bash
git fetch origin
git fetch upstream
git remote set-head upstream --auto            # 上游改了默认分支名也能跟上
TARGET=$(git rev-parse --abbrev-ref upstream/HEAD)   # 例如 upstream/main
```

### 3. 本地 main 追平 origin/main

```bash
git switch main
git merge --ff-only origin/main   # 失败说明本地 main 和 origin/main 分叉了：停下
git merge-base --is-ancestor "$TARGET" main && echo "已是最新"
```

输出「已是最新」就结束，告诉用户上游没有新提交。

### 4. 检查上游是否占了本地渠道编号

fork 的 Claude 订阅渠道用的是 `constant/channel.go` 里编号最大的那个 `ChannelTypeClaudeSubscription`。上游多次新增渠道，占用了这个编号。

```bash
UP_MAX=$(git show "$TARGET":constant/channel.go | sed -n 's/^[[:space:]]*ChannelType[A-Za-z0-9]*[[:space:]]*=[[:space:]]*\([0-9][0-9]*\).*/\1/p' | sort -n | tail -1)
OUR=$(git show main:constant/channel.go | sed -n 's/^[[:space:]]*ChannelTypeClaudeSubscription[[:space:]]*=[[:space:]]*\([0-9][0-9]*\).*/\1/p')
echo "上游最大编号=$UP_MAX 本地订阅渠道=$OUR"
```

`UP_MAX >= OUR` 就是撞号：停下，不要 merge。改号要同时改后端常量、前端 `web/src/features/channels/constants.ts`，还要在 `model/channel_type_migration.go` 追加一段数据迁移，需要用户单独处理。

### 5. 在临时分支上合并

```bash
OLD_MAIN=$(git rev-parse --short main)
UP_SHA=$(git rev-parse --short "$TARGET")
git switch -c chore/sync-upstream
git merge --no-ff --no-edit -m "chore: merge $TARGET ($UP_SHA)" "$TARGET"
```

没有冲突就跳到第 7 步。

### 6. 解决冲突

```bash
git diff --name-only --diff-filter=U        # 冲突文件清单
git log --oneline --no-merges "$TARGET"..main   # fork 自有提交，解决冲突时要保住它们
```

逐个文件处理，原则是**两边的改动都保留**：
- 先看清两边各改了什么：`git show :1:<file>`（共同祖先）、`:2:`（本地 main）、`:3:`（上游）。上游的重构、改名、移动要跟上，fork 的自有功能要搬到新结构里继续生效。
- `web/src/i18n/locales/*.json`：只改冲突块，两边新增的 key 都留下，删掉重复 key，修好逗号。**不要用 jq 或其他 JSON 工具把整个文件重新输出**，文件里有故意写成转义形式的内容（如 `newapi`），重写会把它们改掉。改完检查：

  ```bash
  python3 -c 'import json,sys
  def pairs(p):
      seen, dup = set(), set()
      for k, _ in p:
          (dup if k in seen else seen).add(k)
      if dup:
          raise ValueError(f"重复 key: {sorted(dup)}")
      return dict(p)
  bad = 0
  for f in sys.argv[1:]:
      try:
          json.load(open(f, encoding="utf-8"), object_pairs_hook=pairs)
      except ValueError as e:
          print(f"{f}: {e}"); bad = 1
  sys.exit(bad)' web/src/i18n/locales/*.json
  ```
- 冲突涉及计费文件（范围见 `AGENTS.md` 的 Billing rules 一节）时，先完整读 `.agents/rules/billing.md` 再改。
- 不许改动 new-api / QuantumNous 相关的署名、品牌信息。

遇到下面的情况就停下，把冲突文件和两边的改动讲给用户听，等用户拍板：
- 两边改的是同一段逻辑，而且意图互相矛盾，看不出该保留谁。
- 冲突在数据库迁移代码里（`model/main.go` 的迁移、`model/*migration*.go`）。

一边是软链接、一边是普通文件这类类型冲突，git 会在工作区留下 `<文件>~<提交号>` 形式的副本（例如 `CLAUDE.md~789c97019`）。解决后把这些副本删掉，它们是未跟踪的垃圾文件，`git add` 不会处理它们。

全部解决后：

```bash
gofmt -w <手工解决过的 .go 文件>
git add <已解决的文件>                  # 逐个列出，不用 git add .；一侧删除、决定删掉的文件用 git rm <文件>
git diff --cached --check              # 不能有残留的冲突标记
git status --porcelain                 # 不能再有 U 开头的行
git commit --no-edit                   # 当前在 chore/sync-upstream，hook 允许
git rev-list --parents -n 1 HEAD       # 必须输出 3 个 SHA（自己 + 两个父提交）
```

### 7. 验证（任一失败就停下，不推送）

`main.go` 用 `go:embed` 把 `web/dist` 打进二进制。没有前端构建产物时，`go build` 会报 `pattern web/dist: no matching files found`。所以 `web/dist` 不存在时，先放一个占位页再编译，编译完删掉；已经存在的 `web/dist` 不动。下面这段要放在同一次调用里跑完：

```bash
DIST_PLACEHOLDER=0
if [ ! -e web/dist ]; then mkdir -p web/dist && echo '<!doctype html>' > web/dist/index.html && DIST_PLACEHOLDER=1; fi
go build ./...; BUILD_RC=$?
if [ "$DIST_PLACEHOLDER" = 1 ]; then rm -rf web/dist; fi
test "$BUILD_RC" = 0 && echo "GO-OK"
(cd relaykit && GOWORK=off go build ./...)
```

没输出 `GO-OK` 就是编译失败。

如果这次合并改了 `web/package.json` 或 `web/bun.lock`，先 `(cd web && bun install)`。然后：

```bash
(cd web && bun run typecheck)   # 必须用这个；tsc -p tsconfig.json 会空跑假通过
go test ./router/ ./middleware/ -count=1
```

不管有没有冲突，`router` 和 `middleware` 两个包的测试每次都要跑。上游有测试 `TestAccessTokenRouteRulesCoverEveryDashboardRoute`，它要求每个面板接口都在 `middleware/access_token_routes.go` 里登记了访问令牌的权限范围。上游新增这类检查时，fork 自己加的接口往往没登记，合并本身不会冲突，只有跑测试才看得出来。失败信息里点名的如果是 fork 自己的接口，通常的修法是：照同一组接口用的权限范围补登记，`RootAuth` 下的接口也用普通的权限范围规则。

第 6 步改过的 Go 文件，对它们所在的包跑 `go test ./<包路径>/...`。第 6 步改过的前端文件，对相关测试跑 `(cd web && NODE_OPTIONS=--no-experimental-webstorage bunx vitest run <路径>)`（本机 Node 26 不加这个参数，zustand persist 相关测试会失败）。

### 8. 快进 main 并推送

```bash
git switch main
git merge --ff-only chore/sync-upstream
TOKEN=$(gh auth token --user 0xBB2B)
git -c credential.helper= -c "credential.helper=!f(){ echo username=0xBB2B; echo password=$TOKEN; }; f" push origin main
git fetch origin
test "$(git rev-parse origin/main)" = "$(git rev-parse main)" && echo "推送成功"
git branch -d chore/sync-upstream
```

推送被拒（通常是别人刚往 origin/main 推了东西）：停下，不要 force。告诉用户 main 已在本地合好、还没推上去。

## 中途停下时怎么恢复

- 放弃这次同步：`git merge --abort`（有进行中的 merge 时），然后 `git switch main && git branch -D chore/sync-upstream`。
- 用户手动解决完冲突：从第 6 步的「全部解决后」继续。

## 完成后向用户汇报

- 合并范围：上游从哪个提交到哪个提交，共多少个新提交（`git rev-list --count "$OLD_MAIN..$TARGET"`）。
- 冲突文件，以及每个文件是怎么解决的。
- 验证命令和结果。
- 推送结果：origin/main 现在指向哪个提交。当前停在 main 分支。
