# Taruna Eduka — Design System & Dashboard Guide

Version: 0.1 · Extracted from the current mockup on 2026-10-10 · Frontend baseline: `a928a42`

This document describes the visual language already implemented in `web/` and how to carry it into new dashboards. It is a design and implementation reference, not a redesign or proof that the mockup is production ready.

**Observed** means present in source or checked in the rendered UI. **Guideline** means a rule for building new screens; it may require work that the mockup does not yet implement. **Gap** means an observed limitation to resolve during implementation. Product behavior and permissions remain defined by the [architecture](ARCHITECTURE.md), [reconciliation](RECONCILIATION.md), and [delivery plan](PLAN.md).

## 1. Visual direction

The mockup combines a cool neutral workspace, compact controls, quiet cards, and an indigo–violet gradient. Its signature element is the countdown/progress ring. Operational pages use restrained color so status, numbers, and actions are easy to scan.

Guidelines:

- Use neutral surfaces for the shell, cards, forms, and tables. Let hierarchy come from position, spacing, type, and contrast.
- Use indigo for the main action, links, selected navigation, and active progress. Use violet as the supporting gradient endpoint.
- Reserve green, amber, and red for meaningful state. Pair every status color with a readable label.
- Keep dashboards task focused: what is happening, what needs attention, and where to act.
- Keep the stronger gradients, background grid, and large display typography of the landing page in marketing contexts. Dashboard cards do not need hero decoration.
- Reuse the current components before introducing another visual pattern.

## 2. Color system

### 2.1 Canonical tokens

[app/globals.css](../web/app/globals.css) is the project source of truth. OKLCH values below are exact source values; hexadecimal values are approximate sRGB references sampled from the rendered browser. Use semantic tokens in code rather than copying hex values. Out-of-gamut colors may be clipped in sRGB.

| Token | Light: OKLCH → hex | Dark: OKLCH → hex | Role |
| --- | --- | --- | --- |
| `background` | `0.9812 0.0025 258` → `#F8F9FA` | `0.1791 0.0075 264` → `#101115` | Page canvas |
| `foreground` | `0.2242 0.019 267` → `#181B25` | `0.9481 0.0025 258` → `#EDEEEF` | Main text |
| `surface` | `1 0 0` → `#FFFFFF` | `0.2242 0.0075 264` → `#1A1C1F` | Cards, sidebar |
| `surface-secondary` | `0.965 0.0035 258` → `#F2F3F6` | `0.2603 0.0075 264` → `#222428` | Table headers, hover, secondary areas |
| `surface-tertiary` | `0.9425 0.0045 258` → `#EAECEF` | `0.2965 0.0075 264` → `#2B2D31` | Stronger neutral inset |
| `overlay` | `1 0 0` → `#FFFFFF` | `0.2413 0.0075 264` → `#1E2023` | Popovers and dialogs |
| `muted` | `0.535 0.017 264` → `#686D77` | `0.635 0.012 258` → `#868B92` | Supporting text |
| `default` | `0.945 0.004 258` → `#EBEDEF` | `0.2965 0.0065 264` → `#2B2D30` | Neutral controls and tracks |
| `accent` | `0.549 0.2116 265.6` → `#3562EA` | `0.652 0.199 265.6` → `#5586FF` | Primary action and links |
| `accent-2` | `0.596 0.196 293.7` → `#885FE6` | `0.688 0.183 293.7` → `#A27FFF` | Gradient endpoint |
| `accent-foreground` | `0.99 0 0` → `#FCFCFC` | `0.1 0 0` → `#030303` | Text on solid accent |
| `success` | `0.6534 0.1567 156.7` → `#00AC67` | Inherits light value → `#00AC67` | Completed / confirmed |
| `success-foreground` | `0.2242 0.019 267` → `#181B25` | Inherits light value → `#181B25` | Text on solid success |
| `warning` | `0.7723 0.1585 73.7` → `#F0A31B` | `0.8022 0.1451 76.6` → `#F3B040` | Pending / attention |
| `warning-foreground` | `0.2242 0.019 267` → `#181B25` | `0.16 0.01 267` → `#0B0D12` | Text on solid warning |
| `danger` | `0.58 0.2116 20.6` → `#DB2740` | `0.671 0.1988 22.5` → `#F8555A` | Error / destructive / urgent |
| `danger-foreground` | `0.99 0 0` → `#FCFCFC` | `0.1 0 0` → `#030303` | Text on solid danger |
| `border` | `0.9091 0.0055 258` → `#DFE1E5` | `0.2965 0.0075 264` → `#2B2D31` | Component boundaries |
| `separator` | `0.928 0.0045 258` → `#E5E7EA` | `0.2684 0.0075 264` → `#24262A` | Quieter separators |

Each numeric triplet is wrapped in `oklch(...)` in CSS. Do not infer a separate dark success color: the project currently leaves both success tokens inherited.

### 2.2 Aliases and derived colors

| Token / utility | Existing definition | Usage |
| --- | --- | --- |
| `surface-foreground`, `overlay-foreground`, `default-foreground`, `segment-foreground` | Alias `foreground` | Text on the corresponding surface |
| `surface-secondary-foreground`, `surface-tertiary-foreground` | Alias `foreground` | Secondary surface text |
| `field-background` | Light `surface`; dark `surface-secondary` | Inputs |
| `field-foreground`, `field-placeholder`, `field-border` | `foreground`, `muted`, `border` | Field content and chrome |
| `segment` | Light white; dark `surface-tertiary` | Segmented controls |
| `focus`, `link` | Alias `accent` | Focus indication and links |
| `ring-start`, `ring-end` | Same values as `accent`, `accent-2` | Gradient identity |
| `backdrop` | Light `oklch(0.2 0.01 267 / 45%)`; dark `oklch(0 0 0 / 65%)` | Modal / drawer dimming |
| `bg-gradient-ring`, `text-gradient` | `linear-gradient(135deg, var(--ring-start), var(--ring-end))` | Supporting brand emphasis |
| `bg-grid` | 1px `border` lines every 32px | Landing background |
| Text selection | Accent at 30% mixed with transparent | Selected text |

HeroUI supplies `*-soft`, `*-soft-foreground`, and hover tokens. These are theme-dependent color mixes, not additional hardcoded palettes. For example, `accent-soft` is accent at 15% against transparent in light mode and 12% in dark mode. Its appearance changes with the surface beneath it. Use `bg-accent-soft text-accent-soft-foreground` together; do not substitute solid-accent text colors on a soft background.

### 2.3 Status and category mapping

| Meaning | Existing treatment |
| --- | --- |
| Order pending | Warning soft background + warning soft foreground |
| Order confirmed | Success soft pair |
| Order cancelled | Danger soft pair |
| Session in progress | Accent soft pair |
| Session completed | Success soft pair |
| Difficulty easy / medium / hard | Success / warning / danger soft pair |
| Not started in student donut | Neutral `default` |
| Question type | Secondary chip with border |

Program chips are category colors, not status: SD uses sky, SMP cyan, SMA accent, CASN amber, BUMN emerald, AKPOL rose. The Tailwind families use a `500/12` background and `700` light / `300` dark text; SMA uses the accent soft tokens. Unknown program codes fall back to neutral. Always retain the code label.

Guideline: a rising number is not inherently positive. Pending orders increasing, for example, should not become green simply because the arrow points up. The current `StatCard.delta` maps up to success and down to danger; supply a delta only when that interpretation is correct.

## 3. Typography

Fonts are loaded through `next/font/google` in [app/layout.tsx](../web/app/layout.tsx). Keep this font set.

| Family | Token / class | Intended role |
| --- | --- | --- |
| Inter | `--font-sans` / `font-sans` | Body, navigation, controls, table content |
| Geist | `--font-display` / `font-display` | Display headings and prominent KPI values |
| Geist Mono | `--font-mono` / `font-mono` | Codes, timers, percentages, compact score figures |

Observed type scale at a 16px root:

| Role | Size / usual line height | Weight / treatment |
| --- | --- | --- |
| Chart labels and small ring percentage | 10px / inherited 1.5 unless overridden | Mono; supporting detail |
| Table header | 11px / inherited 1.5 | 500, uppercase, wide tracking |
| Description, badge delta, metadata | 12px / 16px (`text-xs`) | Usually 400; delta 500 |
| Form label | 13px / inherited 1.5 | Inter 500 |
| Body, nav, table, panel title | 14px / 20px (`text-sm`) | Body 400; nav 500; panel title 600 |
| Shell page title | 15px / inherited 1.5 | Inter 600, truncated |
| Modal title / large button | 16px / 24px (`text-base`) | Modal 600; button 500 |
| KPI value | 24px / 32px (`text-2xl`) | Geist 600, tight tracking |
| Landing hero title | 36px / 40px; 48px / 48px at `sm` | Geist 600, tight tracking |

Guidelines:

- Keep dashboard titles compact; reserve hero-sized type for marketing.
- Use 14px as the normal dashboard reading size. Keep 10–12px for secondary metadata; essential chart values must also be available in readable text.
- Use sentence case for actions and headings. Uppercase is reserved for short table headers and program codes.
- Keep monetary values, counts, and identifiers distinct. Use existing `formatIDR`, `formatDate`, and `formatDateTime` conventions rather than manually assembled strings.
- Use `—` for an unavailable metric such as an average with no results. Zero is a real measured value, not a substitute for missing data.
- Truncate names only when the full value remains available in the detail view or another accessible presentation. Do not truncate money or critical status.
- Use tabular numerals where changing values should align; this is guidance for new metrics, not a global setting already present in the mockup.

## 4. Spacing, shape, and elevation

### 4.1 Spacing rhythm

Use the existing 4px spacing rhythm, with small 2px increments for compact internals.

| Size | Existing examples |
| --- | --- |
| 2px | Nav item gap; KPI label margin |
| 4px | Small inline gaps; modal description margin |
| 6px | Label-to-input gap; compact chip padding |
| 8px | Icon/text gap; wrapped quick-action gap |
| 10px | Nav horizontal inset; table header vertical padding |
| 12px | Nav icon/text gap; table cell vertical padding |
| 16px | Grid gap; table cell horizontal padding; standard form-body gap |
| 20px | Dashboard card padding (`p-5`) |
| 24px | Section separation (`mt-6`); desktop horizontal page padding |
| 32px | Desktop vertical page padding |
| 56px | Empty-state vertical padding |

Guideline: related controls use 8–12px gaps, component content uses 16px gaps, and major dashboard sections use 24px separation. Follow the existing page pattern instead of adding arbitrary margins to each child.

### 4.2 Corner radii

The project base radius is **6px** (`--radius: 0.375rem`). HeroUI radii are multiples of that base; its library comments describe another base and should not be copied as project measurements.

| Class / token | Effective radius | Observed use |
| --- | --- | --- |
| `rounded-sm` | 3px | Small details |
| `rounded-md` | 4.5px | Chips, deltas, chart corners |
| `rounded-lg` | 6px | Buttons, nav items, result links |
| `rounded-xl` | 9px | Tables, empty state; default field radius |
| `rounded-2xl` | 12px | Available scale step |
| `rounded-3xl` | 18px | HeroUI card default |
| `rounded-4xl` | 24px | Available scale step |
| `rounded-full` | Pill / circle | Avatars, status dots, order filter pills |

Project CSS explicitly tightens `.button` to 6px and `.chip` to 4.5px. Order filter pills are an observed exception for selection controls; they do not change the default action-button shape.

### 4.3 Borders and shadows

- Tables and many inset rows use a 1px `border` boundary. Use `separator` for quieter divisions.
- Cards use `shadow-surface`; overlays use `shadow-overlay`. Use these tokens instead of new shadows per page.
- The inherited light surface shadow is `0 2px 4px rgba(0,0,0,.04), 0 1px 2px rgba(0,0,0,.06), 0 0 1px rgba(0,0,0,.06)`.
- Dark cards are distinguished mainly through surface color. Do not add bright outlines to every card to simulate light-mode elevation.
- HeroUI's default theme sets `--field-border-width: 0px`; the project overrides it to `1px` so fields stay visible on same-color cards. Verify input boundaries and focus in both themes before shipping a form.

## 5. Responsive layout and application shell

### 5.1 Breakpoints

| Breakpoint | Width | Current behavior |
| --- | --- | --- |
| Base | Below 640px | One-column KPI stack; mobile navigation drawer |
| `sm` | 640px | Two-column KPI grid; shell description shown |
| `md` | 768px | Compact desktop button heights; public navigation links shown |
| `lg` | 1024px | Persistent sidebar; four KPI columns; dashboard insight columns |
| `xl` / `2xl` | 1280px / 1536px | Available Tailwind defaults; no extra dashboard layout required |

These are CSS viewport widths, not device classifications. Test intermediate widths as well as named breakpoints.

### 5.2 Shell specification

Reuse [AppShell](../web/components/app-shell.tsx) for authenticated operational and student pages.

| Region | Observed specification |
| --- | --- |
| Expanded sidebar | 256px, sticky, viewport height, `surface`, right border |
| Collapsed sidebar | 68px; icon navigation |
| Sidebar logo row | 56px high; 14px horizontal inset |
| Sidebar navigation | 8px container inset; items 10px horizontal / 8px vertical; 18px icons |
| Selected navigation | Accent soft background/text plus a 2px gradient left indicator |
| Mobile drawer | Left placement; 288px set on `Drawer.Dialog` (measured); closes after navigation |
| Header | Sticky; 56px; `z-30`; `surface/85`, blur, bottom border |
| Header padding | 16px base; 24px at `lg` |
| Main padding | 16px horizontal / 24px vertical; 24px / 32px at `lg` |
| Student content | Centered `max-w-6xl` = 1152px |
| Admin overview | Uses the available content width |
| Focused builder | Existing tryout builder uses `max-w-3xl` = 768px |

The header holds title, optional description, page actions, language, and theme controls. Use its `actions` slot for a page-wide primary action. Keep account controls in the sidebar/user menu.

Guidelines:

- Give flexible content a shrinking boundary (`min-w-0` where needed). Put wide tables inside their own horizontal scroll region.
- Keep title and core actions usable when descriptions disappear below `sm`. Avoid more header actions than fit at 320–390px.
- Use one-column cards on small screens; keep reading order equal to DOM order.
- For dense tables, preserve necessary columns with internal scrolling or use a deliberate mobile list representation. Do not squeeze six columns into unreadable cells.
- Let legends and action groups wrap or stack. Ensure drawers and dialogs fit the viewport with safe edge spacing.
- A collapsed sidebar still needs accessible names and discoverable labels for icon-only items.

The shell's main column carries `min-w-0` and `DataTable`'s scroll region is `relative`, so wide tables (including absolutely positioned descendants) scroll inside the table. All shell routes were measured without page-level overflow at 320/390/768/1024/1440px. Treat page-level horizontal scrolling as a failed responsive acceptance check.

## 6. Component specifications

### 6.1 Buttons and links

Use HeroUI `Button` for actions and [LinkButton](../web/components/ui/link-button.tsx) for navigation styled as a button. A plain text link is appropriate for “Lihat semua” and other secondary drill-down navigation.

| Variant | Existing treatment | Guideline |
| --- | --- | --- |
| `primary` | Solid accent; accent foreground | Main action for the current task |
| `secondary` | Neutral default fill; accent soft foreground | Supporting navigation/action |
| `tertiary` | Neutral default fill; inherited foreground | Lower-emphasis action when needed |
| `ghost` | Transparent; neutral hover | Toolbar actions, dismiss, theme/language |
| `outline` | Transparent; 1px border | Secondary action requiring a boundary |
| `danger` | Solid danger | Destructive action; verify contrast |
| `danger-soft` | Danger soft pair | Lower-emphasis destructive action |

| Size | Height below `md` | Height at `md` and above | Text / horizontal padding |
| --- | --- | --- | --- |
| `sm` | 36px | 32px | 14px / 12px |
| Default / `md` | 40px | 36px | 14px / 16px |
| `lg` | 44px | 40px | 16px / 16px |

Buttons have 6px corners, weight 500, and an 8px icon/text gap. Icon-only buttons are square at the corresponding height. Library icons are normally 20px below `sm` and 16px from `sm`; small-button icons are 16px, with explicit project icon sizes taking precedence.

States: use library hover, pressed, disabled, pending, and focus behavior. Pressed scale is approximately `.97` (`sm` `.98`, `lg` `.96`); color transitions are 100ms and transform 250ms. Disabled opacity defaults to `.5`.

Guidelines:

- Keep one dominant action in a task region; use verb + object labels such as “Simpan tryout” or “Konfirmasi pembayaran”.
- Give icon-only controls localized accessible names. A tooltip supports discovery but does not replace the name.
- During a mutation, prevent duplicate submission, retain a readable action label, and show pending state until the outcome is known.
- `LinkButton` supplies navigation and visual variants; it does not implement an async mutation, disabled navigation, or authorization.
- Prefer a 44px interactive area for important mobile controls. The current small sizes are an observed baseline, not the preferred touch target for every new screen.

### 6.2 Cards and KPI cards

HeroUI `Card` uses `surface`, an 18px radius, and surface shadow. The library base has 16px padding / 12px gap; dashboard pages commonly override to **20px padding / 16px gap**.

[StatCard](../web/components/stat-card.tsx) contains a 36px accent-soft icon box, 18px icon, 24px Geist value, 14px muted label, and optional 12px soft delta. An optional ring is 36px with a 4px stroke and 10px percentage.

Guidelines: label each KPI with unit and scope; show its period when applicable. Avoid fabricated comparison deltas or progress percentages without a denominator. The mockup's student average uses `—` when there are no results. Keep long currency values intact and allow the card to grow.

### 6.3 Forms and validation

Reuse [form wrappers](../web/components/ui/form.tsx): `TextInput`, `TextareaInput`, `SelectInput`, and `SwitchInput`.

- Labels: 13px, weight 500, foreground. Field stack gap: 6px.
- Description: 12px muted. Error: 12px danger; an error replaces the description.
- `required` and `error` are passed to HeroUI semantics, not only rendered as color.
- Textarea defaults to four rows; select popover is at least the trigger width.
- Switch label/description sit beside the control; keep the whole label understandable.

Guidelines: visible labels are required for normal form fields. Use placeholders as examples, not labels. Group related fields with 16px gaps; use two columns from `sm` only when the labels and input content fit. Preserve entered values after server errors, put field errors next to fields, and provide a form-level message for failures that have no field. Use the right input type and autocomplete purpose.

The question editor is an observed specialized pattern: compact formatting toolbar, bordered textarea, LaTeX/image support, and rendered content preview. Reuse its editor components for question content instead of replacing it with a plain dashboard text field.

### 6.4 Tables, lists, and filters

[DataTable](../web/components/data-table.tsx) is a simple rendering component, not a complete data-grid system.

- Surface wrapper, 9px radius, 1px border, inner horizontal scroll container.
- 11px / 500 uppercase table headers, wide tracking, secondary background; 16px horizontal / 10px vertical padding.
- 14px body; 16px horizontal / 12px vertical padding; borders between rows.
- Clickable rows get a pointer and secondary hover background.
- Current component has no built-in search, sorting, pagination, loading, empty state, or error state.

Guidelines: place count, search/filter controls, and any primary action above the table. Make active filters visible and provide a clear reset. Right-align comparable numeric columns when useful. Keep IDs in mono and secondary styling. With `onRowClick`, rows are focusable, show a focus ring, and open on Enter/Space; explicit links/buttons in rows are still preferred for multiple row actions.

Order status filters are rounded-full custom buttons with 14px medium labels, 14px horizontal / 6px vertical padding, accent fill when selected, and neutral border when inactive. For new filter controls, expose the selection state semantically and support keyboard activation.

Student result rows are bordered 6px-radius links with 14px horizontal / 10px vertical padding, program badge, truncated title, score, and metadata. Dates hide below `sm`; core identity and score stay visible.

### 6.5 Chips and icons

Reuse the [status badge components](../web/components/status-badge.tsx) so status translation and color mapping stay consistent. Chips use `size="sm"`, weight 500, and the project 4.5px radius.

Icons use Lucide with a common 16–18px control size and stroke width around 2. Empty-state icons use 20px / 1.75 stroke. Decorative icons should be hidden from assistive technology; actionable icons need names. Do not use the same checkmark to imply both payment confirmation and exam correctness without a label.

### 6.6 Dialogs and drawers

[FormModal](../web/components/ui/form-modal.tsx) provides the shared modal structure: backdrop, internally scrolling body, title, optional description, optional footer. Title is 16px / 600; description 14px muted; body gap 16px. Sizes are `xs`, `sm`, `md`, `lg`, `xl`, and `full`; the project `xl` maps to library `lg` chrome with a 1024px maximum dialog width.

Guidelines: use a modal for a bounded edit, review, or confirmation; use a full page for long builders. Keep dismissal and submission clear. For irreversible actions, state the affected object and consequence. Verify focus entry, Escape/dismiss behavior, focus containment, and return focus in the actual flow. Keep footer actions reachable when content scrolls. Existing question-picker and navigation drawers are suited to side-by-side selection and navigation.

Drawer width belongs on `Drawer.Dialog` for HeroUI v3; the navigation drawer follows this and measures 288px when open.

### 6.7 Empty states, errors, and feedback

[EmptyState](../web/components/empty-state.tsx): dashed 1px border, 9px radius, 56px vertical padding, centered 12px gap, 44px neutral icon circle, 14px title/description, optional action. Description width is capped at 384px.

Guideline: name why the area is empty and offer the relevant next action. “No purchases yet”, “No matches for these filters”, and “Could not load purchases” require different messages and actions.

Toasts are provided globally at `bottom end`. Use them for concise action outcomes; put durable errors or required decisions near the relevant content. Existing local mock mutations and success toasts do not prove that a server has persisted the action.

### 6.7a Package elements

[ItemElementView](../web/components/admin/item-element.tsx) draws one item-package element the way a student will see it: a symbol as text, an emoji from the bundled Twemoji SVG (`/emoji/<id>.svg`, never the device font), a built-in flat shape as an inline `currentColor` SVG, an image through a short-lived presigned asset link. It is used by the package grid, the add dialog's catalog and the sample column. Wherever the emoji catalog is shown, the CC-BY 4.0 attribution text from `GET /item-packages/catalog` appears too. The package add dialog follows the question import dialog: input, preview table with per-element reason, then confirm.

### 6.8 Charts and countdown rings

| Component | Observed dimensions / behavior | Guideline |
| --- | --- | --- |
| `BarChart` | 160px area; 10px bar gaps; gradient bars; 10px mono labels; hover values | Compare categories; include readable values outside hover |
| `DonutStat` | 140px diameter; 16px stroke; 20px legend gap; 12px labels/counts | Show a small set of part-to-whole statuses; always retain counts |
| `Sparkline` | Default 160×40px; 2px accent line; fading fill | Supplement a KPI with a trend, not an unexplained standalone graph |
| `CountdownRing` | Default 120px; 8px stroke; clamps value to 0–100; rounded ends | Show explicit time/score/progress text and what it represents |
| `KecermatanItem` | Inline; text kinds read as one mono string, emoji/shape/image draw each element at `size-5` via `ItemElementView` | One kecermatan reference or shown item in the attempt and the review; never show a missing-item marker |

The ring uses `ring-start` → `ring-end`; `urgent` changes the stroke to danger. The caller determines urgency. Product timer policy must come from the exam specification, not a new visual threshold invented per screen.

Zero renders as an empty bar; small positive values keep a 4% minimum so they stay visible. Bar values are always visible rather than hover-only, and donut legends wrap below the chart when space is narrow.

### 6.9 Branding and avatars

Use the existing Taruna Eduka image assets through [Logo](../web/components/logo.tsx): full logo at 36px height; collapsed icon at 32px. Preserve aspect ratio and use legible assets for each surface. The current full logo is visually subdued on the dark sidebar; review a suitable dark-surface asset before release.

[GradientAvatar](../web/components/gradient-avatar.tsx) uses initials over a user-color gradient: 28px small, 36px medium, 48px large. Avatar and product-cover colors are decorative identity, not permission or status indicators.

## 7. How to build a dashboard

### 7.1 Define its job before its widgets

Write down the user role, main decision, next action, and data scope. For each KPI define its source, period, unit, and what “missing”, “zero”, and “stale” mean. Use permissions from the product architecture when selecting sections and actions.

| Dashboard | Existing mockup content | Decision the layout should support |
| --- | --- | --- |
| Admin | Students, pending orders, active tryouts, confirmed revenue; program distribution, order status, live-session callout, six recent orders | Find operational work and inspect activity |
| Operator | Shared admin overview; live sessions replace revenue in the fourth KPI | Monitor and act within permitted operations |
| Student | Owned/completed tryouts, average score, balance; tryout status, up to five recent results, quick links | Continue learning and reach owned content/results |

The demo role switcher is a mockup aid, not a production authorization control. Platform-wide admin and tenant-selection interfaces described in the architecture are future scope; the current mockup does not establish their finished design.

### 7.2 Compose the page in a consistent order

1. **Shell:** concise title, optional subtitle, relevant page actions.
2. **Summary:** the relevant KPIs. Current baseline is four cards with `grid gap-4 sm:grid-cols-2 lg:grid-cols-4`; use fewer when that is the actual scope.
3. **Explanation:** charts or status summaries that help interpret those KPIs. Current baseline uses `mt-6 grid gap-4 lg:grid-cols-3` with one panel spanning two columns.
4. **Attention / next action:** an actionable callout when there is work to do, such as the existing live-session monitoring link.
5. **Work / activity:** recent records, a queue, or result links with a path to the full list.

Admin currently puts a two-column program chart beside a one-column status donut, then a live callout and recent-orders table. Student puts a one-column status donut beside a two-column results list, then wrapped quick links. Preserve the role's task instead of giving every dashboard identical charts.

Guideline: keep a single clear visual priority per region. If an urgent queue is the user's primary job, place it before secondary analysis. This is a future composition choice, not the current overview order.

### 7.3 Use the existing building blocks

Minimal layout pattern using current APIs (illustrative; data is supplied by the page):

```tsx
<AppShell title={t.title} description={t.description}>
  <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
    {metrics.map((metric) => (
      <StatCard key={metric.id} {...metric.cardProps} />
    ))}
  </div>
  <div className="mt-6 grid gap-4 lg:grid-cols-3">
    <Card className="gap-4 p-5 lg:col-span-2">{mainInsight}</Card>
    <Card className="gap-4 p-5">{statusSummary}</Card>
  </div>
  <section className="mt-6">{activity}</section>
</AppShell>
```

Import `AppShell` / `StatCard` from `@/components/...` and `Card` from `@heroui/react`. Compose page-specific content directly; do not add a generic dashboard framework for one screen. Use semantic colors such as `bg-surface`, `text-muted`, and `border-border`; central token changes belong in `app/globals.css`.

### 7.4 Extend to other page families

| Page family | Existing starting point | Building guidance |
| --- | --- | --- |
| Management lists | Admin orders, programs, users, questions | Shell + count/filter/action row + table + bounded detail/edit modal |
| Builders | Tryout / kecermatan builders, question editor | Focused width, grouped fields, ordered sections, preview; keep save action clear |
| Monitoring | Admin monitoring | Emphasize status, current session context, and permitted intervention; include freshness and disconnected states in live integration |
| Catalog / owned content | Catalog, student tryouts | Product cards, program labels, relevant entitlement/action; existing catalog grid uses 20px gaps and 1/2/3 columns |
| Wallet / orders / profile | Student pages | Clear balance or record summary, readable history, scoped form sections; avoid unnecessary charts |
| Exam attempt | Tryout subtest and kecermatan attempt | Focused question workspace, timer/progress, answer navigation, clear submission; preserve question content and critical controls on mobile |
| Results | Tryout and kecermatan results | Score hierarchy, understandable status, breakdown, review content; distinguish absent results from a zero score |
| Public / auth | Landing, login, register, checkout | Public navigation or focused form layout; reuse tokens and controls while preserving their different task |

Use the richer landing visual treatment only where discovery/marketing is the goal. Detailed timing, ranking, payment, and entitlement rules remain product requirements.

### 7.5 Specify data states before connecting the API

These are required design considerations for new integration; most are not supplied by the present mock-data components.

| State | Dashboard treatment |
| --- | --- |
| Initial loading | Preserve layout with matching skeletons or an explicit loading region; do not briefly show fake zeros |
| Loaded | Show data with the defined unit, period, and scope |
| Empty | Explain absence and offer the next action |
| Filtered empty | Keep filters visible; explain no matches and offer reset |
| Failure | Explain what could not load; show the API message and `request_id` when present; provide retry without discarding unrelated successful content |
| Partial / stale | Keep usable data visible; mark the affected region and last update when relevant |
| Mutation pending | Prevent duplicate action; show pending state in the initiating control |
| Mutation failed | Preserve context and input; show a useful local error |
| No permission | Present the allowed route/action set; do not imply hidden data is simply zero |

## 8. Accessibility, motion, and language

### 8.1 Acceptance guidance

- Normal text needs at least 4.5:1 contrast; qualifying large text needs 3:1. Check actual foreground/background pairs, including hover, placeholder, and soft states. [W3C contrast guidance](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html).
- Necessary graphical details and visual control identification need 3:1 contrast against adjacent colors, subject to the criterion's exceptions. Decorative card borders are a different case. [W3C non-text contrast guidance](https://www.w3.org/WAI/WCAG22/Understanding/non-text-contrast.html).
- WCAG 2.2 AA target-size minimum is 24×24 CSS px, with spacing and other exceptions. Prefer 44px areas for important touch actions as a project usability guideline. [W3C target-size guidance](https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html).
- Every action must be keyboard reachable with visible focus. Verify custom row actions, filters, collapsed navigation, drawer, and modal flows separately from library controls.
- Use headings to express page/section hierarchy. Charts need a readable summary or equivalent values; color and animation alone cannot communicate status.
- Check zoom, long names, long translations, and narrow viewports. Keep focus and validation messages visible around sticky regions.

Token spot checks (computed from OKLCH): primary light text/fill ≈ 4.98:1; light solid danger with its foreground ≈ 4.64:1; solid success with its dark foreground ≈ 5.77:1 in both themes. These are not measurements of the soft status badges and are a limited spot check, not a full accessibility audit.

### 8.2 Motion

Observed: sidebar width transition 200ms; button transitions 100–250ms; bar entrance 500ms with 40ms stagger; ring transition 700ms. The live-session indicator uses a ping animation. HeroUI buttons include reduced-motion handling; framer-motion (charts, ring) follows it through `MotionConfig reducedMotion="user"` in providers, and the live ping uses `motion-reduce:animate-none`.

Guidelines: respect `prefers-reduced-motion` for custom motion, avoid repeated animation on routine data refresh, and keep urgent information readable without pulsing. Animate only to clarify change or orientation. Timer readability and accuracy matter more than entrance effects.

### 8.3 Themes and localization

Observed: light is the default, automatic system-theme selection is disabled, and the class-based theme can be toggled manually. Indonesian is the default locale; Indonesian/English dictionaries are under `lib/i18n/dict/`.

Guidelines: add user-facing strings to the appropriate dictionary; test both languages with the same layout. Localize accessible names, validation, status, and empty-state copy as well as headings. Keep currency/date formatting deliberate. Theme changes should alter tokens, not per-page hex overrides. The root document language is server-rendered as `id` and synchronized to the selected locale on the client.

## 9. Future tenant branding boundary

The architecture plans tenant-scoped branding; the present mockup uses one Taruna Eduka identity. The following are implementation guidelines, not implemented tenant UI:

- Tenant logo/name and approved accent colors may vary through validated configuration.
- Keep semantic success/warning/danger meaning, fonts, spacing, radii, interaction behavior, and accessible contrast consistent.
- Define both light and dark foreground/background pairs before applying a tenant accent. One supplied hex color is not a complete theme.
- Show active tenant context where users can change tenant; do not rely on a logo or color alone.
- Keep tenant branding separate from role/permission indicators and data scope. A theme must not imply access.

## 10. Review checklist

- [ ] The page has a defined role, task, primary action, and data scope.
- [ ] Existing shell, components, token names, and font roles are reused.
- [ ] KPIs have correct labels, units, periods, missing states, and meaningful deltas.
- [ ] Loading, empty, failure, partial/stale, pending, and permission states are specified where applicable.
- [ ] At 320px, 390px, 768px, 1024px, and 1440px, content is readable and the document does not overflow horizontally.
- [ ] Wide table scrolling is confined to the table region; chart legends and actions fit or wrap.
- [ ] Indonesian/English, long labels, zoom, and large values fit without hiding essential information.
- [ ] Light/dark colors are checked as actual pairs, including destructive controls and focus.
- [ ] Keyboard traversal, icon names, row details, filters, modal/drawer focus, and dismissal work.
- [ ] Touch targets and custom reduced-motion behavior are checked.
- [ ] Success feedback follows the confirmed persistence outcome; API failure preserves useful context.
- [ ] Screenshot review covers representative desktop/mobile and light/dark states; remaining deviations are recorded.

## 11. Source map and verification boundary

| Concern | Source |
| --- | --- |
| Tokens, project overrides, gradients | [globals.css](../web/app/globals.css) |
| Fonts / document language | [layout.tsx](../web/app/layout.tsx) |
| Theme / toast setup | [providers.tsx](../web/app/providers.tsx) |
| Shell / navigation | [app-shell.tsx](../web/components/app-shell.tsx), [nav.ts](../web/lib/nav.ts) |
| KPI / chart / progress | [stat-card.tsx](../web/components/stat-card.tsx), [charts.tsx](../web/components/charts.tsx), [countdown-ring.tsx](../web/components/countdown-ring.tsx) |
| Tables / status / absence | [data-table.tsx](../web/components/data-table.tsx), [status-badge.tsx](../web/components/status-badge.tsx), [empty-state.tsx](../web/components/empty-state.tsx) |
| Fields / dialog / action links | [form.tsx](../web/components/ui/form.tsx), [form-modal.tsx](../web/components/ui/form-modal.tsx), [link-button.tsx](../web/components/ui/link-button.tsx) |
| Branding | [logo.tsx](../web/components/logo.tsx), [gradient-avatar.tsx](../web/components/gradient-avatar.tsx) |
| Admin / student examples | [admin dashboard](../web/app/admin/page.tsx), [student dashboard](../web/app/student/page.tsx), [orders](../web/app/admin/orders/page.tsx) |
| Public / catalog / builder | [landing](../web/app/page.tsx), [catalog](../web/app/catalog/page.tsx), [tryout builder](../web/components/admin/tryout-builder.tsx), [rich-text editor](../web/components/admin/rich-text-editor.tsx) |
| Locale / demo role | [locale-context.tsx](../web/lib/i18n/locale-context.tsx), [session-context.tsx](../web/lib/session-context.tsx) |

Extraction also consulted the installed HeroUI 3.2.1 styles for component defaults and Tailwind's breakpoint/type scale. Project overrides take precedence over library defaults. Library behavior should be rechecked after a dependency upgrade.

Rendered review covered admin light/dark at 1440px, admin light at 390px, student light at 1440px/390px, and orders light at 1440px. Colors, button sizes, fonts, and document widths were sampled from the running existing build. Other page families and components were inspected in source; their full interactive states were not browser-tested in this documentation task. No UI implementation was changed.
