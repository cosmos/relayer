package relayer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	bank "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// SendMsgWithKey allows the user to specify which relayer key will sign the message
func (c *Chain) SendMsgWithKey(msg sdk.Msg, keyName string) (res *sdk.TxResponse, err error) {
	fmt.Println("setting use of key", keyName)
	c.Key = keyName
	res, _, err = c.SendMsg(msg)
	return res, err

}

// FaucetHandler listens for addresses
func (c *Chain) FaucetHandler(fromKey sdk.AccAddress, amounts sdk.Coins) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		c.Log("handling faucet request...")

		byt, err := ioutil.ReadAll(r.Body)
		if err != nil {
			str := "Failed to read request body"
			c.Error(fmt.Errorf("%s: %w", str, err))
			respondWithError(w, http.StatusBadGateway, str)
			return
		}

		var fr FaucetRequest
		err = json.Unmarshal(byt, &fr)
		switch {
		case err != nil:
			str := fmt.Sprintf("Failed to unmarshal request payload: %s", string(byt))
			c.Log(str)
			respondWithError(w, http.StatusBadRequest, str)
			return
		case fr.ChainID != c.ChainID:
			str := fmt.Sprintf("Invalid chain id: exp(%s) got(%s)", c.ChainID, fr.ChainID)
			c.Log(str)
			respondWithError(w, http.StatusBadRequest, str)
			return
		}

		if wait, err := c.checkAddress(fr.Address); err != nil {
			c.Log(fmt.Sprintf("%s hit rate limit, needs to wait %s", fr.Address, wait.String()))
			respondWithError(w, http.StatusTooManyRequests, err.Error())
			return
		}

		if err := c.faucetSend(fromKey, fr.addr(), amounts); err != nil {
			c.Error(err)
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		c.Log(fmt.Sprintf("%s was sent %s successfully", fr.Address, amounts.String()))
		respondWithJSON(w, http.StatusCreated, success{Address: fr.Address, Amount: amounts.String()})
	}
}

func (c *Chain) faucetSend(fromAddr, toAddr sdk.AccAddress, amounts sdk.Coins) error {
	// Set sdk config to use custom Bech32 account prefix

	info, err := c.Keybase.KeyByAddress(fromAddr)
	if err != nil {
		return err
	}

	fmt.Println("From Address", fromAddr.String())
	fmt.Println("To Address", toAddr.String())
	fmt.Println("Amount", amounts)
	res, err := c.SendMsgWithKey(bank.NewMsgSend(fromAddr, toAddr, sdk.NewCoins(amounts...)), info.GetName())

	if err != nil {
		return fmt.Errorf("%w:%s", err, res.RawLog)
	} else if res.Code != 0 {
		return fmt.Errorf("%s", res.RawLog)
	}
	return nil
}

func (c *Chain) checkAddress(addr string) (time.Duration, error) {
	faucetTimeout := 5 * time.Minute
	if val, ok := c.faucetAddrs[addr]; ok {
		sinceLastRequest := time.Since(val)
		if faucetTimeout > sinceLastRequest {
			wait := faucetTimeout - sinceLastRequest
			return wait, fmt.Errorf("%s has requested funds within the last %s, wait %s before trying again",
				addr, faucetTimeout.String(), wait.String())
		}
	}
	c.faucetAddrs[addr] = time.Now()
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
