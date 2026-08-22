package atexit

import (
	"os"
	"runtime"
	"testing"

	"github.com/rclone/rclone/lib/exitcode"
	"github.com/stretchr/testify/assert"
)

type fakeSignal struct{}

func (*fakeSignal) String() string {
	return "fake"
}

func (*fakeSignal) Signal() {
}

var _ os.Signal = (*fakeSignal)(nil)

func TestOrderedFns(t *testing.T) {
	first := func() {}
	second := func() {}
	third := func() {}
	firstHandle := FnHandle(&first)
	secondHandle := FnHandle(&second)
	thirdHandle := FnHandle(&third)

	ordered := orderedFns(map[FnHandle]uint64{
		firstHandle:  1,
		thirdHandle:  3,
		secondHandle: 2,
	})

	assert.Equal(t, []FnHandle{thirdHandle, secondHandle, firstHandle}, ordered)
}

func TestExitCode(t *testing.T) {
	switch runtime.GOOS {
	case "windows", "plan9":
		for _, i := range []os.Signal{
			os.Interrupt,
			os.Kill,
		} {
			assert.Equal(t, exitCode(i), exitcode.UncategorizedError)
		}

	default:
		// SIGINT (2) and SIGKILL (9) are portable numbers specified by POSIX.
		assert.Equal(t, exitCode(os.Interrupt), 128+2)
		assert.Equal(t, exitCode(os.Kill), 128+9)
	}

	// Never a real signal
	assert.Equal(t, exitCode(&fakeSignal{}), exitcode.UncategorizedError)
}
