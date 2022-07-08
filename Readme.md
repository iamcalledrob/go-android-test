# go-android-test - Run Golang tests on a connected Android device or emulator.

## Installing:
```
go install github.com/iamcalledrob/ndkenv@latest
go install github.com/iamcalledrob/go-android-test@latest
```

## Usage:
```
Usage:
  go-android-test [OPTIONS]
Example:
  ANDROID_SERIAL=emulator-1234 go-android-test -s 21 -run TestFoo -v

Device selection is delegated to adb, pass in ANDROID_SERIAL= environment variable to adb to specify
a device to run tests on.

Application Options:
  -s, --min-sdk-version= Minimum android SDK version

```

## Notes:
The utility currently assumes that all builds will require the NDK and 
cgo—and uses [ndkenv](https://github.com/iamcalledrob/ndkenv) to configure the 
cross-compilation.

Testing pure Go code is still possible, but requires the unnecessary step of
having a working NDK.