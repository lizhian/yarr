# yarr

`yarr` is a personal RSS/feed reader. This context captures the product language used for reader-facing UI and issue descriptions.

## Language

**订阅源**:
An RSS, Atom, RDF, or JSON Feed source that produces articles.
_Avoid_: Feed, 源

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

## Relationships

- A **文件夹** contains zero or more **订阅源**.
- A **订阅源** produces zero or more **文章**.
- An **文章** can be **未读** or **已读**.
- An **文章** can be **星标** independently of whether it is read.
- **正文模式** applies to one selected **文章**.

## Example dialogue

> **Dev:** "刷新所有 **订阅源** 后，新内容应该显示在哪里？"
> **Domain expert:** "每个 **订阅源** 产生的 **文章** 显示在文章列表里；如果还没读，就出现在 **未读** 过滤结果中。"

## Flagged ambiguities

- "Feed" is translated as **订阅源** in user-facing UI, not "源".
- "Item" and "Article" are translated as **文章** in user-facing UI, not "条目".
