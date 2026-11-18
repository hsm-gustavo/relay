---
name: Relay website
description: A software catalog and user guide for the Relay desktop HTTP client.
colors:
  paper: "#f8f7fc"
  hero: "#e5e0f9"
  inset: "#eeebf9"
  line: "#bcb4d4"
  ink: "#211936"
  muted: "#534969"
  link: "#423081"
  action: "#e5f35b"
  action-hover: "#d8e847"
  console: "#25203a"
  console-text: "#f4f1ff"
typography:
  display:
    fontFamily: "Bricolage Grotesque, system-ui, sans-serif"
    fontSize: "clamp(3.15rem, 7.2vw, 6rem)"
    fontWeight: 700
    lineHeight: 0.94
    letterSpacing: "-0.04em"
  headline:
    fontFamily: "Bricolage Grotesque, system-ui, sans-serif"
    fontSize: "clamp(1.75rem, 4vw, 2.65rem)"
    fontWeight: 700
    lineHeight: 1.1
    letterSpacing: "-0.04em"
  title:
    fontFamily: "system-ui, sans-serif"
    fontSize: "1.13rem"
    fontWeight: 700
    lineHeight: 1.25
  body:
    fontFamily: "system-ui, sans-serif"
    fontSize: "1rem"
    fontWeight: 400
    lineHeight: 1.65
  label:
    fontFamily: "system-ui, sans-serif"
    fontSize: "0.82rem"
    fontWeight: 400
    lineHeight: 1.65
  mono:
    fontFamily: "ui-monospace, SFMono-Regular, Consolas, monospace"
    fontSize: "0.84rem"
    fontWeight: 400
    lineHeight: 1.6
rounded:
  inline: "0.2rem"
  copy: "0.25rem"
  button: "0.35rem"
  snippet: "0.45rem"
  capture: "0.55rem"
spacing:
  paragraph: "1rem"
  gutter-wide: "3rem"
  gutter-stacked: "1.5rem"
  gutter-compact: "1rem"
  section-gap: "clamp(2.6rem, 5vw, 4rem)"
components:
  button-primary:
    backgroundColor: "{colors.action}"
    textColor: "{colors.ink}"
    rounded: "{rounded.button}"
    padding: "0.55rem 1rem"
  button-primary-hover:
    backgroundColor: "{colors.action-hover}"
    textColor: "{colors.ink}"
  header-link:
    textColor: "{colors.ink}"
  guide-index-link:
    textColor: "{colors.muted}"
    padding: "0.35rem 0.55rem"
  guide-index-link-hover:
    backgroundColor: "{colors.inset}"
    textColor: "{colors.link}"
  workspace-disclosure-open:
    textColor: "{colors.link}"
  workspace-capture:
    backgroundColor: "{colors.console}"
    rounded: "{rounded.capture}"
  command-panel:
    backgroundColor: "{colors.console}"
    textColor: "{colors.console-text}"
    typography: "{typography.mono}"
    rounded: "{rounded.snippet}"
  button-copy:
    backgroundColor: "{colors.console}"
    textColor: "{colors.console-text}"
    rounded: "{rounded.copy}"
    padding: "0 0.45rem"
  button-copy-hover:
    backgroundColor: "{colors.inset}"
    textColor: "{colors.ink}"
  guide-note:
    backgroundColor: "{colors.inset}"
    textColor: "{colors.muted}"
    padding: "0.85rem 1rem"
  inline-code:
    backgroundColor: "{colors.inset}"
    textColor: "{colors.ink}"
    rounded: "{rounded.inline}"
    padding: "0.08em 0.22em"
---

# Design System: Relay website

## Overview

**Creative North Star: "The Software Catalog"**

The website reads like a software catalog printed on lavender and paper white. A wide, tightly set headline identifies Relay. Fine rules organize the page, while a citron download link and a real workspace capture give visitors two clear places to look.

In the guide, blue-black headings and a single reading column carry the instructions. A compact index keeps sections within reach. Dark command blocks let readers spot examples while skimming.

**Key Characteristics**

- Lavender presentation and paper-white reading surfaces.
- Large Bricolage Grotesque headings and compact system type for controls.
- Thin rules, modest corners, and flat surfaces.
- Original-color workspace imagery with a citron region outline.

## Colors

### Primary

- **Citron** fills the download action and outlines the selected screenshot region. It also appears in the skip link, text selection, and keyboard focus on dark command panels. **Citron Hover** deepens the fill when the download link is hovered.

### Secondary

- **Violet Link** identifies inline links, open workspace topics, native disclosure markers, focused controls, and guide headings reached through a hash link.

### Neutral

- **Paper White** is the reading canvas and sticky header. **Hero Lavender** frames the introduction and workspace. **Lavender Inset** marks notes, inline code, index hover, and the footer.
- **Blue-Black Ink** sets headings, brand text, and emphasized terms. **Muted Violet** is used for guide prose, captions, and supporting details.
- **Lavender Rule** separates sections, index entries, and disclosure rows with a one-pixel stroke.
- **Console** and **Console Text** pair on command blocks. The same dark background sits under the workspace image and detail crops.

**The Citron Rule.** Use citron to mark the download, locate a region in the workspace image, or make a keyboard or selection state legible.

## Typography

Bricolage Grotesque Bold is self-hosted and falls back to the system sans-serif. Paragraphs, summaries, and navigation use the system sans-serif. Code and keys use the monospace stack.

- **Display** uses a tight line height and tracking for the product heading. At widths up to 52rem it uses `clamp(2.55rem, 9.5vw, 4.75rem)` and a 12ch maximum width.
- **Headline** carries guide sections. The balanced wrap handles long topic names.
- **Title** uses the body family at a heavier weight for guide subsections and platform names.
- **Body** uses a 1.65 line height. Guide paragraphs, list items, and definitions are capped at 70ch.
- **Label** keeps platform availability close to the download. The workspace caption uses a similar size (0.8rem).
- **Mono** sets multiline examples. Inline code and keycaps scale with the surrounding type (0.87em).

**The Display Type Rule.** Use the display face for the wordmark, product heading, and guide headings. Use system type for explanations and controls.

## Layout

- The shared wrapper caps at 78rem with 3rem side gutters at wide sizes. The introduction pairs the large heading with a supporting column of 16–23rem. The workspace pairs the dominant screenshot with a 13–17rem topic index.
- The guide pairs a sticky 12–15rem index with an article capped at 51rem. Section gaps and top padding give the rules room to separate topics. A 5.5rem scroll inset accommodates the sticky header when following deep links.
- At 66rem, the two installation platform columns stack.
- At 52rem, the introduction, workspace, and guide become single-column and gutters shrink to 1.5rem. The guide index moves into the flow as a two-column list. Each open workspace topic adds a close-up crop of the same capture, while the full figure and its full-size link stay available.
- At 30rem, gutters narrow to 1rem, the header gets shorter, and definition labels stack above their descriptions.

## Elevation & Depth

Surfaces stay flat. Lavender fields, dark command blocks, and fine rules create separation. The sticky navigation uses the paper color and a lower rule.

**The Flat Surface Rule.** Separate regions with a change in fill, a fine rule, or space. Keep shadows out of the visual hierarchy.

## Shapes

Small corners soften controls, command blocks, and captures. The capture uses the broadest radius (0.55rem), and inline code uses the smallest (0.2rem). Notes and index rows remain square. The screenshot region has a two-pixel citron outline with a slight radius (0.22rem).

## Components

### Download link

A bold link with a citron fill, a one-pixel ink border, and a 2.9rem minimum height. Hover deepens the fill. Keyboard focus uses a two-pixel violet outline with a four-pixel offset.

### Navigation and guide index

Header and footer links use compact ink text and gain an underline on hover. The guide index uses muted violet text and a thin left rule on each entry. Hover adds a lavender inset fill and an underline. Its keyboard outline sits inside the row.

### Workspace topics and capture

Use native disclosure controls with fine horizontal dividers. The response topic opens initially. Its open state changes the summary to violet and marks the corresponding screenshot region in citron. The same capture supplies every detail crop. At stacked widths, request and response crops share a 27rem width cap and the collections crop uses a 20rem cap to match the narrower sidebar. A caption and full-size link sit below the main figure.

### Command panels and copy

Keep multiline examples on a dark console panel with horizontally scrollable monospace text. Where the browser supports clipboard writing, the Copy button sits at the upper right and reports success or failure. Hover uses the lavender inset fill, and keyboard focus on the dark surface uses a citron outline.

### Notes and inline examples

Use a pale lavender rectangle for guide callouts. Use the same inset tone for inline code and keycaps. Keycaps add a one-pixel rule. Long inline values may wrap to fit the reading column.

## Do's and Don'ts

### Do

- Do retain the workspace capture’s original colors and match each close-up crop to its topic.
- Do open the guide with a legible index and keep its deep links clear of the sticky header.
- Do keep code and commands readable in narrow layouts.

### Don't

- Don’t fill routine guide links with citron.
- Don’t tint the workspace image or obscure it with decorative overlays.
- Don’t turn platform sections or guide disclosures into elevated cards.
