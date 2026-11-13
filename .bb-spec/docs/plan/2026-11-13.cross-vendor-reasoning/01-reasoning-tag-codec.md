---
name: 01-reasoning-tag-codec
description: 思考记录渠道标记的编码与解码：用中台固定密钥加密渠道编号和上游原值，确定性、防篡改
---
# 渠道标记编解码

## 目标
提供一对编码 / 解码函数：把"渠道编号 + 上游原值"加密成一个字符串标记，并能从标记读回两者；读不出时明确返回失败。

## 业务规则（来源：PRD 01-channel-tag）
- 标记用中台已有的固定密钥加密：`common.CryptoSecret`（部署配 `CRYPTO_SECRET` 时取它，没配时等于 `SESSION_SECRET`，见 `common/init.go:59-65`）。从这把密钥派生专用子密钥，不直接拿它当 AES 密钥。
- 标记字符串里不出现明文渠道编号；用另一把密钥读同一个标记 → 读不出；标记改动任一字符 → 读不出。
- 同一条思考记录在流式的多个事件里要得到同一个标记，所以编码必须是**确定性**的：同一个（渠道编号, 原值）每次编码结果相同。同一渠道的两条不同思考记录，标记不同。
- 密钥不变时，重启后或另一个节点照样能读出之前的标记；密钥变了，之前的标记全部读不出。
- 还原出的原值和上游给的原值逐字节相同，原值里可能有 `+`、`/`、`=`、`-`、`_` 等字符。
- 标记统一以固定前缀 `rtag_` 开头，后接 base64url（无填充）编码的"随机数 ‖ 密文"。只要前缀不对、base64 解不开、密文认证失败、渠道编号不是正整数，一律算读不出。

## 涉及文件
- service/reasoning_tag.go（新建）
- service/reasoning_tag_test.go（新建）

## 函数清单
### service/reasoning_tag.go
| 函数名 | 职责 |
|---|---|
| `EncodeReasoningTag` | 把渠道编号和上游原值编码成 `rtag_` 开头的标记；随机数取"子密钥对（渠道编号, 原值）做 HMAC-SHA256"的前 12 字节，用 AES-256-GCM 加密 |
| `DecodeReasoningTag` | 解码标记，返回渠道编号、上游原值、是否成功；任何一步失败都返回失败，不报错、不 panic |
| `reasoningTagKey` | 用 HMAC-SHA256(`common.CryptoSecret`, 固定用途串) 派生 32 字节子密钥；每次调用时读取当前 `common.CryptoSecret`，测试替换密钥后立即生效 |

## 协作关系
- 只依赖标准库 `crypto/aes`、`crypto/cipher`、`crypto/hmac`、`crypto/sha256`、`encoding/base64`、`encoding/binary` 和 `common.CryptoSecret`。
- 被 02、03（响应方向打标记）和 04（请求方向读标记）调用。

## 验证方式
- 测试入口：`service.EncodeReasoningTag(channelID int, original string) string`、`service.DecodeReasoningTag(tag string) (channelID int, original string, ok bool)`；测试里可直接给 `common.CryptoSecret` 赋值来模拟换密钥，测试结束恢复原值。
- 测试输入 / 预期结果：
- [ ] 编码（5, `msg_f836e1c0-0b4b-47c7-b1f0-3f05f59b2880`）→ 结果以 `rtag_` 开头，不等于原值；解码得到 5 和原值，ok 为 true。
- [ ] 同一（渠道, 原值）编码两次 → 结果相同；同一渠道两个不同原值 → 结果不同；同一原值两个不同渠道 → 结果不同。
- [ ] 原值含 `+/=-_` 和多字节字符（如 `sig+/==-_中`）→ 解码出的原值逐字节相同。
- [ ] 换一把 `common.CryptoSecret` 后解码之前的标记 → ok 为 false；换回原密钥后解码 → 成功（模拟重启后同一密钥仍可读）。
- [ ] 标记任一位置改动一个字符、去掉 `rtag_` 前缀、空串、`rtag_` 加非 base64 内容、普通的 `rs_abc` → ok 为 false，不 panic。
