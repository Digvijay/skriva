# Template Functions

Functions available in all Go templates. Call with Go template syntax: <code v-pre>{{functionName args}}</code>

| Function | Description | Example |
|----------|-------------|---------|
| `formatDate` | Format time.Time | <code v-pre>{{formatDate .Post.Date "Jan 2, 2006"}}</code> |
| `readingTime` | Reading time string | <code v-pre>{{readingTime .Post}}</code> |
| `truncate` | Truncate with ellipsis | <code v-pre>{{truncate .Description 160}}</code> |
| `safeHTML` | Mark trusted HTML | <code v-pre>{{safeHTML .Post.HTML}}</code> |
| `slugify` | Convert to URL slug | <code v-pre>{{slugify .Tag.Name}}</code> |
| `currentYear` | Current year int | <code v-pre>{{currentYear}}</code> |
| `hasPrefix` | String prefix check | <code v-pre>{{if hasPrefix .Nav.Path "/admin"}}</code> |
| `lower` | Lowercase | <code v-pre>{{lower .Tag.Name}}</code> |
| `upper` | Uppercase | <code v-pre>{{upper .Post.Title}}</code> |
| `join` | Join string slice | <code v-pre>{{join .Post.Tags ", "}}</code> |
| `add` | Add two ints | <code v-pre>{{add .Pagination.CurrentPage 1}}</code> |
| `sub` | Subtract two ints | <code v-pre>{{sub .Pagination.TotalPages 1}}</code> |
| `seq` | Generate int sequence | <code v-pre>{{range seq 1 5}}...{{end}}</code> |
| `jsonLD` | JSON-LD script tag | <code v-pre>{{jsonLD .Meta}}</code> |
| `cacheVer` | Cache-bust version | <code v-pre>{{cacheVer}}</code> (hourly) |
| `i18n` | Localized string | <code v-pre>{{i18n "subscribe"}}</code> |

## Usage Notes

::: warning
Only use `safeHTML` on content you trust (admin-authored posts/pages). Never use it on user-submitted content like comments or webmentions.
:::

### i18n Keys

Available translation keys: `home`, `archive`, `tags`, `search`, `read_more`, `min_read`, `published_on`, `comments`, `post_comment`, `subscribe`, `unsubscribe`, `like`, `dislike`, `share`, `related_posts`, `draft`, `featured`, `powered_by`, and more.
