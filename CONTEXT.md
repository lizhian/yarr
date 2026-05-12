# yarr

`yarr` is a personal RSS/feed reader. This context captures the product language used for reader-facing UI and issue descriptions.

## Language

**订阅源**:
An RSS, Atom, RDF, or JSON Feed source that produces articles.
_Avoid_: Feed, 源

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

## Relationships

- A **文件夹** contains zero or more **订阅源**.
- A **订阅源** produces zero or more **文章**.
- **自动刷新** periodically checks **订阅源** for new **文章**.
- An **文章** can be **未读** or **已读**.
- An **文章** can be **收藏** independently of whether it is read.
- An **文章** can have a **缩略图** when it includes an image media link.
- **正文模式** applies to one selected **文章**.
- **工具栏显示** applies to most top-level toolbar actions.
- **工具栏显示** can be **仅文字** or **仅图标**.
- Some compact top-level toolbar actions remain icon-only regardless of **工具栏显示**.
- Narrow layouts may temporarily render top-level toolbar actions as **仅图标** without changing the saved **工具栏显示** preference.

## Example dialogue

> **Dev:** "刷新所有 **订阅源** 后，新内容应该显示在哪里？"
> **Domain expert:** "每个 **订阅源** 产生的 **文章** 显示在文章列表里；如果还没读，就出现在 **未读** 过滤结果中。"

## Flagged ambiguities

- "Feed" is translated as **订阅源** in user-facing UI, not "源".
- "Item" and "Article" are translated as **文章** in user-facing UI, not "条目".
- "Starred" is translated as **收藏** in user-facing UI, not "星标".
- "顶栏图标显示对应的文字" refers to top-level toolbar actions, not icons inside menus or article/feed lists.
- **仅文字** is the default **工具栏显示** mode; **仅图标** preserves the previous compact toolbar behavior.
- `设置`, `上篇`, `下篇`, `关闭`, and `已读` are compact toolbar actions and remain icon-only.
- Top-level toolbar labels are short action labels, not full descriptions; full descriptions remain in button titles.
