package substrate

import (
	"fmt"

	rpcclienttypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"
	beefyclienttypes "github.com/ComposableFi/ics11-beefy/types"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	"github.com/cosmos/ibc-go/v5/modules/core/exported"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ibcMessage is the type used for parsing all possible properties of IBC messages
type ibcMessage struct {
	eventType string
	info      ibcMessageInfo
}

type ibcMessageInfo interface {
	parseAttrs(log *zap.Logger, attrs any)
	MarshalLogObject(enc zapcore.ObjectEncoder) error
}

// alias for the interface map
type ibcEventQueryItem = map[string]any

// substrate ibc events endpoint returns a list of events
// relayted to different transaction, so we need
// to define a unique key
type ibcPacketKey struct {
	sequence   string
	srcChannel string
	srcPort    string
	dstChannel string
	dstPort    string
}

// ibcMessagesFromEvents parses all events of a certain height to find IBC messages
// TODO (NOTE): passing height won't work here using listening to commitment method to process ibc events.
//
//	the heights need to be fetched from the parsed ibc events.
func (scp *SubstrateChainProcessor) ibcMessagesFromEvents(
	ibcEvents rpcclienttypes.IBCEventsQueryResult,
) (messages []ibcMessage) {

	packetAccumulator := make(map[ibcPacketKey]*packetInfo)
	for i := 0; i < len(ibcEvents); i++ {

		info, eventType := scp.parseEvent(ibcEvents[i], packetAccumulator)
		if info == nil {
			// Not an IBC message, don't need to log here
			// event is write acknowledement, so receive packet will be processed by accumulator
			continue
		}

		// update messages
		messages = append(messages, ibcMessage{
			eventType: eventType,
			info:      info,
		})
	}

	// add all of accumulated packets to messages
	for _, pkt := range packetAccumulator {
		messages = append(messages, ibcMessage{
			// TODO (Question): will the packet type always be a receive packet?
			eventType: intoIBCEventType(ReceivePacket),
			info:      pkt,
		})
	}

	return messages
}

func (scp *SubstrateChainProcessor) parseEvent(
	event ibcEventQueryItem,
	packetAccumulator map[ibcPacketKey]*packetInfo,
) (info ibcMessageInfo, eventType string) {

	// there is one itm in the event type map
	// so this loop iterates just one iteration
	for eType, data := range event {

		switch eType {

		case CreateClient, UpgradeClient, ClientMisbehaviour:

			cl := new(clientInfo)
			cl.parseAttrs(scp.log, data)
			info = cl

			eventType = intoIBCEventType(eType)

		case UpdateClient:

			clu := new(clientUpdateInfo)
			clu.parseAttrs(scp.log, data)
			info = clu

			eventType = intoIBCEventType(eType)

		case SendPacket, AcknowledgePacket, TimeoutPacket, TimeoutOnClosePacket:

			pkt := new(packetInfo)
			pkt.parseAttrs(scp.log, data)
			info = pkt

			eventType = intoIBCEventType(eType)

		case ReceivePacket, WriteAcknowledgement:
			// TODO (QUESTION): the cosmos parser uses the packet accumulator for other events like the SendPacket,
			//  is there a reason that doesn't work for us? Can receive packet
			accumKey := genAccumKey(data)

			_, exists := packetAccumulator[accumKey]
			if !exists {
				packetAccumulator[accumKey] = new(packetInfo)
			}

			packetAccumulator[accumKey].parseAttrs(scp.log, data)

		case OpenInitConnection, OpenTryConnection, OpenAckConnection, OpenConfirmConnection:

			con := new(connectionInfo)
			con.parseAttrs(scp.log, data)
			info = con

			eventType = intoIBCEventType(eType)

		case OpenInitChannel, OpenTryChannel, OpenAckChannel, OpenConfirmChannel, CloseInitChannel, CloseConfirmChannel:
			chann := new(channelInfo)
			chann.parseAttrs(scp.log, data)
			info = chann

			eventType = intoIBCEventType(eType)

		default:
			panic("event not recognized")
		}

	}

	return
}

// returns the unique key for packet accumulator cache
func genAccumKey(data any) ibcPacketKey {
	return ibcPacketKey{
		sequence:   cast.ToString(data.(ibcEventQueryItem)["sequence"].(float64)),
		srcChannel: data.(ibcEventQueryItem)["source_channel"].(string),
		srcPort:    data.(ibcEventQueryItem)["source_port"].(string),
		dstChannel: data.(ibcEventQueryItem)["destination_channel"].(string),
		dstPort:    data.(ibcEventQueryItem)["destination_port"].(string),
	}
}

// client info attributes and methods
type clientInfo struct {
	// TODO: is there a reason the fields are exported? Since the clientInfo struct is not exported.
	height          clienttypes.Height
	clientID        string
	clientType      uint32
	consensusHeight clienttypes.Height
}

// ClientState returns the height and id of client
func (cl clientInfo) ClientState() provider.ClientState {
	return provider.ClientState{
		ClientID:        cl.clientID,
		ConsensusHeight: cl.consensusHeight,
	}
}

// MarshalLogObject marshals attributes of client info
func (cl *clientInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("client_id", cl.clientID)
	enc.AddUint64("consensus_height", cl.consensusHeight.RevisionHeight)
	enc.AddUint64("consensus_height_revision", cl.consensusHeight.RevisionNumber)
	return nil
}

// parseAttrs parses the attributes of client info
func (cl *clientInfo) parseAttrs(log *zap.Logger, attributes any) {
	attrs := attributes.(ibcEventQueryItem)

	var err error
	if cl.height, err = parseHeight(attrs["height"]); err != nil {
		log.Error("Error parsing client consensus height",
			zap.Error(err),
		)
		return
	}

	cl.clientID = attrs["client_id"].(string)

	if cl.clientType, err = cast.ToUint32E(attrs["client_type"].(string)); err != nil {
		log.Error("Error parsing client type",
			zap.Error(err),
		)
		return
	}

	if cl.consensusHeight, err = parseHeight(attrs["consensus_height"]); err != nil {
		log.Error("Error parsing client consensus height",
			zap.Error(err),
		)
		return
	}

}

// client update info attributes and methods
type clientUpdateInfo struct {
	common clientInfo
	header beefyclienttypes.Header
}

// MarshalLogObject marshals attributes of client update info
func (clu *clientUpdateInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("client_id", clu.common.clientID)
	enc.AddUint64("consensus_height", clu.common.consensusHeight.RevisionHeight)
	enc.AddUint64("consensus_height_revision", clu.common.consensusHeight.RevisionNumber)
	// TODO: include header if
	return nil
}

// parseAttrs parses the attributes of client update info
func (clu *clientUpdateInfo) parseAttrs(log *zap.Logger, attributes any) {
	attrs := attributes.(ibcEventQueryItem)

	clientInfo := new(clientInfo)
	clientInfo.parseAttrs(log, attrs["common"])
	clu.common = *clientInfo

	if h, headerFound := attrs["header"]; headerFound {
		clu.header = parseHeader(h)
	}
}

// alias type to the provider types, used for adding parser methods
type packetInfo provider.PacketInfo

// MarshalLogObject marshals attributes of packet info
func (pkt *packetInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddUint64("sequence", pkt.Sequence)
	enc.AddString("src_channel", pkt.SourceChannel)
	enc.AddString("src_port", pkt.SourcePort)
	enc.AddString("dst_channel", pkt.DestChannel)
	enc.AddString("dst_port", pkt.DestPort)
	return nil
}

// parseAttrs parses the attributes of packet info
func (pkt *packetInfo) parseAttrs(log *zap.Logger, attributes any) {
	attrs := attributes.(ibcEventQueryItem)

	var err error

	var height exported.Height
	if height, err = parseHeight(attrs["height"]); err != nil {
		log.Error("Error parsing connection height",
			zap.Error(err),
		)
		return
	}
	pkt.Height = height.GetRevisionHeight()

	pkt.Sequence = cast.ToUint64(attrs["sequence"].(float64))
	pkt.SourcePort = attrs["source_port"].(string)
	pkt.SourceChannel = attrs["source_channel"].(string)
	pkt.DestPort = attrs["destination_port"].(string)
	pkt.DestChannel = attrs["destination_channel"].(string)

	if pkt.Data, err = rpcclienttypes.HexDecodeString(attrs["data"].(string)); err != nil {
		log.Error("Error parsing packet data",
			zap.Error(err),
		)
		return
	}

	if pkt.TimeoutHeight, err = parseHeight(attrs["timeout_height"]); err != nil {
		log.Error("Error parsing packet height",
			zap.Error(err),
		)
		return
	}

	if pkt.TimeoutTimestamp, err = parseTimestamp(attrs["timeout_timestamp"]); err != nil {
		log.Error("Error parsing packet timeout timestamp",
			zap.Error(err),
		)
		return
	}

	ack, ackFound := attrs["ack"]
	if ackFound {

		if pkt.Ack, err = rpcclienttypes.HexDecodeString(ack.(string)); err != nil {
			log.Error("Error parsing ack data",
				zap.Error(err),
			)
			return
		}
	}

	// TODO: how to populate order
	// Order: ,
	// TODO: how to populate version
	// Version

}

// alias type to the provider types, used for adding parser methods
type channelInfo provider.ChannelInfo

// MarshalLogObject marshals attributes of channel info
func (ch *channelInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("channel_id", ch.ChannelID)
	enc.AddString("port_id", ch.PortID)
	enc.AddString("counterparty_channel_id", ch.CounterpartyChannelID)
	enc.AddString("counterparty_port_id", ch.CounterpartyPortID)
	return nil
}

// parseAttrs parses the attributes of channel info
func (ch *channelInfo) parseAttrs(log *zap.Logger, attributes any) {
	attrs := attributes.(ibcEventQueryItem)

	var err error

	var height exported.Height
	if height, err = parseHeight(attrs["height"]); err != nil {
		log.Error("Error parsing channel height",
			zap.Error(err),
		)
		return
	}
	ch.Height = height.GetRevisionHeight()

	ch.PortID = attrs["port_id"].(string)
	ch.ChannelID = attrs["channel_id"].(string)
	ch.CounterpartyPortID = attrs["counterparty_port_id"].(string)
	ch.CounterpartyChannelID = attrs["counterparty_channel_id"].(string)
	ch.ConnID = attrs["connection_id"].(string)
}

// alias type to the provider types, used for adding parser methods
type connectionInfo provider.ConnectionInfo

// MarshalLogObject marshals attributes of connection info
func (con *connectionInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("connection_id", con.ConnID)
	enc.AddString("client_id", con.ClientID)
	enc.AddString("counterparty_connection_id", con.CounterpartyConnID)
	enc.AddString("counterparty_client_id", con.CounterpartyClientID)
	return nil
}

// parseAttrs parses the attributes of connection info
func (con *connectionInfo) parseAttrs(log *zap.Logger, attributes any) {
	attrs := attributes.(ibcEventQueryItem)

	var err error

	var height exported.Height
	if height, err = parseHeight(attrs["height"]); err != nil {
		log.Error("Error parsing connection height",
			zap.Error(err),
		)
		return
	}
	con.Height = height.GetRevisionHeight()

	con.ConnID = attrs["connection_id"].(string)
	con.ClientID = attrs["client_id"].(string)
	con.CounterpartyClientID = attrs["counterparty_client_id"].(string)
	con.CounterpartyConnID = attrs["counterparty_connection_id"].(string)
}

// parses the height from an input interface
func parseHeight(i any) (clienttypes.Height, error) {
	height := i.(ibcEventQueryItem)

	revisionNumber, err := cast.ToUint64E(height["revision_number"].(float64))
	if err != nil {
		return clienttypes.Height{}, fmt.Errorf("error parsing revision number: %s", err)
	}

	revisionHeight, err := cast.ToUint64E(height["revision_height"].(float64))
	if err != nil {
		return clienttypes.Height{}, fmt.Errorf("error parsing revision height: %s", err)
	}

	return clienttypes.Height{
		RevisionNumber: revisionNumber,
		RevisionHeight: revisionHeight,
	}, nil
}

// parses timestamp from an input interface
func parseTimestamp(i any) (uint64, error) {
	timestamp := i.(ibcEventQueryItem)

	t, err := cast.ToTimeE(timestamp["time"].(string))
	if err != nil {
		return 0, fmt.Errorf("error parsing timestamp %s", err)
	}
	return uint64(t.UnixNano()), nil
}

// parses beefy header
func parseHeader(i any) (res beefyclienttypes.Header) {
	attrs := i.(ibcEventQueryItem)

	res = beefyclienttypes.Header{
		HeadersWithProof: parseParachainHeaderWithProof(attrs["headers_with_proof"]),
		MMRUpdateProof:   parseUpdateProofs(attrs["mmr_update_proof"]),
	}

	return
}

// parses parachain header and mmr proofs
func parseParachainHeaderWithProof(i any) (res *beefyclienttypes.ParachainHeadersWithProof) {
	attrs := i.(ibcEventQueryItem)

	res = &beefyclienttypes.ParachainHeadersWithProof{
		Headers:   parseParachainHeaders(attrs["headers"]),
		MMRProofs: attrs["mmr_proofs"].([][]byte),
		MMRSize:   attrs["mmr_size"].(uint64),
	}

	return
}

// parses parachain header attributes
func parseParachainHeaders(i any) (res []*beefyclienttypes.ParachainHeader) {
	attrs := i.(rpcclienttypes.IBCEventsQueryResult)
	res = []*beefyclienttypes.ParachainHeader{}

	for i := 0; i < len(attrs); i++ {
		headerAttrs := attrs[i]

		resItem := &beefyclienttypes.ParachainHeader{
			ParachainHeader:     headerAttrs["parachain_header"].([]byte),
			PartialMMRLeaf:      parsePartialMMRLeaf(headerAttrs["partial_mmr_leaf"]),
			ParachainHeadsProof: headerAttrs["parachain_heads_proof"].([][]byte),
			HeadsLeafIndex:      headerAttrs["heads_leaf_index"].(uint32),
			HeadsTotalCount:     headerAttrs["heads_total_count"].(uint32),
			ExtrinsicProof:      headerAttrs["extrinsic_proof"].([][]byte),
			TimestampExtrinsic:  headerAttrs["timestamp_extrinsic"].([]byte),
		}
		res = append(res, resItem)
	}

	return
}

// parses mmr update proof attributes
func parseUpdateProofs(i any) (res *beefyclienttypes.MMRUpdateProof) {
	attrs := i.(ibcEventQueryItem)

	res = &beefyclienttypes.MMRUpdateProof{
		SignedCommitment:   parseSignedCommitment(attrs["signed_commitments"]),
		LatestMMRLeafIndex: attrs["latest_mmr_leaf_index"].(uint64),
		LatestMMRLeaf:      parseBeefyMMRLeaf(attrs["latest_mmr_leaf"]),
		MMRProof:           attrs["mmr_proof"].([][]byte),
		AuthoritiesProof:   attrs["authorities_proof"].([][]byte),
	}

	return
}

// parses partial mmr tree leaf attributes
func parsePartialMMRLeaf(i any) (res *beefyclienttypes.PartialMMRLeaf) {
	attrs := i.(ibcEventQueryItem)

	res = &beefyclienttypes.PartialMMRLeaf{
		Version:               attrs["version"].(beefyclienttypes.U8),
		ParentNumber:          attrs["parent_number"].(uint32),
		ParentHash:            attrs["parent_hash"].(*beefyclienttypes.SizedByte32),
		BeefyNextAuthoritySet: parseAuthoritySet(attrs["beefy_next_authority_set"]),
	}

	return
}

// parses signed commitment and corresposing signatures
func parseSignedCommitment(i any) (res *beefyclienttypes.SignedCommitment) {
	attrs := i.(ibcEventQueryItem)

	res = &beefyclienttypes.SignedCommitment{
		Commitment: parseCommitment(attrs["commitment"]),
		Signatures: parseSignatures(attrs["signatures"]),
	}

	return
}

// parses the commitment and payload attributes
func parseCommitment(i any) (res *beefyclienttypes.Commitment) {
	attrs := i.(ibcEventQueryItem)

	res = &beefyclienttypes.Commitment{
		Payload:        parsePayload(attrs),
		BlockNumber:    attrs["block_number"].(uint32),
		ValidatorSetID: attrs["validator_set_id"].(uint64),
	}

	return
}

// parses the payload id and attributes
func parsePayload(i any) (res []*beefyclienttypes.Payload) {
	sliceAttrs := i.(rpcclienttypes.IBCEventsQueryResult)
	res = []*beefyclienttypes.Payload{}

	for j := 0; j < len(sliceAttrs); j++ {
		attrs := sliceAttrs[j]

		resItem := &beefyclienttypes.Payload{
			PayloadID:   attrs["payload_id"].(*beefyclienttypes.SizedByte2),
			PayloadData: attrs["payload_data"].([]byte),
		}
		res = append(res, resItem)
	}

	return
}

// parses the commitment signatures attributes
func parseSignatures(i any) (res []*beefyclienttypes.CommitmentSignature) {
	sliceAttrs := i.(rpcclienttypes.IBCEventsQueryResult)
	res = []*beefyclienttypes.CommitmentSignature{}

	for j := 0; j < len(sliceAttrs); j++ {
		attrs := sliceAttrs[j]

		resItem := &beefyclienttypes.CommitmentSignature{
			Signature:      attrs["signature"].([]byte),
			AuthorityIndex: attrs["authority_index"].(uint32),
		}
		res = append(res, resItem)
	}

	return
}

// parses mmr leaf attributes of beefy
func parseBeefyMMRLeaf(i any) (res *beefyclienttypes.BeefyMMRLeaf) {
	attrs := i.(ibcEventQueryItem)

	res = &beefyclienttypes.BeefyMMRLeaf{
		Version:               attrs["version"].(beefyclienttypes.U8),
		ParentNumber:          attrs["parent_number"].(uint32),
		ParentHash:            attrs["parent_hash"].(*beefyclienttypes.SizedByte32),
		BeefyNextAuthoritySet: parseAuthoritySet(attrs["beefy_next_authority_set"]),
		ParachainHeads:        attrs["parachain_heads"].(*beefyclienttypes.SizedByte32),
	}

	return
}

// parses authority set attributes
func parseAuthoritySet(i any) (res beefyclienttypes.BeefyAuthoritySet) {
	attrs := i.(ibcEventQueryItem)

	res = beefyclienttypes.BeefyAuthoritySet{
		ID:            attrs["id"].(uint64),
		Len:           attrs["len"].(uint32),
		AuthorityRoot: attrs["authority_root"].(*beefyclienttypes.SizedByte32),
	}

	return
}
