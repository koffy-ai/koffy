# Koffy UI and branding guide

This guide defines the stable visual rules for Koffy Web. Deployments may upload separate user-center and admin logos and favicons from **Admin > Settings** without changing source code.

## Product surfaces

- Authentication and user-center pages use the dark Koffy theme.
- Administration pages use a light, compact workspace theme optimized for repeated operations.
- New pages should follow the theme of the area in which they appear.

## Design tokens

```css
--koffy-bg: #0a1628;
--koffy-bg-elevated: rgba(255, 255, 255, 0.045);
--koffy-bg-field: rgba(0, 0, 0, 0.30);
--koffy-border: rgba(0, 212, 170, 0.20);
--koffy-border-soft: rgba(255, 255, 255, 0.10);
--koffy-border-hover: rgba(0, 128, 255, 0.72);
--koffy-text: #ffffff;
--koffy-text-secondary: rgba(255, 255, 255, 0.68);
--koffy-text-muted: rgba(255, 255, 255, 0.48);
--koffy-cyan: #00d4aa;
--koffy-blue: #0080ff;
--koffy-gradient: linear-gradient(135deg, #00d4aa 0%, #0080ff 100%);
```

Use the gradient for primary actions and high-value numbers. Use semantic colors for success, warning, and error states, with sufficient contrast against the active theme.

## Typography and layout

- Use the system UI font stack and keep letter spacing at `0`.
- Page titles should be `26-32px`; compact panels and cards use smaller headings.
- User pages have a maximum content width of `1280px` and collapse to one column on narrow screens.
- Admin pages favor dense tables, predictable forms, and scan-friendly spacing.
- Fixed-format controls must have stable dimensions so loading and hover states do not shift layout.
- Text, buttons, and validation messages must not overlap at supported mobile and desktop widths.

## Components

- Use Lucide icons for familiar actions and tooltips for unfamiliar icon-only controls.
- Use buttons only for commands, selects for bounded option sets, toggles for binary settings, and search-and-select controls for large datasets.
- Cards are reserved for individual modules or repeated items; do not nest decorative cards.
- Forms use visible labels, inline validation, correct browser autocomplete attributes, and disabled/loading states that remain readable.
- Empty states show a static message or illustration; loading indicators appear only while a request is active.
- Tables keep status, amount, time, and identifier formatting consistent across pages.

## Authentication pages

- Keep the login, registration, and password-reset forms centered and single-column.
- The logo, password visibility icon, CAPTCHA state, SMS countdown, and validation text must remain legible in every state.
- Mobile layouts use the full available width without horizontal scrolling.

## User center

- Navigation order: overview, recharge, account security.
- Show high-frequency account information first: profile, available points, subscriptions, and recent usage.
- Use business-facing Chinese labels rather than internal enum values.
- Full operational identifiers and payment diagnostics belong in the administration console.

## Administration

- Keep the light theme restrained and work-focused.
- Use dropdowns for bounded application, plan, model, and status choices.
- Use keyword search and explicit selection for users; search covers phone, display name, and user ID.
- Destructive or irreversible actions require confirmation and a clear outcome message.

## Branding assets

Koffy supports independently uploaded branding for the user center and administration area:

- User center: `/api/v1/branding/logo?area=center`
- Administration: `/api/v1/branding/logo?area=admin`
- User-center favicon: `/api/v1/branding/favicon?area=center`
- Administration favicon: `/api/v1/branding/favicon?area=admin`

The recommended Logo source is a transparent `702 x 180` PNG. Favicons should be square and are converted to a safe `128 x 128` PNG. When no custom asset exists, Koffy serves neutral embedded defaults.

Uploaded assets live in MySQL rather than the Web image. Pulling or recreating containers therefore preserves production branding; deleting the MySQL volume deletes it.

## Review checklist

- Verify desktop and mobile layouts.
- Verify loading, empty, success, validation, and error states.
- Verify keyboard focus and meaningful accessible labels.
- Verify text contrast, button labels, icons, and countdown states.
- Verify no company-specific assets or deployment details are hard-coded.
