# Delta — Login & Dashboard UI Spec

Reference: financial tracker mockup (dark navy canvas, floating white cards, teal/ink charts).

---

## 1. Design tokens

### Color

| Token | Hex | Use |
|---|---|---|
| `--bg-canvas` | `#110B29` | App background (deep indigo/near-black) |
| `--bg-canvas-alt` | `#1A1138` | Elevated dark surfaces, modals |
| `--surface` | `#FFFFFF` | Cards |
| `--surface-muted` | `#E9ECF1` | Active tab pill, button fill |
| `--ink` | `#0D0B2B` | Primary text, negative bars, area fill |
| `--ink-muted` | `#9AA3AE` | Axis labels, inactive tabs |
| `--accent` | `#3FB8AB` | Positive values, income bars, growth area |
| `--accent-soft` | `#D9F0ED` | Accent tint, hover states |
| `--data-blue` | `#29B6F6` | Donut segment 1 |
| `--data-red` | `#F4524D` | Donut segment 2 / overspend |
| `--divider` | `#E4E7EC` | Grid lines, hairlines |

Semantic: positive = `--accent`, negative = `--ink`, over-limit = `--data-red`.

### Typography

- Family: Inter (or Manrope) — geometric grotesque, tight tracking.
- Display / hero: 64–72px, weight 800, `-0.02em`, line-height 1.05. Used for "Financial Tracker App"-style headings and the login wordmark.
- Card title: 18px / 600
- Value (large figure): 20px / 600, tabular numerals
- Label: 14px / 500, `--ink-muted`
- Axis: 12px / 500, `--ink-muted`

Always enable `font-variant-numeric: tabular-nums` on anything showing money.

### Shape & elevation

- Card radius: `24px`
- Pill / tab radius: `12px`
- Input radius: `12px`
- Card shadow: `0 8px 32px rgba(0,0,0,0.18)`
- Card padding: `24px`
- Grid: 8px base. Card gutter `24px`, section gap `32px`.

---

## 2. Login screen

Split layout, 50/50 on desktop; stacked on mobile.

**Left panel** — `--bg-canvas`, holds the display-type wordmark ("Delta") plus a one-line tagline. Optional: a muted, decorative mini-chart card floating bottom-right at ~40% opacity.

**Right panel** — centered white card, max-width `420px`, radius `24px`, padding `40px`.

Card contents, top to bottom:

1. Heading — "Sign in" (28px / 700)
2. Subtext — "Welcome back" (14px, `--ink-muted`)
3. Email field — label above, 48px tall, 1px `--divider` border, focus ring `--accent` 2px
4. Password field — same, with show/hide toggle on the right
5. Row: "Remember me" checkbox (left) / "Forgot password?" link in `--accent` (right)
6. Primary button — full width, 48px, `--ink` fill, white text, radius `12px`. Hover: `--accent` fill.
7. Divider with "or"
8. OAuth buttons — Google, GitHub. Outlined, `--divider` border, 48px.
9. Footer — "No account? Create one" link

### States

- Idle, focus, error (border + helper text in `--data-red`), loading (button shows spinner, disabled), success (redirect to `/dashboard`).
- Validation on blur, not on every keystroke. Generic error copy on failed auth: "Email or password is incorrect" — never disclose which.

### Auth flow

- `POST /api/auth/login` → `{ email, password }`
- Response sets httpOnly, Secure, SameSite=Lax session cookie. Access token short-lived, refresh token rotated.
- Rate limit: 5 attempts / 15 min per IP + per account, then exponential backoff.
- Passwords hashed with Argon2id.
- `/dashboard` guarded by middleware; unauthenticated → `/login?next=`.

---

## 3. Dashboard

Masonry-ish card grid on `--bg-canvas`. 12-col desktop, 2 cards wide; single column under 768px.

### 3.1 Period tabs (shared component)

Row of pills at the top of each chart card. Active pill: `--surface-muted` fill, `--ink` text. Inactive: transparent, `--ink-muted`.

- Range tabs: Week · Month · Year · Custom
- Month tabs: Jul · Aug · Sep · Oct (scrollable on mobile)

### 3.2 Cash-flow bar chart (card A)

Diverging bars around a zero baseline.

- Income extends up in `--accent`, spending down in `--ink`
- Bar width `12px`, radius `6px` (fully rounded caps)
- Y axis mirrored: 10K / 20K / 30K both directions, labels left, `--ink-muted`
- X axis: weekday or date-bucket labels
- Zero line: 1px `--ink`, full width
- Tap/hover → tooltip with exact in/out for that bucket

### 3.3 Balance area chart (card B)

Smooth filled area straddling the zero line.

- Above zero filled `--accent`, below filled `--ink`
- Monotone cubic interpolation, no hard corners
- Crosshair: vertical 1px line + dot at intersection, value label rendered as plain text above/below the point
- Projected/incomplete range shaded `#F2F4F7` block to the right of "today"
- X axis buckets: `1–7`, `8–15`, `16–23`, `24–31`

### 3.4 Limit card (card C)

Two-row key/value block, label left in `--ink-muted`, figure right in `--ink`, 600 weight:

```
Monthly limit            15 000.00
Remaining                 9 511.00
```

Variant for per-event limits with the same layout, followed by a secondary button ("Edit events") — full width, `--surface-muted` fill, 44px, radius `12px`.

When remaining ≤ 10% of limit, flip the figure to `--data-red`.

### 3.5 Category donut (card D)

- Donut, thickness ~`28px`, rounded segment caps, `6px` gap between segments
- Palette order: `--data-blue`, `--data-red`, `--accent`, `--ink`
- Center: total spend (20px/600) with "this month" beneath in `--ink-muted`
- Legend below: dot + category + percent, two columns

---

## 4. Component inventory

- `<Card>` — white surface wrapper, optional header slot
- `<PeriodTabs>` — controlled, emits selected range
- `<BarChartDiverging>`
- `<AreaChartZero>`
- `<DonutChart>`
- `<StatRow>` — label/value pair
- `<Button>` — `primary` | `secondary` | `ghost`
- `<TextField>` — with label, helper, error
- `<EmptyState>` — for cards with no data in range

---

## 5. Responsive

| Breakpoint | Layout |
|---|---|
| ≥ 1280px | 2-col card grid, max content width 1200px, centered |
| 768–1279px | 2-col, cards shrink; tabs stay inline |
| < 768px | Single column, 16px side padding, tab rows horizontally scrollable, charts at 280px height |

---

## 6. Accessibility

- Contrast: `--ink-muted` on white is ~4.6:1 — fine for labels at 14px+, not for 12px axis text on colored fill.
- Never encode meaning in color alone: pair the donut with a legend, pair positive/negative bars with signed values in tooltips.
- All charts need a visually-hidden data table alternative.
- Full keyboard path through login: tab order email → password → remember → submit. Enter submits.
- Focus visible everywhere: 2px `--accent` outline, 2px offset.
- Respect `prefers-reduced-motion` — skip chart draw-in animations.

---

## 7. Build order

1. Tokens + `<Card>`, `<Button>`, `<TextField>`
2. Login screen (static) → wire auth → session middleware
3. `<PeriodTabs>` + dashboard shell with skeleton cards
4. Charts, one at a time, against mock data
5. Replace mocks with real endpoints
6. Responsive + a11y pass