package layouts

// DockedQuery and CompactQuery are the docked breakpoint as a script sees it:
// --breakpoint-docked in ui/css/theme.css, 48rem, on either side. Scripts read
// them from window.HousecatViewport, which ViewportStub publishes, so the
// width lives here once rather than in every script that asks. rem, not px,
// so the query tracks zoom and the reader's default font size the way the
// CSS breakpoint does.
const (
	DockedQuery  = "(width >= " + dockedWidth + ")"
	CompactQuery = "(width < " + dockedWidth + ")"
)

const dockedWidth = "48rem"

// The mail shell's widths, declared for CSS as the --shell-* tokens on :root
// in app/srv/mailui/app.css. A media query cannot read a custom property, so
// the queries below are built from these copies, and a test in that package
// pins each copy to its token.
const (
	// SidebarWidth is the docked sidebar's width, --shell-sidebar-width.
	SidebarWidth = "16rem"
	// SidebarCollapsedWidth is the icon-only sidebar's width,
	// --shell-sidebar-collapsed-width. Nothing collapses the sidebar yet.
	SidebarCollapsedWidth = "3.5rem"
	// ChatRailWidth is the docked chat rail's width, --shell-rail-width.
	ChatRailWidth = "400px"
	// MainMinWidth is the narrowest the docked main column should get beside
	// the sidebar, and beside the rail too when it is open,
	// --shell-main-min-width.
	MainMinWidth = "35rem"
)

// NarrowQuery and NarrowWithRailQuery are the docked widths too narrow for
// the expanded sidebar, with the chat rail closed and open: the viewport is
// docked but has less room than the sidebar and main's minimum, plus the rail
// when it is open. They measure the viewport rather than main, so collapsing
// the sidebar cannot change the answer. Scripts read them from
// window.HousecatViewport, as narrow and narrowWithRail.
const (
	NarrowQuery         = "(" + dockedWidth + " <= width < calc(" + SidebarWidth + " + " + MainMinWidth + "))"
	NarrowWithRailQuery = "(" + dockedWidth + " <= width < calc(" + SidebarWidth + " + " + ChatRailWidth + " + " + MainMinWidth + "))"
)
