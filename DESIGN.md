# LocalLife AI SaaS Blue Design System

This project is a local-life merchant AI operations SaaS. The UI should feel reliable, commercial, data-driven and practical for small businesses. Use blue as the primary identity, with restrained AI gradients and clear business actions.

## Brand Direction

- Product character: trustworthy SaaS platform, practical AI assistant, local business growth engine.
- Visual keywords: blue, clean, confident, modern, data dashboard, actionable intelligence.
- Avoid: purple-heavy AI default styling, generic white dashboards, decorative elements without business meaning.

## Colors

- Primary blue: `#2563EB`
- Deep navy: `#0F2747`
- Ink: `#0F172A`
- Muted text: `#64748B`
- Page background: `#F4F8FC`
- Card background: `#FFFFFF`
- Cyan accent: `#06B6D4`
- Success green: `#22C55E`
- Warning orange: `#F97316`
- Danger red: `#EF4444`
- Soft border: `#DCE8F5`
- AI gradient: `linear-gradient(135deg, #0F2747, #2563EB 56%, #06B6D4)`

## Layout

- Use large rounded cards, 18-28px radius, with soft blue shadows.
- Put the most important business result at the top of each page.
- Use section headers with a small uppercase eyebrow and a direct Chinese title.
- Keep dashboards scannable: summary cards first, action cards second, tables/details third.
- For merchant pages, every AI suggestion should lead to an action button.

## Components

- Hero cards: deep blue gradient, white title, readable explanation, right-side action buttons.
- Metric cards: light blue or white background, compact label, large value, small explanation.
- Action cards: tinted background by purpose.
- Data panels: white cards with subtle borders, useful empty states, no decorative noise.
- Forms: compact labels, clear placeholders, primary submit button in blue.

## Shared Vue Components

Use the shared components in `web/src/components/design` before writing one-off page styles:

- `PageHero.vue`: top-level page hero, blue SaaS gradient, supports action slot.
- `MetricCard.vue`: KPI card for dashboard, finance, order and AI pages.
- `DataPanel.vue`: standard white panel with eyebrow, title, description and action slot.
- `ActionCard.vue`: actionable recommendation card for AI and operation pages.

Migration rule: update pages one at a time, run the frontend build, then verify in browser. Do not rewrite every page at once.

## Merchant AI Page

The AI page is the flagship differentiator. It must communicate:

- The merchant has a daily AI quota tied to subscription.
- AI reads real orders, products, customers and coupon data.
- AI outputs concrete actions: referral poster, recall coupon, short-video script, campaign draft.
- AI is a paid capability, not just a static report.

Recommended sections:

- AI hero with quota and business value.
- Today's AI action list.
- AI marketing copy generator.
- AI short-video script generator.
- Referral poster and coupon loop metrics.
- Product signals and operational suggestions.

## Admin Platform

- Use a more enterprise dashboard tone.
- Emphasize merchant status, subscription revenue, customer transaction flow, refunds and risk.
- Tables should support search, filters, ranking and operational buttons.
- Risk and finance modules should use blue base with orange/red status accents.

## Customer Mobile Pages

- Mobile-first, simple and warm.
- Blue should be used as trust/action color, not heavy dashboard styling.
- Product cards should be large, tappable and visually appetizing.
- Checkout and coupon deduction must be obvious.

## Copywriting

- Prefer Chinese for user-facing UI.
- Keep labels short and practical.
- Explain business value, not technical implementation.
- Example: use `生成召回方案`, not `Run AI`.
