package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/jessevdk/go-flags"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var opts struct {
	MinSDKVersion int `short:"s" long:"min-sdk-version" description:"Minimum android SDK version" required:"true"`
}

func main() {
	parser := flags.NewParser(&opts, flags.Default|flags.IgnoreUnknown)
	parser.LongDescription = `
Run Golang tests on a connected Android device or emulator.

Device selection is delegated to adb, pass in ANDROID_SERIAL= environment variable to adb to specify
a device to run tests on.

Example:
ANDROID_SERIAL=emulator-1234 go-android-test -s 21 -run TestFoo
`
	leftoverArgs, err := parser.Parse()
	if err != nil {
		os.Exit(1)
	}

	var exitCode int
	defer func() { os.Exit(exitCode) }()

	testDir, _ := os.MkdirTemp("", "go-android-test")
	defer func() { _ = os.RemoveAll(testDir) }()

	// "unique" name for compiled tests.
	stamp := time.Now().Format("Jan-02-06-15-04-05.000000")
	testBinary := filepath.Join(testDir, fmt.Sprintf("tests_%s", stamp))

	adbPath, err := findADB()
	if err != nil {
		fmt.Printf("Fatal: locating adb: %s\n", err)
		exitCode = 1
		return
	}
	fmt.Printf("Located adb at: %s\n", adbPath)

	// Ask the running Emulator for its abi so we know how to compile for it.
	buf := new(bytes.Buffer)
	cmd := exec.Command(adbPath, "-e", "shell", "getprop ro.product.cpu.abilist")
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Printf("Fatal: Is an Android Emulator running?\n")
		exitCode = 1
		return
	}
	abi := strings.TrimSpace(buf.String())
	fmt.Printf("Running Android Emulator ABI is '%s'\n", abi)

	// Cross-compile tests to an Android binary that's runnable on the emulator
	fmt.Println("Cross-compiling tests")
	cmd = exec.Command(
		"ndkenv", "-a", abi, "-s", strconv.Itoa(opts.MinSDKVersion),
		"go", "test", "-c", "-o", testBinary)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Printf("Fatal: compiling tests: %s\n", err)
		exitCode = 1
		return
	}
	fmt.Printf("Compiled tests to '%s'\n", testBinary)

	// We need root on the emulator to copy and run executables
	fmt.Println("Ensuring adb has root permissions")
	buf = new(bytes.Buffer)
	cmd = exec.Command(adbPath, "root")
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Printf("Fatal: running %s: %s\n", cmd, err)
		exitCode = 1
		return
	}
	if strings.Contains(buf.String(), "cannot run as root") {
		fmt.Printf("Android Emulator is not rooted. Use a device without Google Play Services.")
		exitCode = 1
		return
	}

	// Copy test binary into emulator at /data/local/<binary>
	fmt.Println("Copying compiled tests onto device")
	remoteTestBinary := filepath.Join("data", "local", filepath.Base(testBinary))
	cmd = exec.Command(adbPath, "push", testBinary, remoteTestBinary)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Printf("Fatal: running %s: %s\n", cmd, err)
		exitCode = 1
		return
	}

	// Assumes all remaining args are test flags (e.g. -timeout), not flags for compilation or
	// flags to be provided to the resulting binary when run
	// TODO: Support non-test arguments for the test binary
	for i, arg := range leftoverArgs {
		if strings.HasPrefix(arg, "-") {
			leftoverArgs[i] = fmt.Sprintf("-test.%s", strings.TrimPrefix(arg, "-"))
		}
	}

	// Set executable permissions (adb strips), run tests binary, then delete it.
	fmt.Println("Running tests on device")
	testCmd := fmt.Sprintf(
		"chmod +x \"%[1]s\"; \"%[1]s\" %s; rm -f \"%[1]s\"",
		remoteTestBinary, strings.Join(leftoverArgs, " "))
	cmd = exec.Command(adbPath, "shell", testCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		exitCode = exitError.ExitCode()
		return
	}

	return
}
