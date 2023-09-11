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
	log *zap.Logger

	viper *viper.Viper

	homePath string
	debug    bool
	config   *Config
}

func (a *appState) configPath() string {
	return path.Join(a.homePath, "config", "config.yaml")
}

// loadConfigFile reads config file into a.Config if file is present.
func (a *appState) loadConfigFile(ctx context.Context) error {
	cfgPath := a.configPath()

	if _, err := os.Stat(cfgPath); err != nil {
		// don't return error if file doesn't exist
		return nil
	}

	// read the config file bytes
	file, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	// unmarshall them into the wrapper struct
	cfgWrapper := &ConfigInputWrapper{}
	err = yaml.Unmarshal(file, cfgWrapper)
	if err != nil {
		return fmt.Errorf("error unmarshalling config: %w", err)
	}

	// retrieve the runtime configuration from the disk configuration.
	newCfg, err := cfgWrapper.RuntimeConfig(ctx, a)
	if err != nil {
		return err
	}

	// validate runtime configuration
	if err := newCfg.validateConfig(); err != nil {
		return fmt.Errorf("error parsing chain config: %w", err)
	}

	// save runtime configuration in app state
	a.config = newCfg

	return nil
}

// addPathFromFile modifies a.config.Paths to include the content stored in the given file.
// If a non-nil error is returned, a.config.Paths is not modified.
func (a *appState) addPathFromFile(ctx context.Context, stderr io.Writer, file, name string) error {
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

	if err = a.config.ValidatePath(ctx, stderr, p); err != nil {
		return err
	}

	return a.config.AddPath(name, p)
}

// addPathFromUserInput manually prompts the user to specify all the path details.
// It returns any input or validation errors.
// If the path was successfully added, it returns nil.
func (a *appState) addPathFromUserInput(
	ctx context.Context,
	stdin io.Reader,
	stderr io.Writer,
	src, dst, name string,
) error {
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

	if err := a.config.ValidatePath(ctx, stderr, path); err != nil {
		return err
	}

	return a.config.AddPath(name, path)
}

func (a *appState) performConfigLockingOperation(ctx context.Context, operation func() error) error {
	lockFilePath := path.Join(a.homePath, "config", "config.lock")
	fileLock := flock.New(lockFilePath)
	_, err := fileLock.TryLock()
	if err != nil {
		return fmt.Errorf("failed to acquire config lock: %w", err)
	}
	defer func() {
		if err := fileLock.Unlock(); err != nil {
			a.log.Error("error unlocking config file lock, please manually delete",
				zap.String("filepath", lockFilePath),
			)
		}
	}()

	// load config from file and validate it. don't want to miss
	// any changes that may have been made while unlocked.
	if err := a.loadConfigFile(ctx); err != nil {
		return fmt.Errorf("failed to initialize config from file: %w", err)
	}

	// perform the operation that requires config flock.
	if err := operation(); err != nil {
		return err
	}

	// validate config after changes have been made.
	if err := a.config.validateConfig(); err != nil {
		return fmt.Errorf("error parsing chain config: %w", err)
	}

	// marshal the new config
	out, err := yaml.Marshal(a.config.Wrapped())
	if err != nil {
		return err
	}

	cfgPath := a.configPath()

	// Overwrite the config file.
	if err := os.WriteFile(cfgPath, out, 0600); err != nil {
		return fmt.Errorf("failed to write config file at %s: %w", cfgPath, err)
	}

	return nil
}

// updatePathConfig overwrites the config file concurrently,
// locking to read, modify, then write the config.
func (a *appState) updatePathConfig(
	ctx context.Context,
	pathName string,
	clientSrc, clientDst string,
	connectionSrc, connectionDst string,
) error {
	if pathName == "" {
		return errors.New("empty path name not allowed")
	}

	return a.performConfigLockingOperation(ctx, func() error {
		path, ok := a.config.Paths[pathName]
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

func (a *appState) useKey(chainName, key string) error {

	chain, exists := a.config.Chains[chainName]
	if !exists {
		return fmt.Errorf("chain %s not found in config", chainName)
	}

	cc := chain.ChainProvider

	info, err := cc.ListAddresses()
	if err != nil {
		return err
	}
	value, exists := info[key]
	currentValue := a.config.Chains[chainName].ChainProvider.Key()

	if currentValue == key {
		return fmt.Errorf("config is already using %s -> %s for %s", key, value, cc.ChainName())
	}

	if exists {
		fmt.Printf("Config will now use  %s -> %s  for %s\n", key, value, cc.ChainName())
	} else {
		return fmt.Errorf("key %s does not exist for chain %s", key, cc.ChainName())
	}
	return a.performConfigLockingOperation(context.Background(), func() error {
		a.config.Chains[chainName].ChainProvider.UseKey(key)
		return nil
	})
}

func (a *appState) useRpcAddr(chainName string, rpcAddr string) error {

	_, exists := a.config.Chains[chainName]
	if !exists {
		return fmt.Errorf("chain %s not found in config", chainName)
	}

	return a.performConfigLockingOperation(context.Background(), func() error {
		a.config.Chains[chainName].ChainProvider.SetRpcAddr(rpcAddr)
		return nil
	})
}
