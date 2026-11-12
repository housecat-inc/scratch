package git

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiff(t *testing.T) {
	t.Run("lists the files a branch adds over main", func(t *testing.T) {
		requireGit(t)
		a := assert.New(t)
		r := require.New(t)

		clone, _ := setupRepo(t)
		r.NoError(writeFile(filepath.Join(clone, "feature.txt"), "feature\n"))
		runGit(t, clone, "checkout", "-b", "feature")
		runGit(t, clone, "add", ".")
		runGit(t, clone, "commit", "-m", "add feature")

		files, err := Diff(clone, "main", "HEAD")
		r.NoError(err)
		r.Len(files, 1)
		a.Equal("feature.txt", files[0].NewPath)
		a.Equal(StatusAdded, files[0].Status)
		r.Len(files[0].Hunks, 1)
	})
}

func TestHunkKey(t *testing.T) {
	t.Run("matches for identical headers", func(t *testing.T) {
		a := assert.New(t)
		a.Equal(Hunk{Header: "@@ -1,3 +1,4 @@"}.Key(), Hunk{Header: "@@ -1,3 +1,4 @@"}.Key())
	})
	t.Run("differs for different headers", func(t *testing.T) {
		a := assert.New(t)
		a.NotEqual(Hunk{Header: "@@ -1,3 +1,4 @@"}.Key(), Hunk{Header: "@@ -2,3 +2,4 @@"}.Key())
	})
}

func TestParseDiff(t *testing.T) {
	tests := []struct {
		in   string
		name string
		want []File
	}{
		{
			name: "returns no files for empty input",
			in:   "",
			want: nil,
		},
		{
			name: "numbers old and new lines in a modified hunk",
			in: strings.Join([]string{
				"diff --git a/foo.txt b/foo.txt",
				"index abc..def 100644",
				"--- a/foo.txt",
				"+++ b/foo.txt",
				"@@ -1,3 +1,4 @@",
				" a",
				"-b",
				"+B",
				"+B2",
				" c",
				"",
			}, "\n"),
			want: []File{{
				NewPath: "foo.txt",
				OldPath: "foo.txt",
				Status:  StatusModified,
				Hunks: []Hunk{{
					Header:   "@@ -1,3 +1,4 @@",
					OldStart: 1, OldCount: 3,
					NewStart: 1, NewCount: 4,
					Lines: []Line{
						{Content: "a", Kind: LineContext, NewLine: 1, OldLine: 1},
						{Content: "b", Kind: LineDelete, OldLine: 2},
						{Content: "B", Kind: LineAdd, NewLine: 2},
						{Content: "B2", Kind: LineAdd, NewLine: 3},
						{Content: "c", Kind: LineContext, NewLine: 4, OldLine: 3},
					},
				}},
			}},
		},
		{
			name: "marks a new file as added with no old path",
			in: strings.Join([]string{
				"diff --git a/new.txt b/new.txt",
				"new file mode 100644",
				"index 0000000..abcdef",
				"--- /dev/null",
				"+++ b/new.txt",
				"@@ -0,0 +1,2 @@",
				"+hello",
				"+world",
				"",
			}, "\n"),
			want: []File{{
				NewPath: "new.txt",
				Status:  StatusAdded,
				Hunks: []Hunk{{
					Header:   "@@ -0,0 +1,2 @@",
					OldStart: 0, OldCount: 0,
					NewStart: 1, NewCount: 2,
					Lines: []Line{
						{Content: "hello", Kind: LineAdd, NewLine: 1},
						{Content: "world", Kind: LineAdd, NewLine: 2},
					},
				}},
			}},
		},
		{
			name: "marks a removed file as deleted with no new path",
			in: strings.Join([]string{
				"diff --git a/gone.txt b/gone.txt",
				"deleted file mode 100644",
				"--- a/gone.txt",
				"+++ /dev/null",
				"@@ -1,1 +0,0 @@",
				"-bye",
				"",
			}, "\n"),
			want: []File{{
				OldPath: "gone.txt",
				Status:  StatusDeleted,
				Hunks: []Hunk{{
					Header:   "@@ -1,1 +0,0 @@",
					OldStart: 1, OldCount: 1,
					NewStart: 0, NewCount: 0,
					Lines: []Line{{Content: "bye", Kind: LineDelete, OldLine: 1}},
				}},
			}},
		},
		{
			name: "tracks a pure rename without hunks",
			in: strings.Join([]string{
				"diff --git a/old.txt b/new.txt",
				"similarity index 100%",
				"rename from old.txt",
				"rename to new.txt",
				"",
			}, "\n"),
			want: []File{{
				NewPath: "new.txt",
				OldPath: "old.txt",
				Status:  StatusRenamed,
			}},
		},
		{
			name: "flags a modified binary file",
			in: strings.Join([]string{
				"diff --git a/img.png b/img.png",
				"index abc..def 100644",
				"Binary files a/img.png and b/img.png differ",
				"",
			}, "\n"),
			want: []File{{
				Binary:  true,
				NewPath: "img.png",
				OldPath: "img.png",
				Status:  StatusModified,
			}},
		},
		{
			name: "flags an added binary file",
			in: strings.Join([]string{
				"diff --git a/img.png b/img.png",
				"new file mode 100644",
				"index 0000000..abc",
				"Binary files /dev/null and b/img.png differ",
				"",
			}, "\n"),
			want: []File{{
				Binary:  true,
				NewPath: "img.png",
				Status:  StatusAdded,
			}},
		},
		{
			name: "captures the section context after a hunk header",
			in: strings.Join([]string{
				"diff --git a/main.go b/main.go",
				"--- a/main.go",
				"+++ b/main.go",
				"@@ -7,3 +7,4 @@ func main() {",
				" a",
				"+b",
				" c",
				" d",
				"",
			}, "\n"),
			want: []File{{
				NewPath: "main.go", OldPath: "main.go", Status: StatusModified,
				Hunks: []Hunk{{
					Header:   "@@ -7,3 +7,4 @@ func main() {",
					Section:  "func main() {",
					OldStart: 7, OldCount: 3,
					NewStart: 7, NewCount: 4,
					Lines: []Line{
						{Content: "a", Kind: LineContext, NewLine: 7, OldLine: 7},
						{Content: "b", Kind: LineAdd, NewLine: 8},
						{Content: "c", Kind: LineContext, NewLine: 9, OldLine: 8},
						{Content: "d", Kind: LineContext, NewLine: 10, OldLine: 9},
					},
				}},
			}},
		},
		{
			name: "splits consecutive file diffs",
			in: strings.Join([]string{
				"diff --git a/a.txt b/a.txt",
				"--- a/a.txt",
				"+++ b/a.txt",
				"@@ -1 +1 @@",
				"-old",
				"+new",
				"diff --git a/b.txt b/b.txt",
				"--- a/b.txt",
				"+++ b/b.txt",
				"@@ -1 +1 @@",
				"-x",
				"+y",
				"",
			}, "\n"),
			want: []File{
				{
					NewPath: "a.txt", OldPath: "a.txt", Status: StatusModified,
					Hunks: []Hunk{{
						Header:   "@@ -1 +1 @@",
						OldStart: 1, OldCount: 1,
						NewStart: 1, NewCount: 1,
						Lines: []Line{
							{Content: "old", Kind: LineDelete, OldLine: 1},
							{Content: "new", Kind: LineAdd, NewLine: 1},
						},
					}},
				},
				{
					NewPath: "b.txt", OldPath: "b.txt", Status: StatusModified,
					Hunks: []Hunk{{
						Header:   "@@ -1 +1 @@",
						OldStart: 1, OldCount: 1,
						NewStart: 1, NewCount: 1,
						Lines: []Line{
							{Content: "x", Kind: LineDelete, OldLine: 1},
							{Content: "y", Kind: LineAdd, NewLine: 1},
						},
					}},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			got, err := ParseDiff(tt.in)
			r.NoError(err)
			a.Equal(tt.want, got)
		})
	}
}
