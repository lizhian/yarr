# FreshRSS / Google Reader API support

Yarr implements the FreshRSS-compatible Google Reader API at:

```
http(s)://host[:port]/api/greader.php
```

Use the same username and password configured for Yarr authentication. If Yarr
authentication is disabled, the API accepts any credentials, matching the open
access behavior of the existing Fever API.

The implementation targets the common sync subset used by RSS clients:

- `accounts/ClientLogin`
- `reader/api/0/user-info`
- `reader/api/0/token`
- `reader/api/0/subscription/list`
- `reader/api/0/tag/list`
- `reader/api/0/unread-count`
- `reader/api/0/stream/contents/*`
- `reader/api/0/stream/items/ids`
- `reader/api/0/stream/items/contents`
- `reader/api/0/edit-tag`
- `reader/api/0/stream/items/modify`

Read and starred states are stored independently. Adding or removing the starred
state does not change whether an item is read.
