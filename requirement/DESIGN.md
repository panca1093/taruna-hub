# Taruna Eduka — Design System & Dashboard Guide

Version: 0.1 · Extracted from the current mockup on 2026-10-10 · Frontend baseline: `a928a42`

This document describes the visual language already implemented in `web/` and how to carry it into new dashboards. It is a design and implementation reference, not a redesign or proof that the mockup is production ready.

**Observed** means present in source or checked in the rendered UI. **Guideline** means a rule for building new screens; it may require work that the mockup does not yet implement. **Gap** means an observed limitation to resolve during implementation. Product behavior and permissions remain defined by the [architecture](ARCHITECTURE.md), [reconciliation](RECONCILIATION.md), and [delivery plan](PLAN.md).

## 1. Visual direction

The workspace combines a warm cream canvas, plum ink, compact controls, quiet cards and a rose accent (owner decision 2026-10-31, handoff `PALETTE-DECISION.md`). Student, admin and auth share these tokens; a dark plum sidebar anchors both shells. Its signature element is the countdown/progress ring, now plum to rose. Operational pages use restrained color so status, numbers, and actions are easy to scan.

**Visual parity pass (2026-10-31).** The workspace now follows the handoff prototype (`docs/handoffs/2026-10-31-student-workspace/prototype`) for shell, type, radii, shadows, icons and component anatomy: a floating rounded plum icon rail, a borderless 82px topbar, Geist display headings at weight 550 with negative tracking, Inter 14px body, 10/16/21/23px radii, a soft low-alpha shadow, and lucide icons at stroke 1.65. Where the prototype and older text in this file disagree on looks, the prototype wins. The marketing landing stays on `.palette-classic` and is opted out of every change below (type base, radii, component restyle, icon stroke).

Guidelines:

- Use warm neutral surfaces for the shell, cards, forms, and tables. Let hierarchy come from position, spacing, type, and contrast.
- Use the plum/rose `accent` for the main action, links, selected navigation, and active progress. `accent-2` is the lighter rose ring endpoint.
- Reserve green, amber, and red for meaningful state. Pair every status color with a readable label. Rose is identity, never a status color.
- Keep dashboards task focused: what is happening, what needs attention, and where to act.
- Keep the stronger gradients, background grid, and large display typography of the landing page in marketing contexts. Dashboard cards do not need hero decoration.
- Reuse the current components before introducing another visual pattern.

## 2. Color system

### 2.1 Canonical tokens

[app/globals.css](../web/app/globals.css) is the project source of truth. OKLCH values below are exact source values; hexadecimal values are the handoff palette references they were converted from (`warm` / `dark plum` notes mark derived in-between tones). Use semantic tokens in code rather than copying hex values. Out-of-gamut colors may be clipped in sRGB.

| Token | Light: OKLCH → hex | Dark: OKLCH → hex | Role |
| --- | --- | --- | --- |
| `background` | `0.964 0.007 53.4` → #F7F2EF | `0.219 0.011 312` → #1C191E | Page canvas |
| `foreground` | `0.333 0.021 342.4` → #3E3239 | `0.955 0.012 342.6` → #F6EDF2 | Main text |
| `surface` | `0.995 0.005 78.3` → #FFFDFA | `0.268 0.014 310.8` → #28242B | Cards, header, forms |
| `surface-secondary` | `0.974 0.006 60` → warm off-white | `0.3 0.015 312` → dark plum | Table headers, hover, secondary areas |
| `surface-tertiary` | `0.945 0.009 30` → warm inset | `0.33 0.017 314` → dark plum inset | Stronger neutral inset |
| `overlay` | `0.995 0.005 78.3` → #FFFDFA | `0.285 0.014 311` → dark plum | Popovers and dialogs |
| `muted` | `0.533 0.026 341.9` → #786771 | `0.753 0.021 335.8` → #B8AAB4 | Supporting text |
| `default` | `0.94 0.01 10` → warm neutral | `0.34 0.018 320` → dark plum neutral | Neutral controls and tracks |
| `accent` | `0.442 0.063 341.6` → #6B455D | `0.819 0.056 346.8` → #E0B6CB | Primary action and links (plum in light, pale rose in dark) |
| `accent-2` | `0.643 0.058 346.7` → #A87F94 | `0.715 0.07 341.1` → #C292B0 | Lighter rose ring endpoint |
| `accent-foreground` | `0.985 0.005 78` → cream | `0.2 0.02 335` → dark plum | Text on solid accent |
| `success` | `0.495 0.061 163.4` → muted prototype green | `0.814 0.064 153.3` → pale green | Completed / confirmed (`.palette-classic` keeps `#00AC67` with a dark ink) |
| `success-foreground` | `0.985 0.005 78` → cream | `0.2 0.02 155` → dark green ink | Text on solid success (light ink on the dark green, ≥4.5:1) |
| `warning` | `0.7723 0.1585 73.7` → `#F0A31B` | `0.8022 0.1451 76.6` → `#F3B040` | Pending / attention |
| `warning-foreground` | `0.2242 0.019 267` → `#181B25` | `0.16 0.01 267` → `#0B0D12` | Text on solid warning |
| `danger` | `0.58 0.2116 20.6` → `#DB2740` | `0.671 0.1988 22.5` → `#F8555A` | Error / destructive / urgent |
| `danger-foreground` | `0.99 0 0` → `#FCFCFC` | `0.1 0 0` → `#030303` | Text on solid danger |
| `border` | `0.91 0.009 8.6` → #E7DFE0 | `0.363 0.022 326` → #443A44 | Component boundaries |
| `separator` | `0.925 0.008 8` → warm hairline | `0.32 0.02 324` → dark hairline | Quieter separators |
| `peach` | `0.878 0.031 54.7` → #E8D2C4 | `0.409 0.037 344` → #59424F | Warm tint of the primary practice card |
| `sidebar` | `0.286 0.018 333.8` → #30272E | `0.188 0.009 317.7` → #151216 | Dark plum rail (student and admin) |
| `sidebar-foreground` | `0.904 0.013 345.5` → #E6DCE1 | `0.929 0.014 336.2` → #EEE4EB | Text and icons on the rail |
| `sidebar-muted` | mix of foreground 62% / sidebar | same formula | Inactive rail items, tree connector |
| `sidebar-active` | foreground 13% over transparent | same formula | Selected / hovered rail item |
| `sidebar-accent` | `0.819 0.056 346.8` | `0.819 0.056 346.8` | Active indicator and focus ring on the rail |
| `sidebar-border` | foreground 14% over transparent | same formula | Rail dividers |

Each numeric triplet is wrapped in `oklch(...)` in CSS. Success, warning and danger keep their green, amber and red meaning in both themes. Do not infer a separate dark success color: the project currently leaves both success tokens inherited.

### 2.2 Aliases and derived colors

| Token / utility | Existing definition | Usage |
| --- | --- | --- |
| `surface-foreground`, `overlay-foreground`, `default-foreground`, `segment-foreground` | Alias `foreground` | Text on the corresponding surface |
| `surface-secondary-foreground`, `surface-tertiary-foreground` | Alias `foreground` | Secondary surface text |
| `field-background` | Light `surface`; dark `surface-secondary` | Inputs |
| `.palette-classic` | Scoped class restoring the pre-rose indigo values for `background`, `surface*`, `accent*`, `ring-*`, `border`, `separator`, `muted` | Opt-out for surfaces that must not be rebranded (public landing). Not used by the workspace. Also re-declares the `var()`-derived tokens (`*-foreground`, `field-*`, `focus`, `link`) so they resolve against the classic values. Portalled HeroUI overlays (popover, modal, dropdown) render outside the scope and keep the rose palette, so the landing must not open overlays that expect classic colors |
| `field-foreground`, `field-placeholder`, `field-border` | `foreground`, `muted`, `border` | Field content and chrome |
| `segment` | Light white; dark `surface-tertiary` | Segmented controls |
| `focus`, `link` | Alias `accent` | Focus indication and links |
| `ring-start`, `ring-end` | Same values as `accent`, `accent-2` (plum to rose) | Gradient identity |
| `backdrop` | Light `oklch(0.2 0.02 335 / 45%)`; dark `oklch(0 0 0 / 65%)` | Modal / drawer dimming |
| `bg-gradient-ring`, `text-gradient` | `linear-gradient(135deg, var(--ring-start), var(--ring-end))` | Supporting brand emphasis |
| `bg-grid` | 1px `border` lines every 32px | Landing background |
| Text selection | Accent at 30% mixed with transparent | Selected text |

HeroUI supplies `*-soft`, `*-soft-foreground`, and hover tokens. These are theme-dependent color mixes, not additional hardcoded palettes. For example, `accent-soft` is accent at 15% against transparent in light mode and 12% in dark mode. Its appearance changes with the surface beneath it. Use `bg-accent-soft text-accent-soft-foreground` together; do not substitute solid-accent text colors on a soft background.

**Text on tinted surfaces.** On `bg-peach` and `bg-accent-soft` (hero Panels, order and top-up status banners), `text-muted` falls below 4.5:1. Use `text-foreground/80` for body and caption text there. `text-muted` stays for plain `surface` and `background`.

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
| Tryout question / options (exam runtime) | 18px (`text-lg`) question, 16px (`text-base`) options | Inter 400; larger than dashboard body on client request |
| Kecermatan items (exam runtime) | Reference 20px (`text-xl`); soal 24px, 30px at `sm`; answer keys 24px | Geist Mono 800, one line per box (boxes grow from a min width); images 24px on phones, 28px / 36px from `sm` |
| Landing hero title | 36px / 40px; 48px / 48px at `sm` | Geist 600, tight tracking |

#### 3.1 Named type steps (prototype scale)

Defined in `globals.css` (`@theme static`). Use these utilities; never `text-[13px]`. Geist steps bake in weight 550 and tracking; pair them with `font-display`. Inter steps are plain sizes.

| Utility | Size / tracking | Prototype use |
| --- | --- | --- |
| `text-display` | 36px, -1.3px, lh 1.15 | Page h1 (student) |
| `text-display-sm` | 30px, -1px | Page h1 on mobile and in admin |
| `text-hero-lg` / `text-hero` | 29px (-0.9) / 27px (-0.8) | Entry cards, status banners, dialog titles |
| `text-heading` | 20px, -0.5px | Panel and section h2 |
| `text-heading-sm` | 18px, -0.3px | Section header, small panel title |
| `text-title` | 17px, lh 1.35 | Card h3, empty-state title |
| `text-metric-xl` / `-lg` / `text-metric` | 42px / 35px / 21px | Wallet amount, chart summary, metric strip |
| `text-score` | 70px, -3px | Result score |
| `text-brand` | 34px, -3px, 650 | The rail "T." mark |
| `text-small` | 13px | Descriptions, row text |
| `text-caption` | 12px | Buttons, table cells, form labels, menu rows |
| `text-label` | 11px | Breadcrumb, kicker, table header, chips |
| `text-micro` / `text-nano` | 10px / 9px | Hints, tags, rail labels |

Base rules (workspace only): `body` 14px / 1.5; `h1` = display, `h2` = heading, `h3` = title, all Geist 550. A `text-*` or `font-*` utility on the element overrides them.

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

The base `--radius` stays 6px for HeroUI internals. Workspace surfaces use these named radii (`@theme static`), taken from the prototype:

| Utility | Radius | Use |
| --- | --- | --- |
| `rounded-chip` | 7px | Chips, deltas, small tags, progress bars |
| `rounded-control` | 10px | Buttons, inputs, selects, icon tiles, menu rows |
| `rounded-tile` | 16px | Rail nav items, list rows, metric strip, tables, banners |
| `rounded-menu` | 19px | Dropdowns and popovers |
| `rounded-card` | 21px | Panels, product cards, `Card` |
| `rounded-hero` | 23px | Tinted hero cards, dialogs, mobile bottom nav |
| `rounded-rail` | 27px | The floating rail |
| `rounded-full` | pill | Avatars, filter pills |

The HeroUI restyle in `@layer components` applies these to `.button`, `.input`, `.select__trigger`, `.chip`, `.card`, `.modal__dialog`, dropdown and popover. It sits in the components layer so a caller's utility class (`rounded-full`, `p-6`) still wins. Elements inside `.palette-classic` keep the old values.

### 4.3 Borders and shadows

- Tables and many inset rows use a 1px `border` boundary. Use `separator` for quieter divisions.
- Prototype shadows, tinted by `--shadow-tint` / `--shadow-tint-strong` (light and dark values defined): `shadow-soft` (`0 16px 45px`, metric strips, performance cards), `shadow-rail`, `shadow-menu` (popovers), `shadow-dialog`. Plain panels carry a border and no shadow. `shadow-rail-glow` is the glow on the active rail indicator.
- Legacy: HeroUI `shadow-surface` / `shadow-overlay` still exist; prefer the tokens above in new work.
- The inherited light surface shadow is `0 2px 4px rgba(0,0,0,.04), 0 1px 2px rgba(0,0,0,.06), 0 0 1px rgba(0,0,0,.06)`.
- Dark cards are distinguished mainly through surface color. Do not add bright outlines to every card to simulate light-mode elevation.
- HeroUI's default theme sets `--field-border-width: 0px`; the project overrides it to `1px` so fields stay visible on same-color cards. Verify input boundaries and focus in both themes before shipping a form.

## 5. Responsive layout and application shell

### 5.1 Breakpoints

| Breakpoint | Width | Current behavior |
| --- | --- | --- |
| Base | Below 640px | One-column KPI stack; student bottom navigation bar, admin drawer |
| `sm` | 640px | Two-column KPI grid; shell description shown |
| `md` | 768px | Compact desktop button heights; public navigation links shown |
| `lg` | 1024px | Persistent sidebar; four KPI columns; dashboard insight columns |
| `xl` / `2xl` | 1280px / 1536px | Available Tailwind defaults; no extra dashboard layout required |

These are CSS viewport widths, not device classifications. Test intermediate widths as well as named breakpoints.

### 5.2 Shell specification

Reuse [AppShell](../web/components/app-shell.tsx) for authenticated operational and student pages. Navigation lives in [lib/nav.ts](../web/lib/nav.ts); labels come from `lib/i18n/dict/nav.ts`, shell strings from `dict/shell.ts`.

| Region | Specification |
| --- | --- |
| Layout | At `lg`: `p-inset` (14px) page inset, 32px gap between rail and workspace, 24px right padding. Below `lg`: 20px side padding |
| Rail (all roles) | Floating `rounded-rail`, `bg-sidebar`, `shadow-rail`, sticky at `top-inset`, height `h-rail-full` (viewport minus 2 insets; not `h-rail`, which is the 88px spacing token). Collapsed `w-rail` (88px); expanded `w-rail-open` (224px, student) or `w-rail-admin` (248px, staff). Round collapse button on the right edge. Student default collapsed, staff default expanded; the user's toggle is kept in component state |
| Rail brand | "T." mark (`text-brand`, dot in `sidebar-accent`) at the top, linking to the role home; expanded adds "Taruna Eduka" and a tagline |
| Rail bottom | Theme control (moon/sun icon, label when expanded); student expanded also shows the "Selangkah lebih siap." note |
| Collapsed sidebar (staff) | Ringkasan plus one module icon button per multi-item group (operations `MonitorDot`, content `Layers`, finance `Wallet`, system `SlidersHorizontal`; the icon never repeats a child icon), separated by a hairline. The button takes `bg-rail-active` and the accent bar when the route is inside its group. Hover (150ms in, 200ms out) or click/Enter/Space opens a fixed flyout to the right of the rail (`rounded-menu bg-sidebar shadow-menu`) with the group name and its child links on the dashed tree; click moves focus to the first link, Esc returns focus to the button, outside click or navigation closes it, one flyout at a time. Button has `aria-haspopup`, `aria-expanded` and the group name as its label |
| Nav item (student) | Collapsed: icon over a `text-nano` label, `min-h-18` (72px), `rounded-tile`. Expanded: row, 50px high, 13px label |
| Nav item (staff) | Row, 48px; group children use 40px rows under the dashed tree |
| Student navigation | Exactly three items: Dashboard `/student`, Tryout `/student/tryouts` (also active on `/student/tryout/*` and `/student/kecermatan/*`), Katalog `/catalog`, as a flat list with no group header or tree connector |
| Student mobile | No drawer. A floating bottom bar (`sidebar` surface, `rounded-hero`, 70px, 16px side inset) shows the same three destinations; main gets 112px bottom padding below `lg` |
| Admin navigation | Groups: Ringkasan (single link), Operasional (Monitoring, Siswa, Hasil, Hasil Kecermatan), Konten (Tryout, Tes Kecermatan, Paket Item, Bank Soal, Produk, Program), Keuangan (Pesanan, Top-up), Sistem (Akun Pengguna, Pengaturan). Role guards (`adminOnly`) are unchanged; a group with no visible child is hidden |
| Admin groups | Single-open accordion: header button (`aria-expanded`, `aria-controls`); only the group holding the active page is open on load, opening one closes the other, and a group the user closed stays closed until the route moves to another group, which then opens. The panel animates height over 200ms ease-out with the `rows-collapsed` / `rows-expanded` utilities (`grid-template-rows` 0fr / 1fr) and the chevron rotates with it; `motion-reduce` disables both. A closed panel is `inert`. The mobile drawer uses the same accordion |
| Group tree connector | Children hang off a dashed vertical line with a dashed branch to each child, like a file tree. Drawn with `border-dashed` in `sidebar-muted/60` on `li::before` (vertical, half height on the last child) and `li::after` (branch). Never solid |
| Admin mobile | Left drawer, 288px set on `Drawer.Dialog`, `sidebar` surface, closes after navigation |
| Topbar | Not sticky, no border, no background: `min-h-topbar` (82px) at `lg`, 76px below. Student: the 132px logo (light plate in dark theme), then on the right the `WalletPopover` trigger and the account menu. Staff: menu button below `lg`, then a "Group > Page" crumb derived from the nav, then language and theme toggles (`md`+) and the account menu |
| Page heading | `AppShell` renders `PageHeading` at the top of `main` from `title`, `description`, `breadcrumb` and `actions` (h1 `text-display` for students, `text-display-sm` for staff). Pages that render their own h1 must not also pass `title` |
| Main padding | 4px horizontal at `lg`, 20px (shell) below; 20px top; student bottom padding clears the mobile nav |
| Content width | Full available width at 1920 and 2560px. No wrapper imposes a maximum; pages opt in with the `maxWidth` prop |
| Focused builder | Existing tryout builder uses `max-w-3xl` = 768px |

Quick wallet: the `WalletPopover` trigger sits in the student topbar and is the single mount point for the wallet popover.

Account menu (header, every role): avatar, name and role open a HeroUI `Dropdown` (arrow keys, type-ahead, Escape closes and returns focus to the trigger). Students get Profil `/student/profile`, Riwayat pesanan `/student/orders`, Saldo dan transaksi `/student/wallet`; all roles get Tema and Bahasa (the existing theme and locale state, shown with their current value) and Keluar (danger, existing logout flow and redirect). The header no longer carries separate theme and language buttons.

Guidelines:

- Give flexible content a shrinking boundary (`min-w-0` where needed). Put wide tables inside their own horizontal scroll region.
- Keep title and core actions usable when descriptions disappear below `sm`. Avoid more header actions than fit at 320–390px.
- Use one-column cards on small screens; keep reading order equal to DOM order.
- For dense tables, preserve necessary columns with internal scrolling or use a deliberate mobile list representation. Do not squeeze six columns into unreadable cells.
- Let legends and action groups wrap or stack. Ensure drawers and dialogs fit the viewport with safe edge spacing.
- A collapsed sidebar still needs accessible names and discoverable labels for icon-only items (`title` plus `aria-label`).

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

Reuse the [status badge components](../web/components/status-badge.tsx) so status translation and color mapping stay consistent. Chips use `size="sm"` (11px), weight 500, and the `rounded-chip` 7px radius. `StatusChip` (tone + optional dot) covers statuses without a typed badge.

**Icon rule.** Icons are `lucide-react`. The prototype's 19-icon SVG sprite is a hand-drawn lucide-style set (same 24px grid, round caps and joins, 1.65 stroke), so no sprite is ported. `globals.css` forces `stroke-width: 1.65` on every `svg.lucide` outside the landing; do not pass `strokeWidth` (a `stroke-*` utility still wins if truly needed). Sizes come from `Icon` in `components/icon.tsx`: `rail` 22px, `nav` 21px, `md` 20px, `sm` 16px (buttons, kickers), `xs` 14px (arrows, chevrons), `hero` 34px (empty state). Student nav icons: Dashboard `LayoutDashboard`, Tryout `BookOpen`, Katalog `LayoutGrid`. Decorative icons should be hidden from assistive technology; actionable icons need names. Do not use the same checkmark to imply both payment confirmation and exam correctness without a label.

### 6.6 Dialogs and drawers

[FormModal](../web/components/ui/form-modal.tsx) provides the shared modal structure: backdrop, internally scrolling body, title, optional description, optional footer. Title is 16px / 600; description 14px muted; body gap 16px. Sizes are `xs`, `sm`, `md`, `lg`, `xl`, and `full`; the project `xl` maps to library `lg` chrome with a 1024px maximum dialog width.

Guidelines: use a modal for a bounded edit, review, or confirmation; use a full page for long builders. Keep dismissal and submission clear. For irreversible actions, state the affected object and consequence. Verify focus entry, Escape/dismiss behavior, focus containment, and return focus in the actual flow. Keep footer actions reachable when content scrolls. Existing question-picker and navigation drawers are suited to side-by-side selection and navigation.

Drawer width belongs on `Drawer.Dialog` for HeroUI v3; the navigation drawer follows this and measures 288px when open.

### 6.7 Empty states, errors, and feedback

[EmptyState](../web/components/empty-state.tsx): dashed 1px border, 9px radius, 56px vertical padding, centered 12px gap, 44px neutral icon circle, 14px title/description, optional action. Description width is capped at 384px.

Guideline: name why the area is empty and offer the relevant next action. “No purchases yet”, “No matches for these filters”, and “Could not load purchases” require different messages and actions.

Toasts are provided globally at `bottom end`. Use them for concise action outcomes; put durable errors or required decisions near the relevant content. Existing local mock mutations and success toasts do not prove that a server has persisted the action.

### 6.7a Package elements

[ItemElementView](../web/components/admin/item-element.tsx) draws one item-package element the way a student will see it: a symbol as text, an emoji from the bundled Twemoji SVG (`/emoji/<id>.svg`, never the device font), a built-in flat or solid shape as an inline `currentColor` SVG (solids shade their faces with fill opacity, never a new color), an image through a short-lived presigned asset link. It is used by the package grid, the add dialog's catalog and the sample column. Wherever the emoji catalog is shown, the CC-BY 4.0 attribution text from `GET /item-packages/catalog` appears too. The package add dialog follows the question import dialog: input, preview table with per-element reason, then confirm.

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

### 6.10 Shared visual helpers (visual parity pass)

Reuse these before writing page-local markup. All are small, prop-driven, and token-only.

| Component (file) | Props | Use |
| --- | --- | --- |
| `PageHeading`, `Breadcrumb` (`page-heading.tsx`) | `title`, `description?`, `breadcrumb?: Crumb[]`, `actions?`, `size?: "display" \| "compact"` | Page h1 block. `AppShell` already renders it |
| `Icon` (`icon.tsx`) | `icon`, `size?: rail\|nav\|md\|sm\|xs\|hero`, `className?` | Sized lucide icon at the prototype stroke |
| `CardKicker` (`page-blocks.tsx`) | `icon?`, `children`, `className?` | 11px accent label at the top of a card. No built-in margin: pass the gap below as `className="mb-N"` (it cannot be overridden against a built-in class, because `cn()` has no tailwind-merge) |
| `Panel` (`page-blocks.tsx`) | `tone?: plain\|peach\|soft\|canvas`, `elevated?`, `radius?: tile\|card\|hero`, `padding?: none\|sm\|md\|lg\|xl`, `as?` | Card surface; `padding` is `p-6 sm:p-7` by default (`md`), `sm` = `p-5 sm:p-6`, `lg` = `p-6 sm:p-7.5`, `xl` = `p-6 sm:p-9`, `none` for edge-to-edge content. Never override padding through `className`; `peach`/`soft` are the tinted hero cards |
| `SectionHeader` (`page-blocks.tsx`) | `title`, `description?`, `href?`, `linkLabel?`, `actions?` | h2 with a link on the right |
| `TextLink` (`page-blocks.tsx`) | `href`, `arrow?`, `size?: md\|sm` | `sm` = `min-h-8 text-label` (dense rows); `md` = `min-h-9 text-caption`. Do not override min-h or text size through `className`.  Accent link with trailing arrow |
| `NoteBox` (`page-blocks.tsx`) | `tone?: neutral\|success`, `icon?` | Quiet bordered note |
| `ScoreBar` (`page-blocks.tsx`) | `label`, `valueLabel?`, `value` (0-100) | Thin progress bar with `progressbar` semantics |
| `Metric`, `MetricStrip` (`metric-strip.tsx`) | `value`, `label`, `icon?`, `note?`, `size?: md\|lg\|xl` | Bordered row of numbers with dividers. Two columns below `sm` (icons and `note` hidden), one row from `sm` |
| `FilterPills`, `UnderlineTabs` (`filter-pills.tsx`) | `options: {id,label,count?}[]`, `value`, `onChange`, `ariaLabel`, `idPrefix?` (tabs) | Rounded filters; underlined hub tabs. Controlled. `UnderlineTabs` follows the WAI-ARIA tabs pattern (roving tabindex, Arrow/Home/End select). Render one panel with `role="tabpanel"`, `id={tabPanelId(idPrefix)}` and `aria-labelledby={tabId(idPrefix, value)}` |
| `StatusChip` (`status-badge.tsx`) | `tone: success\|warning\|danger\|accent\|neutral`, `dot?` | Generic status chip |
| `ExamTimerPill` (`countdown-ring.tsx`) | `time`, `urgent?` | Prototype exam timer pill; display only |
| `DataTable` (`data-table.tsx`) | existing props plus `footer?` | Table with a summary strip |
| `EmptyState` (`empty-state.tsx`) | existing props plus `bare?` | Rose icon, Geist title; `bare` inside a panel |
| `Logo` (`logo.tsx`) | existing props plus `size?: sm\|md`, `plate?` | 132px topbar logo with dark-theme plate |
| `GradientAvatar` | unchanged props | Flat peach disc with accent initials unless explicit colors are given |

Layout utilities in `globals.css` replace arbitrary grid values: `grid-cols-detail` (main + 17.5rem aside), `grid-cols-hero-pair`, `grid-cols-wallet`, `grid-cols-score`, `max-w-popover`. Use them with `lg:`/`md:` variants; do not write `grid-cols-[...]` in components.

Tinted tokens added for these: `bg-peach`, `bg-accent-soft`, `text-rose` / `bg-rose`, `bg-success-soft` with `text-success-soft-foreground` (prototype green), `bg-rail-active`, `bg-sidebar-hover`, `logo-plate`.

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

### 7.4a Approved account entry and student overview

Owner-approved on 2026-10-31. `AuthLayout` is the shared layout for login, registration, verification and recovery. Desktop has equal columns, a 24px gap, a 380px form and a restrained accent-tinted illustration panel. `/login` is the student entry (form right); staff enter `/portal` directly (form left), with no public portal link. `/login/staff` redirects to `/portal`. Below 800px the illustration panel is hidden and the form has at least 24px horizontal inset. Forms use 48px primary controls, visible labels, autofill semantics and an accessible password visibility control. Idle input boundaries mix `muted` at 75% with `surface`; focused/invalid borders retain their semantic state. Error text uses `danger` on `surface`, avoiding a tinted background that would lower text contrast.

The illustration reuses `CountdownRing` decoratively. Its 224px region contains only the ring and book icon; caption and two feature cards are separate rows in normal flow. Never position their text over the ring. Entrance animation is one sequence; CSS and the ring both respect reduced motion. Colors and radii remain derived from existing tokens; dark auth headers use a light semantic backdrop for the existing logo asset.

Registration opens `/verify-email` after the server establishes the session. The code field is one accessible numeric six-digit input with paste/autofill support. Wrong/expired codes, pending verification, resend errors and session restoration are distinct from success. Verification remains optional (D01): skip opens `/student`, where a non-blocking reminder links back to verification. No prototype codes or simulated success belong in the application.

The student overview reuses `AppShell` at the full available width beside the sidebar, without a centered maximum-width cap; retain the shell's 16px mobile and 24px desktop horizontal padding. Order, with 24px separation: heading row (greeting, "Lihat katalog" outline button); two large practice cards (Tryout on `peach`, Tes kecermatan on `accent-soft`, 24/32px padding, 3xl radius, each with one primary action; the Kecermatan card shows the server's `attempts_remaining` for the featured test); one compact stats strip (owned tryouts, Kecermatan quota summed from the tests list, completed tryouts, average score) divided by `separator`; then a two-thirds / one-third row with "Latihan saya" (owned tryouts with access end, Kecermatan tests with available attempts) and "Hasil terakhir" (latest completed tryout scores plus the note that tryout and Kecermatan results are separate). Module grids have 20px gaps. A `null` average renders "—"; `0` renders as a real score. Balance is not on the dashboard; it lives behind the header wallet trigger. New accounts show the promo variants of both cards, real empty states for both lists and the three-step guide; the shell-level `EmailVerifyBanner` carries the unverified reminder inside the same content boundary. Login and register both land on this page, so geometry is identical and only data differs. Loading uses placeholders that mirror the layout (never zeros), errors offer retry, and data is keyed to the signed-in user so account switches never show stale values. `StatCard compact` is unchanged and unused here.

### 7.4b Student workspace flows

Built 2026-10-31 from the [workspace handoff](../docs/handoffs/2026-10-31-student-workspace/HANDOFF.md). The prototype there is a visual reference only.

- **Return intent.** A protected student page sends an anonymous user to `/login?next=<path>`. `next` is carried through register and verify-email. [`lib/return-intent.ts`](../web/lib/return-intent.ts) accepts only same-origin paths under `/student`, `/checkout` or `/catalog`, checked against the role of the user who signed in. Staff always land on their home.
- **Practice hub.** `/student/tryouts` has two tabs, Tryout and Tes kecermatan. They use `role="tablist"`, an accent underline and a count pill, and the tab is local state rather than part of the URL.
- **Readiness.** A tryout subtest starts from `/subtest/:id/start` and a Kecermatan test from its `/attempt` step. Each is a full page with the rules and a required "siap" checkbox. Submit confirmation and the next-subtest transition are full-page states, not dialogs, and the timer stays visible.
- **Exam frame.** Show the question first and the "Navigasi soal" card beside it; below `lg` the navigator stacks under the question.
  - The timer is a `font-mono` pill with `role="timer"`, on `accent-soft`. It switches to `danger-soft` near the deadline.
  - Before the server clock syncs it shows `--:--`, never a placeholder duration.
- **Save state.** An answer reads as saved only after the server ACK.
  - Pending or offline answers say they are not yet saved, with an offline banner (`warning-soft`, `role="alert"`).
  - Kecermatan answers are write-once. When retries are exhausted, a persistent banner offers a manual resend. On a 409 the server's value wins and the page refetches.
- **Attempt addressing.** Tryout and Kecermatan results, and tryout review, are addressed by `?attempt=<session_id>`.
  - A foreign or unknown id shows an unavailable state, and an unfinished attempt a not-ready state.
  - Held explanations show a held state, and keys are never requested before `explanations_released`.
- **Payment method selector.** A radiogroup of full-width rows: radio dot, icon, title and one-line description. The selected row uses `border-accent bg-accent-soft`. An unavailable method is disabled with an explanation, or not rendered when the gateway reports 503.
- **Order and top-up status pages.** `/student/orders/:id` and `/student/wallet/topups/:id` share one layout.
  - A status banner (soft semantic tint, icon, title and one sentence) sits above the detail and summary cards.
  - Held payment exceptions show a `warning-soft` notice with no refund promise. Access actions appear only for the server's final status.
  - Pending entities poll every 5s and stop on unmount, and every pending page has a manual refresh.
- **Wallet.** The header trigger opens `WalletPopover`, a modal showing the exact balance, recent entries and links. Top-up never happens inside it: "Isi saldo" navigates to `/student/wallet/topup`, which replaces itself with the status page after creation.
  - Amounts are whole-rupiah digit strings, and the minimum always comes from the server.
- **Idempotency in the UI.** Keep one key per logical purchase or top-up, replaced only when the payload changes or after a definite failure. An unknown create outcome locks the inputs and retries with the same key.

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
