package keystore

const (
	ErrTextKeyNotFound             = "key not found"
	ErrTextKeyWithAddressNotFound  = "key with address not found"
	ErrTextAddressExists           = "account with address already exists in keyring"
	ErrTextPubkeyExists            = "public key already exists in keystore"
	ErrTextUnknownKeyringBackend   = "unknown keyring backend"
	ErrTextFailedToRead            = "failed to read"
	ErrTextFailedToOpen            = "failed to open"
	ErrTextTooManyWrongPassphrases = "too many failed passphrase attempts"
	ErrTextIncorrectPassphrase     = "incorrect passphrase"
	ErrTextPassphraseDoNotMatch    = "passphrase do not match"
)
