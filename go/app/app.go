// Assumes CGO is enabled.
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sprout/go/platform/database"
	"sprout/go/platform/database/config"
	"sprout/go/platform/http/server/auth"
	"sprout/go/platform/x"
	"sprout/go/platform/x/compress"
	"sprout/go/platform/x/workqueue"
	"strings"
	"sync"
	"time"

	"github.com/Data-Corruption/lmdb-go/wrap"
	"github.com/Data-Corruption/stdx/xhttp"
	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/bot"
	"github.com/urfave/cli/v3"
)

type App struct {
	Name    string
	Version string
	Config  *config.Config
	Client  *bot.Client
	DB      *wrap.DB
	Log     *xlog.Logger
	Net     struct {
		BaseURL      string // e.g., "https://example.com/"
		UserAgent    string // User-Agent string for network requests
		Server       *xhttp.Server
		RedditQueue  *workqueue.Queue
		YoutubeQueue *workqueue.Queue
		SettingsAuth *auth.Manager
		DownloadAuth *auth.Manager
	}
	Paths struct {
		Storage string // (e.g., ~/.appName)
		Runtime string // (e.g., XDG_RUNTIME_DIR/name, fallback to /tmp/name-USER)
	}
	Cleanup     []func() error
	PostCleanup func() error
	PostSetOnce sync.Once
	CloseOnce   sync.Once
	Context     context.Context
}

func (a *App) Init(ctx context.Context, cmd *cli.Command, name, version string) (context.Context, error) {
	a.Name = name
	a.Version = version
	a.Net.UserAgent = fmt.Sprintf(
		"Mozilla/5.0 (compatible; Halsey/%s; +https://halsey.regfile.net)",
		strings.TrimPrefix(version, "v"),
	)

	// paths
	var err error
	if a.Paths.Storage, err = getStoragePath(a.Name); err != nil {
		return nil, err
	}
	if a.Paths.Runtime, err = getRuntimePath(a.Name); err != nil {
		return nil, err
	}

	// migration guard before touching anything
	if !cmd.Bool("migrate") {
		if err := a.Mguard(); err != nil {
			return ctx, fmt.Errorf("failed to setup migration guard: %w", err)
		}
	} else {
		fmt.Printf("%s version %s\n", name, version)
	}

	// logger
	initLogLevel := x.Ternary(cmd.String("log") == "debug", "debug", "none")
	a.Log, err = xlog.New(filepath.Join(a.Paths.Storage, "logs"), initLogLevel)
	if err != nil {
		return ctx, fmt.Errorf("failed to initialize logger: %w", err)
	}
	a.AddCleanup(a.Log.Close)

	a.Log.Debugf("Starting %s, version: %s, storage path: %s, runtime path: %s",
		a.Name, a.Version, a.Paths.Storage, a.Paths.Runtime)

	// database
	a.DB, _, err = wrap.New(filepath.Join(a.Paths.Storage, "db"), database.DBINameList)
	if err != nil {
		if a.DB != nil {
			a.DB.Close()
		}
		return ctx, fmt.Errorf("failed to initialize database: %w", err)
	}
	a.AddCleanup(func() error {
		a.DB.Close()
		return nil
	})
	a.Log.Debug(ctx, "Database initialized")

	// config
	a.Config, err = config.Init(a.DB)
	if err != nil {
		return ctx, fmt.Errorf("failed to initialize config: %w", err)
	}
	a.Log.Debug(ctx, "Config initialized")

	// calculate BaseURL
	if a.Net.BaseURL, err = getBaseURL(a.Config); err != nil {
		return ctx, fmt.Errorf("failed to get base URL: %w", err)
	}
	a.Log.Debugf("Base URL: %s", a.Net.BaseURL)

	// set log level
	if initLogLevel != "debug" {
		cfgLogLevel, err := config.Get[string](a.Config, "logLevel")
		if err != nil {
			return ctx, fmt.Errorf("failed to get log level from config: %w", err)
		}
		if err := a.Log.SetLevel(cfgLogLevel); err != nil {
			return ctx, fmt.Errorf("failed to set log level: %w", err)
		}
	}
	// put logger into context
	ctx = xlog.IntoContext(ctx, a.Log)

	// daily update check / notification
	if err := a.Notify(); err != nil {
		a.Log.Errorf("Update notification failed: %v", err)
	}

	// init Hardware Acceleration
	compress.InitHWAccel(ctx)

	// init auth managers
	a.Net.SettingsAuth = auth.New(nil, nil)
	a.Net.DownloadAuth = auth.New(nil, nil)

	// init Queues
	a.Net.RedditQueue = workqueue.New(3*time.Second, 2*time.Second)
	a.Net.YoutubeQueue = workqueue.New(3*time.Second, 2*time.Second)
	a.AddCleanup(func() error {
		a.Net.RedditQueue.Close()
		a.Net.YoutubeQueue.Close()
		return nil
	})

	a.Context = ctx
	return ctx, nil
}

func (a *App) AddCleanup(f func() error) {
	a.Cleanup = append(a.Cleanup, f)
}

func (a *App) SetPostCleanup(f func() error) {
	a.PostSetOnce.Do(func() {
		a.PostCleanup = f
	})
}

func (a *App) Close() {
	a.CloseOnce.Do(func() {
		// call cleanup funcs in reverse order
		for i := len(a.Cleanup) - 1; i >= 0; i-- {
			if err := a.Cleanup[i](); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to clean up: %v\n", err)
			}
		}
		// call post cleanup func if set
		if a.PostCleanup != nil {
			time.Sleep(1 * time.Second) // just to be safe
			if err := a.PostCleanup(); err != nil {
				fmt.Fprintf(os.Stderr, "Post cleanup failure: %v\n", err)
			}
		}
	})
}

// getStoragePath calculates the storage path for the application (~/.appName).
func getStoragePath(appName string) (string, error) {
	// non-root: use current user's home.
	if os.Geteuid() != 0 {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home dir: %w", err)
		}
		return filepath.Join(home, "."+appName), nil
	}

	// root: require an invoking non-root user (sudo/doas).
	home, err := invokingUserHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "."+appName), nil
}

// getRuntimePath calculates the runtime path for the application.
// Prefers XDG_RUNTIME_DIR, falls back to /tmp/appName-USER.
func getRuntimePath(appName string) (string, error) {
	// prefer XDG_RUNTIME_DIR (typically /run/user/UID)
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, appName), nil
	}

	// fallback for non-systemd systems
	// include username to avoid conflicts in shared /tmp
	username := os.Getenv("USER")
	if username == "" {
		u, err := user.Current()
		if err != nil {
			return "", fmt.Errorf("cannot determine current user: %w", err)
		}
		username = u.Username
	}

	return filepath.Join("/tmp", appName+"-"+username), nil
}

func invokingUserHome() (string, error) {
	// prefer UID (avoids name ambiguities).
	if uid := firstNonEmpty(os.Getenv("SUDO_UID"), os.Getenv("DOAS_UID")); uid != "" && uid != "0" {
		u, err := user.LookupId(uid)
		if err != nil {
			return "", fmt.Errorf("cannot lookup uid %s: %w", uid, err)
		}
		if u.HomeDir == "" {
			return "", fmt.Errorf("empty home for uid %s", uid)
		}
		return u.HomeDir, nil
	}

	// fallback to username if UID not present.
	if uname := firstNonEmpty(os.Getenv("SUDO_USER"), os.Getenv("DOAS_USER")); uname != "" {
		u, err := user.Lookup(uname)
		if err != nil {
			return "", fmt.Errorf("cannot lookup user %q: %w", uname, err)
		}
		if u.Uid == "0" {
			return "", errors.New("invoking user resolves to root; aborting")
		}
		if u.HomeDir == "" {
			return "", fmt.Errorf("empty home for user %q", uname)
		}
		return u.HomeDir, nil
	}

	return "", errors.New("refusing to run as real root: no SUDO_*/DOAS_* env present")
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

func getBaseURL(c *config.Config) (string, error) {
	port, err := config.Get[int](c, "port")
	if err != nil {
		return "", fmt.Errorf("failed to get port from config: %w", err)
	}
	host, err := config.Get[string](c, "host")
	if err != nil {
		return "", fmt.Errorf("failed to get host from config: %w", err)
	}
	proxyPort, err := config.Get[int](c, "proxyPort")
	if err != nil {
		return "", fmt.Errorf("failed to get proxyPort from config: %w", err)
	}

	host = x.Ternary(host != "", host, "localhost")
	port = x.Ternary(proxyPort != 0, proxyPort, port)
	hidePort := port == 80 || port == 443
	scheme := x.Ternary(port == 443, "https", "http")
	baseURL := fmt.Sprintf("%s://%s%s", scheme, host, x.Ternary(hidePort, "", fmt.Sprintf(":%d", port)))
	return baseURL, nil
}
