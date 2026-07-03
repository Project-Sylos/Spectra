package mount

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"codeberg.org/Sylos/Spectra/internal/api"
	"codeberg.org/Sylos/Spectra/internal/config"
	"codeberg.org/Sylos/Spectra/internal/types"
	"codeberg.org/Sylos/Spectra/sdk"
)

// Options configures the mount manager.
type Options struct {
	ConfigPath string
	RemoteURL  string
	WithAPI    bool
	Mounts     map[string]string
}

// Manager coordinates multiple world mounts and optional API server.
type Manager struct {
	fs      *sdk.SpectraFS
	mounts  []*FUSEMount
	httpSrv *http.Server
	opts    Options
	cfg     *types.Config
}

// NewManager creates a manager from options.
func NewManager(opts Options) (*Manager, error) {
	if opts.RemoteURL != "" && opts.WithAPI {
		return nil, fmt.Errorf("--with-api cannot be used with --remote")
	}
	if IsNotSupported() {
		return nil, fmt.Errorf("FUSE mount is not supported on this platform")
	}

	cfg, err := loadManagerConfig(opts)
	if err != nil {
		return nil, err
	}

	mounts, err := ResolveMounts(cfg, opts.Mounts)
	if err != nil {
		return nil, err
	}

	var fs *sdk.SpectraFS
	if opts.RemoteURL == "" {
		fs, err = sdk.New(opts.ConfigPath)
		if err != nil {
			return nil, fmt.Errorf("initialize Spectra SDK: %w", err)
		}
	}

	m := &Manager{
		fs:   fs,
		opts: opts,
		cfg:  cfg,
	}

	for _, spec := range mounts {
		var backend Backend
		if opts.RemoteURL != "" {
			backend = NewHTTPBackend(opts.RemoteURL, spec.World)
		} else {
			backend = NewSDKBackend(fs, spec.World)
		}
		fuseMount, err := MountServer(spec.Path, backend)
		if err != nil {
			m.shutdownMounts()
			if fs != nil {
				fs.Close()
			}
			return nil, fmt.Errorf("mount world %s at %s: %w", spec.World, spec.Path, err)
		}
		fmt.Printf("Mounted world %q at %s\n", spec.World, spec.Path)
		m.mounts = append(m.mounts, fuseMount)
	}

	withAPI := opts.WithAPI
	if !withAPI && cfg.Mount != nil {
		withAPI = cfg.Mount.WithAPI
	}
	if withAPI {
		if fs == nil {
			m.Shutdown()
			return nil, fmt.Errorf("--with-api requires in-process SDK (cannot use with --remote)")
		}
		if err := m.startAPI(); err != nil {
			m.Shutdown()
			return nil, err
		}
	}

	return m, nil
}

func loadManagerConfig(opts Options) (*types.Config, error) {
	if opts.ConfigPath != "" {
		return config.LoadFromFile(opts.ConfigPath)
	}
	if len(opts.Mounts) == 0 {
		return nil, fmt.Errorf("config file or --mount overrides required")
	}
	return &types.Config{SecondaryTables: map[string]float64{}}, nil
}

func (m *Manager) startAPI() error {
	addr := fmt.Sprintf("%s:%d", m.cfg.API.Host, m.cfg.API.Port)
	server := api.NewServer(m.fs, &m.cfg.API)
	m.httpSrv = &http.Server{
		Addr:         addr,
		Handler:      server.GetRouter(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		fmt.Printf("API server listening on http://%s/api/v1/\n", addr)
		if err := m.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "API server error: %v\n", err)
		}
	}()
	return nil
}

// Run blocks until SIGINT/SIGTERM, then unmounts cleanly.
func (m *Manager) Run() error {
	if len(m.mounts) == 0 {
		return fmt.Errorf("no mounts active")
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup
	for _, mount := range m.mounts {
		wg.Add(1)
		go func(fm *FUSEMount) {
			defer wg.Done()
			fm.Wait()
		}(mount)
	}

	<-sigChan
	fmt.Println("\nShutting down mounts...")
	m.Shutdown()
	wg.Wait()
	return nil
}

// Shutdown unmounts all filesystems and stops services.
func (m *Manager) Shutdown() {
	if m.httpSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = m.httpSrv.Shutdown(ctx)
	}
	m.shutdownMounts()
	if m.fs != nil {
		_ = m.fs.Close()
	}
}

func (m *Manager) shutdownMounts() {
	for _, mount := range m.mounts {
		if err := mount.Unmount(); err != nil {
			fmt.Fprintf(os.Stderr, "unmount %s: %v\n", mount.MountPoint(), err)
		}
	}
}
