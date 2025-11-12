package cli

import (
	"testing"

	wrk "github.com/mioxin/kbempgo/internal/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFileCollection(t *testing.T) {
	fexpected := map[string]wrk.AvatarInfo{
		"54747": {ActualName: "54747.jpg", Num: 1, Size: 29, Hash: "7549da98ec1383ce"},
		"54755": {ActualName: "54755.jpg", Num: 1, Size: 29, Hash: "001d9c68e09e3b2f"},
		"54760": {ActualName: "54760 (2).jpg", Num: 2, Size: 33, Hash: "a1b99ab927a22f02"},
		"54877": {ActualName: "54877 (2).jpg", Num: 2, Size: 33, Hash: "23974fabd80666c1"},
	}

	e := &employCommand{}

	fc, err := e.getFileCollection("./testdata")
	require.NoError(t, err)

	assert.Equal(t, fc, fexpected)
}
