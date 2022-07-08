package main

import (
	"bytes"
	"fmt"
	"github.com/jessevdk/go-flags"
	"io"
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
	abi, _, err := cmd(adbPath, "shell", "getprop ro.product.cpu.abilist")
	if err != nil {
		fmt.Printf("Fatal: Is an Android Emulator running?\n")
		exitCode = 1
		return
	}
	fmt.Printf("Running Android Emulator ABI is '%s'\n", abi)

	// Cross-compile tests to an Android binary that's runnable on the emulator
	fmt.Println("Cross-compiling tests")
	_, _, err = cmd("ndkenv", "-a", abi, "-s", strconv.Itoa(opts.MinSDKVersion),
		"go", "test", "-c", "-o", testBinary)
	if err != nil {
		fmt.Printf("Fatal: compiling tests: %s\n", err)
		exitCode = 1
		return
	}
	fmt.Printf("Compiled tests to '%s'\n", testBinary)

	// Copy test binary into emulator at /data/local/tmp/<binary>
	// Note: Binaries run from /data/local/tmp have greater permissions than those run from other
	// locations. Notably, they're executable even when non-root, and allow for dlopen() to open
	// system libraries (like libart.so)
	fmt.Println("Copying compiled tests onto device")
	remoteTmpDir := filepath.Join("/data", "local", "tmp")
	remoteTestBinary := filepath.Join(remoteTmpDir, filepath.Base(testBinary))
	if _, _, err = cmd(adbPath, "push", testBinary, remoteTestBinary); err != nil {
		fmt.Printf("Fatal: Copying to device: %s\n", err)
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

	// This allows loading of system libraries (i.e. libart.so) to instantiate a JVM from inside
	// the native process.
	// Thanks to https://gershnik.github.io/2021/03/26/load-art-from-native.html for this!
	var libraryPaths string
	switch abi {
	case "x86", "arm":
		libraryPaths = "/apex/com.android.art/lib:/apex/com.android.runtime/lib"
	default:
		libraryPaths = "/apex/com.android.art/lib64:/apex/com.android.runtime/lib64"
	}

	// Run tests binary, then delete it.
	fmt.Println("Running tests on device")
	testCmd := fmt.Sprintf(
		"LD_LIBRARY_PATH=%s \"%[2]s\" %s; rm -f \"%[2]s\"",
		libraryPaths, remoteTestBinary, strings.Join(leftoverArgs, " "))
	_, _, err = cmd(adbPath, "shell", testCmd)
	if err != nil {
		exitCode = err.(*exec.ExitError).ExitCode()
		return
	}

	return
}

func cmd(name string, arg ...string) (stdout string, stderr string, err error) {
	outBuf, errBuf := new(bytes.Buffer), new(bytes.Buffer)
	c := exec.Command(name, arg...)
	c.Stdout = io.MultiWriter(outBuf, os.Stdout)
	c.Stderr = io.MultiWriter(errBuf, os.Stderr)
	err = c.Run()
	stdout = strings.TrimSpace(outBuf.String())
	stderr = strings.TrimSpace(errBuf.String())
	if err != nil {
		err = fmt.Errorf("running %s: %s\n", c, err)
		return
	}
	return
}
