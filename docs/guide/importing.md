# Importing Content

## Ghost

```bash
blog import ghost \
  --export /path/to/ghost-export.json \
  --images /path/to/ghost/content/images
```

This preserves:

- Slugs, dates, tags, featured status
- Feature images and inline images (downloaded/copied)
- HTML content converted to clean Markdown

## WordPress

```bash
blog import wordpress \
  --export /path/to/wordpress-export.xml
```

Converts:

- HTML to Markdown (headings, bold/italic, links, images, lists, code blocks)
- Tags and categories
- Slugs, dates, excerpts, draft status
- Common HTML entities decoded
