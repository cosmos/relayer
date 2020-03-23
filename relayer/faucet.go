package relayer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	ckeys "github.com/cosmos/cosmos-sdk/client/keys"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/bank"
)

// SendMsgWithKey allows the user to specify which relayer key will sign the message
func (src *Chain) SendMsgWithKey(datagram sdk.Msg, keyName string) (res sdk.TxResponse, err error) {
	var out []byte
	if out, err = src.BuildAndSignTxWithKey([]sdk.Msg{datagram}, keyName); err != nil {
		return res, err
	}
	return src.BroadcastTxCommit(out)

}

// BuildAndSignTxWithKey allows the user to specify which relayer key will sign the message
func (src *Chain) BuildAndSignTxWithKey(datagram []sdk.Msg, keyName string) ([]byte, error) {
	// Set sdk config to use custom Bech32 account prefix
	sdkConf := sdk.GetConfig()
	sdkConf.SetBech32PrefixForAccount(src.AccountPrefix, src.AccountPrefix+"pub")

	// Fetch account and sequence numbers for the account
	info, err := src.Keybase.Get(keyName)
	if err != nil {
		return nil, err
	}

	acc, err := auth.NewAccountRetriever(src.Cdc, src).GetAccount(info.GetAddress())
	if err != nil {
		return nil, err
	}

	return auth.NewTxBuilder(
		auth.DefaultTxEncoder(src.Amino), acc.GetAccountNumber(),
		acc.GetSequence(), src.Gas, src.GasAdjustment, false, src.ChainID,
		src.Memo, sdk.NewCoins(), src.getGasPrices()).WithKeybase(src.Keybase).
		BuildAndSign(info.GetName(), ckeys.DefaultKeyPass, datagram)
}

// FaucetHandler listens for addresses
func (src *Chain) FaucetHandler(fromKey sdk.AccAddress, amount sdk.Coin) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var fr FaucetRequest
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&fr); err != nil || fr.ChainID != src.ChainID {
			respondWithError(w, http.StatusBadRequest, "Invalid request payload")
			return
		}
		defer r.Body.Close()

		if wait, err := src.checkAddress(fr.Address); err != nil {
			src.Log(fmt.Sprintf("%s hit rate limit, needs to wait %s", fr.Address, wait.String()))
			respondWithError(w, http.StatusTooManyRequests, err.Error())
			return
		}

		if err := src.faucetSend(fromKey, fr.addr(), amount); err != nil {
			src.Error(err)
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		src.Log(fmt.Sprintf("%s was sent %s successfully", fr.Address, amount.String()))
		respondWithJSON(w, http.StatusCreated, success{Address: fr.Address, Amount: amount.String()})
	}
}

func (src *Chain) faucetSend(fromAddr, toAddr sdk.AccAddress, amount sdk.Coin) error {
	// Set sdk config to use custom Bech32 account prefix
	sdkConf := sdk.GetConfig()
	sdkConf.SetBech32PrefixForAccount(src.AccountPrefix, src.AccountPrefix+"pub")

	info, err := src.Keybase.GetByAddress(fromAddr)
	if err != nil {
		return err
	}
	res, err := src.SendMsgWithKey(bank.NewMsgSend(fromAddr, toAddr, sdk.NewCoins(amount)), info.GetName())
	if err != nil || res.Code != 0 {
		cs, err := GetCodespace(res.Codespace, int(res.Code))
		if err != nil {
			return err
		}
		return fmt.Errorf("failed to send transaction: %w\ncodespaceErr(%s)\n%s", err, cs, res.String())
	}
	return nil
}

func (src *Chain) checkAddress(addr string) (time.Duration, error) {
	faucetTimeout := 5 * time.Minute
	if val, ok := src.faucetAddrs[addr]; ok {
		sinceLastRequest := time.Since(val)
		if faucetTimeout > sinceLastRequest {
			wait := (faucetTimeout - sinceLastRequest)
			return wait, fmt.Errorf("%s has requested funds within the last %s, wait %s before trying again", addr, faucetTimeout.String(), wait.String())
		}
	}
	src.faucetAddrs[addr] = time.Now()
	return 1 * time.Second, nil
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, err := w.Write(response)
	if err != nil {
		fmt.Printf("error writing to the underlying response")
	}
}

// FaucetRequest represents a request to the facuet
type FaucetRequest struct {
	ChainID string `json:"chain-id"`
	Address string `json:"address"`
}

func (fr FaucetRequest) addr() sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(fr.Address)
	return addr
}

type success struct {
	Address string `json:"address"`
	Amount  string `json:"amount"`
}
