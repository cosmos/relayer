package substrate

import (
	"fmt"
	"strconv"

	rpcclienttypes "github.com/ComposableFi/go-substrate-rpc-client/v4/types"
	beefyclienttypes "github.com/ComposableFi/ics11-beefy/types"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	"github.com/cosmos/relayer/v2/relayer/provider"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ibcMessage is the type used for parsing all possible properties of IBC messages
type ibcMessage struct {
	eventType string
	info      ibcMessageInfo
}

type ibcMessageInfo interface {
	parseAttrs(log *zap.Logger, attrs interface{})
	MarshalLogObject(enc zapcore.ObjectEncoder) error
}

// alias for the interface map
type ibcEventQueryItem (map[string]interface{})

// substrate ibc events endpoint returns a list of events
// relayted to different transaction, so we need
// to define a unique key
type ibcPacketKey struct {
	sequence   uint64
	srcChannel string
	srcPort    string
	dstChannel string
	dstPort    string
}

// ibcMessagesFromEvents parses all events of a certain height to find IBC messages
func (scp *SubstrateChainProcessor) ibcMessagesFromEvents(
	ibcEvents rpcclienttypes.IBCEventsQueryResult,
	height uint64,
) (messages []ibcMessage) {

	packetAccumulator := make(map[ibcPacketKey]*packetInfo)
	for i := 0; i < len(ibcEvents); i++ {

		info, eventType := scp.parseEvent(ibcEvents[i], height, packetAccumulator)
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
			eventType: intoIBCEventType(ReceivePacket),
			info:      pkt,
		})
	}

	return messages
}

func (scp *SubstrateChainProcessor) parseEvent(
	event ibcEventQueryItem,
	height uint64,
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

			accumKey := genAccumKey(data)

			_, exists := packetAccumulator[accumKey]
			if !exists {
				packetAccumulator[accumKey] = &packetInfo{Height: height}
			}

			packetAccumulator[accumKey].parseAttrs(scp.log, data)

		case OpenInitConnection, OpenTryConnection, OpenAckConnection, OpenConfirmConnection:

			con := &connectionInfo{Height: height}
			con.parseAttrs(scp.log, data)
			info = con

			eventType = intoIBCEventType(eType)

		case OpenInitChannel, OpenTryChannel, OpenAckChannel, OpenConfirmChannel, CloseInitChannel, CloseConfirmChannel:
			chann := &channelInfo{Height: height}
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
func genAccumKey(data interface{}) ibcPacketKey {
	return ibcPacketKey{
		sequence:   data.(ibcEventQueryItem)["sequence"].(uint64),
		srcChannel: data.(ibcEventQueryItem)["source_channel"].(string),
		srcPort:    data.(ibcEventQueryItem)["source_port"].(string),
		dstChannel: data.(ibcEventQueryItem)["destination_channel"].(string),
		dstPort:    data.(ibcEventQueryItem)["destination_port"].(string),
	}
}

// client info attributes and methods
type clientInfo struct {
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
func (cl *clientInfo) parseAttrs(log *zap.Logger, attributes interface{}) {
	attrs := attributes.(ibcEventQueryItem)

	var err error
	if cl.height, err = parseHeight(attrs["height"]); err != nil {
		log.Error("error parsing client consensus height: ",
			zap.Error(err),
		)
		return
	}

	cl.clientID = attrs["client_id"].(string)
	cl.clientType = attrs["client_type"].(uint32)

	if cl.consensusHeight, err = parseHeight(attrs["consensus_height"]); err != nil {
		log.Error("error parsing client consensus height: ",
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
func (clu *clientUpdateInfo) parseAttrs(log *zap.Logger, attributes interface{}) {
	attrs := attributes.(ibcEventQueryItem)

	clientInfo := new(clientInfo)
	clientInfo.parseAttrs(log, attrs["common"])
	clu.common = *clientInfo

	clu.header = parseHeader(attrs["header"])
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
func (pkt *packetInfo) parseAttrs(log *zap.Logger, attributes interface{}) {
	attrs := attributes.(ibcEventQueryItem)

	pkt.Sequence = attrs["sequence"].(uint64)
	pkt.SourcePort = attrs["source_port"].(string)
	pkt.SourceChannel = attrs["source_channel"].(string)
	pkt.DestPort = attrs["destination_port"].(string)
	pkt.DestChannel = attrs["destination_channel"].(string)
	pkt.Data = attrs["data"].([]byte)

	var err error
	if pkt.TimeoutHeight, err = parseHeight(attrs["timeout_height"]); err != nil {
		log.Error("error parsing packet height: ",
			zap.Error(err),
		)
		return
	}

	pkt.TimeoutTimestamp = attrs["timeout_timestamp"].(uint64)

	ack, found := attrs["ack"]
	if found {
		pkt.Ack = ack.([]byte)
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
func (ch *channelInfo) parseAttrs(log *zap.Logger, attributes interface{}) {
	attrs := attributes.(ibcEventQueryItem)

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
func (con *connectionInfo) parseAttrs(log *zap.Logger, attributes interface{}) {
	attrs := attributes.(ibcEventQueryItem)

	con.ConnID = attrs["connection_id"].(string)
	con.ClientID = attrs["client_id"].(string)
	con.CounterpartyClientID = attrs["counterparty_client_id"].(string)
	con.CounterpartyConnID = attrs["counterparty_connection_id"].(string)
}

// parses the attributes of ibc core client type
func parseHeight(i interface{}) (clienttypes.Height, error) {
	height := i.(ibcEventQueryItem)

	revisionNumber, err := strconv.ParseUint(height["revision_number"].(string), 10, 64)
	if err != nil {
		return clienttypes.Height{}, fmt.Errorf("error parsing revision number: %s", err)
	}

	revisionHeight, err := strconv.ParseUint(height["revision_height"].(string), 10, 64)
	if err != nil {
		return clienttypes.Height{}, fmt.Errorf("error parsing revision height: %s", err)
	}

	return clienttypes.Height{
		RevisionNumber: revisionNumber,
		RevisionHeight: revisionHeight,
	}, nil
}

// parses beefy header
func parseHeader(i interface{}) (res beefyclienttypes.Header) {
	attrs := i.(ibcEventQueryItem)

	res = beefyclienttypes.Header{
		HeadersWithProof: parseParachainHeaderWithProof(attrs["headers_with_proof"]),
		MMRUpdateProof:   parseUpdateProofs(attrs["mmr_update_proof"]),
	}

	return
}

// parses parachain header and mmr proofs
func parseParachainHeaderWithProof(i interface{}) (res *beefyclienttypes.ParachainHeadersWithProof) {
	attrs := i.(ibcEventQueryItem)

	res = &beefyclienttypes.ParachainHeadersWithProof{
		Headers:   parseParachainHeaders(attrs["headers"]),
		MMRProofs: attrs["mmr_proofs"].([][]byte),
		MMRSize:   attrs["mmr_size"].(uint64),
	}

	return
}

// parses parachain header attributes
func parseParachainHeaders(i interface{}) (res []*beefyclienttypes.ParachainHeader) {
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
func parseUpdateProofs(i interface{}) (res *beefyclienttypes.MMRUpdateProof) {
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
func parsePartialMMRLeaf(i interface{}) (res *beefyclienttypes.PartialMMRLeaf) {
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
func parseSignedCommitment(i interface{}) (res *beefyclienttypes.SignedCommitment) {
	attrs := i.(ibcEventQueryItem)

	res = &beefyclienttypes.SignedCommitment{
		Commitment: parseCommitment(attrs["commitment"]),
		Signatures: parseSignatures(attrs["signatures"]),
	}

	return
}

// parses the commitment and payload attributes
func parseCommitment(i interface{}) (res *beefyclienttypes.Commitment) {
	attrs := i.(ibcEventQueryItem)

	res = &beefyclienttypes.Commitment{
		Payload:        parsePayload(attrs),
		BlockNumber:    attrs["block_number"].(uint32),
		ValidatorSetID: attrs["validator_set_id"].(uint64),
	}

	return
}

// parses the payload id and attributes
func parsePayload(i interface{}) (res []*beefyclienttypes.Payload) {
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
func parseSignatures(i interface{}) (res []*beefyclienttypes.CommitmentSignature) {
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
func parseBeefyMMRLeaf(i interface{}) (res *beefyclienttypes.BeefyMMRLeaf) {
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
func parseAuthoritySet(i interface{}) (res beefyclienttypes.BeefyAuthoritySet) {
	attrs := i.(ibcEventQueryItem)

	res = beefyclienttypes.BeefyAuthoritySet{
		ID:            attrs["id"].(uint64),
		Len:           attrs["len"].(uint32),
		AuthorityRoot: attrs["authority_root"].(*beefyclienttypes.SizedByte32),
	}

	return
}
