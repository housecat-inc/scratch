# Typography

Generated from `ui/storybook/registry/typography.go` by `go test ./storybook -update` (from `ui/`);
edit the table there, not this file. The Foundations/Typography story in Storybook
renders the same table with a sample of every style.

Text is styled with `type-<use>-<size>[-<weight>]` utilities from
`ui/css/typography.css`, never with Tailwind's `text-<size>`, `font-<weight>`,
`font-mono`, `font-display` or `italic`. One class sets every text property --
family, size, line-height, letter-spacing, weight, variation settings, style and
transform -- so a call site names the role of its text instead of assembling a
look, and nothing leaks in from an ancestor. Sizes are per use: 100 is the
smallest size that use comes in. The unnamed weight is that use's regular one.

## Choosing a use

| Use | Purpose | Reach for it when |
| --- | --- | --- |
| `body` | Interface text: strings that live inside a component. | One or two lines whose vertical rhythm comes from the component around them, on Tailwind's compact UI leading. If the text wraps past two lines and is read straight through, it is a paragraph instead. |
| `caption` | Secondary and meta text. | Text that qualifies something else rather than being the thing: when it happened, who did it, how many there are, what to do next. Usually muted, always short. |
| `paragraph` | Continuous reading text. | Prose the eye tracks line after line across a wide measure, so it gets looser leading than body. Anything rendered from a message, a note or markdown. |
| `title` | Headings. | The name of a page, section, card, dialog or item, set apart from the text under it. Semibold with tight leading, and tracking that tightens as the size grows. Page titles are -heavy. |
| `label` | Uppercase, tracked section labels and eyebrows. | A short all-caps word or two that names a group of content and sits above it. Not for form labels, which are body-200-medium, and not for anything a user reads as a sentence. |
| `code` | Monospace. | Anything a machine produced or will consume, where characters need to line up. Sized at 90% of the matching body step to balance the monospace font visually, with the same line height. |
| `display` | The brand face. | The Housecat wordmark and the rare hero figure; nothing else. Concourse ships as two cuts rather than a weight axis, so -heavy and <b> swap the family to Concourse 6. A one-off larger figure is a text-[...] override on 300. |

## Uses

### body

Interface text: strings that live inside a component. One or two lines whose vertical rhythm comes from the component around them, on Tailwind's compact UI leading. If the text wraps past two lines and is read straight through, it is a paragraph instead.

Used for:

- row and card titles, menu and tab items, buttons
- form descriptions, table cells, inputs
- chips, badges and empty-state messages

| Class | Renders |
| --- | --- |
| `type-body-100` | 12px on 16px |
| `type-body-200` | 14px on 20px |
| `type-body-300` | 16px on 24px |
| `type-body-400` | 18px on 28px |

Weights: regular 400, medium 500, heavy 600. `<b>` and `<strong>` are medium (500): the quiet emphasis the app has always used for a name in a row, where semibold reads as shouting. -heavy is for a whole element that needs to stand out.

### caption

Secondary and meta text. Text that qualifies something else rather than being the thing: when it happened, who did it, how many there are, what to do next. Usually muted, always short.

Used for:

- timestamps and relative dates
- helper text under a field
- counts in a sidebar, tooltips, breadcrumb hints

| Class | Renders |
| --- | --- |
| `type-caption-100` | 12px on 16px |

Weights: regular 400, medium 500, heavy 600. `<b>` and `<strong>` are medium (500), so an emphasised name in a meta line stays quiet.

### paragraph

Continuous reading text. Prose the eye tracks line after line across a wide measure, so it gets looser leading than body. Anything rendered from a message, a note or markdown.

Used for:

- mail message bodies
- notes, and task or project descriptions rendered from markdown
- multi-sentence help

| Class | Renders |
| --- | --- |
| `type-paragraph-200` | 14px, leading 1.625 |
| `type-paragraph-300` | 16px, leading 1.75 |

Weights: regular 400, heavy 600. `<b>` and `<strong>` are semibold (600), because <strong> in rendered prose is expected to read as clearly strong.

### title

Headings. The name of a page, section, card, dialog or item, set apart from the text under it. Semibold with tight leading, and tracking that tightens as the size grows. Page titles are -heavy.

Used for:

- page titles (500-heavy), section headings (400-heavy, 300)
- card and dialog titles (200)
- group headings inside a form (100)

| Class | Renders |
| --- | --- |
| `type-title-100` | 16px |
| `type-title-200` | 18px |
| `type-title-300` | 20px |
| `type-title-400` | 24px |
| `type-title-500` | 30px |
| `type-title-600` | 36px |

Weights: regular 600, heavy 700. `<b>` and `<strong>` are bold (700), one step up from the semibold default.

### label

Uppercase, tracked section labels and eyebrows. A short all-caps word or two that names a group of content and sits above it. Not for form labels, which are body-200-medium, and not for anything a user reads as a sentence.

Used for:

- sidebar group headings such as Projects and Chats
- column headings in a sheet
- an eyebrow above a stat

| Class | Renders |
| --- | --- |
| `type-label-100` | 12px on 16px |
| `type-label-200` | 14px on 20px |

Weights: regular 600, heavy 700. `<b>` and `<strong>` are bold (700).

### code

Monospace. Anything a machine produced or will consume, where characters need to line up. Sized at 90% of the matching body step to balance the monospace font visually, with the same line height.

Used for:

- identifiers and slugs
- logs, JSON and editors
- keyboard shortcuts

| Class | Renders |
| --- | --- |
| `type-code-100` | 10.8px on 16px |
| `type-code-200` | 12.6px on 20px |
| `type-code-300` | 14.4px on 24px |

Weights: regular 400, heavy 600. `<b>` and `<strong>` are semibold (600).

### display

The brand face. The Housecat wordmark and the rare hero figure; nothing else. Concourse ships as two cuts rather than a weight axis, so -heavy and <b> swap the family to Concourse 6. A one-off larger figure is a text-[...] override on 300.

Used for:

- the wordmark in the top bar (200-heavy) and the sidebar (100-heavy)
- the sign-in wordmark (300-heavy)
- the home hero number (300 with text-[60px])

| Class | Renders |
| --- | --- |
| `type-display-100` | 1.2rem |
| `type-display-200` | 1.35rem |
| `type-display-300` | 24px |

Weights: regular Concourse 4, heavy Concourse 6. `<b>` and `<strong>` are the Concourse 6 cut at the same weight.

## Emphasis

`<b>`/`<strong>` and `<i>`/`<em>` take their look from the nearest `type-*`
ancestor. Each utility passes its use's optional `--type-<use>-font-bold`,
`-bold-var`, `-italic-font`, `-italic-weight`, `-italic-style` and `-italic-var`
tokens through to the `b` and `i` rules, which fall back to inherit (italic for
style) when a use never declares them. Inline emphasis is a `<b>` or `<i>`.

When the element cannot change its tag -- a link, a span that truncates, a
button -- `type-bold` and `type-italic` are the same thing as classes. They read the
same tokens, work on the element that carries the `type-*` class or on any
descendant, and are what to write instead of Tailwind's `font-bold` and `italic`,
which cannot be redefined and stay banned.

## Overrides

A single-axis utility after a `type-*` class still applies and is how a
layout-driven adjustment is made: `type-title-200 leading-none`,
`type-body-100 tracking-widest`, `type-display-300 text-[60px]`. Line-heights are
unitless ratios and letter-spacing is in em, so leading and tracking scale with
an overridden size. `TwMerge` treats `type-*` as one group that replaces an
earlier size, weight, family, style, transform, leading or tracking utility, so
`TwMerge("type-body-200-medium", props.Class)` lets a caller pass `type-body-300`
and get exactly that. Color is a separate axis and stays a separate class.

## Tokens

Every value comes from a `--type-<use>-*` token declared on `:root` in
`ui/css/typography.css`: `-font`, `-var`, `-weight`, `-weight-medium`,
`-weight-bold`, and per size `-size`, `-lh` and `-ls`. A use is retuned in one
place, and an ancestor can override a token for a subtree.

## Adding a style

Add the `@utility` block to `ui/css/typography.css` and the size or use to the
table in `ui/storybook/registry/typography.go`, then run
`go test ./storybook -update` from `ui/` to regenerate this page, the story and its
golden.
