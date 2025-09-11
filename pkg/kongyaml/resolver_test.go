package kongyaml

import (
	"os"
	"path"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoader(t *testing.T) {
	type nested struct {
		Bar string `name:"bar"`
	}

	type cli struct {
		NoFlag  string `name:"no-flag"`
		Foo     string `name:"foo"`
		Nested  nested `embed:"" prefix:"nested-"`
		Command struct {
		} `cmd:""`
	}

	testCases := []struct {
		file     string
		expected any
	}{
		{"empty.yml", &cli{}},
		{"flat.yml", &cli{Foo: "bar"}},
		{"nested.yml", &cli{Nested: nested{Bar: "baz"}}},
	}

	for _, tc := range testCases {
		t.Run(tc.file, func(t *testing.T) {
			fd, err := os.Open(path.Join("testdata", tc.file))
			require.NoError(t, err)
			defer fd.Close() // nolint

			ld, err := Loader(fd)
			assert.NoError(t, err)

			ncli := new(cli)

			parser, err := kong.New(ncli, kong.Resolvers(ld))
			require.NoError(t, err)

			_, err = parser.Parse([]string{"command"})
			require.NoError(t, err)
			assert.Equal(t, tc.expected, ncli)
		})
	}

}
