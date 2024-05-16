package relayer_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/cosmos/relayer/v2/relayer"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestRelayMsgs_IsMaxTx(t *testing.T) {
	rm := relayer.RelayMsgs{
		MaxTxSize:    10,
		MaxMsgLength: 10,
	}
	require.True(t, rm.IsMaxTx(1, 11), "only exceeded tx size")
	require.True(t, rm.IsMaxTx(11, 1), "only exceeded message length")
	require.False(t, rm.IsMaxTx(4, 5), "neither exceeded")

	rm = relayer.RelayMsgs{
		MaxTxSize:    0,
		MaxMsgLength: 10,
	}
	require.True(t, rm.IsMaxTx(11, 1), "exceeded set max message length")
	require.False(t, rm.IsMaxTx(5, 100), "did not exceed set max message length")

	rm = relayer.RelayMsgs{
		MaxTxSize:    10,
		MaxMsgLength: 0,
	}
	require.True(t, rm.IsMaxTx(1, 11), "exceeded set max tx size")
	require.False(t, rm.IsMaxTx(100, 5), "did not exceed set max tx size")

	rm = relayer.RelayMsgs{
		MaxTxSize:    0,
		MaxMsgLength: 0,
	}
	require.False(t, rm.IsMaxTx(9999999, 99999999), "no limits to exceed")
}

// fakeRelayerMessage is a dummy implementation of provider.RelayerMessage.
type fakeRelayerMessage struct {
	t, b string
}

var _ provider.RelayerMessage = fakeRelayerMessage{}

func (m fakeRelayerMessage) Type() string {
	return m.t
}

func (m fakeRelayerMessage) MsgBytes() ([]byte, error) {
	return []byte(m.b), nil
}

func TestRelayMsgs_Send_Success(t *testing.T) {
	// Fixtures for test.
	// src appends to srcSent and dst appends to dstSent.
	var srcSent []provider.RelayerMessage
	src := relayer.RelayMsgSender{
		ChainID: "src",
		SendMessages: func(ctx context.Context, msgs []provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
			srcSent = append(srcSent, msgs...)
			return nil, true, nil
		},
	}

	var dstSent []provider.RelayerMessage
	dst := relayer.RelayMsgSender{
		ChainID: "dst",
		SendMessages: func(ctx context.Context, msgs []provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
			dstSent = append(dstSent, msgs...)
			return nil, true, nil
		},
	}

	srcMsg := fakeRelayerMessage{t: "srctype", b: "srcdata"}
	dstMsg := fakeRelayerMessage{t: "dsttype", b: "dstdata"}

	t.Run("sends in a single batch when there are no limits", func(t *testing.T) {
		// Clear state (in case this test is ever reordered).
		srcSent = nil
		dstSent = nil

		rm := relayer.RelayMsgs{
			Src: []provider.RelayerMessage{srcMsg},
			Dst: []provider.RelayerMessage{dstMsg},
		}

		result := rm.Send(context.Background(), zaptest.NewLogger(t), src, dst, "")
		require.Equal(t, result.SuccessfulSrcBatches, 1)
		require.Equal(t, result.SuccessfulDstBatches, 1)
		require.NoError(t, result.SrcSendError)
		require.NoError(t, result.DstSendError)

		require.Equal(t, []provider.RelayerMessage{srcMsg}, srcSent)
		require.Equal(t, []provider.RelayerMessage{dstMsg}, dstSent)
	})

	t.Run("sends all messages when max message length exceeded", func(t *testing.T) {
		// Clear state from previous test.
		srcSent = nil
		dstSent = nil

		rm := relayer.RelayMsgs{
			Src: []provider.RelayerMessage{srcMsg, srcMsg, srcMsg},
			Dst: []provider.RelayerMessage{dstMsg, dstMsg, dstMsg},

			MaxMsgLength: 2,
		}

		result := rm.Send(context.Background(), zaptest.NewLogger(t), src, dst, "")
		require.Equal(t, result.SuccessfulSrcBatches, 2)
		require.Equal(t, result.SuccessfulDstBatches, 2)
		require.NoError(t, result.SrcSendError)
		require.NoError(t, result.DstSendError)

		require.Equal(t, []provider.RelayerMessage{srcMsg, srcMsg, srcMsg}, srcSent)
		require.Equal(t, []provider.RelayerMessage{dstMsg, dstMsg, dstMsg}, dstSent)
	})

	t.Run("sends all messages when max tx size exceeded", func(t *testing.T) {
		// Clear state from previous test.
		srcSent = nil
		dstSent = nil

		rm := relayer.RelayMsgs{
			Src: []provider.RelayerMessage{srcMsg, srcMsg, srcMsg},
			Dst: []provider.RelayerMessage{dstMsg, dstMsg, dstMsg},

			MaxMsgLength: 2,
		}

		result := rm.Send(context.Background(), zaptest.NewLogger(t), src, dst, "")
		require.Equal(t, result.SuccessfulSrcBatches, 2)
		require.Equal(t, result.SuccessfulDstBatches, 2)
		require.NoError(t, result.SrcSendError)
		require.NoError(t, result.DstSendError)

		require.Equal(t, []provider.RelayerMessage{srcMsg, srcMsg, srcMsg}, srcSent)
		require.Equal(t, []provider.RelayerMessage{dstMsg, dstMsg, dstMsg}, dstSent)
	})
}

func TestRelayMsgs_Send_Errors(t *testing.T) {
	t.Run("one batch and one error", func(t *testing.T) {
		srcErr := fmt.Errorf("source error")
		src := relayer.RelayMsgSender{
			ChainID: "src",
			SendMessages: func(ctx context.Context, msgs []provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
				return nil, false, srcErr
			},
		}

		dstErr := fmt.Errorf("dest error")
		dst := relayer.RelayMsgSender{
			ChainID: "dst",
			SendMessages: func(ctx context.Context, msgs []provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
				return nil, false, dstErr
			},
		}

		srcMsg := fakeRelayerMessage{t: "srctype", b: "srcdata"}
		dstMsg := fakeRelayerMessage{t: "dsttype", b: "dstdata"}

		rm := relayer.RelayMsgs{
			Src: []provider.RelayerMessage{srcMsg},
			Dst: []provider.RelayerMessage{dstMsg},
		}

		result := rm.Send(context.Background(), zaptest.NewLogger(t), src, dst, "")
		require.Equal(t, result.SuccessfulSrcBatches, 0)
		require.Equal(t, result.SuccessfulDstBatches, 0)
		require.ErrorIs(t, result.SrcSendError, srcErr)
		require.ErrorIs(t, result.DstSendError, dstErr)
	})

	t.Run("multiple batches and all errors", func(t *testing.T) {
		srcErr1, srcErr2 := fmt.Errorf("source error 1"), fmt.Errorf("source error 2")
		var srcCalls int
		src := relayer.RelayMsgSender{
			ChainID: "src",
			SendMessages: func(ctx context.Context, msgs []provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
				srcCalls++
				switch srcCalls {
				case 1:
					return nil, false, srcErr1
				case 2:
					return nil, false, srcErr2
				default:
					panic("src.SendMessages called too many times")
				}
			},
		}

		dstErr1, dstErr2 := fmt.Errorf("dest error 1"), fmt.Errorf("dest error 2")
		var dstCalls int
		dst := relayer.RelayMsgSender{
			ChainID: "dst",
			SendMessages: func(ctx context.Context, msgs []provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
				dstCalls++
				switch dstCalls {
				case 1:
					return nil, false, dstErr1
				case 2:
					return nil, false, dstErr2
				default:
					panic("dst.SendMessages called too many times")
				}
			},
		}

		srcMsg := fakeRelayerMessage{t: "srctype", b: "srcdata"}
		dstMsg := fakeRelayerMessage{t: "dsttype", b: "dstdata"}

		rm := relayer.RelayMsgs{
			Src: []provider.RelayerMessage{srcMsg, srcMsg, srcMsg},
			Dst: []provider.RelayerMessage{dstMsg, dstMsg, dstMsg},

			MaxMsgLength: 2,
		}

		result := rm.Send(context.Background(), zaptest.NewLogger(t), src, dst, "")
		require.Equal(t, result.SuccessfulSrcBatches, 0)
		require.Equal(t, result.SuccessfulDstBatches, 0)
		require.ErrorIs(t, result.SrcSendError, srcErr1)
		require.ErrorIs(t, result.SrcSendError, srcErr2)
		require.ErrorIs(t, result.DstSendError, dstErr1)
		require.ErrorIs(t, result.DstSendError, dstErr2)
	})

	t.Run("two batches with success then error", func(t *testing.T) {
		srcErr := fmt.Errorf("source error")
		var srcCalls int
		src := relayer.RelayMsgSender{
			ChainID: "src",
			SendMessages: func(ctx context.Context, msgs []provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
				srcCalls++
				switch srcCalls {
				case 1:
					return nil, true, nil
				case 2:
					return nil, false, srcErr
				default:
					panic("src.SendMessages called too many times")
				}
			},
		}

		dstErr := fmt.Errorf("dest error")
		var dstCalls int
		dst := relayer.RelayMsgSender{
			ChainID: "dst",
			SendMessages: func(ctx context.Context, msgs []provider.RelayerMessage, memo string) (*provider.RelayerTxResponse, bool, error) {
				dstCalls++
				switch dstCalls {
				case 1:
					return nil, true, nil
				case 2:
					return nil, false, dstErr
				default:
					panic("dst.SendMessages called too many times")
				}
			},
		}

		srcMsg := fakeRelayerMessage{t: "srctype", b: "srcdata"}
		dstMsg := fakeRelayerMessage{t: "dsttype", b: "dstdata"}

		rm := relayer.RelayMsgs{
			Src: []provider.RelayerMessage{srcMsg, srcMsg, srcMsg},
			Dst: []provider.RelayerMessage{dstMsg, dstMsg, dstMsg},

			MaxMsgLength: 2,
		}

		result := rm.Send(context.Background(), zaptest.NewLogger(t), src, dst, "")
		require.Equal(t, result.SuccessfulSrcBatches, 1)
		require.Equal(t, result.SuccessfulDstBatches, 1)
		require.ErrorIs(t, result.SrcSendError, srcErr)
		require.ErrorIs(t, result.DstSendError, dstErr)
	})
}
