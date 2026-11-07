package main

// version is the program's version, reported by ezconfig version. A
// release updates it here and in winres/ (the Windows version details).
// A test build can still stamp its own: go build -ldflags "-X main.version=..."
var version = "0.27.0-rc3"
