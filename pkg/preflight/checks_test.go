package preflight

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakeOutput struct {
	output string
	rc     int
}

var (
	execOutputs = map[string]fakeOutput{
		"nproc 4":  {"4\n", 0},
		"nproc 8":  {"8\n", 0},
		"nproc 16": {"16\n", 0},
		"kvm":      {"kvm\n", 0},
		"metal":    {"none\n", 1},
	}
)

// It turns out to be really irritating to mock exec.Command().
// The trick here is: in normal execution (i.e. not under test),
// execCommand is a pointer to exec.Command(), so it just runs as
// usual.  When under test, we replace execCommand with a function
// that in turn calls _this_ function, with one of the keys in the
// execOutputs above.  This function then runs the TestHelperProcess
// test, passing through that key...
func fakeExecCommand(command string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

// ...and TestHelperProcess looks up the key in execOutputs, and
// behaves as mocked, i.e. it will print the fake output to STDOUT,
// and will exit with the fake return code.
// One irritating subtlety here is that the `go test` framework
// may emit junk to STDERR, so this mocking technique only really
// works for mocking return codes and content on STDOUT.  Still,
// that's OK for our purposes here.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) < 1 {
		os.Exit(1)
	}

	output, ok := execOutputs[args[0]]
	if !ok {
		os.Exit(1)
	} else {
		fmt.Print(output.output)
		os.Exit(output.rc)
	}
}

func TestCPUCheck(t *testing.T) {
	defer func() { execCommand = exec.Command }()

	expectedOutputs := map[string]string{
		"nproc 4":  "Only 4 CPU cores detected. Harvester requires at least 8 cores for testing and 16 for production use.",
		"nproc 8":  "8 CPU cores detected. Harvester requires at least 16 cores for production use.",
		"nproc 16": "",
	}

	check := CPUCheck{}
	for key, expectedOutput := range expectedOutputs {
		execCommand = func(command string, args ...string) *exec.Cmd {
			return fakeExecCommand(key)
		}
		msg, err := check.Run()
		assert.Nil(t, err)
		assert.Equal(t, expectedOutput, msg)
	}
}

func TestVirtCheck(t *testing.T) {
	defer func() { execCommand = exec.Command }()

	expectedOutputs := map[string]string{
		"kvm":   "System is virtualized (kvm) which is not supported.",
		"metal": "",
	}

	check := VirtCheck{}
	for key, expectedOutput := range expectedOutputs {
		execCommand = func(command string, args ...string) *exec.Cmd {
			return fakeExecCommand(key)
		}
		msg, err := check.Run()
		assert.Nil(t, err)
		assert.Equal(t, expectedOutput, msg)
	}

}

func TestMemoryCheck(t *testing.T) {
	defaultMemInfo := procMemInfo
	defer func() { procMemInfo = defaultMemInfo }()

	expectedOutputs := map[string]string{
		"./testdata/meminfo-512MiB": "Only 458112KiB RAM detected. Harvester requires at least 32GiB for testing and 64GiB for production use.",
		"./testdata/meminfo-32GiB":  "31GiB RAM detected. Harvester requires at least 64GiB for production use.",
		"./testdata/meminfo-64GiB":  "",
	}

	check := MemoryCheck{}
	for file, expectedOutput := range expectedOutputs {
		procMemInfo = file
		msg, err := check.Run()
		assert.Nil(t, err)
		assert.Equal(t, expectedOutput, msg)
	}
}

func TestKVMHostCheck(t *testing.T) {
	defaultDevKvm := devKvm
	defer func() { devKvm = defaultDevKvm }()

	expectedOutputs := map[string]string{
		"./testdata/dev-kvm-does-not-exist": "Harvester requires hardware-assisted virtualization, but /dev/kvm does not exist.",
		"./testdata/dev-kvm":                "",
	}

	check := KVMHostCheck{}
	for file, expectedOutput := range expectedOutputs {
		devKvm = file
		msg, err := check.Run()
		assert.Nil(t, err)
		assert.Equal(t, expectedOutput, msg)
	}
}
