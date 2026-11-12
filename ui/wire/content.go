package wire

// SettingsCardPaths are the pages the settings card draws, by path prefix,
// which each shell renders in its settings-card-paths script for the content
// lane (ui/dom/content.ts), which swaps a link to one of them in place.
type SettingsCardPaths []string
