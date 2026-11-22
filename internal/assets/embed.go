// Package assets owns the runtime resources embedded in the executable.
package assets

import "embed"

// Files contains fonts, icons and provider artwork using stable resource paths.
//
//go:embed resources/fonts/*.ttf.gz resources/app-icon.png resources/zhongyao-*.png resources/providers/*.svg
var Files embed.FS

// SceneFixtures contains built-in preview data, independent of user databases.
//
//go:embed resources/qa-fixtures.json
var SceneFixtures []byte
