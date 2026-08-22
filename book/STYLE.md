# SuperDB Documentation Style Guide

This is the style guide for the SuperDB documentation in `book/src`.  It
exists so that pages written by different people at different times read as
though they came from one source.

## How to use this guide

Read the sections that apply to what you're writing.  Most contributions
touch only reference pages, so [Page anatomy](#page-anatomy) and
[Examples](#examples) cover the common case.

**For anything this guide does not specify, follow the
[Google developer documentation style guide](https://developers.google.com/style).**
That fallback is deliberate: it keeps this document short and focused on
what is specific to SuperDB, rather than trying to restate general
technical-writing advice that others maintain better than we can.

Some rules here are enforced automatically; see
[What's enforced](#whats-enforced) for the current state.  Everything else
is enforced by review.

## Voice and tone

Write for a reader who knows SQL and JSON but not SuperDB.

* Use second person ("you") when addressing the reader.  Avoid "we" except
  when describing a decision the project made.
* Use present tense.  An operator "returns" a value; it does not "will
  return" one.
* Use active voice.  Prefer "`cut` retains only the fields enumerated" over
  "only the fields enumerated are retained".
* Be direct.  Avoid "simply", "just", "obviously", and "of course", as they add
  nothing and could appear condescending to readers.

## Page anatomy

### Reference pages

Reference pages document a single operator, function, aggregate function,
type, or expression form.  They follow a fixed shape:

````markdown
# <name>

<one-line summary>

## Synopsis

```
<signature>
```

## Description

<prose>

## Examples

<examples>
````

Rules:

* The `# <name>` heading is the bare name of the thing, in the exact case
  the language uses (`cut`, `abs`, `collect_map`).
* The one-line summary sits directly below the H1, starts lowercase, and
  has no trailing period.  Write it as an imperative verb phrase
  ("round a number", "extract subsets of record fields into new records").
  Noun phrases ("natural logarithm") and third-person forms ("returns a
  null value") are legacy and should be converted when you touch the page.
* Use `## Examples` even when there is only one example.
* Do not add other H2 sections unless the page genuinely needs one.  A few
  pages carry extras such as `## Errors` or split their examples by topic;
  that's fine when warranted, but the default is these three.
* The `Synopsis` code fence carries no language tag.  The signature is not
  runnable code and highlighting it as if it were is misleading.

### Operator pages: data-order badge

Operator pages carry a data-order badge between the H1 and the summary,
linking to the explanation in `super-sql/intro.md`:

```markdown
# cut

[✅](../intro.md#data-order)&ensp; extract subsets of record fields into new records
```

The badge indicates what the operator does to the order of its input:

| Badge | Meaning |
| --- | --- |
| ✅ | Order is preserved |
| 🎲 | Order is undefined |
| 🔀 | Order is changed deterministically |

An operator that behaves differently in different modes may carry more than
one badge, as `from` does.  Always follow the badge (or the last badge) with
`&ensp;` before the summary text.

Function and aggregate-function pages do not carry badges.

### Non-reference pages

Introductory, conceptual, and tutorial pages do not follow a fixed shape.
Use whatever headings serve the material, but keep to the formatting rules
below.

## Fixed width, emphasis, and capitalization

### Use fixed width for

* Command names — `super`
* SuperSQL query text — `put`, `SELECT`
* Type names that are language tokens — `int64`, `VARCHAR`
* References to operator or function usage — "the `<expr>` argument to
  `eval`"
* Input, output, and parameter values — "the value of N defaults to `1`",
  "`values 1 > 0` produces the literal value `true`"

Exceptions:

* Don't use fixed width inside link anchor text.  Rendered links already
  carry color and underline emphasis.
* Don't use fixed width for abstract values.  In "when the `<predicate>` in
  a ternary conditional is true", the word "true" is plain, so the reader
  doesn't think only the literal `true` qualifies.
* Don't use fixed width for data types that aren't language tokens, such as
  "record" or "array".

### Don't use italics for language names

Refer to a construct by its name in fixed width, not italics:

```markdown
<!-- Do this -->
The `avg` aggregate function computes the average value of its input.

<!-- Not this -->
The _avg_ aggregate function computes the average value of its input.
```

Italics are for emphasis and for example captions (see below), not for
naming things the language defines.

### Capitalize SQL keywords

```markdown
<!-- Do this -->
`cut` is much like a SQL [SELECT](../sql/select.md) clause

<!-- Not this -->
`cut` is much like a SQL [select](../sql/select.md) clause
```

### Proper names

Spell these exactly: SuperDB, SuperSQL, SUP, BSUP, CSUP, JSON, SQL, GitHub,
JavaScript.

## Links

The anchor text should contain _only_ the name of the operator, function,
or other entity being linked:

```markdown
<!-- Do this -->
the [put](put.md) operator performs field assignment
`cut` is much like a SQL [SELECT](../sql/select.md) clause
While [`from`](from.md) is often used with files, `from` also works with URLs

<!-- Not this -->
the [put operator](put.md) performs field assignment
`cut` is much like a [SELECT clause](../sql/select.md)
While [`from`](from) is often used with files, [`from`](from.md) also works with URLs
```

Link from the first reference in a section only.  Linking to the same place
again later in the same section is occasionally right — for instance, when
closing a section by pointing at more examples — but it should be a
deliberate choice, not the default.

Use relative paths with the `.md` extension so links work both in the built
book and when browsing the repo on GitHub.

## Examples

Examples are executable.  A `mdtest-spq` block is run by the test suite and
its output compared against what the page claims, so an example that drifts
out of sync with the code fails CI.  Write examples accordingly: they are
tests as much as they are documentation.

### Structure

Separate consecutive examples with a horizontal rule, and caption each one
with a short italic phrase:

````markdown
## Examples

---

_A simple Unix-like cut_

```mdtest-spq
# spq
cut a,c
# input
{a:1,b:2,c:3}
# expected output
{a:1,c:3}
```

---

_Missing fields show up as missing errors_

```mdtest-spq
# spq
cut a,d
# input
{a:1,b:2,c:3}
# expected output
{a:1,d:error("missing")}
```
````

Captions are sentence fragments describing what the example shows.  They
start with a capital letter and carry no trailing period.  Don't use the
older `Caption:` form with a colon and no rule.

### Layout

Check how an example renders locally before committing it.

* **Default** — no info-string attributes.  Use this unless the example
  needs otherwise.
* **Stacked** — `{data-layout="stacked"}`.  Use when the example contains
  long lines that would produce a horizontal scrollbar in the default
  layout.
* **Inlined** — `{data-layout='no-labels'} {style='margin:auto;width:85%'}`.
  Use for a small example inlined with prose, where the **Query** /
  **Input** / **Result** labels and full width would read as a section
  break.  Default styling is still right for the larger back-to-back
  example blocks at the bottom of reference pages.

When a block needs both an mdtest keyword and a layout attribute, put the
keyword first: ```` ```mdtest-spq fails {data-layout="stacked"} ````.

### Skipped examples

`mdtest-spq-skip` and `mdtest-command-skip` mark examples that are not
executed.  They are debt: an unskipped example is verified against the
running code, and a skipped one is a claim nobody checks.  Don't add new
ones without a comment saying why, and prefer fixing or deleting a skipped
example to leaving it in place.

## Admonitions

Use GitHub-style admonitions with no space after the `>`:

```markdown
>[!NOTE]
> Some parenthetical information.

>[!TIP]
> Advice that helps but isn't required.

>[!WARNING]
> Something that will bite the reader.
```

Use them sparingly.  A page where every third paragraph is a NOTE has
buried its own emphasis.

## Mechanics

* **Line width.** Hard-wrap prose at roughly 80 columns.  Break at sentence
  or clause boundaries where you can — it keeps diffs small and readable.
  Don't reflow paragraphs you aren't otherwise editing; it turns a one-line
  change into an unreviewable diff.
* **Sentence spacing.** Two spaces after a sentence-ending period.  This is
  the established style across most of the book; match it.
* **Headings.** One H1 per page, as the first line.  Don't skip levels.
* **Tables.** Fine for enumerating options or mappings.  Don't use them for
  prose.

## SUMMARY.md

`book/src/SUMMARY.md` defines the book's table of contents and its ordering.
Every page must appear in it, or mdbook will not build it into the book.

Within a group of reference pages, list entries alphabetically.

A line prefixed with `!!` is invisible to mdbook and is how we hide a
section that isn't ready to publish:

```markdown
!!- [Tutorials](tutorials/intro.md)
!!    - [Super-structured Data](tutorials/super-structured.md)
```

Pages hidden this way are still linted and their examples are still tested.

## Drafts and unfinished content

Don't leave `TODO` markers in pages that are visible in the built book.
They read to a user as an admission that the documentation is unreliable,
and reviewers stop seeing them within a week.

If a page needs work, either track it in a GitHub issue or hide the whole
section from `SUMMARY.md` using the `!!` prefix until it's ready.  Internal
notes to other maintainers belong in the issue, not the page.

## What's enforced

| Check | Tool | Where |
| --- | --- | --- |
| Markdown structure and whitespace | markdownlint | `make markdown-lint` |
| Link validity | linkspector | CI, and nightly |
| Example correctness | mdtest | `make test-heavy` |

Everything else in this guide is enforced by review.
