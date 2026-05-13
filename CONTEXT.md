# yarr

`yarr` is a personal RSS/feed reader. This context captures the product language used for reader-facing UI and issue descriptions.

## Language

**订阅源**:
An RSS, Atom, RDF, or JSON Feed source that produces articles.
_Avoid_: Feed, 源

**RSSHub 基础链接**:
The HTTP(S) base URL of the RSSHub provider used to resolve RSSHub subscription links.
_Avoid_: RSSHub host, RSSHub provider URL

**RSSHub 订阅链接**:
A portable subscription source link beginning with `rsshub://` whose path is resolved against the RSSHub base URL.
_Avoid_: RSSHub URL, RSSHub shortcut

**RSSHub 快速添加**:
A settings action that creates an RSSHub subscription link from a supported service template and a user-entered identifier.
_Avoid_: RSSHub preset, RSSHub shortcut

**自动刷新**:
A setting that periodically checks subscription sources for new articles without user action.
_Avoid_: Auto-refresh, 自动更新

**文件夹**:
A user-created grouping for subscription sources.
_Avoid_: Folder, 分类

**文章**:
A single item published by a subscription source.
_Avoid_: Item, 条目

**文章详情**:
The reading view for one selected article.
_Avoid_: 详情页, Item detail

**未读**:
An article state meaning the user has not marked or consumed the article as read.
_Avoid_: Unread

**已读**:
An article state meaning the user has consumed or marked the article as read.
_Avoid_: Read

**收藏**:
An article state meaning the user intentionally saved the article for later attention.
_Avoid_: Starred, 星标

**全部**:
An article list view that includes read, unread, and saved articles.
_Avoid_: All

**正文模式**:
A reading mode that extracts and displays the article body inside yarr.
_Avoid_: Read Here, 阅读这里

**工具栏显示**:
A user preference that controls whether top-level toolbar actions are shown as icons, text, or both.
_Avoid_: 顶栏图标模式, 按钮样式

**字体显示**:
A user preference that controls the font family used across yarr's user interface and article reading surface.
_Avoid_: 字体切换, 正文字体

**霞鹜文楷**:
The default **字体显示** option for yarr.
_Avoid_: 霞鹭文楷, LXGW WenKai

**Maple Mono NF-CN**:
A **字体显示** option for users who prefer a monospaced interface font.
_Avoid_: Maple Mono NF CN

**仅图标**:
A compact toolbar display mode where top-level toolbar actions show only their icon.
_Avoid_: 图标模式, 紧凑模式

**仅文字**:
A toolbar display mode where most top-level toolbar actions show labels without icons.
_Avoid_: 文字模式, 无图标模式

**缩略图**:
A small image preview shown alongside an article in the article list.
_Avoid_: Thumbnail, 预览图

**文章列表布局**:
A user preference that controls whether articles are shown as a list or as cards in the article list.
_Avoid_: 卡片模型, Item layout

**订阅源设置**:
A group of actions for one subscription source, including opening source links, renaming, moving, and deleting it.
_Avoid_: 订阅项设置, Feed settings

**文件夹设置**:
A group of actions for one folder, including renaming and deleting it.
_Avoid_: Folder settings

**卡片模式**:
A variable-height article list layout where articles with thumbnails show the thumbnail above the title and articles without thumbnails show as text-only cards.
_Avoid_: 卡片模型, Grid mode

**列表模式**:
The default article list layout that shows articles in the existing compact row format.
_Avoid_: Row mode, 默认模式

**移动端视图**:
A narrow-screen reader layout where only one navigation layer is visible at a time.
_Avoid_: WAP 页面, Mobile

**层级返回**:
A mobile navigation behavior where Back moves from article details to the article list, then from the article list to the subscription source list.
_Avoid_: Browser back, 返回上一页

## Relationships

- A **文件夹** contains zero or more **订阅源**.
- A **订阅源** produces zero or more **文章**.
- **文章详情** presents one selected **文章**.
- **文章详情** indents text paragraphs without indenting paragraphs that contain media or structural content.
- **文章详情** centers article images.
- An **RSSHub 订阅链接** is stored as the **订阅源** link and resolves through the current **RSSHub 基础链接** when yarr fetches it.
- Changing the **RSSHub 基础链接** changes where all saved **RSSHub 订阅链接** resolve without rewriting those links.
- An **RSSHub 基础链接** must be configured before an **RSSHub 订阅链接** can be added or refreshed.
- **RSSHub 快速添加** creates an **RSSHub 订阅链接** and then follows the normal **订阅源** creation flow.
- **RSSHub 快速添加** supports Bilibili user videos by UID and Telegram channels by 频道 ID.
- OPML import and export preserve **RSSHub 订阅链接** in portable form.
- **自动刷新** periodically checks **订阅源** for new **文章**.
- **自动刷新** can be disabled or set to 1m, 5m, 10m, 30m, or 1h from settings.
- An **文章** can be **未读** or **已读**.
- An **文章** can be **收藏** independently of whether it is read.
- **全部** includes **已读**, **未读**, and **收藏** articles.
- An **文章** can have a **缩略图** when it includes an image media link.
- **文章列表布局** can present **文章** in **列表模式** or **卡片模式**.
- **列表模式** is the default **文章列表布局**.
- **卡片模式** presents **文章** as a single-column card flow.
- **卡片模式** places the **缩略图** above the article title when the **文章** has a **缩略图**.
- **卡片模式** does not add an image placeholder when the **文章** has no **缩略图**.
- **卡片模式** allows article titles to wrap naturally instead of truncating them to a fixed line count.
- **卡片模式** preserves each **文章**'s subscription source, relative time, and read or starred state indicators.
- **卡片模式** gives each **文章** a lightweight card boundary while preserving clear selected and read-state styling across themes.
- **文章列表布局** changes only how **文章** are displayed, not how they are selected, opened, marked as read, or navigated.
- The article list toolbar can switch **文章列表布局** between **列表模式** and **卡片模式** with a single toggle action.
- The article list toolbar layout toggle remains available in **移动端视图** as an icon-only compact action.
- The article list toolbar layout toggle indicates the layout it will switch to, not the current **文章列表布局**.
- **正文模式** applies to one selected **文章**.
- **移动端视图** presents the **订阅源** list, **文章** list, and selected **文章** details as separate navigation layers.
- **层级返回** applies only in **移动端视图**.
- **层级返回** applies whether the current layer was reached during the current browser session or restored from saved reader settings.
- **层级返回** treats selected article details as a single layer; moving between articles does not create per-article Back steps.
- **层级返回** from the article list to the subscription source list clears the current **订阅源** or **文件夹** selection.
- **层级返回** keeps in-app layer changes and browser Back history aligned, whether the user changes layers through browser Back or yarr toolbar controls.
- **层级返回** does not expose the current layer in the address bar.
- **工具栏显示** applies to most top-level toolbar actions.
- **工具栏显示** can be **仅文字** or **仅图标**.
- **字体显示** applies globally to yarr's interface and article reading surface.
- **字体显示** can be **霞鹜文楷** or **Maple Mono NF-CN**.
- **霞鹜文楷** is the default **字体显示** option.
- **字体显示** is changed from the main settings menu, near **主题**.
- Some compact top-level toolbar actions remain icon-only regardless of **工具栏显示**.
- Narrow layouts may temporarily render top-level toolbar actions as **仅图标** without changing the saved **工具栏显示** preference.
- **订阅源设置** is opened from the corresponding **订阅源** row in the subscription source list.
- **文件夹设置** is opened from the corresponding **文件夹** row in the subscription source list.
- **订阅源设置** and **文件夹设置** are not article list toolbar actions.

## Example dialogue

> **Dev:** "刷新所有 **订阅源** 后，新内容应该显示在哪里？"
> **Domain expert:** "每个 **订阅源** 产生的 **文章** 显示在文章列表里；如果还没读，就出现在 **未读** 过滤结果中。"
>
> **Dev:** "用户在 **移动端视图** 的文章详情里点返回时应该离开 yarr 吗？"
> **Domain expert:** "不，先通过 **层级返回** 回到文章列表；再返回才回到订阅源列表。"

## Flagged ambiguities

- "Feed" is translated as **订阅源** in user-facing UI, not "源".
- "Item" and "Article" are translated as **文章** in user-facing UI, not "条目".
- `rsshub://...` is an **RSSHub 订阅链接** stored in portable form, not a one-time import shortcut expanded to HTTP(S) at creation time.
- "快速添加 RSSHub" means **RSSHub 快速添加**, not a separate subscription source type or a bypass of the normal creation flow.
- **RSSHub 基础链接** means an HTTP(S) base URL normalized without a trailing slash.
- "WAP 页面" means **移动端视图**: the narrow-screen responsive layout, not a separate page or server route.
- "返回" means **层级返回** inside yarr before browser-level history navigation.
- "Starred" is translated as **收藏** in user-facing UI, not "星标".
- "顶栏图标显示对应的文字" refers to top-level toolbar actions, not icons inside menus or article/feed lists.
- "卡片模型" means **卡片模式** in the **文章列表布局**, not a data model.
- "快速切换卡片模式" means a top-level article list toolbar toggle, not a keyboard shortcut or settings menu option.
- "文章列表紧凑模式" means **列表模式** in the **文章列表布局**, not **仅图标** toolbar display.
- The **文章列表布局** toggle icon should represent **列表模式** or **卡片模式** directly, not reuse an unrelated layered-content icon.
- **仅文字** is the default **工具栏显示** mode; **仅图标** preserves the previous compact toolbar behavior.
- "字体切换功能" means the global **字体显示** preference, not an article-body-only font setting.
- "霞鹭文楷" means **霞鹜文楷**; use the official font name in user-facing text.
- `设置`, `上篇`, `下篇`, `关闭`, and `已读` are compact toolbar actions and remain icon-only.
- Top-level toolbar labels are short action labels, not full descriptions; full descriptions remain in button titles.
- "订阅项设置" means **订阅源设置** for one **订阅源**, not a setting for articles in the **文章** list.
