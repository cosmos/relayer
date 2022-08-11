package keystore

const (
	ErrTextKeyNotFound             = "key not found: %v"
	ErrTextKeyWithAddressNotFound  = "key with address %s not found"
	ErrTextAddressExists           = "account with address %s already exists in keyring, delete the key first if you want to recreate it"
	ErrTextPubkeyExists            = "public key already exists in keybase"
	ErrTextUnknownKeyringBackend   = "unknown keyring backend %v"
	ErrTextFailedToRead            = "failed to read %s: %v"
	ErrTextFailedToOpen            = "failed to open %s: %v"
	ErrTextTooManyWrongPassphrases = "too many failed passphrase attempts"
	ErrTextIncorrectPassphrase     = "incorrect passphrase"
	ErrTextPassphraseDoNotMatch    = "passphrase do not match"
)
