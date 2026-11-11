// Package breadcrumb is a one-line trail of the pages above the current one,
// after shadcn's Breadcrumb: Root, List, Item, Link, Page, Separator and
// Ellipsis, plus Switcher, the section crumb as an outlined chip that opens a
// selectbox of its siblings: the whole chip on the section's own page, or,
// split, just its chevron, the name beside it linking to the section's root.
//
// Every text part is trimmed to its cap height (text-box), so the trail
// centres on capitals and the separator, a chevron one cap tall, sits
// between them rather than low on the line box.
//
// The trail never wraps. Its overflow is CSS only, in the parts rather than
// the caller:
//
//   - The last item shrinks a hundred times faster than the others, so the
//     current page truncates first, down to its minimum width.
//   - A middle item with Collapse set leaves the trail, with the separator
//     after it, when the Root's container is narrower than 20rem, and an
//     Ellipsis placed before those items shows in their stead. The trail
//     cannot measure its own overflow without script, so the collapse is
//     tied to the container's width.
//   - A Pinned item never shrinks, for a crumb that must stay whole, such as
//     a section's Switcher.
package breadcrumb

import (
	"bytes"
	"context"
	"io"

	"github.com/a-h/templ"
)

// separatorContent is Separator's children, or the default chevron when there
// are none. A templ component cannot be compared with NopComponent, so the
// children are rendered to tell.
func separatorContent(ctx context.Context) templ.Component {
	children := templ.GetChildren(ctx)
	var buf bytes.Buffer
	if err := children.Render(templ.ClearChildren(ctx), &buf); err != nil || buf.Len() == 0 {
		return separatorChevron()
	}
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := w.Write(buf.Bytes())
		return err
	})
}
