# Settings Renovation

## Contract and Boundaries

The reader remains a Go binary with SQLite, embedded assets, Vue, and the existing CSS library. No API route, database migration, persisted preference key, OPML attribute, Fever contract, or Google Reader contract changes.

The detail column has two presentation states: reading and settings. Opening settings leaves the selected article and its DOM mounted. Settings own their page ancestry, per-page scroll position, object-keyed text drafts, pending writes, and feedback. The reading view does not acquire a second copy of article state.

## Architecture Audit

| Area | Responsibility / call chain | Decision |
| --- | --- | --- |
| CLI/platform | flags and environment -> storage -> server/worker; platform-specific tray | Preserve. Build tags and platform implementations are not dead code. |
| HTTP | route registration -> auth/forms -> storage/worker/content | Preserve routes and package boundaries. Changes are limited to asset template composition and the normalized template access check. |
| Storage | schema migrations -> feed/folder/item/settings persistence | Preserve. Migration history and convenience constructors have callers in compatibility/import paths and tests. |
| Refresh | worker scheduling -> RSSHub selection -> crawler -> parser -> storage | Preserve. Icon refresh wrappers have server and test callers; no speculative consolidation. |
| Parsing/content | format-specific parser -> shared models; content extraction and sanitization | Preserve. Similar format parsers have distinct semantics. Existing parser TODO is not removal evidence. |
| Frontend | API wrapper -> reader root + settings/navigation mixins -> components/templates | Split by lifecycle and ownership, not arbitrary file size. |
| CSS | reader/layout/themes + settings workspace | Keep shared theme tokens; replace modal-sized settings rules rather than accumulate overrides. |

## Confirmed Removals

- The old dropdown component and its custom CSS had no template or dynamic component callers. The remaining modal is used only for destructive confirmation.
- The two `list_items` API wrappers had no callers and pointed at routes that were not registered. All article lists continue using `/api/items`.
- Prompt/alert modal variants, their input state, and their wrappers are replaced by a shared inline field component and page feedback.
- Modal-only settings grids, constrained nested scrolling, font-size button styling, and custom hidden checkbox decoration are removed. Native numeric and checkbox controls retain keyboard semantics.
- The main template delegates settings to a named Go template. Inspection found that repeated slashes and dot segments bypassed the old template check; the static handler now checks normalized URL paths, with regression tests for root and base-path deployments. All additional JS/CSS assets use the existing version parameter.

## Visual Mandate

Bounded remodel: preserve the compact three-column reader, existing type choices, images/icons, and three themes. The old system settings dialog measured 330px wide even on desktop and placed an overlay over both navigation columns. Settings now occupy the existing detail column with a fixed compact header and one scrolling body.

Neutral light surfaces, fine dividers, reduced toolbar shadows, and visible focus rings establish hierarchy without floating section cards. Existing article cards use 8px radii. No new artwork, icon library, framework, or build pipeline is introduced; subscription icons and article media remain the visual assets.

An independent design preview was created before changing the product template. Browser screenshots validate the implemented workspace, not just the preview.

## Verification

- `make test`: existing Go suites plus asset composition/version checks.
- `make host`: embedded single-binary build.
- `go vet -tags 'sqlite_foreign_keys sqlite_json' ./...`: static analysis.
- `make test-ui`: real Chrome through Playwright, disposable SQLite database, and a local RSS/article fixture server. Requires Node, Playwright, and installed Chrome. Use `PLAYWRIGHT_CHANNEL` for a different installed channel; `NODE_PATH` may point to a shared Playwright installation.
- `YARR_UI_EMBEDDED=1 make test-ui`: run the same suite against embedded assets instead of disk-loaded debug assets.
- `YARR_UI_SCREENSHOTS=out/ui make test-ui`: optional screenshots at 1440, 1024, 768, 390, and 320px, in all themes. Screenshots and temporary databases are not committed.

Coverage includes reading DOM/scroll restoration, same-article selection, shortcut isolation, inline drafts/cancellation, failed and late saves, mobile Back, parent scroll restoration, all settings subpages, content defaults/selectors, retained iframe instances, new folders, OPML import/export, RSSHub list persistence, backups, authentication changes without reload, confirmation focus/Escape, partial deletion/retry, and missing settings objects. The content-mode UI test mocks the extracted-content response because the crawler correctly blocks loopback URLs; extraction and selectors retain their Go coverage.

Real external RSSHub provider availability and platform tray GUI behavior are outside this frontend runtime suite; their existing Go tests/build contracts remain unchanged. Whole-module rewrites, dependency upgrades, and unrelated storage/worker changes are deferred.

The responsive suite also checks long unbroken values and the existing maximum 30px font size at 320px width. Inline editor actions wrap, and narrow settings containers stack labels and option controls instead of clipping text. Browser storage failures leave local preferences unchanged and show inline feedback.

## Review and Rollback

Review the settings/navigation behavior separately from visual changes. Asset splitting must be reverted together with script/template inclusion changes, otherwise the boot sequence is incomplete. No data migration is involved, so returning to the previous executable preserves database compatibility. Do not revert user changes or restore an entire dirty working tree.
