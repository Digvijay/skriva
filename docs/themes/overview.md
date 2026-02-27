# Themes Overview

Skriva ships with four bundled themes and supports custom themes.

## Bundled Themes

| Theme          | Style                           | Layout        | Color Modes         |
| -------------- | ------------------------------- | ------------- | ------------------- |
| **Classic**    | Hanselman-inspired, clean       | Single-column | Light + Dark + Auto |
| **Newsletter** | Ghost-style, editorial          | Two-column    | Light + Dark + Auto |
| **GitHub**     | GitHub Primer design system     | Single-column | Light + Dark + Auto |
| **Editorial**  | Magazine-style, content-focused | Single-column | Light + Dark + Auto |

## Switching Themes

1. Go to **Admin → Settings → Themes**
2. Browse available themes with live CSS preview
3. Click **Activate** on your chosen theme

Or set `theme: "classic"` in `site.yaml`.

## Custom Themes

User-installed themes go in `data/content/themes/<name>/` and take priority over bundled themes with the same name.

See [Creating a Theme](/themes/creating) for the step-by-step guide.

## Color Mode Toggle

All themes use a `data-color-mode` attribute on `<html>` with three states:

- `light` — forced light
- `dark` — forced dark
- `auto` — follows OS preference

The toggle is persisted in `localStorage`. Theme CSS uses CSS custom properties scoped to each mode.
