package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFileCollection(t *testing.T) {
	fexpected := map[string]avatarInfo{
		"54747": {
			ActualName: "54747.jpg",
			Num:        1,
			Size:       8013,
			Hash:       "a39277ae8e0480b1",
		},
		"54755": {
			ActualName: "54755.jpg",
			Num:        1,
			Size:       32993,
			Hash:       "1a213a9dd0b41bbd",
		},
		"54760": {
			ActualName: "54760 (2).jpg",
			Num:        2,
			Size:       7747,
			Hash:       "b60b5bb476455012",
		},
		"54877": {
			ActualName: "54877 (2).jpg",
			Num:        2,
			Size:       10486,
			Hash:       "937da1369f46513a"},
	}

	e := &employCommand{
		Glob: &Globals{
			Avatars: "./testdata",
		},
	}

	fc, err := e.getFileCollection()
	require.NoError(t, err)

	assert.Equal(t, fc, fexpected)
}
