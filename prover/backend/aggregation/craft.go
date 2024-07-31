package aggregation

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	public_input "github.com/consensys/zkevm-monorepo/prover/public-input"
	"path"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/zkevm-monorepo/prover/backend/blobdecompression"
	"github.com/consensys/zkevm-monorepo/prover/backend/execution"
	"github.com/consensys/zkevm-monorepo/prover/backend/execution/bridge"
	"github.com/consensys/zkevm-monorepo/prover/backend/files"
	"github.com/consensys/zkevm-monorepo/prover/circuits/aggregation"
	"github.com/consensys/zkevm-monorepo/prover/config"
	"github.com/consensys/zkevm-monorepo/prover/crypto/state-management/hashtypes"
	"github.com/consensys/zkevm-monorepo/prover/crypto/state-management/smt"
	"github.com/consensys/zkevm-monorepo/prover/utils"
	"github.com/consensys/zkevm-monorepo/prover/utils/types"
	"github.com/sirupsen/logrus"
)

const (
	// Indicates the depth of a Merkle-tree for L2 messages, and implicitly how
	// many messages can be stored in a single root hash.
	l2MsgMerkleTreeDepth     = 5
	l2MsgMerkleTreeMaxLeaves = 1 << l2MsgMerkleTreeDepth
)

// Collect the fields, to make the aggregation proof
func collectFields(cfg *config.Config, req *Request) (*CollectedFields, error) {

	var (
		l2MessageHashes   = []string{}
		l2MsgBlockOffsets = []bool{}
		cf                = &CollectedFields{
			L2MsgTreeDepth:                          l2MsgMerkleTreeDepth,
			ParentAggregationLastBlockTimestamp:     uint(req.ParentAggregationLastBlockTimestamp),
			LastFinalizedL1RollingHash:              req.ParentAggregationLastL1RollingHash,
			LastFinalizedL1RollingHashMessageNumber: uint(req.ParentAggregationLastL1RollingHashMessageNumber),
		}
	)

	for i, execReqFPath := range req.ExecutionProofs {
		po := &execution.Response{}
		fpath := path.Join(cfg.Execution.DirTo(), execReqFPath)
		f := files.MustRead(fpath)

		if err := json.NewDecoder(f).Decode(po); err != nil {
			return nil, fmt.Errorf("fields collection, decoding %s, %w", execReqFPath, err)
		}

		if i == 0 {
			cf.LastFinalizedBlockNumber = uint(po.FirstBlockNumber) - 1
			cf.ParentStateRootHash = po.ParentStateRootHash
		}

		if po.ProverMode == config.ProverModeProofless {
			cf.IsProoflessJob = true
		}

		if i > 0 && po.HasParentStateRootHashMismatch {
			utils.Panic("conflated batch %v reports a parent state hash mismatch, but this is not the first batch of the sequence", i)
		}

		// This is purposefuly overwritten at each iteration over i. We want to
		// keep the final velue.
		cf.FinalBlockNumber = uint(po.FirstBlockNumber + len(po.BlocksData) - 1)

		for _, blockdata := range po.BlocksData {
			l2MessageHashes = append(l2MessageHashes, blockdata.L2ToL1MsgHashes...)
			l2MsgBlockOffsets = append(l2MsgBlockOffsets, len(blockdata.L2ToL1MsgHashes) > 0)
			cf.HowManyL2Msgs += uint(len(blockdata.L2ToL1MsgHashes))

			// The goal is that we want to keep the final value
			lastRollingHashEvent := blockdata.LastRollingHashUpdatedEvent
			if lastRollingHashEvent != (bridge.RollingHashUpdated{}) {
				cf.L1RollingHash = lastRollingHashEvent.RollingHash
				cf.L1RollingHashMessageNumber = uint(lastRollingHashEvent.MessageNumber)
			}
			cf.FinalTimestamp = uint(blockdata.TimeStamp)
		}

		// Append the proof claim to the list of collected proofs
		if !cf.IsProoflessJob {
			pClaim, err := parseProofClaim(po.Proof, po.DebugData.FinalHash, po.VerifyingKeyShaSum)
			if err != nil {
				return nil, fmt.Errorf("could not parse the proof claim for `%v` : %w", fpath, err)
			}
			cf.ProofClaims = append(cf.ProofClaims, *pClaim)
		}
	}

	for i, decompReqFPath := range req.CompressionProofs {
		dp := &blobdecompression.Response{}
		fpath := path.Join(cfg.BlobDecompression.DirTo(), decompReqFPath)
		f := files.MustRead(fpath)

		if err := json.NewDecoder(f).Decode(dp); err != nil {
			return nil, fmt.Errorf("fields collection, decoding %s, %w", fpath, err)
		}

		if i == 0 {
			cf.DataParentHash = dp.DataParentHash
			cf.ParentAggregationFinalShnarf = dp.PrevShnarf
		}

		// These fields are overwritten after every iteration on purpose. We want
		// to keep the last value only.
		cf.FinalShnarf = dp.ExpectedShnarf
		cf.DataHashes = append(cf.DataHashes, dp.DataHash)

		// Append the proof claim to the list of collected proofs
		if !cf.IsProoflessJob {
			pClaim, err := parseProofClaim(dp.DecompressionProof, dp.Debug.PublicInput, dp.VerifyingKeyShaSum)
			if err != nil {
				return nil, fmt.Errorf("could not parse the proof claim for `%v` : %w", fpath, err)
			}
			cf.ProofClaims = append(cf.ProofClaims, *pClaim)
		}
	}

	// If we did not collect the rolling hash, we instead pass the last
	// finalized one in the collected fields
	if len(cf.L1RollingHash) == 0 {
		cf.L1RollingHash = req.ParentAggregationLastL1RollingHash
		cf.L1RollingHashMessageNumber = uint(req.ParentAggregationLastL1RollingHashMessageNumber)
	}

	cf.L2MessagingBlocksOffsets = utils.HexEncodeToString(PackOffsets(l2MsgBlockOffsets))
	cf.L2MsgRootHashes = PackInMiniTrees(l2MessageHashes)

	return cf, nil

}

// Prepare the response without running the actual proof
// TODO @gbotrel well, this is a bit of a lie, we do run the proof, don't we?
func CraftResponse(cfg *config.Config, cf *CollectedFields) (resp *Response, err error) {

	if err := validate(cf); err != nil {
		return resp, err
	}

	resp = &Response{
		DataHashes:                          cf.DataHashes,
		DataParentHash:                      cf.DataParentHash,
		ParentStateRootHash:                 cf.ParentStateRootHash,
		ParentAggregationLastBlockTimestamp: cf.ParentAggregationLastBlockTimestamp,
		FinalTimestamp:                      cf.FinalTimestamp,
		L1RollingHash:                       cf.L1RollingHash,
		L1RollingHashMessageNumber:          cf.L1RollingHashMessageNumber,
		L2MerkleRoots:                       cf.L2MsgRootHashes,
		L2MsgTreesDepth:                     cf.L2MsgTreeDepth,
		L2MessagingBlocksOffsets:            cf.L2MessagingBlocksOffsets,
		LastFinalizedBlockNumber:            cf.LastFinalizedBlockNumber,
		FinalBlockNumber:                    cf.FinalBlockNumber,
		ParentAggregationFinalShnarf:        cf.ParentAggregationFinalShnarf,
		FinalShnarf:                         cf.FinalShnarf,
	}

	// @alex: proofless jobs are triggered once during the migration introducing
	// the compression and the aggregation.
	if cf.IsProoflessJob {
		return resp, nil
	}

	pubInputParts := public_input.Aggregation{
		FinalShnarf:                             cf.FinalShnarf,
		ParentAggregationFinalShnarf:            cf.ParentAggregationFinalShnarf,
		ParentStateRootHash:                     cf.ParentStateRootHash,
		ParentAggregationLastBlockTimestamp:     cf.ParentAggregationLastBlockTimestamp,
		FinalTimestamp:                          cf.FinalTimestamp,
		LastFinalizedBlockNumber:                cf.LastFinalizedBlockNumber,
		FinalBlockNumber:                        cf.FinalBlockNumber,
		LastFinalizedL1RollingHash:              cf.LastFinalizedL1RollingHash,
		L1RollingHash:                           cf.L1RollingHash,
		LastFinalizedL1RollingHashMessageNumber: cf.LastFinalizedL1RollingHashMessageNumber,
		L1RollingHashMessageNumber:              resp.L1RollingHashMessageNumber,
		L2MsgRootHashes:                         cf.L2MsgRootHashes,
		L2MsgMerkleTreeDepth:                    l2MsgMerkleTreeDepth,
	}

	resp.AggregatedProofPublicInput = pubInputParts.GetPublicInputHex()

	// This log is aimed at helping debugging in-depth when the proofs are
	// reverted because the public input mismatches. The content of this log
	// can be compared with data on tenderly.
	logrus.Infof("public inputs components for range (%v-%v): %++v",
		pubInputParts.LastFinalizedBlockNumber+1,
		pubInputParts.FinalBlockNumber,
		pubInputParts,
	)

	resp.AggregatedVerifierIndex = cfg.Aggregation.VerifierID
	resp.AggregatedProverVersion = cfg.Version

	resp.AggregatedProof, err = makeProof(cfg, cf, resp.AggregatedProofPublicInput)
	if err != nil {
		return nil, fmt.Errorf("failed to prove the aggregation: %w", err)
	}

	return resp, nil
}

// validate the content of the collected fields.
func validate(cf *CollectedFields) (err error) {

	utils.ValidateHexString(&err, cf.FinalShnarf, "FinalizedShnarf : %w", 32)
	utils.ValidateHexString(&err, cf.ParentStateRootHash, "ParentStateRootHash : %w", 32)
	utils.ValidateHexString(&err, cf.L1RollingHash, "L1RollingHash : %w", 32)
	utils.ValidateHexString(&err, cf.L2MessagingBlocksOffsets, "L2MessagingBlocksOffsets : %w", -1)
	utils.ValidateTimestamps(&err, cf.ParentAggregationLastBlockTimestamp, cf.FinalTimestamp)
	utils.ValidateHexString(&err, cf.DataParentHash, "DataParentHash : %w", 32)

	for i := range cf.L2MsgRootHashes {
		wrapper := fmt.Sprintf("L2MsgRootHashes[%d] : ", i) + "%w"
		utils.ValidateHexString(&err, cf.L2MsgRootHashes[i], wrapper, 32)
	}

	for i := range cf.DataHashes {
		wrapper := fmt.Sprintf("DataHashes[%d] : ", i) + "%w"
		utils.ValidateHexString(&err, cf.DataHashes[i], wrapper, 32)
	}

	return err
}

// Pack an array of boolean into an offset list. The offset list encodes the
// position of all the boolean whose value is true. Each position is encoded
// as a big-endian uint16.
func PackOffsets(unpacked []bool) []byte {
	resWrite := &bytes.Buffer{}
	tmp := [2]byte{}

	for i, b := range unpacked {
		if b {
			// @alex: issue #2261 requires the prover to start counting from 1
			// and not from zero for the offsets.
			binary.BigEndian.PutUint16(tmp[:], uint16(i+1))
			resWrite.Write(tmp[:])
		}
	}

	return resWrite.Bytes()
}

func nextMultipleOf(n, multiple int) int {
	return multiple * ((n + multiple - 1) / multiple)
}

// Hash the L2 messages into Merkle trees or arity 2 and depth
// `l2MsgMerkleTreeDepth`. The leaves are zero-padded on the right.
func PackInMiniTrees(l2MsgHashes []string) []string {

	paddedLen := nextMultipleOf(len(l2MsgHashes), l2MsgMerkleTreeMaxLeaves)
	paddedL2MsgHashes := make([]string, paddedLen)
	copy(paddedL2MsgHashes, l2MsgHashes)

	res := []string{}

	for i := 0; i < paddedLen; i += l2MsgMerkleTreeMaxLeaves {

		digests := make([]types.Bytes32, l2MsgMerkleTreeMaxLeaves)

		// Convert the leaves into digests that can be processed by the smt
		// package.
		for j := range digests {
			leaf := paddedL2MsgHashes[i+j]
			decoded, err := utils.HexDecodeString(leaf)
			copy(digests[j][:], decoded)

			if err != nil {
				panic(err)
			}
		}

		tree := smt.BuildComplete(digests, hashtypes.Keccak)
		res = append(res, tree.Root.Hex())
	}

	return res
}

func parseProofClaim(
	proofHexString string,
	publicInputHexString string,
	verifyinKeyShasum string,
) (*aggregation.ProofClaimAssignment, error) {

	proofByte, err := utils.HexDecodeString(proofHexString)
	if err != nil {
		return nil, fmt.Errorf("could not parse proof as an hex string: the proof `%v`: %w", proofHexString, err)
	}

	if verifyinKeyShasum == "" {
		return nil, fmt.Errorf("the verifying key shasum is empty")
	}

	// This can potentially panic if the checksum is not a valid one.
	res := &aggregation.ProofClaimAssignment{
		VerifyingKeyShasum: types.FullBytes32FromHex(verifyinKeyShasum),
	}

	// This should be fine because, this field is set by the execution/compression
	// prover.
	if _, err := res.PublicInput.SetString(publicInputHexString); err != nil {
		return nil, fmt.Errorf("the public input could not be parsed from bytes: %x because: %w", publicInputHexString, err)
	}

	// @alex: the proof need to be (pre)-allocated before being read
	res.Proof = plonk.NewProof(ecc.BLS12_377)
	if _, err := res.Proof.ReadFrom(bytes.NewBuffer(proofByte)); err != nil {
		return nil, fmt.Errorf("could not parse the proof from bytes: %v", err)
	}

	return res, nil
}
