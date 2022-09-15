package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/cosmos/relayer/v2/relayer"
	"github.com/juju/fslock"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// appState is the modifiable state of the application.
type appState struct {
	// Log is the root logger of the application.
	// Consumers are expected to store and use local copies of the logger
	// after modifying with the .With method.
	Log *zap.Logger

	Viper *viper.Viper

	HomePath string
	Debug    bool
	Config   *Config
}

// AddPathFromFile modifies a.config.Paths to include the content stored in the given file.
// If a non-nil error is returned, a.config.Paths is not modified.
func (a *appState) AddPathFromFile(ctx context.Context, stderr io.Writer, file, name string) error {
	if _, err := os.Stat(file); err != nil {
		return err
	}

	byt, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	p := &relayer.Path{}
	if err = json.Unmarshal(byt, &p); err != nil {
		return err
	}

	if err = a.Config.ValidatePath(ctx, stderr, p); err != nil {
		return err
	}

	return a.Config.Paths.Add(name, p)
}

// AddPathFromUserInput manually prompts the user to specify all the path details.
// It returns any input or validation errors.
// If the path was successfully added, it returns nil.
func (a *appState) AddPathFromUserInput(ctx context.Context, stdin io.Reader, stderr io.Writer, src, dst, name string) error {
	// TODO: confirm name is available before going through input.

	var (
		value string
		err   error
		path  = &relayer.Path{
			Src: &relayer.PathEnd{
				ChainID: src,
			},
			Dst: &relayer.PathEnd{
				ChainID: dst,
			},
		}
	)

	fmt.Fprintf(stderr, "enter src(%s) client-id...\n", src)
	if value, err = readLine(stdin); err != nil {
		return err
	}

	path.Src.ClientID = value

	if err = path.Src.Vclient(); err != nil {
		return err
	}

	fmt.Fprintf(stderr, "enter src(%s) connection-id...\n", src)
	if value, err = readLine(stdin); err != nil {
		return err
	}

	path.Src.ConnectionID = value

	if err = path.Src.Vconn(); err != nil {
		return err
	}

	fmt.Fprintf(stderr, "enter dst(%s) client-id...\n", dst)
	if value, err = readLine(stdin); err != nil {
		return err
	}

	path.Dst.ClientID = value

	if err = path.Dst.Vclient(); err != nil {
		return err
	}

	fmt.Fprintf(stderr, "enter dst(%s) connection-id...\n", dst)
	if value, err = readLine(stdin); err != nil {
		return err
	}

	path.Dst.ConnectionID = value

	if err = path.Dst.Vconn(); err != nil {
		return err
	}

	if err := a.Config.ValidatePath(ctx, stderr, path); err != nil {
		return err
	}

	return a.Config.Paths.Add(name, path)
}

// OverwriteConfig overwrites the config files on disk with the serialization of cfg,
// and it replaces a.Config with cfg.
//
// It is possible to use a brand new Config argument,
// but typically the argument is a.Config.
func (a *appState) OverwriteConfig(cfg *Config) error {
	cfgPath := path.Join(a.HomePath, "config", "config.yaml")
	if _, err := os.Stat(cfgPath); err != nil {
		return fmt.Errorf("failed to check existence of config file at %s: %w", cfgPath, err)
	}

	a.Viper.SetConfigFile(cfgPath)
	if err := a.Viper.ReadInConfig(); err != nil {
		// TODO: if we failed to read in the new config, should we restore the old config?
		return fmt.Errorf("failed to read config file at %s: %w", cfgPath, err)
	}

	// ensure validateConfig runs properly
	if err := validateConfig(cfg); err != nil {
		return fmt.Errorf("failed to validate config at %s: %w", cfgPath, err)
	}

	// marshal the new config
	out, err := yaml.Marshal(cfg.Wrapped())
	if err != nil {
		return err
	}

	// Overwrite the config file.
	if err := os.WriteFile(a.Viper.ConfigFileUsed(), out, 0600); err != nil {
		return fmt.Errorf("failed to write config file at %s: %w", cfgPath, err)
	}

	// Write the config back into the app state.
	a.Config = cfg
	return nil
}

// OverwriteConfigOnTheFly overwrites the config file concurrently,
// locking to read, modify, then write the config.
func (a *appState) OverwriteConfigOnTheFly(
	cmd *cobra.Command,
	pathName string,
	clientSrc, clientDst string,
	connectionSrc, connectionDst string,
) error {
	if pathName == "" {
		return errors.New("empty path name not allowed")
	}

	// use lock file to guard concurrent access to config.yaml
	lockFilePath := path.Join(a.HomePath, "config", "config.lock")
	lock := fslock.New(lockFilePath)
	err := lock.LockWithTimeout(10 * time.Second)
	if err != nil {
		return fmt.Errorf("failed to acquire config lock: %w", err)
	}
	defer func() {
		if err := lock.Unlock(); err != nil {
			a.Log.Error("error unlocking config file lock, please manually delete",
				zap.String("filepath", lockFilePath),
			)
		}
	}()

	// load config from file and validate it. don't want to miss
	// any changes that may have been made while unlocked.
	if err := initConfig(cmd, a); err != nil {
		return fmt.Errorf("failed to initialize config from file: %w", err)
	}

	path, ok := a.Config.Paths[pathName]
	if !ok {
		return fmt.Errorf("config does not exist for that path: %s", pathName)
	}
	if clientSrc != "" {
		path.Src.ClientID = clientSrc
	}
	if clientDst != "" {
		path.Dst.ClientID = clientDst
	}
	if connectionSrc != "" {
		path.Src.ConnectionID = connectionSrc
	}
	if connectionDst != "" {
		path.Dst.ConnectionID = connectionDst
	}

	// marshal the new config
	out, err := yaml.Marshal(a.Config.Wrapped())
	if err != nil {
		return err
	}

	cfgPath := a.Viper.ConfigFileUsed()

	// Overwrite the config file.
	if err := os.WriteFile(cfgPath, out, 0600); err != nil {
		return fmt.Errorf("failed to write config file at %s: %w", cfgPath, err)
	}

	return nil
}
