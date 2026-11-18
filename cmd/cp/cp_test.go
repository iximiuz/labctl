package cp

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

type splitRemotePathCase struct {
	arg    string
	playID string
	path   string
	remote bool
}

func TestSplitRemotePath(t *testing.T) {
	tests := []splitRemotePathCase{
		{"65e78a64366c2b0cf9ddc34c:~/some/file", "65e78a64366c2b0cf9ddc34c", "~/some/file", true},
		{"65e78a64366c2b0cf9ddc34c:/a:b", "65e78a64366c2b0cf9ddc34c", "/a:b", true},
		{"65e78a64366c2b0cf9ddc34c:", "65e78a64366c2b0cf9ddc34c", "", true},
		{":~/some/file", "", "~/some/file", true},
		{"./some/file", "", "", false},
		{"/abs/some/file", "", "", false},
	}

	if runtime.GOOS == "windows" {
		tests = append(tests,
			splitRemotePathCase{`C:\some\file`, "", "", false},
			splitRemotePathCase{`C:some\file`, "", "", false},
			splitRemotePathCase{`C:/some/file`, "", "", false},
			splitRemotePathCase{`\\server\share\file`, "", "", false},
		)
	} else {
		tests = append(tests, splitRemotePathCase{`C:\some\file`, "C", `\some\file`, true})
	}

	if runtime.GOOS == "windows" {
		tests = append(tests,
			splitRemotePathCase{`\\?\C:\some\file`, "", "", false},
			splitRemotePathCase{`\\?\UNC\server\share\file`, "", "", false},
		)
	}

	for _, tt := range tests {
		playID, path, remote := splitRemotePath(tt.arg)
		assert.Equal(t, tt.remote, remote, tt.arg)
		assert.Equal(t, tt.playID, playID, tt.arg)
		assert.Equal(t, tt.path, path, tt.arg)
	}
}

func TestScpLocalPath(t *testing.T) {
	assert.Equal(t, "./some/file", scpLocalPath("./some/file"))

	if runtime.GOOS == "windows" {
		assert.Equal(t, `C:\some\file`, scpLocalPath(`\\?\C:\some\file`))
		assert.Equal(t, `\\server\share\file`, scpLocalPath(`\\?\UNC\server\share\file`))
		assert.Equal(t, `C:\some\file`, scpLocalPath(`C:\some\file`))
	} else {
		assert.Equal(t, `\\?\C:\some\file`, scpLocalPath(`\\?\C:\some\file`))
	}
}
