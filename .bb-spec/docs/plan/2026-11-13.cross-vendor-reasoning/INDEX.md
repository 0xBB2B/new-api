# cross-vendor-reasoning 实施计划
## 阶段 1：标记编解码
- [01-reasoning-tag-codec](01-reasoning-tag-codec.md) — 渠道标记的加密编码与解码
## 阶段 2：响应打标记与请求处理
- [02-responses-output-tag](02-responses-output-tag.md) — Responses 响应里的思考记录编号改写成标记 [依赖: 01-reasoning-tag-codec]
- [03-claude-output-tag](03-claude-output-tag.md) — Claude 响应里的思考块签名改写成标记 [依赖: 01-reasoning-tag-codec, 02-responses-output-tag]
- [04-request-filter-by-tag](04-request-filter-by-tag.md) — 发往上游前按标记还原或清理思考记录 [依赖: 01-reasoning-tag-codec]
## 阶段 3：聊天格式转换
- [05-chat-convert-reasoning](05-chat-convert-reasoning.md) — 转聊天格式时思考记录进 reasoning_content
