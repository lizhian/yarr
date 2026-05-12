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

**自动刷新**:
A setting that periodically checks subscription sources for new articles without user action.
_Avoid_: Auto-refresh, 自动更新

**文件夹**:
A user-created grouping for subscription sources.
_Avoid_: Folder, 分类

**文章**:
A single item published by a subscription source.
_Avoid_: Item, 条目

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

**仅图标**:
A compact toolbar display mode where top-level toolbar actions show only their icon.
_Avoid_: 图标模式, 紧凑模式

**仅文字**:
A toolbar display mode where most top-level toolbar actions show labels without icons.
_Avoid_: 文字模式, 无图标模式

**缩略图**:
A small image preview shown alongside an article in the article list.
_Avoid_: Thumbnail, 预览图

**移动端视图**:
A narrow-screen reader layout where only one navigation layer is visible at a time.
_Avoid_: WAP 页面, Mobile

**层级返回**:
A mobile navigation behavior where Back moves from article details to the article list, then from the article list to the subscription source list.
_Avoid_: Browser back, 返回上一页

## Relationships

- A **文件夹** contains zero or more **订阅源**.
- A **订阅源** produces zero or more **文章**.
- An **RSSHub 订阅链接** is stored as the **订阅源** link and resolves through the current **RSSHub 基础链接** when yarr fetches it.
- Changing the **RSSHub 基础链接** changes where all saved **RSSHub 订阅链接** resolve without rewriting those links.
- An **RSSHub 基础链接** must be configured before an **RSSHub 订阅链接** can be added or refreshed.
- OPML import and export preserve **RSSHub 订阅链接** in portable form.
- **自动刷新** periodically checks **订阅源** for new **文章**.
- An **文章** can be **未读** or **已读**.
- An **文章** can be **收藏** independently of whether it is read.
- **全部** includes **已读**, **未读**, and **收藏** articles.
- An **文章** can have a **缩略图** when it includes an image media link.
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
- Some compact top-level toolbar actions remain icon-only regardless of **工具栏显示**.
- Narrow layouts may temporarily render top-level toolbar actions as **仅图标** without changing the saved **工具栏显示** preference.

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
- **RSSHub 基础链接** means an HTTP(S) base URL normalized without a trailing slash.
- "WAP 页面" means **移动端视图**: the narrow-screen responsive layout, not a separate page or server route.
- "返回" means **层级返回** inside yarr before browser-level history navigation.
- "Starred" is translated as **收藏** in user-facing UI, not "星标".
- "顶栏图标显示对应的文字" refers to top-level toolbar actions, not icons inside menus or article/feed lists.
- **仅文字** is the default **工具栏显示** mode; **仅图标** preserves the previous compact toolbar behavior.
- `设置`, `上篇`, `下篇`, `关闭`, and `已读` are compact toolbar actions and remain icon-only.
- Top-level toolbar labels are short action labels, not full descriptions; full descriptions remain in button titles.
