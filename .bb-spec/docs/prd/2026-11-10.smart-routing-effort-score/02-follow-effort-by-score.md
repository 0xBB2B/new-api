---
name: 02-follow-effort-by-score
description: 「跟随打分」按思考需求在档位范围内模型支持的档位里按比例选力度
date: 2026-11-10
---

# 跟随打分按比例选力度

## 用户故事
### P0：作为管理员，我想让「跟随打分」按思考需求在我框的范围里选力度，以便范围里每一档都能用上

**用例（主路径）**
- 场景：auto-gpt 困难档是 gpt-5.6-sol（支持 none、low、medium、high、xhigh、max），思考力度设成「跟随打分 medium～xhigh」。范围里模型支持的是 medium、high、xhigh 三档。
- 操作：员工发了五条都被分到困难档的消息，思考需求依次是 0、0.30、0.50、0.72、1。
- 预期：上游依次收到 medium、medium、high、xhigh、xhigh。

**用例（边界 / 异常）**
- 场景：简单档是 glm-5.3（只支持 low、high、max），设成「跟随打分 low～max」。
- 操作：思考需求依次是 0.2、0.5、0.9。
- 预期：上游依次收到 low、high、max。分档只在模型支持的三档里进行，不会出现 medium、xhigh。
- 场景：gpt-5.6-sol 那一档，打分超时，或者思考需求无效。
- 预期：上游收到 xhigh（范围里模型支持的最高一档）。
- 场景：管理员事后把 gpt-5.6-sol 的能力覆盖成只支持 low 和 xhigh，档位还存着「跟随打分 medium～high」，范围里一档模型支持的都没有。
- 操作：思考需求依次是 0.3、0.8。
- 预期：先在范围的 medium、high 两档里按比例选出 medium、high，再换到模型支持的最近一档：上游依次收到 low（日志写「由 medium 降档」）、xhigh（日志写「由 high 升档」）。

## 约束
- C1：「跟随打分」、用户新消息、打分成功且思考需求有效时，候选档位是七档（none、minimal、low、medium、high、xhigh、max）里在最低～最高之间、且目标模型支持的档位，从低到高共 n 档。把 0～1 平均分成 n 段，思考需求落在第几段就取第几档：分数 s 取从低往高数第 ⌊s × n⌋ 档（从 0 开始数），s = 1 时取最高一档。
- C2：候选档位为 0 档时，在最低～最高之间的七档（不看模型能力）里按 C1 同样的分段选出一档，再按现有的换档规则换成模型支持的最近一档（上下一样近取更高），日志照常写「由 X 降档」或「由 X 升档」。
- C3：打分失败（超时、上游报错、格式不对）或思考需求无效时，取候选档位里最高的一档；候选档位为 0 档时，取范围的最高值，再按换档规则换档。
- C4：打分把握不足而改用默认档时，只要思考需求有效，默认档如果是「跟随打分」，照样按 C1 计算。
- C5：「要想多深」不影响力度：思考需求相同时，要想多深是低还是高，发出的力度都相同。
- C6：工具结果续跑、Responses 压缩请求，以及模型不支持思考、「不覆盖」「固定」、顾问接手这几种情况，力度的算法都和现在一样：续跑沿用本轮用户新消息发出的力度；会话没有记录时，「跟随打分」按 C3 处理。
- C7：已保存的「跟随打分 X～Y」配置不用重新保存，升级后直接按 C1～C3 生效。
- C8：Responses、Chat Completions、Anthropic 三种请求格式都按 C1～C6 处理，结果相同。
- C9：档位「思考力度」旁 ⓘ 的说明，只把「跟随打分」那一段换成新文字，「不覆盖」和「力度下拉」那两句保持现在的写法。中文全文：「不覆盖：保留客户端传来的值，模型不支持这一档时自动换成最接近的档位。跟随打分：打分会给出「思考需求」（0～1，目标越模糊、问题越难越大），按比例在最低和最高之间、这个模型支持的档位里选；例如范围里有 3 档，0～0.33 取最低一档，0.67～1 取最高一档。打分失败时用其中最高的一档。力度下拉只列这个模型支持的档位」。英文全文："Not overridden keeps the value sent by the client and switches to the closest level when the model does not support it. Follow scoring uses the thinking need from scoring (0 to 1, higher when the goal is vague or the problem is hard) and picks proportionally among the levels this model supports between the minimum and maximum; with 3 levels, 0 to 0.33 picks the lowest and 0.67 to 1 the highest. The highest of them is used when scoring fails. The dropdowns only list the levels this model supports"。模型不支持思考时，这一行显示的 ⓘ 说明是同一段文字。

## 验收
- [ ] C1：gpt-5.6-sol「跟随打分 medium～xhigh」，思考需求 0 / 0.30 / 0.50 / 0.72 / 1 → 上游依次收到 medium / medium / high / xhigh / xhigh；glm-5.3「跟随打分 low～max」，思考需求 0.2 / 0.5 / 0.9 → 上游依次收到 low / high / max。
- [ ] C2：gpt-5.6-sol 被覆盖成只支持 low、xhigh，「跟随打分 medium～high」，思考需求 0.3 / 0.8 → 上游依次收到 low、xhigh，日志依次写「由 medium 降档」「由 high 升档」。
- [ ] C3：「跟随打分 medium～xhigh」，打分超时 → 上游收到 xhigh；思考需求为 1.5 → 上游收到 xhigh；能力被覆盖成只支持 low、medium、high 时打分超时 → 上游收到 high。
- [ ] C4：打分把握低于门槛、默认档是「跟随打分 medium～xhigh」、思考需求 0.30 → 上游收到 medium。
- [ ] C5：思考需求都是 0.30，要想多深分别是低和高，两次都进同一档「跟随打分 medium～xhigh」→ 上游都收到 medium。
- [ ] C6：一轮用户新消息发了 high，同一会话随后的工具结果续跑 → 上游收到 high；新会话第一条就是工具结果续跑、「跟随打分 medium～xhigh」→ 上游收到 xhigh；「固定 medium」「不覆盖」、顾问接手、模型不支持思考的现有用例结果不变。
- [ ] C7：用升级前保存的「跟随打分 medium～xhigh」配置启动，不重新保存，思考需求 0.72 → 上游收到 xhigh。
- [ ] C8：C1 的用例分别用 Responses、Chat Completions、Anthropic 格式各跑一次 → 上游收到的力度相同。
- [ ] C9：编辑虚拟模型 → 档位 → 鼠标移到「思考力度」旁的 ⓘ → 中文界面显示 C9 的中文全文，英文界面显示 C9 的英文全文；档位模型换成 gpt-4.1 后，ⓘ 里是同一段文字。

## 验证路径
1. 配置 auto-gpt 困难档 gpt-5.6-sol「跟随打分 medium～xhigh」，用桩打分模型依次返回思考需求 0、0.30、0.50、0.72、1 → 抓上游请求，力度依次是 medium、medium、high、xhigh、xhigh。
2. 让桩打分模型返回思考需求 1.5 → 上游力度 xhigh，请求不标「打分失败」。
3. 把 gpt-5.6-sol 覆盖成只支持 low、xhigh，档位改成「跟随打分 medium～high」，思考需求 0.3 → 上游力度 low，日志写「由 medium 降档」。
4. 鼠标移到「思考力度」旁的 ⓘ → 看到新的说明文字。
