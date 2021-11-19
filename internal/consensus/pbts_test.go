package consensus

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	tmtimemocks "github.com/tendermint/tendermint/libs/time/mocks"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

type pbtsTestHarness struct {
	s                 pbtsTestConfiguration
	observedState     *State
	observedValidator *validatorStub
	otherValidators   []*validatorStub
	chainID           string

	proposalCh, roundCh, blockCh, voteCh <-chan tmpubsub.Message

	currentHeight  int64
	currentRound   int32
	proposerOffset int
}

type pbtsTestConfiguration struct {
	ts                         types.TimestampParams
	timeoutPropose             time.Duration
	proposedTimes              []time.Time
	proposalDeviverTime        []time.Time
	genesisTime                time.Time
	height2ProposalDeliverTime time.Time
	height2ProposedBlockTime   time.Time
	height3ProposalDeliverTime time.Time
	height3ProposedBlockTime   time.Time
}

func newPBTSTestHarness(t *testing.T, tc pbtsTestConfiguration) pbtsTestHarness {
	cfg := configSetup(t)
	cfg.Consensus.TimeoutPropose = tc.timeoutPropose
	cs, vss := randState(cfg, 4)
	cs.state.LastBlockTime = tc.genesisTime
	return pbtsTestHarness{
		s:                 tc,
		observedValidator: vss[0],
		observedState:     cs,
		otherValidators:   vss[1:],
		currentHeight:     1,
		chainID:           cfg.ChainID(),
		roundCh:           subscribe(cs.eventBus, types.EventQueryNewRound),
		proposalCh:        subscribe(cs.eventBus, types.EventQueryCompleteProposal),
		blockCh:           subscribe(cs.eventBus, types.EventQueryNewBlock),
		voteCh:            subscribe(cs.eventBus, types.EventQueryVote),
	}
}

func (p *pbtsTestHarness) genesisHeight(t *testing.T) {
	startTestRound(p.observedState, p.currentHeight, p.currentRound)
	ensureNewRound(t, p.roundCh, p.currentHeight, p.currentRound)
	propBlock, partSet := p.observedState.createProposalBlock()
	bid := types.BlockID{Hash: propBlock.Hash(), PartSetHeader: partSet.Header()}
	ensureProposal(t, p.proposalCh, p.currentHeight, p.currentRound, bid)

	ensurePrevote(t, p.voteCh, p.currentHeight, p.currentRound)
	signAddVotes(p.observedState, tmproto.PrevoteType, p.chainID, p.s.height2ProposedBlockTime, bid, p.otherValidators...)

	signAddVotes(p.observedState, tmproto.PrecommitType, p.chainID, p.s.height2ProposedBlockTime, bid, p.otherValidators...)

	ensureNewBlock(t, p.blockCh, p.currentHeight)
	p.currentHeight++
	incrementHeight(p.otherValidators...)
}

func (p *pbtsTestHarness) height2(t *testing.T) heightResult {
	signer := p.otherValidators[0].PrivValidator
	return p.nextHeight(t, signer, p.s.height2ProposalDeliverTime, p.s.height2ProposedBlockTime, p.s.height3ProposedBlockTime)
}

func (p *pbtsTestHarness) height3(t *testing.T) heightResult {
	signer := p.otherValidators[1].PrivValidator
	return p.nextHeight(t, signer, p.s.height3ProposalDeliverTime, p.s.height3ProposedBlockTime, time.Now())
}

// TODO: Read from the vote chan to determine vote timings.
func (p *pbtsTestHarness) nextHeight(t *testing.T, proposer types.PrivValidator, dt time.Time, proposedTime, nextProposedTime time.Time) heightResult {
	ensureNewRound(t, p.roundCh, p.currentHeight, p.currentRound)

	b, _ := p.observedState.createProposalBlock()
	b.Height = p.currentHeight
	b.Header.Height = p.currentHeight
	b.Header.Time = proposedTime

	k, err := proposer.GetPubKey(context.Background())
	assert.Nil(t, err)
	b.Header.ProposerAddress = k.Address()
	ps := b.MakePartSet(types.BlockPartSizeBytes)
	bid := types.BlockID{Hash: b.Hash(), PartSetHeader: ps.Header()}
	prop := types.NewProposal(p.currentHeight, 0, -1, bid)
	tp := prop.ToProto()

	if err := proposer.SignProposal(context.Background(), p.observedState.state.ChainID, tp); err != nil {
		t.Fatalf("error signing proposal: %s", err)
	}
	time.Sleep(time.Until(dt))
	prop.Signature = tp.Signature
	if err := p.observedState.SetProposalAndBlock(prop, b, ps, "peerID"); err != nil {
		t.Fatal(err)
	}
	ensureProposal(t, p.proposalCh, p.currentHeight, 0, bid)

	signAddVotes(p.observedState, tmproto.PrevoteType, p.chainID, nextProposedTime, bid, p.observedValidator)
	signAddVotes(p.observedState, tmproto.PrevoteType, p.chainID, nextProposedTime, bid, p.otherValidators...)

	signAddVotes(p.observedState, tmproto.PrecommitType, p.chainID, nextProposedTime, bid, p.observedValidator)
	signAddVotes(p.observedState, tmproto.PrecommitType, p.chainID, nextProposedTime, bid, p.otherValidators...)
	ensureNewBlock(t, p.blockCh, p.currentHeight)
	p.currentHeight++
	incrementHeight(p.otherValidators...)
	return heightResult{}
}

func (p *pbtsTestHarness) run(t *testing.T) resultSet {
	p.genesisHeight(t)
	r2 := p.height2(t)
	r3 := p.height3(t)
	return resultSet{
		height2: r2,
		height3: r3,
	}
}

type resultSet struct {
	height2 heightResult
	height3 heightResult
}
type heightResult struct {
	prevote           *types.Vote
	prevoteIssuedAt   time.Time
	precommit         *types.Vote
	precommitIssuedAt time.Time
}

func TestValidatorWaitsForPreviousBlockTime(t *testing.T) {
	h := newPBTSTestHarness(t, pbtsTestConfiguration{
		genesisTime:                time.Now(),
		timeoutPropose:             time.Second * 3,
		height2ProposalDeliverTime: time.Now().Add(time.Second * 1),
		height2ProposedBlockTime:   time.Now().Add(time.Second * 1),
		height3ProposalDeliverTime: time.Now().Add(time.Second * 3),
		height3ProposedBlockTime:   time.Now().Add(time.Second * 3),
	})
	results := h.run(t)
	fmt.Println(results)
	/*
		assert.NotNil(t, h.r.height2Prevote.BlockID.Hash)
		assert.NotNil(t, h.r.height2Precommit.BlockID.Hash)
	*/
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
