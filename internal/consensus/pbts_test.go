package consensus

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tmtime "github.com/tendermint/tendermint/libs/time"
	tmtimemocks "github.com/tendermint/tendermint/libs/time/mocks"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

// want something that lets me easily generate a pbts test case that can
// vary in a reasonable way related to where proposer based timestamps may break things
// let's write an imaginary test and see what is missing

func TestValidatorWaits(t *testing.T) {

	// todo
	// create a block that is signed over by a quorum of validators

	cfg := configSetup(t)
	// rand state creates a genesis file from the configuration file
	// it creates the number of validators passed in as arg 2
	// randState then
	cs1, vss := randState(cfg, 4)
	height, round := cs1.Height, cs1.Round
	newRoundCh := subscribe(cs1.eventBus, types.EventQueryNewRound)
	//	proposalCh := subscribe(cs1.eventBus, types.EventQueryCompleteProposal)
	newBlockCh := subscribe(cs1.eventBus, types.EventQueryNewBlock)
	startTestRound(cs1, height, round)
	ensureNewRound(t, newRoundCh, height, round)
	cs1.createProposalBlock() // changeProposer(t, cs1, vs2)

	// make the second validator the proposer by incrementing round
	round++
	ensureNewRound(t, newRoundCh, height, round)

	// make a fake commit from a block from h-1
	b := types.MakeBlock(height, []types.Tx{}, &types.Commit{}, []types.Evidence{})
	bt := tmtime.Now().Add(5 * time.Minute)
	b.Header.Populate(
		version.Consensus{}, "", bt, types.BlockID{},
		[]byte{}, []byte{},
		[]byte{}, []byte{}, []byte{},
		types.Address{},
	)
	partSet := b.MakePartSet(types.BlockPartSizeBytes)
	blockID := types.BlockID{Hash: b.Hash(), PartSetHeader: partSet.Header()}
	proposal := types.NewProposal(height, round, -1, blockID)

	if err := vss[1].SignProposal(context.Background(), cs1.state.ChainID, proposal.ToProto()); err != nil {
		t.Fatalf("error signing proposal: %s", err)
	}
	if err := cs1.SetProposalAndBlock(proposal, b, partSet, "peerID"); err != nil {
		t.Fatal(err)
	}

	fmt.Println("adding votes")
	//	ensureProposal(t, proposalCh, height, round, blockID)
	signAddVotes(cfg, cs1, tmproto.PrecommitType, blockID.Hash, blockID.PartSetHeader, vss[:4]...)
	// wait to finish commit, propose in next height
	ensureNewBlock(t, newBlockCh, 0)
	assert.Equal(t, bt, cs1.state.LastBlockTime)
	// create the height for h-1 with timestamp t1, where t1 is in the future
	// set this block as the block from h-1 of a validator
	// do not deliver a proposal
	// ensure that the validator waits until after t1
	// create a proposal with timestamp t2
	// ensure that the validator
	// create he
	// what do I do here

	// create a state at height h
	// want to hand the suite:
	// Time of h-1
	// Time that the proposer will set into the block
	// Time that the validator will receive the block
	//
	// Want to record
	// What time the validator ended the propose step
	// How the validator voted
	//
	//

}

func TestProposerWaitTime(t *testing.T) {
	genesisTime, err := time.Parse(time.RFC3339, "2019-03-13T23:00:00Z")
	require.NoError(t, err)
	testCases := []struct {
		name              string
		previousBlockTime time.Time
		localTime         time.Time
		expectedWait      time.Duration
	}{
		{
			name:              "block time greater than local time",
			previousBlockTime: genesisTime.Add(5 * time.Nanosecond),
			localTime:         genesisTime.Add(1 * time.Nanosecond),
			expectedWait:      4 * time.Nanosecond,
		},
		{
			name:              "local time greater than block time",
			previousBlockTime: genesisTime.Add(1 * time.Nanosecond),
			localTime:         genesisTime.Add(5 * time.Nanosecond),
			expectedWait:      0,
		},
		{
			name:              "both times equal",
			previousBlockTime: genesisTime.Add(5 * time.Nanosecond),
			localTime:         genesisTime.Add(5 * time.Nanosecond),
			expectedWait:      0,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {

			mockSource := new(tmtimemocks.Source)
			mockSource.On("Now").Return(testCase.localTime)

			ti := proposerWaitTime(mockSource, testCase.previousBlockTime)
			assert.Equal(t, testCase.expectedWait, ti)
		})
	}
}

func TestProposalTimeout(t *testing.T) {
	genesisTime, err := time.Parse(time.RFC3339, "2019-03-13T23:00:00Z")
	require.NoError(t, err)
	testCases := []struct {
		name              string
		localTime         time.Time
		previousBlockTime time.Time
		accuracy          time.Duration
		msgDelay          time.Duration
		expectedDuration  time.Duration
	}{
		{
			name:              "MsgDelay + 2 * Accuracy has not quite elapsed",
			localTime:         genesisTime.Add(525 * time.Millisecond),
			previousBlockTime: genesisTime.Add(6 * time.Millisecond),
			accuracy:          time.Millisecond * 10,
			msgDelay:          time.Millisecond * 500,
			expectedDuration:  1 * time.Millisecond,
		},
		{
			name:              "MsgDelay + 2 * Accuracy equals current time",
			localTime:         genesisTime.Add(525 * time.Millisecond),
			previousBlockTime: genesisTime.Add(5 * time.Millisecond),
			accuracy:          time.Millisecond * 10,
			msgDelay:          time.Millisecond * 500,
			expectedDuration:  0,
		},
		{
			name:              "MsgDelay + 2 * Accuracy has elapsed",
			localTime:         genesisTime.Add(725 * time.Millisecond),
			previousBlockTime: genesisTime.Add(5 * time.Millisecond),
			accuracy:          time.Millisecond * 10,
			msgDelay:          time.Millisecond * 500,
			expectedDuration:  0,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {

			mockSource := new(tmtimemocks.Source)
			mockSource.On("Now").Return(testCase.localTime)

			tp := types.TimestampParams{
				Accuracy: testCase.accuracy,
				MsgDelay: testCase.msgDelay,
			}

			ti := proposalStepWaitingTime(mockSource, testCase.previousBlockTime, tp)
			assert.Equal(t, testCase.expectedDuration, ti)
		})
	}
}

/*
func makeCommit(blockID BlockID, height int64, round int32,
	voteSet *VoteSet, validators []PrivValidator, now time.Time) (*Commit, error) {

	// all sign
	for i := 0; i < len(validators); i++ {
		pubKey, err := validators[i].GetPubKey(context.Background())
		if err != nil {
			return nil, fmt.Errorf("can't get pubkey: %w", err)
		}
		vote := &Vote{
			ValidatorAddress: pubKey.Address(),
			ValidatorIndex:   int32(i),
			Height:           height,
			Round:            round,
			Type:             tmproto.PrecommitType,
			BlockID:          blockID,
			Timestamp:        now,
		}

		_, err = signAddVote(validators[i], vote, voteSet)
		if err != nil {
			return nil, err
		}
	}

	return voteSet.MakeCommit(), nil
}
*/
