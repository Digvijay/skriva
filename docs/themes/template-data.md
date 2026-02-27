# Template Data

Every template receives a `TemplateData` struct. Theme authors can rely on these fields.

## Top-Level Fields

| Field              | Type           | Available In             | Description                                                   |
| ------------------ | -------------- | ------------------------ | ------------------------------------------------------------- |
| `.Site`            | `SiteConfig`   | All pages                | Title, tagline, author, social links, footer, feature toggles |
| `.Meta`            | `SEOMeta`      | All pages                | Title, description, canonical URL, OG data                    |
| `.Nav`             | `NavData`      | All pages                | Current path, active section                                  |
| `.Post`            | `*Post`        | `post.html`              | Single post                                                   |
| `.Posts`           | `[]Post`       | `home`, `tag`, `archive` | Post list                                                     |
| `.Page`            | `*Page`        | `page.html`              | Static page                                                   |
| `.Tags`            | `[]TagInfo`    | All pages                | All tags with counts                                          |
| `.Tag`             | `*TagInfo`     | `tag.html`               | Current tag                                                   |
| `.Stats`           | `*PostStats`   | `post.html`              | Views, likes, dislikes                                        |
| `.Comments`        | `[]Comment`    | `post.html`              | Approved comments                                             |
| `.Pagination`      | `*Pagination`  | `home.html`              | Page numbers, prev/next                                       |
| `.CSRFToken`       | `string`       | All pages                | CSRF token for forms                                          |
| `.Year`            | `int`          | All pages                | Current year                                                  |
| `.Error`           | `string`       | Error pages              | Error message                                                 |
| `.RelatedPosts`    | `[]Post`       | `post.html`              | Posts with shared tags                                        |
| `.SearchQuery`     | `string`       | `search.html`            | Current query                                                 |
| `.SearchResults`   | `[]Post`       | `search.html`            | Results                                                       |
| `.Webmentions`     | `[]Webmention` | `post.html`              | Verified webmentions                                          |
| `.FediAddress`     | `string`       | All pages                | e.g. `@blog@example.com`                                      |
| `.TableOfContents` | `[]TOCEntry`   | `post.html`              | Headings for ToC                                              |
| `.SeriesPosts`     | `[]Post`       | `post.html`              | All posts in same series                                      |

## Post Fields

| Field          | Type        | Description                                               |
| -------------- | ----------- | --------------------------------------------------------- |
| `.Title`       | `string`    | Post title                                                |
| `.Slug`        | `string`    | URL slug                                                  |
| `.Date`        | `time.Time` | Publication date                                          |
| `.Tags`        | `[]string`  | Tag names                                                 |
| `.Description` | `string`    | SEO description                                           |
| `.Image`       | `string`    | Featured image filename                                   |
| `.Featured`    | `bool`      | Featured flag                                             |
| `.Draft`       | `bool`      | Draft flag                                                |
| `.TOC`         | `bool`      | Show table of contents                                    |
| `.Series`      | `string`    | Series name                                               |
| `.SeriesOrder` | `int`       | Position in series                                        |
| `.Content`     | `string`    | Raw markdown                                              |
| `.HTML`        | `string`    | Rendered HTML (use <code v-pre>{{safeHTML .HTML}}</code>) |
| `.ReadingTime` | `int`       | Estimated minutes                                         |
| `.WordCount`   | `int`       | Word count                                                |
| `.Permalink`   | `string`    | URL path                                                  |

## Site Config Fields

| Field                      | Description        |
| -------------------------- | ------------------ |
| `.Site.Title`              | Blog title         |
| `.Site.Tagline`            | Blog tagline       |
| `.Site.BaseURL`            | Canonical base URL |
| `.Site.Theme`              | Active theme name  |
| `.Site.Locale`             | Language code      |
| `.Site.Author.Name`        | Author name        |
| `.Site.Author.Bio`         | Author biography   |
| `.Site.Author.Avatar`      | Avatar image path  |
| `.Site.Social.Github`      | GitHub username    |
| `.Site.Social.Twitter`     | Twitter username   |
| `.Site.ActivityPubEnabled` | AP toggle          |
| `.Site.WebmentionEnabled`  | WM toggle          |
| `.Site.IndieAuthEnabled`   | IA toggle          |
| `.Site.MicropubEnabled`    | MP toggle          |
