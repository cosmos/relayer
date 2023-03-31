package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/gofrs/flock"
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

// initConfig reads config file into a.Config if file is present.
func (a *appState) initConfig(ctx context.Context) error {
	cfgPath := path.Join(a.HomePath, "config", "config.yaml")
	if _, err := os.Stat(cfgPath); err != nil {
		// don't return error if file doesn't exist
		return nil
	}
	a.Viper.SetConfigFile(cfgPath)
	if err := a.Viper.ReadInConfig(); err != nil {
		return err
	}
	// read the config file bytes
	file, err := os.ReadFile(a.Viper.ConfigFileUsed())
	if err != nil {
		return fmt.Errorf("error reading file:", err)
	}

	// unmarshall them into the wrapper struct
	cfgWrapper := &ConfigInputWrapper{}
	err = yaml.Unmarshal(file, cfgWrapper)
	if err != nil {
		return fmt.Errorf("error unmarshalling config:", err)
	}

	// verify that the channel filter rule is valid for every path in the config
	for _, p := range cfgWrapper.Paths {
		if err := p.ValidateChannelFilterRule(); err != nil {
			return fmt.Errorf("error initializing the relayer config for path %s: %w", p.String(), err)
		}
	}

	// build the config struct
	chains := make(relayer.Chains)
	for chainName, pcfg := range cfgWrapper.ProviderConfigs {
		prov, err := pcfg.Value.(provider.ProviderConfig).NewProvider(
			a.Log.With(zap.String("provider_type", pcfg.Type)),
			a.HomePath, a.Debug, chainName,
		)
		if err != nil {
			return fmt.Errorf("failed to build ChainProviders: %w", err)
		}

		if err := prov.Init(ctx); err != nil {
			return fmt.Errorf("failed to initialize provider: %w", err)
		}

		chain := relayer.NewChain(a.Log, prov, a.Debug)
		chains[chainName] = chain
	}

	a.Config = &Config{
		Global: cfgWrapper.Global,
		Chains: chains,
		Paths:  cfgWrapper.Paths,
	}

	// ensure config has []*relayer.Chain used for all chain operations
	if err := validateConfig(a.Config); err != nil {
		return fmt.Errorf("error parsing chain config: %w", err)
	}

	return nil
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

func (a *appState) PerformConfigLockingOperation(ctx context.Context, operation func() error) error {
	lockFilePath := path.Join(a.HomePath, "config", "config.lock")
	fileLock := flock.New(lockFilePath)
	_, err := fileLock.TryLock()
	if err != nil {
		return fmt.Errorf("failed to acquire config lock: %w", err)
	}
	defer func() {
		if err := fileLock.Unlock(); err != nil {
			a.Log.Error("error unlocking config file lock, please manually delete",
				zap.String("filepath", lockFilePath),
			)
		}
	}()

	// load config from file and validate it. don't want to miss
	// any changes that may have been made while unlocked.
	if err := a.initConfig(ctx); err != nil {
		return fmt.Errorf("failed to initialize config from file: %w", err)
	}

	if err := operation(); err != nil {
		return err
	}

	if err := validateConfig(a.Config); err != nil {
		return fmt.Errorf("error parsing chain config: %w", err)
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

// OverwriteConfigOnTheFly overwrites the config file concurrently,
// locking to read, modify, then write the config.
func (a *appState) OverwriteConfigOnTheFly(
	ctx context.Context,
	pathName string,
	clientSrc, clientDst string,
	connectionSrc, connectionDst string,
) error {
	if pathName == "" {
		return errors.New("empty path name not allowed")
	}

	return a.PerformConfigLockingOperation(ctx, func() error {
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
		return nil
	})
}
