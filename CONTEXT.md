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

**星标**:
An article state meaning the user intentionally saved the article for later attention.
_Avoid_: Starred, 收藏

**正文模式**:
A reading mode that extracts and displays the article body inside yarr.
_Avoid_: Read Here, 阅读这里

**缩略图**:
A small image preview shown alongside an article in the article list.
_Avoid_: Thumbnail, 预览图

## Relationships

- A **文件夹** contains zero or more **订阅源**.
- A **订阅源** produces zero or more **文章**.
- An **RSSHub 订阅链接** is stored as the **订阅源** link and resolves through the current **RSSHub 基础链接** when yarr fetches it.
- Changing the **RSSHub 基础链接** changes where all saved **RSSHub 订阅链接** resolve without rewriting those links.
- An **RSSHub 基础链接** must be configured before an **RSSHub 订阅链接** can be added or refreshed.
- OPML import and export preserve **RSSHub 订阅链接** in portable form.
- **自动刷新** periodically checks **订阅源** for new **文章**.
- An **文章** can be **未读** or **已读**.
- An **文章** can be **星标** independently of whether it is read.
- An **文章** can have a **缩略图** when it includes an image media link.
- **正文模式** applies to one selected **文章**.

## Example dialogue

> **Dev:** "刷新所有 **订阅源** 后，新内容应该显示在哪里？"
> **Domain expert:** "每个 **订阅源** 产生的 **文章** 显示在文章列表里；如果还没读，就出现在 **未读** 过滤结果中。"

## Flagged ambiguities

- "Feed" is translated as **订阅源** in user-facing UI, not "源".
- "Item" and "Article" are translated as **文章** in user-facing UI, not "条目".
- `rsshub://...` is an **RSSHub 订阅链接** stored in portable form, not a one-time import shortcut expanded to HTTP(S) at creation time.
- **RSSHub 基础链接** means an HTTP(S) base URL normalized without a trailing slash.
