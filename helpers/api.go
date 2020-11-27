package helpers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/cosmos/relayer/relayer"
	"github.com/gogo/protobuf/proto"
)

// SuccessJSONResponse prepares data and writes a HTTP success
func SuccessJSONResponse(status int, v interface{}, w http.ResponseWriter) {
	out, err := json.Marshal(v)
	if err != nil {
		WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	WriteSuccessResponse(status, out, w)
}

// SuccessProtoResponse prepares data and writes a HTTP success
func SuccessProtoResponse(status int, chain *relayer.Chain, v proto.Message, w http.ResponseWriter) {
	out, err := chain.Encoding.Marshaler.MarshalJSON(v)
	if err != nil {
		WriteErrorResponse(http.StatusInternalServerError, err, w)
		return
	}
	WriteSuccessResponse(status, out, w)
}

// WriteSuccessResponse writes a HTTP success given a status code and data
func WriteSuccessResponse(statusCode int, data []byte, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(statusCode)

	w.Write(data)
}

type errorResponse struct {
	Err string `json:"err"`
}

// TODO: do we need better errors
// errors for things like:
// - out of funds
// - transaction errors
// Lets utilize the codec to make these error returns
// useful to users and allow them to take proper action

// WriteErrorResponse writes a HTTP error given a status code and an error message
func WriteErrorResponse(statusCode int, err error, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(statusCode)

	_ = json.NewEncoder(w).Encode(errorResponse{
		Err: err.Error(),
	})

	return
}

// ParseHeightFromRequest parse height from query params and if not found, returns latest height
func ParseHeightFromRequest(r *http.Request, chain *relayer.Chain) (int64, error) {
	heightStr := r.URL.Query().Get("height")

	if len(heightStr) == 0 {
		height, err := chain.QueryLatestHeight()
		if err != nil {
			return 0, err
		}
		return height, nil
	}

	height, err := strconv.ParseInt(heightStr, 10, 64) //convert to int64
	if err != nil {
		return 0, err
	}
	return height, nil
}

// ParsePaginationParams parse limit and offset query params in request
func ParsePaginationParams(r *http.Request) (uint64, uint64, error) {
	offsetStr := r.URL.Query().Get("offset")
	limitStr := r.URL.Query().Get("limit")

	var offset, limit uint64
	var err error

	if len(offsetStr) != 0 {
		offset, err = strconv.ParseUint(offsetStr, 10, 64) //convert to int64
		if err != nil {
			return offset, limit, err
		}
	}

	if len(limitStr) != 0 {
		limit, err = strconv.ParseUint(limitStr, 10, 64) //convert to int64
		if err != nil {
			return offset, limit, err
		}
	}

	return offset, limit, nil
}
