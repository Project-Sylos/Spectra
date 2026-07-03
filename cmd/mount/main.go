package main

import (
	"flag"
	"fmt"
	"os"

	"codeberg.org/Sylos/Spectra/internal/mount"
)

func main() {
	remoteURL := flag.String("remote", "", "Remote Spectra API base URL (e.g. http://localhost:8086)")
	withAPI := flag.Bool("with-api", false, "Also start the HTTP API server (in-process only)")
	flag.Func("mount", "Mount override WORLD:PATH (repeatable)", func(value string) error {
		world, path, err := mount.ParseMountFlag(value)
		if err != nil {
			return err
		}
		mountOverrides[world] = path
		return nil
	})
	flag.Parse()

	configPath := "internal/config/default.json"
	if flag.NArg() > 0 {
		configPath = flag.Arg(0)
	}

	opts := mount.Options{
		ConfigPath: configPath,
		RemoteURL:  *remoteURL,
		WithAPI:    *withAPI,
		Mounts:     mountOverrides,
	}

	mgr, err := mount.NewManager(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start mount: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Spectra FUSE mounts active. Press Ctrl+C to unmount.")
	if err := mgr.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Mount error: %v\n", err)
		os.Exit(1)
	}
}

var mountOverrides = map[string]string{}
