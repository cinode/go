package graph_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/cinode/go/pkg/blenc"
	"github.com/cinode/go/pkg/datastore"
	"github.com/cinode/go/pkg/structure/graph"
	"github.com/stretchr/testify/require"
)

func TestSampleScenario(t *testing.T) {
	ds := datastore.InMemory()
	be := blenc.FromDatastore(ds)

	ctx := context.Background()

	fs, err := graph.NewCinodeFS(
		ctx,
		be,
		graph.NewRootDynamicLink(),
	)
	require.NoError(t, err)
	require.NotNil(t, fs)

	path1 := []string{"dir", "subdir", "file.txt"}

	ep1, err := fs.SetEntryFile(ctx,
		path1,
		strings.NewReader("Hello world!"),
		"",
	)
	require.NoError(t, err)
	require.NotNil(t, ep1)

	ep2, err := fs.FindEntry(
		ctx,
		path1,
	)
	require.NoError(t, err)
	require.NotNil(t, ep2)

	require.Equal(t, ep1.String(), ep2.String())

	// Get the entrypoint to the root
	ep3, err := fs.FindEntry(ctx, []string{})
	require.ErrorIs(t, err, graph.ErrModifiedDirectory)
	require.Nil(t, ep3)

	err = fs.Flush(ctx)
	require.NoError(t, err)

	contentMap := make([]struct {
		path    []string
		content string
	}, 1000)

	for i := 0; i < 1000; i++ {
		contentMap[i].path = []string{
			fmt.Sprintf("dir%d", i%7),
			fmt.Sprintf("subdir%d", i%19),
			fmt.Sprintf("file%d.txt", i),
		}
		contentMap[i].content = fmt.Sprintf("Hello world! from file %d!", i)
	}

	for _, c := range contentMap {
		_, err := fs.SetEntryFile(ctx,
			c.path,
			strings.NewReader(c.content),
			"",
		)
		require.NoError(t, err)
	}

	checkContentMap := func(fs graph.CinodeFS) {
		for _, c := range contentMap {
			ep, err := fs.FindEntry(ctx, c.path)
			require.NoError(t, err)
			require.Contains(t, ep.MimeType(), "text/plain")

			rc, err := fs.OpenEntrypointData(ctx, ep)
			require.NoError(t, err)
			defer rc.Close()

			data, err := io.ReadAll(rc)
			require.NoError(t, err)

			require.Equal(t, c.content, string(data))
		}
	}
	checkContentMap(fs)

	err = fs.Flush(ctx)
	require.NoError(t, err)

	checkContentMap(fs)

	// fs2, err := graph.NewCinodeFS(
	// 	ctx,
	// 	blenc.FromDatastore(ds),
	// 	graph.RootEntrypointString()
	// )

	// TODO:
	//  * reopening the datastore
}
