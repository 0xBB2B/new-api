# DESIGN.md

> 本项目的设计规范。出原型、写界面代码、审查界面代码都以它为准；新建和修改都要经用户同意后才写入。
>
> 写法要求：
> - 每项写具体数值和单位（px、ms 等），不写"简洁""大方""适中"这类形容词代替数值。
> - 颜色按用途命名，每个颜色写色值。
> - 间距和圆角的每个值都是 4 的倍数（0 也算）；表示全圆角（胶囊形按钮、圆形头像）时写 9999px，不算违规。
> - 颜色对比度符合 WCAG 2.2 AA：正文文字对背景至少 4.5:1；大号文字（24px 及以上，或 18.66px 及以上加粗）、图标、输入框和按钮的边框对相邻背景至少 3:1。
> - 项目用不上的一节写"不适用：<原因>"。

本文如实记录 `web/` 前端现在的样子。标了「（不达标）」的值是现状，已全部记进「无障碍」一节的例外表。

先说几个词：
- **主题预设**：用户在「主题设置」抽屉里能切换的一整套配色，写在 `<body data-theme-preset="…">` 上。本文所有数值以默认预设（不写这个属性）为准。
- **圆角档位、密度档位**：同一个抽屉里还能单独选圆角（`data-theme-radius`）和字号间距的密度（`data-theme-scale`）。默认都不写属性，本文按默认值记录。
- **oklch**：源码里颜色的写法，括号里的 hex 是换算到普通屏幕（sRGB）后的值。少数颜色超出普通屏幕能显示的范围，换算时按通道截断，广色域屏上看起来会更鲜艳一点。
- **对比度**：两种颜色的亮度差，按 WCAG 2.2 的相对亮度公式用脚本实算。半透明颜色先叠到它下面的背景上再算。

## 1. 风格定位

给 AI 接口网关的管理员和普通用户用的后台控制台：左侧栏导航、上方细顶栏、中间一块白色圆角内容面板，整体是灰白中性色加一个天蓝色主色，强调信息密度和安静的界面。

能检查的特征：
- 控件统一矮、圆：按钮和输入框高 32px、圆角 16px（高度的一半，看起来是胶囊形）。
- 信息密度中等偏高：表格表头高 40px、数据行高 60px、单元格内边距 8px，正文字号 14px。
- 层次主要靠 1px 细线和浅灰底区分，不靠阴影：卡片和弹窗没有默认阴影，只有 1px 的 10% 前景色描边。

主题预设一共 10 套：Default（默认，本文记录的这套）、Anthropic、Simple Large-font、Underground、Rose Garden、Lake View、Sunset Glow、Forest Whisper、Ocean Breeze、Lavender Dream。除 Default、Anthropic、Simple Large-font 外，其余 7 套会把卡片、弹层、边框、侧栏等背景按主色淡淡染色。各预设自带的默认圆角：Underground 8px、Rose Garden 16px、Lake View 12px、Sunset Glow 16px、Forest Whisper 8px、Ocean Breeze 4.8px（不达标）、Lavender Dream 16px、Simple Large-font 8px、Anthropic 10px（不达标）。Anthropic 预设默认用衬线字体（Lora）。Simple Large-font 预设把 1 个间距单位放大到 4.8px，并放大全部字号。

## 2. 颜色

项目支持深色模式（`<html class="dark">`），默认跟随系统。下表是默认预设的两套值。

| 用途 | 名字 | 浅色 | 深色 | 对比度（对哪个背景） |
|---|---|---|---|---|
| 主色（主按钮、链接） | primary | `oklch(0.692 0.141 243.716)`（#3ea4ec） | `oklch(0.54 0.142 248.516)`（#0e72bc） | 作链接文字对 background：浅 2.72:1（不达标），深 3.30:1（不达标）；作按钮色块对 background：浅 2.72:1（不达标），深 3.30:1 |
| 主色上的文字 | on-primary（primary-foreground） | `oklch(1 0 0)`（#ffffff） | `oklch(1 0 0)`（#ffffff） | 对 primary：浅 2.72:1（不达标），深 5.05:1 |
| 页面背景、内容面板背景 | background | `oklch(1 0 0)`（#ffffff） | `oklch(0.235 0 0)`（#1e1e1e） | — |
| 侧栏和页面外框背景 | sidebar | `color-mix(in oklch, foreground 0.5%, background)`（#fefefe） | `oklch(0.225 0 0)`（#1c1c1c） | 内容面板 background 对它：浅 1.01:1，深 1.03:1（只靠阴影区分） |
| 卡片背景 | card | `oklch(1 0 0)`（#ffffff） | `oklch(0.285 0 0)`（#2a2a2a） | — |
| 弹窗、下拉、提示条背景 | surface（popover） | `oklch(1 0 0)`（#ffffff） | `oklch(0.305 0 0)`（#2f2f2f） | — |
| 正文文字 | text（foreground） | `oklch(0.145 0 0)`（#0a0a0a） | `oklch(0.965 0 0)`（#f3f3f3） | 对 background：浅 19.79:1，深 15.06:1；对 card：深 12.97:1；对 popover：深 12.10:1 |
| 次要文字、占位文字 | text-muted（muted-foreground） | `oklch(0.49 0 0)`（#606060） | `oklch(0.78 0 0)`（#b7b7b7） | 对 background：浅 6.26:1，深 8.33:1；对 muted：浅 5.74:1，深 6.69:1；对弹窗底栏：浅 6.00:1，深 6.69:1 |
| 浅灰底（悬停底、标签页底、弹窗底栏底） | muted | `oklch(0.97 0 0)`（#f5f5f5） | `oklch(0.305 0 0)`（#2f2f2f） | — |
| 次按钮底 | secondary | `oklch(0.95 0 0)`（#eeeeee） | `oklch(0.335 0 0)`（#373737） | — |
| 次按钮文字 | on-secondary（secondary-foreground） | `oklch(0.42 0.16 250)`（#004c9f） | `oklch(0.9 0.05 250)`（#c6e1ff） | 对 secondary：浅 7.16:1，深 8.93:1 |
| 强调底（搜索框悬停、品牌名悬停） | accent | `color-mix(in oklch, primary 12%, background)`（#eaf4fe） | `color-mix(in oklch, primary 20%, background)`（#222e3b） | accent-foreground 对它：浅 17.82:1，深 13.21:1 |
| 分隔线 | border | `oklch(0.93 0 0)`（#e8e8e8） | `oklch(1 0 0 / 10%)`（叠在 background 上 #343434） | 对 background：浅 1.23:1，深 1.35:1（分隔线不要求 3:1） |
| 输入框、按钮边框 | border-strong（input） | `oklch(0.93 0 0)`（#e8e8e8） | `oklch(1 0 0 / 17%)`（叠在输入框自身底色上 #4e4e4e） | 对 background：浅 1.23:1（不达标），深 2.00:1（不达标）；浅色描边按钮用的是 border 色，同为 1.23:1（不达标） |
| 成功 | success | `oklch(0.596 0.145 163.225)`（#009966） | `oklch(0.696 0.17 162.48)`（#00bc7d） | 作文字对 background：浅 3.67:1（不达标），深 6.77:1；白字对它：浅 3.51:1；深字对它：深 8.03:1 |
| 警告 | warning | `oklch(0.681 0.162 75.834)`（#d08700） | `oklch(0.769 0.188 70.08)`（#fe9a00） | 作文字对 background：浅 2.93:1（不达标），深 7.77:1；警告徽标文字对徽标底：浅 2.66:1（不达标），深 6.53:1 |
| 错误 | danger（destructive） | `oklch(0.577 0.245 27.325)`（#e7000b） | `oklch(0.704 0.191 22.216)`（#ff6467） | 作文字对 background：浅 4.76:1，深 5.76:1；对 card：深 4.96:1；危险按钮文字对危险按钮底：浅 3.99:1（不达标），深 4.28:1（不达标） |
| 信息 | info | `oklch(0.588 0.158 241.966)`（#0084d1） | `oklch(0.613 0.14 239.919)`（#018dcf） | 作文字对 background：浅 4.02:1（不达标），深 4.53:1 |
| 聚焦描边 | focus（ring） | `oklch(0.708 0.16 249.003)`（#42a6ff），按钮和输入框用 50% 透明的 3px 光圈（叠在白底上 #a0d3ff） | `oklch(0.554 0.148 250.726)`（#1a75c5），光圈同样 50% 透明（叠在深底上 #1c4a72） | 光圈对 background：浅 1.59:1（不达标），深 1.79:1（不达标）；实色对 background：浅 2.58:1（不达标），深 3.49:1 |
| 侧栏选中项底 | sidebar-accent | `color-mix(in oklch, primary 12%, background)`（#eaf4fe） | `color-mix(in oklch, primary 20%, background)`（#222e3b） | — |
| 侧栏选中项文字 | sidebar-accent-foreground | `oklch(0.579 0.196 255.909)`（#0276ea） | `oklch(0.9 0.05 250)`（#c6e1ff） | 对 sidebar-accent：浅 3.95:1（不达标），深 10.28:1 |
| 侧栏聚焦描边 | sidebar-ring | `oklch(0.58 0.2 250)`（#007bea） | `oklch(0.7 0.17 250)`（#39a3ff） | 对 sidebar：浅 4.15:1，深 6.38:1 |
| 侧栏分组标题 | （muted-foreground 70% 不透明） | 叠在 sidebar 上 #8f8f8f | 叠在 sidebar 上 #898989 | 对 sidebar：浅 3.18:1（不达标，字号 11px），深 4.86:1 |
| 表头底 | table-header | `color-mix(in oklch, foreground 1.5%, background)`（#fbfbfb） | `color-mix(in oklch, foreground 6%, background)`（#282828） | 次要文字对它：浅 6.04:1 |
| 停用行底 / 悬停 / 左边条 | table-disabled / -hover / -border | #f6f6f6 / #f2f2f2 / #d2d2d2（前景色 3% / 4.5% / 16% 混入 background） | #2a2a2a / #2f2f2f / #4a4a4a（前景色 7% / 9.5% / 24% 混入 background） | — |
| 提示气泡 | tooltip（底用 foreground，字用 background） | 底 #0a0a0a，字 #ffffff | 底 #f3f3f3，字 #1e1e1e | 浅 19.79:1，深 15.06:1 |
| 弹窗遮罩 | overlay | 纯黑 10% 不透明，加 4px 背景模糊 | 同左 | — |
| 未保存提示（设置页标题旁） | （Tailwind amber） | 字 amber-700 `oklch(55.5% 0.163 48.998)`（#bb4d00），底 amber-500 10%（#fff5e5） | 字 amber-300（#ffd230），底 amber-400 10%（#342e1b） | 浅 4.67:1，深 9.38:1 |
| 图表色 1~5 | chart-1 ~ chart-5 | #35a9ff、#00a9b3、#9095e8、#d068d8、#20b46b | #66b6ff、#00c9d1、#a9a5ff、#e088e5、#5ccb89 | 只用于图形，未单独核对 |
| 骨架屏底 / 高光 | skeleton-base / -highlight | #f5f5f5 / #fafafa | #373737 / #525252 | — |

提示条（sonner toast）的彩色底和文字：

| 类型 | 底色（源码写法） | 浅色 底 / 字 | 深色 底 / 字 | 对比度（字对底） |
|---|---|---|---|---|
| 成功 | `color-mix(in oklch, success 16%, popover)`，字 = success | #ddefe5 / #009966 | #32443b / #00bc7d | 浅 3.06:1（不达标），深 4.22:1（不达标） |
| 信息 | `color-mix(in oklch, info 16%, popover)`，字 = info | #dbecf9 / #0084d1 | #313d46 / #018dcf | 浅 3.32:1（不达标），深 3.01:1（不达标） |
| 警告 | `color-mix(in oklch, warning 18%, popover)`，字 = warning | #f8ead7 / #d08700 | #514231 / #fe9a00 | 浅 2.47:1（不达标），深 4.53:1 |
| 错误 | `color-mix(in oklch, destructive 16%, popover)`，字 = destructive | #ffdfdb / #e7000b | #4f3938 / #ff6467 | 浅 3.83:1（不达标），深 3.68:1（不达标） |
| 普通 | 底 popover、字 popover-foreground、边框 border | #ffffff / #0a0a0a | #2f2f2f / #f3f3f3 | 浅 19.79:1，深 12.10:1 |

边框色：各类型 = 对应颜色 35%（警告 38%）混入 border。

## 3. 字体与字号

- 字体：浏览器默认无衬线字体（CSS 写的是 `sans-serif`），用户在主题设置里选"Sans"时换成 Public Sans，选"Serif"时换成 Lora 加一串中文宋体后备。未确认：默认 `sans-serif` 实际是哪款字体由用户系统决定（macOS 一般是 Helvetica 加苹方，Windows 一般是 Arial 加微软雅黑），源码里没有指定。
- 等宽字体：Tailwind 默认 `ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace`。
- 表格单元格和 `.tabular-nums` 用等宽数字（`font-variant-numeric: tabular-nums`）。

| 用途 | 字号 | 行高 | 字重 |
|---|---|---|---|
| 页面标题（标题区 h2） | 16px（宽度小于 640px）/ 18px（640px 及以上） | 24px / 28px | 700，字距 -0.025em |
| 区块标题（设置页 h3、折叠面板标题） | 16px | 24px | 600 |
| 卡片标题 | 16px（小号卡片 14px） | 22px（1.375 倍） | 500 |
| 弹窗标题 | 16px | 16px（1 倍） | 500 |
| 正文（按钮、表单、卡片、弹窗、提示条正文） | 14px | 20px | 400；按钮、标签文字、表格单元格 500 |
| 输入框文字 | 16px（宽度小于 768px，全局强制）/ 14px（768px 及以上） | 24px / 20px | 400 |
| 辅助文字（说明、表格次要信息、徽标、提示气泡） | 12px | 16px | 400；徽标 500 |
| 小号按钮文字 | 12.8px（`text-[0.8rem]`） | 20px | 500 |
| 侧栏分组标题 | 11px，全大写，字距 0.05em | 继承 1.5 倍（16.5px） | 500 |
| 提示条（toast）标题 / 描述 | 13px / 12px | 1.5 倍 / 继承 | 500 / 400 |
| 代码 | 12px（最常用，`font-mono text-xs`），也有 14px | 16px / 20px | 400 |

## 4. 间距

默认 1 个间距单位 = 4px（`--spacing: 0.25rem`，项目没改）。

- 尺度：基础组件和页面框架现在在用的值是 0、2、3、4、6、8、10、12、14、16、20、24、72 px。其中 2、3、6、10、14 px 不是 4 的倍数（不达标），主要出现在：按钮和输入框左右内边距 10px、按钮图标与文字间距 6px、徽标上下内边距 2px、标签页外框内边距 3px、提示气泡上下内边距 6px、页面标题区下内边距 10px、侧栏二级菜单左右外边距 14px。
- 页面外边距：内容面板距视口上、右、下各 8px、左 0（768px 及以上）；面板内页面标题区和内容区左右 12px（小于 640px）/ 16px（640px 及以上）。
- 卡片内边距：上下 16px、左右 16px、内部元素间距 16px；小号卡片全部 12px。
- 表单项之间：标签和输入框间距 8px；设置页表单项上下 24px、两列之间 20px；开关行上下内边距 10px（不达标）。
- 按钮内边距：默认 / 大号 左右 10px（不达标，带图标那一侧 8px）；小号左右 10px（不达标）；超小号左右 8px。

## 5. 圆角、边框、阴影

默认基准圆角 `--radius: 1rem` = 16px，其余档位按倍数算出来。

| 项 | 值 |
|---|---|
| 小圆角（`rounded-md`：小号按钮、标签页按钮、侧栏菜单项、提示气泡） | 12.8px（基准 × 0.8，不达标） |
| 更小圆角（`rounded-sm`：骨架条、键盘提示） | 9.6px（基准 × 0.6，不达标） |
| 中圆角（`rounded-lg`：按钮、输入框、下拉框、提示条、警示框、数据表外框、标签页外框） | 16px |
| 大圆角（`rounded-xl`：卡片、弹窗、内容面板） | 22.4px（基准 × 1.4，不达标） |
| 更大圆角档位 | `rounded-2xl` 28.8px、`rounded-3xl` 35.2px（不达标）；`rounded-4xl` 41.6px（不达标，徽标在用，徽标高 20px，看起来是胶囊形） |
| 全圆角（胶囊形按钮、圆形头像、开关、圆点） | 9999px |
| 边框宽度 | 1px；数据表停用行左侧色条 4px；侧栏外框拖拽条 2px |
| 卡片阴影 | 默认无阴影，只有 1px 描边（前景色 10%，浅 #e7e7e7 / 深 #333333）。宽度 641px 及以上悬停时：`0 4px 12px rgb(0 0 0 / 0.06)`（深色 0.3），同时上移 1px |
| 弹窗阴影 | 无阴影，只有 1px 描边（同卡片）；下拉菜单和气泡弹层用 `0 4px 6px -1px rgb(0 0 0 / 0.1), 0 2px 4px -2px rgb(0 0 0 / 0.1)` |
| 内容面板阴影 | `0 1px 3px 0 rgb(0 0 0 / 0.1), 0 1px 2px -1px rgb(0 0 0 / 0.1)` |
| 提示条阴影 | `0 4px 12px rgba(0, 0, 0, 0.1)` |

用户可选的圆角档位：none 0px、sm 4.8px（不达标）、md 8px、lg 12px、xl 16px。

## 6. 布局与断点

- 断点：Tailwind 默认 sm 640px、md 768px、lg 1024px、xl 1280px、2xl 1536px。另有：JS 判断"手机"用宽度小于 768px；数据表宽度 640px 及以下改成卡片列表；卡片悬停效果从 641px 起；提示条 600px 及以下铺满宽度；内容区"居中"布局从 1280px 起。
- 页面最大宽度：默认不限宽（主题设置里"内容布局"默认为铺满）。用户选"居中"时，宽度 1280px 及以上把内容限制在 1280px 并居中。
- 栅格：没有全局栅格。设置页表单宽度 1024px 及以上分 2 列，列间距 20px、行间距 24px，多行输入框和开关行占满两列；小于 1024px 为 1 列。

页面框架（登录后的控制台）：

| 部分 | 位置和尺寸 | 写法（HTML 结构和类名） | 不同断点下怎么变 |
|---|---|---|---|
| 顶栏 | 贴顶固定，高 48px，全宽，透明底（透出侧栏同色的外框底） | `<header class="sticky top-0 z-40 h-[var(--app-header-height,3rem)] w-full bg-transparent">` | 左右内边距 8px → 640px 起 12px；元素间距 6px → 640px 起 8px；顶部导航链接 1024px 起才直接显示，以下收进一个 28px 的菜单按钮；搜索框小于 640px 只是 32px 的图标按钮，640px 起 160px 宽，1024px 起 208px，1280px 起 256px；主题设置按钮小于 768px 隐藏 |
| 侧栏 | 顶栏下方贴左，高 = 视口高 - 48px；展开宽 208px；内含 8px 内边距 | `<div data-slot="sidebar" data-variant="inset" class="group peer hidden md:block">` 里 `<div data-slot="sidebar-container" class="fixed top-[var(--app-header-height)] … w-(--sidebar-width) p-2 md:flex">` | 768px 及以上常驻，可收起成只显示图标（图标栏 44px，加内边距共 62px，悬停菜单项右侧出提示气泡），按 ⌘/Ctrl + B 切换；小于 768px 隐藏，改成从左滑出的抽屉，宽 272px |
| 导航 | 侧栏内分组列表；系统设置页时侧栏整体换成"系统管理"分组，顶部加"返回"按钮 | `<div data-slot="sidebar-group" class="px-2 py-1">` 下 `<ul data-slot="sidebar-menu" class="flex flex-col gap-1">`，每项 `<a data-slot="sidebar-menu-button" class="h-8 rounded-md p-2 text-sm …">` | 收起时只剩 32×32px 图标按钮，带子菜单的项改成向右弹出的下拉菜单 |
| 页面标题区 | 内容面板顶部，左标题右操作按钮 | `<div class="shrink-0 px-3 pt-3 pb-2.5 sm:px-4 sm:pt-5 sm:pb-3">` 内 `<div class="flex flex-wrap items-center justify-between gap-x-3 gap-y-2 sm:gap-x-4">` | 内边距：小于 640px 上 12px、左右 12px、下 10px（不达标）；640px 起上 20px、左右 16px、下 12px。标题和按钮一行放不下时，按钮区换到下一行并靠右 |
| 内容区 | 标题区下方，占满剩余高度，自己滚动 | `<div class="min-h-0 flex-1 overflow-auto px-3 pt-1 pb-3 sm:px-4 sm:pt-1.5 sm:pb-4">` | 左右 12px → 640px 起 16px；上 4px → 6px（不达标）；下 12px → 16px |

内容面板本身（包住标题区、内容区、底栏）：768px 及以上是一块白色圆角面板，距视口上、右、下 8px，圆角 22.4px，带内容面板阴影；侧栏收起时左边也留 8px；小于 768px 没有外边距、圆角和阴影。面板高 = 视口高 - 48px - 16px。

页面底栏（可选）：内容区下方，`bg-background border-t`，内边距左右 12px、上下 10px（不达标）→ 640px 起左右 16px、上下 12px，没内容时整个隐藏。

页面框架写法：

外框和顶栏：

```html
<div data-slot="sidebar-wrapper" class="group/sidebar-wrapper has-data-[variant=inset]:bg-sidebar flex min-h-svh w-full flex-col"
     style="--sidebar-width:13rem; --sidebar-width-icon:2.75rem">
  <!-- 键盘用户按 Tab 第一个出现的"跳到主内容"链接 -->
  <a href="#content" class="bg-primary text-primary-foreground fixed start-44 z-999 -translate-y-52 px-4 py-2 text-sm font-medium shadow-sm focus:translate-y-3">Skip to Main</a>

  <header class="sticky top-0 z-40 h-[var(--app-header-height,3rem)] w-full shrink-0 bg-transparent">
    <div class="flex h-full items-center gap-1.5 px-2 sm:gap-2 sm:px-3">
      <button data-slot="sidebar-trigger" class="…ghost 按钮… size-8">侧栏开关图标</button>
      <div class="@container/system-brand flex min-w-0 flex-1 items-center gap-1">
        <a href="/" class="text-foreground inline-flex h-7 min-w-0 items-center gap-1.5 rounded-md px-1.5 text-sm font-medium hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring/40">
          <div class="flex size-5 shrink-0 overflow-hidden rounded-md"><img class="size-full rounded-md object-cover"></div>
          <span class="max-w-[12rem] truncate">站点名</span>
        </a>
        <!-- 版本号 -->
      </div>
      <div class="ms-auto flex shrink-0 items-center gap-1 sm:gap-2">
        <div class="me-1 hidden lg:block">
          <nav class="hidden items-center space-x-4 lg:flex xl:space-x-6">
            <a class="hover:text-primary text-sm font-medium text-muted-foreground">链接</a>
          </nav>
        </div>
        <button data-slot="button" class="…outline 按钮… bg-muted/25 text-muted-foreground hover:bg-accent relative h-8 w-8 sm:w-40 lg:w-52 xl:w-64 justify-start rounded-md text-sm font-normal shadow-none">搜索</button>
        <!-- 通知按钮、语言按钮（36×36px）、主题设置按钮（32×32px，md 以下隐藏）、头像按钮（24×24px） -->
      </div>
    </div>
  </header>

  <div class="flex min-h-0 w-full flex-1">
    <!-- 侧栏，见下 -->
    <main data-slot="sidebar-inset" class="bg-background relative flex w-full flex-1 flex-col @container/content h-[calc(100svh-var(--app-header-height,0px)-(var(--spacing)*4))] min-h-0 overflow-hidden md:m-2 md:ml-0 md:rounded-xl md:shadow-sm">
      <!-- 页面，见「页面标题区和内容区」 -->
    </main>
  </div>
</div>
```

侧栏和导航（768px 及以上）：

```html
<div data-slot="sidebar" data-variant="inset" data-state="expanded" data-collapsible="" data-side="left" class="group peer text-sidebar-foreground hidden md:block">
  <div data-slot="sidebar-gap" class="relative w-(--sidebar-width) bg-transparent transition-[width] duration-200 ease-linear"></div>
  <div data-slot="sidebar-container" class="fixed top-[var(--app-header-height,0px)] bottom-0 z-10 hidden h-[calc(100svh-var(--app-header-height,0px))] w-(--sidebar-width) p-2 transition-[left,right,width] duration-200 ease-linear md:flex">
    <div data-slot="sidebar-inner" class="bg-sidebar flex size-full flex-col">
      <!-- 只在系统设置等二级视图出现：返回按钮 -->
      <div data-slot="sidebar-header" class="border-sidebar-border flex flex-col gap-2 border-b px-2 py-2">
        <a data-slot="sidebar-menu-button" class="… text-muted-foreground hover:text-foreground gap-1.5 font-medium">‹ 返回</a>
      </div>
      <div data-slot="sidebar-content" class="no-scrollbar flex min-h-0 flex-1 flex-col overflow-auto py-2">
        <div data-slot="sidebar-group" class="relative flex w-full min-w-0 flex-col px-2 py-1">
          <div data-slot="sidebar-group-label" class="text-muted-foreground/70 flex h-8 items-center rounded-md px-2 text-[11px] font-medium tracking-wider uppercase">分组标题</div>
          <ul data-slot="sidebar-menu" class="flex w-full min-w-0 flex-col gap-1 text-sm">
            <li data-slot="sidebar-menu-item" class="group/menu-item relative">
              <a data-slot="sidebar-menu-button" data-active class="flex h-8 w-full items-center gap-2 rounded-md p-2 text-sm hover:bg-muted data-active:bg-sidebar-accent data-active:font-medium data-active:text-sidebar-accent-foreground focus-visible:ring-2 ring-sidebar-ring [&_svg]:size-4">
                <svg>图标 16px</svg><span class="min-w-0 flex-1 truncate">菜单名</span>
              </a>
              <!-- 有子菜单时：右侧 16px 箭头，展开后 -->
              <ul data-slot="sidebar-menu-sub" class="border-sidebar-border mx-3.5 flex flex-col gap-1 border-l px-2.5 py-0.5">
                <li><a data-slot="sidebar-menu-sub-button" class="flex h-7 items-center gap-2 rounded-md px-2 hover:bg-muted data-active:bg-sidebar-accent">子菜单</a></li>
              </ul>
            </li>
          </ul>
        </div>
      </div>
    </div>
  </div>
  <button data-slot="sidebar-rail" class="absolute inset-y-0 z-20 hidden w-4 sm:flex hover:after:bg-sidebar-border after:w-[2px]">拖拽条</button>
</div>
```

小于 768px 时侧栏改为抽屉：`<div data-slot="sidebar" data-mobile="true" class="bg-sidebar text-sidebar-foreground fixed z-60 w-(--sidebar-width) p-0" style="--sidebar-width:17rem">`，里面结构同上，遮罩为纯黑 10% 加 4px 模糊。

页面标题区和内容区（所有用 `SectionPageLayout` 的页面，共 15 处，系统设置页也是）：

```html
<main class="flex min-h-0 flex-1 flex-col overflow-hidden">
  <div class="shrink-0 px-3 pt-3 pb-2.5 sm:px-4 sm:pt-5 sm:pb-3">
    <!-- 可选面包屑：<div class="mb-2 sm:mb-3">…</div> -->
    <div class="flex flex-wrap items-center justify-between gap-x-3 gap-y-2 sm:gap-x-4">
      <div class="min-w-0 flex-1">
        <h2 class="truncate text-base font-bold tracking-tight sm:text-lg">页面标题</h2>
      </div>
      <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2 sm:gap-x-4">操作按钮</div>
    </div>
  </div>
  <div class="min-h-0 flex-1 overflow-auto px-3 pt-1 pb-3 sm:px-4 sm:pt-1.5 sm:pb-4">内容</div>
  <div class="bg-background shrink-0 border-t px-3 py-2.5 empty:hidden sm:px-4 sm:py-3"><!-- 底栏，空则隐藏 --></div>
</main>
```

系统设置页的框架：

- 侧栏：进入 `/system-settings/*` 后侧栏换成"系统管理"一个分组（站点与品牌、认证、计费与支付、模型、请求策略、安全与限制、控制台内容、运维），每项有子菜单；顶部是"返回"按钮，下方有 1px 分隔线。
- 标题区左边：标题文字后面跟一个状态插槽，表单有未保存改动时显示"未保存的更改"小胶囊（高 20px、左右内边距 8px、11px 字、全圆角、左侧 6px 圆点，颜色见「颜色」表的"未保存提示"）。
- 标题区右上角：操作按钮区。页面框架先放一个空容器，各设置小节用 `SettingsPageFormActions`（或更通用的 `SettingsPageActionsPortal`）通过 React Portal 把按钮"传送"进这个容器。所以按钮在代码里写在表单里面，但显示在标题右侧。
  - 按钮顺序：左边"重置"（可选，描边按钮，小号，前面 16px 的旋转箭头图标），右边"保存"（主按钮，小号，前面 16px 的保存图标）。两个按钮高 28px，间距 8px。保存中时两个按钮都禁用，保存按钮文字换成"保存中…"。
  - 一行放不下时，按钮区整体换行到标题下面，仍然靠右。
- 内容区：一列，元素间距 16px；每个设置小节是 `<section class="flex flex-col gap-4">`，小节标题默认隐藏（因为页面标题已经写了）；表单是 2 列栅格（见上方"栅格"）。

```html
<div class="shrink-0 px-3 pt-3 pb-2.5 sm:px-4 sm:pt-5 sm:pb-3">
  <div class="flex flex-wrap items-center justify-between gap-x-3 gap-y-2 sm:gap-x-4">
    <div class="min-w-0 flex-1">
      <h2 class="truncate text-base font-bold tracking-tight sm:text-lg">
        <span class="inline-flex max-w-full min-w-0 items-center gap-2 align-middle">
          <span class="truncate">速率限制</span>
          <span class="inline-flex min-w-0 shrink-0 items-center">
            <!-- 有未保存改动时： -->
            <span class="inline-flex h-5 items-center gap-1.5 rounded-full bg-amber-500/10 px-2 text-[11px] font-medium whitespace-nowrap text-amber-700 ring-1 ring-amber-500/20 ring-inset dark:bg-amber-400/10 dark:text-amber-300 dark:ring-amber-400/20">
              <span class="size-1.5 rounded-full bg-amber-500 dark:bg-amber-300"></span>未保存的更改
            </span>
          </span>
        </span>
      </h2>
    </div>
    <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2 sm:gap-x-4">
      <div class="flex flex-wrap items-center justify-end gap-2"><!-- 框架放的空容器 -->
        <div class="flex flex-wrap items-center justify-end gap-2"><!-- SettingsPageActionsPortal 包的一层 -->
          <button data-slot="button" class="…outline，size=sm…">↺ 重置</button>
          <button data-slot="button" class="…default，size=sm…">保存图标 保存</button>
        </div>
      </div>
    </div>
  </div>
</div>
<div class="min-h-0 flex-1 overflow-auto px-3 pt-1 pb-3 sm:px-4 sm:pt-1.5 sm:pb-4">
  <div class="flex h-full min-h-0 w-full flex-col gap-4">
    <section class="flex flex-col gap-4">
      <form class="grid min-w-0 gap-x-5 gap-y-6 lg:grid-cols-2">
        <div data-slot="form-item" class="grid gap-2">标签 + 输入框 + 说明</div>
        <div data-slot="form-item" class="flex min-w-0 flex-row items-center justify-between gap-4 py-2.5 lg:col-span-2">开关行</div>
        <div class="bg-muted/20 min-w-0 space-y-3 rounded-xl border px-3 py-2.5">一组相关控件</div>
      </form>
    </section>
  </div>
</div>
```

## 7. 组件

### 按钮

- 写法：

```html
<!-- 主按钮 -->
<button data-slot="button" class="group/button inline-flex shrink-0 items-center justify-center rounded-lg border border-transparent bg-clip-padding text-sm font-medium whitespace-nowrap transition-all outline-none select-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 active:not-aria-[haspopup]:translate-y-px disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 [&_svg:not([class*='size-'])]:size-4 bg-primary text-primary-foreground [a]:hover:bg-primary/80 h-8 gap-1.5 px-2.5 has-data-[icon=inline-start]:pl-2 has-data-[icon=inline-end]:pr-2">
  <svg data-icon="inline-start"></svg><span>保存</span>
</button>

<!-- 次按钮（描边） -->
<button data-slot="button" class="…同上公共部分… border-border bg-background hover:bg-muted hover:text-foreground aria-expanded:bg-muted dark:border-input dark:bg-input/30 dark:hover:bg-input/50 h-8 gap-1.5 px-2.5">取消</button>
```

- 其他变体：secondary（底 secondary、字 secondary-foreground，悬停底 80% 不透明）；ghost（透明，悬停底 muted，深色 muted 50%）；destructive（底 = 错误色 10%，深色 20%，字 = 错误色，悬停 20% / 30%）；link（字 = 主色，悬停加下划线，下划线偏移 4px）。
- 高度：默认 32px；lg 36px；sm 28px；xs 24px；纯图标按钮 32×32 / lg 36×36 / sm 28×28 / xs 24×24px。
- 内边距：默认、lg、sm 左右 10px（不达标）；xs 左右 8px；带图标那一侧默认 8px、sm/xs 6px（不达标）。图标和文字间距：默认、lg 6px（不达标），sm、xs 4px。图标默认 16px，sm 14px，xs 12px。
- 圆角：默认、lg 16px；sm、xs 12.8px（不达标；放在按钮组里时变 16px）。字号：默认 14px，sm 12.8px，xs 12px。

| 状态 | 主按钮 | 次按钮 |
|---|---|---|
| 默认 | 底 primary（浅 #3ea4ec / 深 #0e72bc），字 #ffffff，1px 透明边框 | 浅：底 #ffffff、1px 边框 border #e8e8e8、字 #0a0a0a；深：底 input 30%（#292929）、1px 边框 input（#4e4e4e）、字 #f3f3f3 |
| 悬停 | 渲染成 `<button>` 时没有变化；只有渲染成 `<a>` 时底变 primary 80%（浅 #65b6f0 / 深 #11619d） | 浅：底 muted #f5f5f5；深：底 input 50% |
| 按下 | 下移 1px（展开菜单的按钮除外）；没开"减少动态效果"时再缩到 98% | 同主按钮 |
| 禁用 | 整体 50% 不透明，不响应鼠标 | 同主按钮 |
| 聚焦（键盘） | 边框变 ring（浅 #42a6ff / 深 #1a75c5），外加 3px ring 50% 光圈 | 同主按钮 |

另有校验失败状态（`aria-invalid`）：边框变错误色，外加 3px 错误色 20% 光圈（深色 40%）。

### 输入框

- 写法：

```html
<input data-slot="input" class="border-input placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-ring/50 disabled:bg-input/50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:bg-input/30 dark:disabled:bg-input/80 h-8 w-full min-w-0 rounded-lg border bg-transparent px-2.5 py-1 text-base transition-colors outline-none focus-visible:ring-3 focus-visible:ring-inset disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:ring-3 aria-invalid:ring-inset md:text-sm">
```

- 尺寸：高 32px，左右内边距 10px（不达标），上下 4px，圆角 16px，1px 边框。多行输入框（textarea）：最小高 64px、最大高 384px、上下内边距 8px、左右 10px（不达标），随内容长高。下拉选择框（select）：高 32px（sm 28px），左 10px、右 8px。
- 表单标签：14px、500、行高 14px，必填时文字后面跟红色 `*`；说明文字 14px 次要文字色；错误文字 14px 错误色，显示在输入框下面。

| 状态 | 样子 |
|---|---|
| 默认 | 浅：透明底（显示白色背景）、1px 边框 #e8e8e8、字 #0a0a0a、占位字 #606060；深：底 input 30%（#292929）、边框 #4e4e4e、占位字 #b7b7b7 |
| 悬停 | 没有变化（下拉选择框在深色下底变 input 50%） |
| 按下 | 和聚焦一样（按下即获得焦点） |
| 禁用 | 底变 input 50%（浅 #f3f3f3）/ 深色 input 80%（#3d3d3d），整体 50% 不透明，鼠标显示禁止符号 |
| 聚焦 | 边框变 ring，内侧 3px ring 50% 光圈（光圈画在框内） |

### 表格

- 写法（数据表 `DataTable` 套在基础表格外面）：

```html
<div class="overflow-hidden rounded-lg border">                          <!-- 数据表外框，圆角 16px -->
  <div class="min-h-0 flex-1 overflow-auto">
    <table data-slot="table" class="w-full caption-bottom text-sm tabular-nums [&_td]:text-sm [&_td]:font-medium [&_th]:text-sm [&_[data-table-text=secondary]]:text-xs [&_[data-table-text=secondary]]:font-normal">
      <thead data-slot="table-header" class="sticky top-0 z-10 bg-(--table-header) [&_tr]:border-b">
        <tr data-slot="table-row" class="border-b transition-colors …">
          <th data-slot="table-head" class="text-foreground h-10 px-2 text-left align-middle font-medium whitespace-nowrap">列名</th>
        </tr>
      </thead>
      <tbody data-slot="table-body" class="[&>tr]:h-15 [&_tr:last-child]:border-0">
        <tr data-slot="table-row" class="group border-b transition-colors hover:[background-color:color-mix(in_oklch,var(--muted)_50%,var(--background))] data-[state=selected]:bg-muted">
          <td data-slot="table-cell" class="p-2 align-middle whitespace-nowrap">值</td>
        </tr>
      </tbody>
    </table>
  </div>
</div>
```

- 尺寸：表头行高 40px，表头单元格左右内边距 8px；数据行高 60px，单元格四周内边距 8px；行之间 1px 分隔线，最后一行没有；单元格文字 14px、500、等宽数字，次要文字 12px、400。
- 宽度 640px 及以下，数据表改成卡片列表：外框圆角 16px、1px 边框、卡片之间 1px 分隔线。

| 状态 | 样子（行） |
|---|---|
| 默认 | 底 = 页面背景（浅 #ffffff / 深 #1e1e1e），下边 1px 分隔线；表头底 table-header（浅 #fbfbfb / 深 #282828） |
| 悬停 | 底变 muted 和 background 各半（浅 #fafafa / 深 #262626），过渡 120ms |
| 按下 | 没有专门样式 |
| 禁用（停用的数据行） | 底 table-disabled（浅 #f6f6f6 / 深 #2a2a2a），悬停 #f2f2f2 / #2f2f2f，首列左侧 4px 色条（浅 #d2d2d2 / 深 #4a4a4a） |
| 聚焦 | 行本身不能聚焦；行内按钮、复选框按各自的聚焦样式 |

另有选中状态：底变 muted（浅 #f5f5f5 / 深 #2f2f2f）。没开"减少动态效果"时，表格行出现时从下方 4px 淡入，用时 200ms，每行依次晚 25ms，第 11 行起统一晚 250ms。

### 卡片

- 写法：

```html
<div data-slot="card" data-size="default" class="group/card bg-card text-card-foreground ring-foreground/10 flex flex-col gap-4 overflow-hidden rounded-xl py-4 text-sm ring-1 has-data-[slot=card-footer]:pb-0 data-[size=sm]:gap-3 data-[size=sm]:py-3">
  <div data-slot="card-header" class="group/card-header @container/card-header grid auto-rows-min items-start gap-1 rounded-t-xl px-4 group-data-[size=sm]/card:px-3 has-data-[slot=card-action]:grid-cols-[1fr_auto]">
    <div data-slot="card-title" class="text-base leading-snug font-medium group-data-[size=sm]/card:text-sm">标题</div>
    <div data-slot="card-description" class="text-muted-foreground text-sm">说明</div>
    <div data-slot="card-action" class="col-start-2 row-span-2 row-start-1 self-start justify-self-end">右上角操作</div>
  </div>
  <div data-slot="card-content" class="px-4 group-data-[size=sm]/card:px-3">内容</div>
  <div data-slot="card-footer" class="bg-muted/50 flex items-center rounded-b-xl border-t p-4 group-data-[size=sm]/card:p-3">底部</div>
</div>
```

- 尺寸：圆角 22.4px（不达标），上下内边距 16px（小号 12px），左右 16px（小号 12px），区块间距 16px（小号 12px），标题和说明间距 4px。

| 状态 | 样子 |
|---|---|
| 默认 | 底 card（浅 #ffffff / 深 #2a2a2a），1px 描边（浅 #e7e7e7 / 深 #333333），无阴影 |
| 悬停 | 宽度 641px 及以上、没开"减少动态效果"时：上移 1px，阴影 `0 4px 12px rgb(0 0 0 / 0.06)`（深色 0.3），过渡 150ms；加 `data-card-hover="false"` 的卡片没有这个效果 |
| 按下 | 没有专门样式 |
| 禁用 | 没有专门样式 |
| 聚焦 | 没有专门样式（卡片本身不能聚焦） |

### 弹窗

- 写法：

```html
<!-- 普通弹窗 Dialog -->
<div data-slot="dialog-overlay" class="data-open:animate-in data-open:fade-in-0 data-closed:animate-out data-closed:fade-out-0 fixed inset-0 isolate z-50 bg-black/10 duration-100 supports-backdrop-filter:backdrop-blur-xs"></div>
<div data-slot="dialog-content" class="bg-popover text-popover-foreground ring-foreground/10 data-open:animate-in data-open:fade-in-0 data-open:zoom-in-95 data-closed:animate-out data-closed:fade-out-0 data-closed:zoom-out-95 fixed top-[var(--dialog-viewport-center,50dvh)] left-1/2 z-50 grid max-h-[var(--dialog-available-height,calc(100dvh-2rem))] w-full max-w-[calc(100%-2rem)] -translate-x-1/2 -translate-y-1/2 gap-4 overflow-y-auto overscroll-contain rounded-xl p-4 text-sm ring-1 duration-100 outline-none sm:max-w-sm">
  <div data-slot="dialog-header" class="flex flex-col gap-2">
    <h2 data-slot="dialog-title" class="text-base leading-none font-medium">标题</h2>
    <p data-slot="dialog-description" class="text-muted-foreground text-sm *:[a]:underline *:[a]:underline-offset-3 *:[a]:hover:text-foreground">说明</p>
  </div>
  内容
  <div data-slot="dialog-footer" class="bg-muted/50 -mx-4 -mb-4 flex flex-col-reverse gap-2 rounded-b-xl border-t p-4 sm:flex-row sm:justify-end">
    <button data-slot="button" class="…outline…">取消</button>
    <button data-slot="button" class="…default…">确定</button>
  </div>
  <button data-slot="dialog-close" class="…ghost，icon-sm… absolute top-2 right-2"><svg>×</svg><span class="sr-only">Close</span></button>
</div>

<!-- 确认弹窗 AlertDialog（不带右上角关闭按钮） -->
<div data-slot="alert-dialog-content" data-size="default" class="group/alert-dialog-content bg-popover text-popover-foreground ring-foreground/10 … fixed … grid w-full gap-4 rounded-xl p-4 ring-1 duration-100 outline-none data-[size=default]:max-w-xs data-[size=sm]:max-w-xs data-[size=default]:sm:max-w-sm">
  <div data-slot="alert-dialog-header" class="grid grid-rows-[auto_1fr] place-items-center gap-1.5 text-center sm:group-data-[size=default]/alert-dialog-content:place-items-start sm:group-data-[size=default]/alert-dialog-content:text-left">
    <div data-slot="alert-dialog-media" class="bg-muted mb-2 inline-flex size-10 items-center justify-center rounded-md">可选图标 24px</div>
    <h2 data-slot="alert-dialog-title" class="text-base font-medium">标题</h2>
    <p data-slot="alert-dialog-description" class="text-muted-foreground text-sm text-balance md:text-pretty">说明</p>
  </div>
  <div data-slot="alert-dialog-footer" class="bg-muted/50 -mx-4 -mb-4 flex flex-col-reverse gap-2 rounded-b-xl border-t p-4 group-data-[size=sm]/alert-dialog-content:grid group-data-[size=sm]/alert-dialog-content:grid-cols-2 sm:flex-row sm:justify-end">
    <button data-slot="alert-dialog-cancel" class="…outline…">取消</button>
    <button data-slot="alert-dialog-action" class="…default 或 destructive…">确认</button>
  </div>
</div>
```

- 尺寸：普通弹窗宽度 = 视口宽 - 32px，640px 起最大 384px；业务里常用的通用弹窗封装（`@/components/dialog`）640px 起最大 672px、内边距 24px、底栏也是 24px。确认弹窗最大 320px，默认尺寸 640px 起最大 384px。内边距 16px，区块间距 16px，圆角 22.4px（不达标），高度最多到视口高减去上下各至少 16px 的安全距离，超出后弹窗内部滚动。
- 底栏：浅灰底（muted 50%，浅 #fafafa / 深 #2f2f2f），上边 1px 分隔线，内边距 16px，按钮间距 8px；小于 640px 按钮竖排且主操作在上，640px 起横排靠右。确认弹窗选小号时底栏是两列等宽。
- 确认弹窗标题区：小于 640px 居中，640px 起靠左；标题、说明间距 6px（不达标）；可选图标框 40×40px、圆角 12.8px（不达标）。

| 状态 | 样子（关闭按钮、操作按钮） |
|---|---|
| 默认 | 关闭按钮：28×28px 透明底，16px 叉号图标，前景色，距右上角各 8px；操作按钮按「按钮」一节 |
| 悬停 | 关闭按钮底变 muted（浅 #f5f5f5，深色 muted 50%）；操作按钮按「按钮」一节 |
| 按下 | 下移 1px，没开"减少动态效果"时缩到 98% |
| 禁用 | 50% 不透明，不响应鼠标 |
| 聚焦 | 边框变 ring，外加 3px ring 50% 光圈；弹窗打开时焦点进入弹窗 |

### 提示条

- 写法：

```html
<!-- 轻提示 toast（sonner），全局一个，位置在页面顶部居中 -->
<section aria-label="Notifications alt+T">
  <ol data-sonner-toaster data-sonner-theme="light|dark" class="toaster group" style="--normal-bg:var(--popover); --normal-text:var(--popover-foreground); --normal-border:var(--border); --success-bg:color-mix(in oklch,var(--success) 16%,var(--popover)); …; --border-radius:var(--radius); --width:356px; --gap:14px">
    <li data-sonner-toast data-styled="true" data-rich-colors="true" data-type="success|info|warning|error">
      <button data-close-button aria-label="Close toast">×</button>
      <div data-icon><svg class="size-4">16px 图标</svg></div>
      <div data-content>
        <div data-title>标题</div>
        <div data-description>描述</div>
      </div>
      <button data-button>操作</button>
    </li>
  </ol>
</section>

<!-- 页内警示框 Alert -->
<div data-slot="alert" role="alert" class="group/alert relative grid w-full gap-0.5 rounded-lg border px-2.5 py-2 text-left text-sm has-[>svg]:grid-cols-[auto_1fr] has-[>svg]:gap-x-2 has-data-[slot=alert-action]:pr-18 *:[svg]:row-span-2 *:[svg]:translate-y-0.5 *:[svg:not([class*='size-'])]:size-4 bg-card text-card-foreground">
  <svg>图标</svg>
  <div data-slot="alert-title" class="font-medium group-has-[>svg]/alert:col-start-2 [&_a]:underline [&_a]:underline-offset-3 [&_a]:hover:text-foreground">标题</div>
  <div data-slot="alert-description" class="text-muted-foreground text-sm text-balance md:text-pretty [&_a]:underline [&_a]:underline-offset-3 [&_a]:hover:text-foreground">说明</div>
  <div data-slot="alert-action" class="absolute top-2 right-2">可选操作</div>
</div>
```

- 轻提示：宽 356px（600px 及以下铺满，左右各留 16px），距顶 24px（窄屏 16px），多条叠放间距 14px；内边距 16px、图标和文字间距 6px（不达标）、圆角 16px、1px 边框、阴影 `0 4px 12px rgba(0,0,0,0.1)`；标题 13px、500，描述 12px；自动消失时间 5000ms；每条都有关闭按钮。颜色见「颜色」一节的提示条表。
- 轻提示关闭按钮：20×20px 圆形（不达标，小于 24×24px），在提示条右上角往外探出 35%；底、边框、叉号颜色跟随提示类型。
- 轻提示里的操作按钮（`data-button`）：高 24px、左右 8px、圆角 4px、12px 字、500，底 = 提示文字色，字 = 提示底色。
- 警示框：圆角 16px，1px 边框（border 色），左右内边距 10px（不达标）、上下 8px，标题和说明间距 2px（不达标），带图标时图标 16px、图标和文字间距 8px，右上角留操作区时右内边距 72px。默认变体底 card、字 card-foreground；错误变体字 = 错误色，说明 = 错误色 90%（深色下对 card 4.28:1，不达标）。

| 状态 | 样子（关闭按钮、操作链接） |
|---|---|
| 默认 | 轻提示关闭按钮：底和边框跟随提示类型，叉号 = 提示文字色；普通类型浅色边框 hsl(0 0% 93%)、叉号 hsl(0 0% 9%)。警示框里的链接：带下划线（偏移 3px），颜色同所在文字 |
| 悬停 | 普通类型浅色：关闭按钮底 hsl(0 0% 97.3%)、边框 hsl(0 0% 90.9%)；深色：底 hsl(0 0% 12%)、边框 hsl(0 0% 25%)；彩色类型浅色下没有变化。警示框链接文字变正文色 |
| 按下 | 关闭按钮没开"减少动态效果"时缩到 98% |
| 禁用 | 不适用：提示条里没有禁用状态 |
| 聚焦 | 关闭按钮：`0 4px 12px rgba(0,0,0,0.1)` 加 2px 黑色 20% 外圈；操作按钮：2px 黑色 40% 外圈 |

### 其他常用基础组件（简表）

| 组件 | 写法要点 | 尺寸和样子 |
|---|---|---|
| 徽标 Badge | `<span class="inline-flex h-5 w-fit items-center gap-1 rounded-4xl border border-transparent px-2 py-0.5 text-xs font-medium">` | 高 20px、左右 8px、上下 2px（不达标）、圆角 41.6px（不达标，显示为胶囊），图标 12px；变体：主色、次色、错误（错误色 10% 底）、警告（警告色 10% 底 + 40% 边框）、描边、透明、链接 |
| 标签页 Tabs | 外框 `<div data-slot="tabs-list" class="inline-flex h-8 rounded-lg bg-muted p-[3px] text-muted-foreground">`，按钮 `<button data-slot="tabs-trigger" class="h-[calc(100%-1px)] flex-1 gap-1.5 rounded-md border border-transparent px-1.5 py-0.5 text-sm font-medium text-foreground/60 hover:text-foreground data-active:bg-background data-active:text-foreground data-active:shadow-sm">` | 外框高 32px、内边距 3px（不达标）、圆角 16px；按钮左右 6px、上下 2px（不达标）、圆角 12.8px（不达标）；未选中字：浅色前景 60%（#686868，对 muted 5.11:1）、深色 #b7b7b7；选中：白底加小阴影，深色底 input 30%、1px input 边框；"线条"变体选中时底部 2px 下划线 |
| 提示气泡 Tooltip | `<div data-slot="tooltip-content" class="bg-foreground text-background inline-flex w-fit max-w-xs items-center gap-1.5 rounded-md px-3 py-1.5 text-xs">` | 最大宽 320px、左右 12px、上下 6px（不达标）、圆角 12.8px（不达标）、12px 字；离触发元素 4px；带 10px 小箭头（圆角 2px，不达标）；鼠标移上去立即出现（延迟 0ms） |
| 下拉菜单 / 气泡弹层 | `rounded-lg p-1 shadow-md ring-1 ring-foreground/10 bg-popover`；气泡弹层宽 288px、内边距 10px（不达标）、间距 10px | 圆角 16px，阴影见「圆角、边框、阴影」 |
| 开关 Switch、复选框 | 边框 input、选中底 primary | 开关滑块 16×16px（小号 12×12px），全圆角 9999px |

## 8. 动效

| 项 | 时长 | 缓动 |
|---|---|---|
| 悬停、按下反馈（按钮、输入框、标签页颜色变化） | 150ms | `cubic-bezier(0.4, 0, 0.2, 1)` |
| 表格行悬停底色 | 120ms | `ease` |
| 卡片悬停上移和阴影 | 150ms | `ease` |
| 侧栏子菜单展开、收起 | 300ms | `ease-out` |
| 折叠面板（Accordion）展开、收起 | 200ms | `ease-out` |
| 菜单箭头旋转 | 200ms | `cubic-bezier(0.4, 0, 0.2, 1)` |
| 侧栏展开、收起成图标栏 | 200ms | `linear` |
| 弹窗出现、消失（Dialog、AlertDialog 及遮罩） | 100ms，淡入淡出 + 从 95% 放大到 100% | `ease` |
| 下拉菜单、气泡弹层 | 100ms，淡入 + 95% 放大 + 从触发方向滑入 8px | `ease` |
| 提示气泡 | 150ms，同上 | `ease` |
| 侧边抽屉（手机侧栏、主题设置） | 200ms（遮罩 150ms） | `ease-in-out` |
| 轻提示进出、叠放位移 | 400ms | 浏览器默认 `ease` |
| 切换页面 | 150ms，从下方 8px、4px 模糊淡入 | `cubic-bezier(0.33, 1, 0.68, 1)` |
| 侧栏切换视图（进入 / 离开系统设置） | 150ms，左右移动 8px 淡入淡出 | `cubic-bezier(0.33, 1, 0.68, 1)` |
| 表格行出现 | 200ms，每行依次晚 25ms | `cubic-bezier(0.33, 1, 0.68, 1)` |
| 骨架屏闪光 | 1800ms 循环 | `ease-in-out` |

- 用户系统设置了"减少动态效果"时：关掉页面切换、侧栏视图切换、卡片悬停上移、按钮按下缩小、表格行出现、侧栏子菜单展开动画、骨架屏闪光、首页各类入场动画、轻提示动画。以下几项照常播放：弹窗、下拉菜单、提示气泡的淡入放大（100~150ms），按钮按下下移 1px，侧栏展开收起（200ms），折叠面板展开（200ms）。

## 9. 无障碍

- 聚焦：按钮、输入框、下拉选择框、徽标聚焦时边框变 focus 色，外加 3px focus 色 50% 光圈（输入框光圈画在框内）；侧栏菜单项是 2px 的 sidebar-ring 实色描边；顶栏站点名是 2px focus 色 40% 光圈；其余元素全局 outline 颜色为 focus 色 50%。现状对比度不够：光圈对背景浅色 1.59:1、深色 1.79:1（不达标）。
- 点击区域：至少 24×24 px。现状：最小的按钮是 xs 和 icon-xs（24px），头像按钮 24×24px，达标；轻提示关闭按钮 20×20px（不达标）。
- 颜色不是唯一的信息来源：轻提示每种类型都有不同图标（对勾、i、三角、叉、转圈）；表单出错时除了红色边框还有字段下方的错误文字和 `aria-invalid`；必填项有红色 `*`；未保存提示有圆点加文字。
- 表单：每个输入框都有看得见的标签（`<label>` / `FormLabel`）；顶栏搜索按钮靠 `aria-label`，按钮里也显示"搜索"字样（小于 640px 时只剩图标）。
- 跳到主内容：每个控制台页面第一个可聚焦元素是"Skip to Main"链接，指向 `#content`。现状：页面上没有 `id="content"` 的元素，点了不会跳（不达标）。
- 页面主区域：侧栏右边的内容面板是 `<main>`，各页面的 `Main` 组件里又包了一层 `<main>`，一页里出现两个嵌套的 `<main>`（不达标）。

例外：用户不接受某条不达标的修改建议时记在这里，以后检查时不再报。

下面这些都是上游主题和组件的原样。fork 改了会在每次同步上游时冲突，所以照现状保留。

| 项 | 实际值 | 用户确认日期 |
|---|---|---|
| 主按钮白字对主色、主色作链接文字 | 浅色主色 #3ea4ec，白字对它 2.72:1；作链接文字时浅色对白底 2.72:1，深色对深底 3.30:1 | 2026-11-09 |
| 输入框、描边按钮边框 | 浅色 #e8e8e8 对白底 1.23:1；深色实际 #4e4e4e 对深底 2.00:1；浅色描边按钮用 `border-border` | 2026-11-09 |
| 聚焦光圈 | 3px、focus 色 50% 透明，对背景浅色 1.59:1、深色 1.79:1；浅色 ring 实色 2.58:1 | 2026-11-09 |
| 浅色警告色作文字 | #d08700 对白底 2.93:1，警告徽标 2.66:1，警告轻提示 2.47:1 | 2026-11-09 |
| 轻提示文字对轻提示底 | 浅色成功 3.06、信息 3.32、错误 3.83；深色成功 4.22、信息 3.01、错误 3.68（单位 :1） | 2026-11-09 |
| 浅色成功、信息色作文字 | 成功 #009966 对白底 3.67:1，信息 #0084d1 对白底 4.02:1 | 2026-11-09 |
| 危险按钮文字、错误警示框描述 | 危险按钮文字对按钮底浅色 3.99:1、深色 4.28:1；深色错误警示框描述对卡片底 4.28:1 | 2026-11-09 |
| 侧栏选中项文字（浅色） | #0276ea 对选中底 #eaf4fe 3.95:1，14px | 2026-11-09 |
| 侧栏分组标题（浅色） | 11px，次要文字色 70% 透明，对侧栏底 3.18:1 | 2026-11-09 |
| 跳到主内容链接 | 指向 `#content`，页面上没有这个元素 | 2026-11-09 |
| 嵌套 `<main>` | 侧栏右边内容面板和 `Main` 组件各是一个 `<main>` | 2026-11-09 |
| 圆角档位 | sm 9.6px、md 12.8px、xl 22.4px、2xl 28.8px、3xl 35.2px、4xl 41.6px；Ocean Breeze 4.8px、Anthropic 10px、圆角档位 sm 4.8px | 2026-11-09 |
| 基础组件间距 | 按钮、输入框、多行输入框、警示框左右 10px；按钮图标间距 6px；警示框标题和说明间距 2px；徽标上下 2px；标签页外框 3px、按钮左右 6px 上下 2px；提示气泡上下 6px；确认弹窗标题区间距 6px | 2026-11-09 |
| 页面框架间距 | 页面标题区下内边距 10px；内容区 640px 起上内边距 6px；页面底栏上下 10px；顶栏元素间距 6px；侧栏二级菜单外边距 14px、内边距左右 10px 上下 2px；设置页开关行和控件组上下 10px | 2026-11-09 |
| 轻提示关闭按钮、图标间距 | 关闭按钮 20×20px；图标和文字间距 6px | 2026-11-09 |

## 10. 终端界面

不适用：这是网页前端项目，没有命令行界面。
