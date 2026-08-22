package upstream

import (
	"context"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageSource(t *testing.T) {
	ctx := context.Background()
	dataFs, err := mockfs.NewFs(ctx, "data", "", nil)
	require.NoError(t, err)
	usageFs, err := mockfs.NewFs(ctx, "usage", "", nil)
	require.NoError(t, err)

	dataFs.Features().About = func(context.Context) (*fs.Usage, error) {
		t.Fatal("data backend About must not be called when a usage source is configured")
		return nil, nil
	}
	const wantFree = int64(1234)
	usageFs.Features().About = func(context.Context) (*fs.Usage, error) {
		return &fs.Usage{Free: fs.NewUsageValue(wantFree)}, nil
	}

	f := &Fs{
		RootFs:    dataFs,
		usageFs:   usageFs,
		usage:     &fs.Usage{},
		cacheTime: time.Minute,
	}

	free, err := f.GetFreeSpace()
	require.NoError(t, err)
	assert.Equal(t, wantFree, free)
}
