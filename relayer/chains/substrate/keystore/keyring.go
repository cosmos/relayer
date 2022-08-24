package keystore

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ComposableFi/go-substrate-rpc-client/v4/signature"

	"github.com/99designs/keyring"
	"github.com/cosmos/cosmos-sdk/client/input"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

// Backend options for Keyring
const (
	BackendFile   = "file"
	BackendTest   = "test"
	BackendMemory = "memory"

	infoSuffix = "info"
)

const (
	keyringFileDirName = "keyring-file"
	keyringTestDirName = "keyring-test"
)

const (
	maxPassphraseEntryAttempts = 3
)

type keystore struct {
	db keyring.Keyring
}

// Keyring exposes operations over a backend supported by github.com/99designs/keyring.
type Keyring interface {
	// List returns a list of all stored keys.
	List() ([]Info, error)

	// Key fetches and returns a key using its uid.
	Key(uid string) (Info, error)

	// Delete remove keys from the keyring.
	Delete(uid string) error

	// NewAccount converts a mnemonic to a private key and BIP-39 HD Path and persists it.
	// It fails if there is an existing key Info with the same address.
	NewAccount(name, mnemonic string, network uint16) (Info, error)
}

func New(
	appName, backend string, rootDir string, userInput io.Reader,
) (Keyring, error) {
	var (
		db  keyring.Keyring
		err error
	)

	switch backend {
	case BackendMemory:
		return NewInMemory(), err
	case BackendTest:
		db, err = keyring.Open(newTestBackendKeyringConfig(appName, rootDir))
	case BackendFile:
		db, err = keyring.Open(newFileBackendKeyringConfig(appName, rootDir, userInput))
	default:
		return nil, fmt.Errorf("%s: %s", ErrTextUnknownKeyringBackend, backend)
	}

	if err != nil {
		return nil, err
	}

	return newKeystore(db), nil
}

// NewInMemory creates a transient keyring useful for testing
// purposes and on-the-fly key generation.
// Keybase options can be applied when generating this new Keybase.
func NewInMemory() Keyring {
	return newKeystore(keyring.NewArrayKeyring(nil))
}

func newTestBackendKeyringConfig(appName, dir string) keyring.Config {
	return keyring.Config{
		AllowedBackends: []keyring.BackendType{keyring.FileBackend},
		ServiceName:     appName,
		FileDir:         filepath.Join(dir, keyringTestDirName),
		FilePasswordFunc: func(_ string) (string, error) {
			return "test", nil
		},
	}
}

func newFileBackendKeyringConfig(name, dir string, buf io.Reader) keyring.Config {
	fileDir := filepath.Join(dir, keyringFileDirName)

	return keyring.Config{
		AllowedBackends:  []keyring.BackendType{keyring.FileBackend},
		ServiceName:      name,
		FileDir:          fileDir,
		FilePasswordFunc: newRealPrompt(fileDir, buf),
	}
}

func newRealPrompt(dir string, buf io.Reader) func(string) (string, error) {
	return func(prompt string) (string, error) {
		keyhashStored := false
		keyhashFilePath := filepath.Join(dir, "keyhash")

		var keyhash []byte

		_, err := os.Stat(keyhashFilePath)

		switch {
		case err == nil:
			keyhash, err = ioutil.ReadFile(keyhashFilePath)
			if err != nil {
				return "", fmt.Errorf("%s: %s: %s", ErrTextFailedToRead, keyhashFilePath, err)
			}

			keyhashStored = true

		case os.IsNotExist(err):
			keyhashStored = false

		default:
			return "", fmt.Errorf("%s: %s: %s", ErrTextFailedToOpen, keyhashFilePath, err)
		}

		failureCounter := 0

		for {
			failureCounter++
			if failureCounter > maxPassphraseEntryAttempts {
				return "", fmt.Errorf(ErrTextTooManyWrongPassphrases)
			}

			buf := bufio.NewReader(buf)
			pass, err := input.GetPassword("Enter keyring passphrase:", buf)
			if err != nil {
				// NOTE: LGTM.io reports a false positive alert that states we are printing the password,
				// but we only log the error.
				//
				// lgtm [go/clear-text-logging]
				fmt.Fprintln(os.Stderr, err)
				continue
			}

			if keyhashStored {
				if err := bcrypt.CompareHashAndPassword(keyhash, []byte(pass)); err != nil {
					fmt.Fprintln(os.Stderr, ErrTextIncorrectPassphrase)
					continue
				}

				return pass, nil
			}

			reEnteredPass, err := input.GetPassword("Re-enter keyring passphrase:", buf)
			if err != nil {
				// NOTE: LGTM.io reports a false positive alert that states we are printing the password,
				// but we only log the error.
				//
				// lgtm [go/clear-text-logging]
				fmt.Fprintln(os.Stderr, err)
				continue
			}

			if pass != reEnteredPass {
				fmt.Fprintln(os.Stderr, ErrTextPassphraseDoNotMatch)
				continue
			}

			passwordHash, err := bcrypt.GenerateFromPassword([]byte(pass), 2)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}

			if err := ioutil.WriteFile(keyhashFilePath, passwordHash, 0555); err != nil {
				return "", err
			}

			return pass, nil
		}
	}
}

func newKeystore(kr keyring.Keyring) keystore {
	return keystore{kr}
}

func (ks keystore) key(infoKey string) (Info, error) {
	bs, err := ks.db.Get(infoKey)
	if err != nil {
		return nil, errors.Errorf("%s: %s", ErrTextKeyNotFound, infoKey)
	}
	if len(bs.Data) == 0 {
		return nil, errors.Errorf("%s: %s", ErrTextKeyNotFound, infoKey)
	}
	return unmarshalInfo(bs.Data)
}

func (ks keystore) Delete(name string) error {
	info, err := ks.Key(name)
	if err != nil {
		return err
	}

	err = ks.db.Remove(info.GetAddress())
	if err != nil {
		return err
	}

	err = ks.db.Remove(infoKey(name))
	if err != nil {
		return err
	}

	return nil
}

func (ks keystore) List() ([]Info, error) {
	var res []Info

	keys, err := ks.db.Keys()
	if err != nil {
		return nil, err
	}

	sort.Strings(keys)

	for _, key := range keys {
		if !strings.HasSuffix(key, infoSuffix) {
			continue
		}
		rawInfo, err := ks.db.Get(key)
		if err != nil {
			return nil, err
		}

		if len(rawInfo.Data) == 0 {
			return nil, errors.Errorf("%s: %s", ErrTextKeyNotFound, key)
		}

		info, err := unmarshalInfo(rawInfo.Data)
		if err != nil {
			return nil, err
		}

		res = append(res, info)
	}

	return res, nil
}

func (ks keystore) Key(uid string) (Info, error) {
	infoKey := infoKey(uid)
	return ks.key(infoKey)
}

func (ks keystore) NewAccount(name string, mnemonic string, network uint16) (Info, error) {
	kp, err := signature.KeyringPairFromSecret(mnemonic, network)
	if err != nil {
		return nil, fmt.Errorf("failed to create keypair: %w", err)
	}

	if _, err := ks.KeyByAddress(kp.Address); err == nil {
		return nil, fmt.Errorf("%s: %s", ErrTextAddressExists, kp.Address)
	}

	return ks.writeLocalKey(name, kp)
}

func (ks keystore) writeLocalKey(name string, keypair signature.KeyringPair) (Info, error) {
	info := newLocalInfo(name, keypair)
	if err := ks.writeInfo(info); err != nil {
		return nil, err
	}

	return info, nil
}

func (ks keystore) writeInfo(info Info) error {
	key := infoKeyBz(info.GetName())
	serializedInfo, err := marshalInfo(info)
	if err != nil {
		return err
	}

	exists, err := ks.existsInDb(info)
	if err != nil {
		return err
	}
	if exists {
		return errors.New(ErrTextPubkeyExists)
	}

	err = ks.db.Set(keyring.Item{
		Key:  string(key),
		Data: serializedInfo,
	})
	if err != nil {
		return err
	}

	err = ks.db.Set(keyring.Item{
		Key:  info.GetAddress(),
		Data: key,
	})
	if err != nil {
		return err
	}

	return nil
}

// existsInDb returns true if key is in DB. Error is returned only when we have error
// different thant ErrKeyNotFound
func (ks keystore) existsInDb(info Info) (bool, error) {

	if _, err := ks.db.Get(info.GetAddress()); err == nil {
		return true, nil // address lookup succeeds - info exists
	} else if err != keyring.ErrKeyNotFound {
		return false, err // received unexpected error - returns error
	}

	if _, err := ks.db.Get(infoKey(info.GetName())); err == nil {
		return true, nil // uid lookup succeeds - info exists
	} else if err != keyring.ErrKeyNotFound {
		return false, err // received unexpected error - returns
	}

	// both lookups failed, info does not exist
	return false, nil
}

func (ks keystore) KeyByAddress(address string) (Info, error) {
	ik, err := ks.db.Get(address)
	if err != nil {
		return nil, errors.Errorf("%s: %s", ErrTextKeyWithAddressNotFound, address)
	}

	if len(ik.Data) == 0 {
		return nil, errors.Errorf("%s: %s", ErrTextKeyWithAddressNotFound, address)
	}
	return ks.key(string(ik.Data))
}
