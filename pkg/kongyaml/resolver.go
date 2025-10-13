package kongyaml

import (
	"fmt"
	"io"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/goccy/go-yaml"
)

// Loader ...
func Loader(r io.Reader) (kong.Resolver, error) {
	values := make(map[string]any)

	err := yaml.NewDecoder(r).Decode(&values)

	if err != nil && err != io.EOF {
		// NOTE(vermakov): masking of EOF required to be able to read empty yamls
		//                 (e.g. sample file where all options commented out)
		return nil, fmt.Errorf("kyaml: %w", err)
	}

	flatten := make(map[string]any, len(values))
	recursiveFlatten(flatten, values, "")

	var f kong.ResolverFunc = func(context *kong.Context, parent *kong.Path, flag *kong.Flag) (any, error) {
		_ = context
		_ = parent
		key := strings.ReplaceAll(flag.Name, "_", "-")

		raw, ok := flatten[key]
		if ok {
			return raw, nil
		}

		return nil, nil
	}

	return f, nil
}

func recursiveFlatten(out map[string]any, in map[string]any, prefix string) {
	for key, value := range in {
		kk := strings.ReplaceAll(prefix+key, "_", "-")

		if mv, ok := value.(map[string]any); ok {
			recursiveFlatten(out, mv, kk+"-")
		} else {
			out[kk] = value
		}
	}
}
