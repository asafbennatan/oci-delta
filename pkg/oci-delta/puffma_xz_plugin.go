package ocidelta

import (
	"strings"

	"github.com/asafbennatan/puffma"
)

// puffmaXzPluginName identifies puffmaXzPlugin patches in the delta stream.
const puffmaXzPluginName = "puffma-xz"

// puffmaXzPlugin adapts the puffma library as a tar-diff/tar-patch content
// diff plugin for .xz-compressed files. Generic binary diffing (bsdiff,
// zstd dictionaries) can't diff two independently-compressed xz files
// effectively, since LZMA's entropy coding makes similar inputs produce
// unrelated compressed bytes; puffma diffs the decoded LZMA token stream
// instead. It implements both tar-diff's ContentDiffPlugin (Name/MayApply/
// Diff) and tar-patch's ContentDiffPlugin (Name/Apply).
type puffmaXzPlugin struct{}

func (puffmaXzPlugin) Name() string { return puffmaXzPluginName }

func (puffmaXzPlugin) MayApply(basenames []string) bool {
	for _, b := range basenames {
		if strings.HasSuffix(b, ".xz") {
			return true
		}
	}
	return false
}

func (puffmaXzPlugin) Diff(oldData, newData []byte) ([]byte, error) {
	return puffma.Diff(oldData, newData)
}

func (puffmaXzPlugin) Apply(oldData, patch []byte) ([]byte, error) {
	return puffma.Apply(oldData, patch)
}
