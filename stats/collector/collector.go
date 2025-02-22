package collector

import (
	"github.com/idena-network/idena-go/blockchain/types"
	"github.com/idena-network/idena-go/common"
	"github.com/idena-network/idena-go/core/appstate"
	"github.com/idena-network/idena-go/core/validators"
	statsTypes "github.com/idena-network/idena-go/stats/types"
	"github.com/shopspring/decimal"
	"math/big"
	"time"
)

type StatsCollector interface {
	EnableCollecting()
	CompleteCollecting()

	SetValidation(validation *statsTypes.ValidationStats)
	SetMinScoreForInvite(score float32)

	SetValidationResults(validationResults map[common.ShardId]*types.ValidationResults)

	SetTotalReward(amount *big.Int)
	SetTotalValidationReward(amount *big.Int, share *big.Int)
	SetTotalStakingReward(amount *big.Int, share *big.Int)
	SetTotalCandidateReward(amount *big.Int, share *big.Int)
	SetTotalFlipsReward(amount *big.Int, share *big.Int)
	SetTotalReportsReward(amount *big.Int, share *big.Int)
	SetTotalInvitationsReward(amount *big.Int, share *big.Int)
	SetTotalFoundationPayouts(amount *big.Int)
	SetTotalZeroWalletFund(amount *big.Int)
	AddValidationReward(balanceDest, stakeDest common.Address, age uint16, balance, stake *big.Int)
	AddCandidateReward(balanceDest, stakeDest common.Address, balance, stake *big.Int)
	AddStakingReward(balanceDest, stakeDest common.Address, stakedAmount *big.Int, balance, stake *big.Int)
	AddFlipsReward(balanceDest, stakeDest common.Address, balance, stake *big.Int, flipsToReward []*types.FlipToReward)
	AddReportedFlipsReward(balanceDest, stakeDest common.Address, shardId common.ShardId, flipIdx int, balance, stake *big.Int)
	AddInvitationsReward(balanceDest, stakeDest common.Address, balance, stake *big.Int, age uint16, txHash *common.Hash,
		epochHeight uint32, isSavedInviteWinner bool)
	AddFoundationPayout(addr common.Address, balance *big.Int)
	AddZeroWalletFund(addr common.Address, balance *big.Int)

	AddProposerReward(balanceDest, stakeDest common.Address, balance, stake *big.Int, stakeWeight *big.Float)
	AddFinalCommitteeReward(balanceDest, stakeDest common.Address, balance, stake *big.Int, stakeWeight *big.Float)

	BeforeSetPenalty(addr common.Address, seconds uint16, appState *appstate.AppState)

	AddMintedCoins(amount *big.Int)
	AddPenaltyBurntCoins(addr common.Address, amount *big.Int)
	AddInviteBurntCoins(addr common.Address, amount *big.Int, tx *types.Transaction)
	AddFeeBurntCoins(addr common.Address, feeAmount *big.Int, burntRate float32, tx *types.Transaction)
	AddKilledBurntCoins(addr common.Address, amount *big.Int)
	AddBurnTxBurntCoins(addr common.Address, tx *types.Transaction)

	AfterAddStake(addr common.Address, amount *big.Int, appState *appstate.AppState)

	AddActivationTxBalanceTransfer(tx *types.Transaction, amount *big.Int)
	AddKillTxStakeTransfer(tx *types.Transaction, amount *big.Int)

	BeginVerifiedStakeTransferBalanceUpdate(addrFrom, addrTo common.Address, appState *appstate.AppState)
	BeginTxBalanceUpdate(tx *types.Transaction, appState *appstate.AppState)
	BeginProposerRewardBalanceUpdate(balanceDest, stakeDest common.Address, potentialPenaltyPayment *big.Int, appState *appstate.AppState)
	BeginCommitteeRewardBalanceUpdate(balanceDest, stakeDest common.Address, potentialPenaltyPayment *big.Int, appState *appstate.AppState)
	BeginEpochRewardBalanceUpdate(balanceDest, stakeDest common.Address, appState *appstate.AppState)
	BeginFailedValidationBalanceUpdate(addr common.Address, appState *appstate.AppState)
	BeginPenaltyBalanceUpdate(addr common.Address, appState *appstate.AppState)
	BeginEpochPenaltyResetBalanceUpdate(addr common.Address, appState *appstate.AppState)
	BeginIdentityClearingBalanceUpdate(addr common.Address, appState *appstate.AppState)
	BeginDustClearingBalanceUpdate(addr common.Address, appState *appstate.AppState)
	BeginSavedStakeBalanceUpdate(addr common.Address, appState *appstate.AppState)
	CompleteBalanceUpdate(appState *appstate.AppState)

	BeginApplyingTx(tx *types.Transaction, appState *appstate.AppState)
	CompleteApplyingTx(appState *appstate.AppState)
	AddTxFee(feeAmount *big.Int)

	AddContractStake(amount *big.Int)
	AddContractBalanceUpdate(address common.Address, getCurrentBalance GetBalanceFunc, newBalance *big.Int, appState *appstate.AppState)
	AddContractBurntCoins(address common.Address, getAmount GetBalanceFunc)
	AddContractTerminationBurntCoins(address common.Address, stake, refund *big.Int)

	AddOracleVotingDeploy(contractAddress common.Address, startTime uint64, votingMinPayment *big.Int,
		fact []byte, state byte, votingDuration, publicVotingDuration uint64, winnerThreshold, quorum byte,
		committeeSize uint64, ownerFee byte, ownerDeposit, oracleRewardFund *big.Int, refundRecipient *common.Address, hash []byte)
	AddOracleVotingCallStart(state byte, startBlock uint64, epoch uint16, votingMinPayment *big.Int, vrfSeed []byte, committeeSize uint64, networkSize int)
	AddOracleVotingCallVoteProof(voteHash []byte, newSecretVotesCount *uint64, discriminated bool)
	AddOracleVotingCallVote(vote byte, salt []byte, newOptionVotes *uint64, newOptionAllVotes uint64,
		newSecretVotesCount *uint64, delegatee *common.Address, prevPoolVote []byte, newPrevOptionVotes *uint64, discriminated bool)
	AddOracleVotingCallFinish(state byte, result *byte, fund, oracleReward, ownerReward *big.Int)
	AddOracleVotingCallProlongation(startBlock *uint64, epoch uint16, vrfSeed []byte, committeeSize, networkSize uint64,
		newEpochWithoutGrowth *byte, newProlongVoteCount *uint64)
	AddOracleVotingCallAddStake()
	AddOracleVotingTermination(fund, oracleReward, ownerReward *big.Int)

	AddOracleLockDeploy(contractAddress common.Address, oracleVotingAddress common.Address, value byte, successAddress common.Address,
		failAddress common.Address)
	AddOracleLockCallPush(success bool, oracleVotingResult byte, transfer *big.Int)
	AddOracleLockCallCheckOracleVoting(votedValue byte, err error)
	AddOracleLockTermination(dest common.Address)

	AddRefundableOracleLockDeploy(contractAddress common.Address, oracleVotingAddress common.Address, value byte,
		successAddress common.Address, successAddressErr error, failAddress common.Address, failAddressErr error,
		refundDelay, depositDeadline uint64, oracleVotingFee byte, state byte, sum *big.Int)
	AddRefundableOracleLockCallDeposit(ownSum, sum, fee *big.Int)
	AddRefundableOracleLockCallPush(state byte, oracleVotingExists bool, oracleVotingResult byte, oracleVotingResultErr error, transfer *big.Int, refundBlock uint64)
	AddRefundableOracleLockCallRefund(balance *big.Int, coef decimal.Decimal)
	AddRefundableOracleLockTermination(dest common.Address)

	AddMultisigDeploy(contractAddress common.Address, minVotes, maxVotes, state byte)
	AddMultisigCallAdd(address common.Address, newState *byte)
	AddMultisigCallSend(dest common.Address, amount []byte)
	AddMultisigCallPush(dest common.Address, amount []byte, voteAddressCnt, voteAmountCnt int)
	AddMultisigTermination(dest common.Address)

	AddTimeLockDeploy(contractAddress common.Address, timestamp uint64)
	AddTimeLockCallTransfer(dest common.Address, amount *big.Int)
	AddTimeLockTermination(dest common.Address)

	AddTxReceipt(txReceipt *types.TxReceipt, appState *appstate.AppState)

	RemoveMemPoolTx(tx *types.Transaction)
	SubmitVoteCountingStepResult(round uint64, step uint8, votesByBlock map[common.Hash]map[common.Address]*types.Vote, necessaryVotesCount, checkedRoundVotes int)
	SubmitVoteCountingResult(round uint64, step uint8, validators *validators.StepValidators, hash common.Hash, cert *types.FullBlockCert, err error)
	SubmitProofProposal(round uint64, hash common.Hash, proposerPubKey []byte, modifier int)
	SubmitBlockProposal(proposal *types.BlockProposal, receivingTime time.Time)

	AddRemovedTransitiveDelegation(delegator, delegatee common.Address)
}

type GetBalanceFunc func(address common.Address) *big.Int

type collectorStub struct {
}

func NewStatsCollector() StatsCollector {
	return &collectorStub{}
}

func (c *collectorStub) EnableCollecting() {
	// do nothing
}

func (c *collectorStub) RemoveMemPoolTx(tx *types.Transaction) {
	// do nothing
}

func (c *collectorStub) SubmitVoteCountingStepResult(round uint64, step uint8, votesByBlock map[common.Hash]map[common.Address]*types.Vote, necessaryVotesCount, checkedRoundVotes int) {
	// do nothing
}

func (c *collectorStub) SubmitVoteCountingResult(round uint64, step uint8, validators *validators.StepValidators, hash common.Hash, cert *types.FullBlockCert, err error) {
	// do nothing
}

func (c *collectorStub) SubmitProofProposal(round uint64, hash common.Hash, proposerPubKey []byte, modifier int) {
	// do nothing
}

func (c *collectorStub) SubmitBlockProposal(proposal *types.BlockProposal, receivingTime time.Time) {
	// do nothing
}

func (c *collectorStub) SetValidation(validation *statsTypes.ValidationStats) {
	// do nothing
}

func SetValidation(c StatsCollector, validation *statsTypes.ValidationStats) {
	if c == nil {
		return
	}
	c.SetValidation(validation)
}

func (c *collectorStub) SetMinScoreForInvite(score float32) {
	// do nothing
}

func SetMinScoreForInvite(c StatsCollector, score float32) {
	if c == nil {
		return
	}
	c.SetMinScoreForInvite(score)
}

func (c *collectorStub) SetValidationResults(validationResults map[common.ShardId]*types.ValidationResults) {
	// do nothing
}

func SetValidationResults(c StatsCollector, validationResults map[common.ShardId]*types.ValidationResults) {
	if c == nil {
		return
	}
	c.SetValidationResults(validationResults)
}

func (c *collectorStub) SetTotalReward(amount *big.Int) {
	// do nothing
}

func SetTotalReward(c StatsCollector, amount *big.Int) {
	if c == nil {
		return
	}
	c.SetTotalReward(amount)
}

func (c *collectorStub) SetTotalValidationReward(amount *big.Int, share *big.Int) {
	// do nothing
}

func SetTotalValidationReward(c StatsCollector, amount *big.Int, share *big.Int) {
	if c == nil {
		return
	}
	c.SetTotalValidationReward(amount, share)
}

func (c *collectorStub) SetTotalStakingReward(amount *big.Int, share *big.Int) {
	// do nothing
}

func SetTotalStakingReward(c StatsCollector, amount *big.Int, share *big.Int) {
	if c == nil {
		return
	}
	c.SetTotalStakingReward(amount, share)
}

func (c *collectorStub) SetTotalCandidateReward(amount *big.Int, share *big.Int) {
	// do nothing
}

func SetTotalCandidateReward(c StatsCollector, amount *big.Int, share *big.Int) {
	if c == nil {
		return
	}
	c.SetTotalCandidateReward(amount, share)
}

func (c *collectorStub) SetTotalFlipsReward(amount *big.Int, share *big.Int) {
	// do nothing
}

func SetTotalFlipsReward(c StatsCollector, amount *big.Int, share *big.Int) {
	if c == nil {
		return
	}
	c.SetTotalFlipsReward(amount, share)
}

func (c *collectorStub) SetTotalReportsReward(amount *big.Int, share *big.Int) {
	// do nothing
}

func SetTotalReportsReward(c StatsCollector, amount *big.Int, share *big.Int) {
	if c == nil {
		return
	}
	c.SetTotalReportsReward(amount, share)
}

func (c *collectorStub) SetTotalInvitationsReward(amount *big.Int, share *big.Int) {
	// do nothing
}

func SetTotalInvitationsReward(c StatsCollector, amount *big.Int, share *big.Int) {
	if c == nil {
		return
	}
	c.SetTotalInvitationsReward(amount, share)
}

func (c *collectorStub) SetTotalFoundationPayouts(amount *big.Int) {
	// do nothing
}

func SetTotalFoundationPayouts(c StatsCollector, amount *big.Int) {
	if c == nil {
		return
	}
	c.SetTotalFoundationPayouts(amount)
}

func (c *collectorStub) SetTotalZeroWalletFund(amount *big.Int) {
	// do nothing
}

func SetTotalZeroWalletFund(c StatsCollector, amount *big.Int) {
	if c == nil {
		return
	}
	c.SetTotalZeroWalletFund(amount)
}

func (c *collectorStub) AddValidationReward(balanceDest, stakeDest common.Address, age uint16, balance, stake *big.Int) {
	// do nothing
}

func AddValidationReward(c StatsCollector, balanceDest, stakeDest common.Address, age uint16, balance, stake *big.Int) {
	if c == nil {
		return
	}
	c.AddValidationReward(balanceDest, stakeDest, age, balance, stake)
}

func (c *collectorStub) AddStakingReward(balanceDest, stakeDest common.Address, stakedAmount *big.Int, balance, stake *big.Int) {
	// do nothing
}

func AddStakingReward(c StatsCollector, balanceDest, stakeDest common.Address, stakedAmount *big.Int, balance, stake *big.Int) {
	if c == nil {
		return
	}
	c.AddStakingReward(balanceDest, stakeDest, stakedAmount, balance, stake)
}

func (c *collectorStub) AddCandidateReward(balanceDest, stakeDest common.Address, balance, stake *big.Int) {
	// do nothing
}

func AddCandidateReward(c StatsCollector, balanceDest, stakeDest common.Address, balance, stake *big.Int) {
	if c == nil {
		return
	}
	c.AddCandidateReward(balanceDest, stakeDest, balance, stake)
}

func (c *collectorStub) AddFlipsReward(balanceDest, stakeDest common.Address, balance, stake *big.Int, flipsToReward []*types.FlipToReward) {
	// do nothing
}

func AddFlipsReward(c StatsCollector, balanceDest, stakeDest common.Address, balance, stake *big.Int, flipsToReward []*types.FlipToReward) {
	if c == nil {
		return
	}
	c.AddFlipsReward(balanceDest, stakeDest, balance, stake, flipsToReward)
}

func (c *collectorStub) AddReportedFlipsReward(balanceDest, stakeDest common.Address, shardId common.ShardId, flipIdx int, balance, stake *big.Int) {
	// do nothing
}

func AddReportedFlipsReward(c StatsCollector, balanceDest, stakeDest common.Address, shardId common.ShardId, flipIdx int, balance, stake *big.Int) {
	if c == nil {
		return
	}
	c.AddReportedFlipsReward(balanceDest, stakeDest, shardId, flipIdx, balance, stake)
}

func (c *collectorStub) AddInvitationsReward(balanceDest, stakeDest common.Address, balance, stake *big.Int, age uint16,
	txHash *common.Hash, epochHeight uint32, isSavedInviteWinner bool) {
	// do nothing
}

func AddInvitationsReward(c StatsCollector, balanceDest, stakeDest common.Address, balance, stake *big.Int, age uint16,
	txHash *common.Hash, epochHeight uint32, isSavedInviteWinner bool) {
	if c == nil {
		return
	}
	c.AddInvitationsReward(balanceDest, stakeDest, balance, stake, age, txHash, epochHeight, isSavedInviteWinner)
}

func (c *collectorStub) AddFoundationPayout(addr common.Address, balance *big.Int) {
	// do nothing
}

func AddFoundationPayout(c StatsCollector, addr common.Address, balance *big.Int) {
	if c == nil {
		return
	}
	c.AddFoundationPayout(addr, balance)
}

func (c *collectorStub) AddZeroWalletFund(addr common.Address, balance *big.Int) {
	// do nothing
}

func AddZeroWalletFund(c StatsCollector, addr common.Address, balance *big.Int) {
	if c == nil {
		return
	}
	c.AddZeroWalletFund(addr, balance)
}

func (c *collectorStub) AddProposerReward(balanceDest, stakeDest common.Address, balance, stake *big.Int, stakeWeight *big.Float) {
	// do nothing
}

func AddProposerReward(c StatsCollector, balanceDest, stakeDest common.Address, balance, stake *big.Int, stakeWeight *big.Float) {
	if c == nil {
		return
	}
	c.AddProposerReward(balanceDest, stakeDest, balance, stake, stakeWeight)
}

func (c *collectorStub) AddFinalCommitteeReward(balanceDest, stakeDest common.Address, balance, stake *big.Int, stakeWeight *big.Float) {
	// do nothing
}

func AddFinalCommitteeReward(c StatsCollector, balanceDest, stakeDest common.Address, balance, stake *big.Int, stakeWeight *big.Float) {
	if c == nil {
		return
	}
	c.AddFinalCommitteeReward(balanceDest, stakeDest, balance, stake, stakeWeight)
}

func (c *collectorStub) CompleteCollecting() {
	// do nothing
}

func (c *collectorStub) BeforeSetPenalty(addr common.Address, seconds uint16, appState *appstate.AppState) {
	// do nothing
}

func BeforeSetPenalty(c StatsCollector, addr common.Address, seconds uint16, appState *appstate.AppState) {
	if c == nil {
		return
	}
	c.BeforeSetPenalty(addr, seconds, appState)
}

func (c *collectorStub) AddMintedCoins(amount *big.Int) {
	// do nothing
}

func AddMintedCoins(c StatsCollector, amount *big.Int) {
	if c == nil {
		return
	}
	c.AddMintedCoins(amount)
}

func (c *collectorStub) AddPenaltyBurntCoins(addr common.Address, amount *big.Int) {
	// do nothing
}

func AddPenaltyBurntCoins(c StatsCollector, addr common.Address, amount *big.Int) {
	if c == nil {
		return
	}
	c.AddPenaltyBurntCoins(addr, amount)
}

func (c *collectorStub) AddInviteBurntCoins(addr common.Address, amount *big.Int, tx *types.Transaction) {
	// do nothing
}

func AddInviteBurntCoins(c StatsCollector, addr common.Address, amount *big.Int, tx *types.Transaction) {
	if c == nil {
		return
	}
	c.AddInviteBurntCoins(addr, amount, tx)
}

func (c *collectorStub) AddFeeBurntCoins(addr common.Address, feeAmount *big.Int, burntRate float32,
	tx *types.Transaction) {
	// do nothing
}

func AddFeeBurntCoins(c StatsCollector, addr common.Address, feeAmount *big.Int, burntRate float32,
	tx *types.Transaction) {
	if c == nil {
		return
	}
	c.AddFeeBurntCoins(addr, feeAmount, burntRate, tx)
}

func (c *collectorStub) AddKilledBurntCoins(addr common.Address, amount *big.Int) {
	// do nothing
}

func AddKilledBurntCoins(c StatsCollector, addr common.Address, amount *big.Int) {
	if c == nil {
		return
	}
	c.AddKilledBurntCoins(addr, amount)
}

func (c *collectorStub) AddBurnTxBurntCoins(addr common.Address, tx *types.Transaction) {
	// do nothing
}

func AddBurnTxBurntCoins(c StatsCollector, addr common.Address, tx *types.Transaction) {
	if c == nil {
		return
	}
	c.AddBurnTxBurntCoins(addr, tx)
}

func (c *collectorStub) AfterAddStake(addr common.Address, amount *big.Int, appState *appstate.AppState) {
	// do nothing
}

func AfterAddStake(c StatsCollector, addr common.Address, amount *big.Int, appState *appstate.AppState) {
	if c == nil {
		return
	}
	c.AfterAddStake(addr, amount, appState)
}

func (c *collectorStub) AddActivationTxBalanceTransfer(tx *types.Transaction, amount *big.Int) {
	// do nothing
}

func AddActivationTxBalanceTransfer(c StatsCollector, tx *types.Transaction, amount *big.Int) {
	if c == nil {
		return
	}
	c.AddActivationTxBalanceTransfer(tx, amount)
}

func (c *collectorStub) AddKillTxStakeTransfer(tx *types.Transaction, amount *big.Int) {
	// do nothing
}

func AddKillTxStakeTransfer(c StatsCollector, tx *types.Transaction, amount *big.Int) {
	if c == nil {
		return
	}
	c.AddKillTxStakeTransfer(tx, amount)
}

func (c *collectorStub) BeginVerifiedStakeTransferBalanceUpdate(addrFrom, addrTo common.Address, appState *appstate.AppState) {
	// do nothing
}

func BeginVerifiedStakeTransferBalanceUpdate(c StatsCollector, addrFrom, addrTo common.Address, appState *appstate.AppState) {
	if c == nil {
		return
	}
	c.BeginVerifiedStakeTransferBalanceUpdate(addrFrom, addrTo, appState)
}

func (c *collectorStub) BeginTxBalanceUpdate(tx *types.Transaction, appState *appstate.AppState) {
	// do nothing
}

func BeginTxBalanceUpdate(c StatsCollector, tx *types.Transaction, appState *appstate.AppState) {
	if c == nil {
		return
	}
	c.BeginTxBalanceUpdate(tx, appState)
}

func (c *collectorStub) BeginProposerRewardBalanceUpdate(balanceDest, stakeDest common.Address, potentialPenaltyPayment *big.Int, appState *appstate.AppState) {
	// do nothing
}

func BeginProposerRewardBalanceUpdate(c StatsCollector, balanceDest, stakeDest common.Address, potentialPenaltyPayment *big.Int, appState *appstate.AppState) {
	if c == nil {
		return
	}
	c.BeginProposerRewardBalanceUpdate(balanceDest, stakeDest, potentialPenaltyPayment, appState)
}

func (c *collectorStub) BeginCommitteeRewardBalanceUpdate(balanceDest, stakeDest common.Address, potentialPenaltyPayment *big.Int, appState *appstate.AppState) {
	// do nothing
}

func BeginCommitteeRewardBalanceUpdate(c StatsCollector, balanceDest, stakeDest common.Address, potentialPenaltyPayment *big.Int, appState *appstate.AppState) {
	if c == nil {
		return
	}
	c.BeginCommitteeRewardBalanceUpdate(balanceDest, stakeDest, potentialPenaltyPayment, appState)
}

func (c *collectorStub) BeginEpochRewardBalanceUpdate(balanceDest, stakeDest common.Address, appState *appstate.AppState) {
	// do nothing
}

func BeginEpochRewardBalanceUpdate(c StatsCollector, balanceDest, stakeDest common.Address, appState *appstate.AppState) {
	if c == nil {
		return
	}
	c.BeginEpochRewardBalanceUpdate(balanceDest, stakeDest, appState)
}

func (c *collectorStub) BeginFailedValidationBalanceUpdate(addr common.Address, appState *appstate.AppState) {
	// do nothing
}

func BeginFailedValidationBalanceUpdate(c StatsCollector, addr common.Address, appState *appstate.AppState) {
	if c == nil {
		return
	}
	c.BeginFailedValidationBalanceUpdate(addr, appState)
}

func (c *collectorStub) BeginPenaltyBalanceUpdate(addr common.Address, appState *appstate.AppState) {
	// do nothing
}

func BeginPenaltyBalanceUpdate(c StatsCollector, addr common.Address, appState *appstate.AppState) {
	if c == nil {
		return
	}
	c.BeginPenaltyBalanceUpdate(addr, appState)
}

func (c *collectorStub) BeginEpochPenaltyResetBalanceUpdate(addr common.Address, appState *appstate.AppState) {
	// do nothing
}

func BeginEpochPenaltyResetBalanceUpdate(c StatsCollector, addr common.Address, appState *appstate.AppState) {
	if c == nil {
		return
	}
	c.BeginEpochPenaltyResetBalanceUpdate(addr, appState)
}

func (c *collectorStub) BeginIdentityClearingBalanceUpdate(addr common.Address, appState *appstate.AppState) {
	// do nothing
}

func BeginIdentityClearingBalanceUpdate(c StatsCollector, addr common.Address, appState *appstate.AppState) {
	if c == nil {
		return
	}
	c.BeginIdentityClearingBalanceUpdate(addr, appState)
}

func (c *collectorStub) BeginDustClearingBalanceUpdate(addr common.Address, appState *appstate.AppState) {
	// do nothing
}

func BeginDustClearingBalanceUpdate(c StatsCollector, addr common.Address, appState *appstate.AppState) {
	if c == nil {
		return
	}
	c.BeginDustClearingBalanceUpdate(addr, appState)
}

func (c *collectorStub) BeginSavedStakeBalanceUpdate(addr common.Address, appState *appstate.AppState) {
	// do nothing
}

func BeginSavedStakeBalanceUpdate(c StatsCollector, addr common.Address, appState *appstate.AppState) {
	if c == nil {
		return
	}
	c.BeginSavedStakeBalanceUpdate(addr, appState)
}

func (c *collectorStub) CompleteBalanceUpdate(appState *appstate.AppState) {
	// do nothing
}

func CompleteBalanceUpdate(c StatsCollector, appState *appstate.AppState) {
	if c == nil {
		return
	}
	c.CompleteBalanceUpdate(appState)
}

func (c *collectorStub) BeginApplyingTx(tx *types.Transaction, appState *appstate.AppState) {
	// do nothing
}

func BeginApplyingTx(c StatsCollector, tx *types.Transaction, appState *appstate.AppState) {
	if c == nil {
		return
	}
	c.BeginApplyingTx(tx, appState)
}

func (c *collectorStub) CompleteApplyingTx(appState *appstate.AppState) {
	// do nothing
}

func CompleteApplyingTx(c StatsCollector, appState *appstate.AppState) {
	if c == nil {
		return
	}
	c.CompleteApplyingTx(appState)
}

func (c *collectorStub) AddTxFee(feeAmount *big.Int) {
	// do nothing
}

func AddTxFee(c StatsCollector, feeAmount *big.Int) {
	if c == nil {
		return
	}
	c.AddTxFee(feeAmount)
}

func (c *collectorStub) AddContractStake(amount *big.Int) {
	// do nothing
}

func AddContractStake(c StatsCollector, amount *big.Int) {
	if c == nil {
		return
	}
	c.AddContractStake(amount)
}

func (c *collectorStub) AddContractBalanceUpdate(address common.Address, getCurrentBalance GetBalanceFunc, newBalance *big.Int, appState *appstate.AppState) {
	// do nothing
}

func AddContractBalanceUpdate(c StatsCollector, address common.Address, getCurrentBalance GetBalanceFunc, newBalance *big.Int, appState *appstate.AppState) {
	if c == nil {
		return
	}
	c.AddContractBalanceUpdate(address, getCurrentBalance, newBalance, appState)
}

func (c *collectorStub) AddContractBurntCoins(address common.Address, getAmount GetBalanceFunc) {
	// do nothing
}

func AddContractBurntCoins(c StatsCollector, address common.Address, getAmount GetBalanceFunc) {
	if c == nil {
		return
	}
	c.AddContractBurntCoins(address, getAmount)
}

func (c *collectorStub) AddContractTerminationBurntCoins(address common.Address, stake, refund *big.Int) {
	// do nothing
}

func AddContractTerminationBurntCoins(c StatsCollector, address common.Address, stake, refund *big.Int) {
	if c == nil {
		return
	}
	c.AddContractTerminationBurntCoins(address, stake, refund)
}

func (c *collectorStub) AddOracleVotingDeploy(contractAddress common.Address, startTime uint64,
	votingMinPayment *big.Int, fact []byte, state byte, votingDuration, publicVotingDuration uint64, winnerThreshold, quorum byte,
	committeeSize uint64, ownerFee byte, ownerDeposit, oracleRewardFund *big.Int, refundRecipient *common.Address, hash []byte) {
	// do nothing
}

func AddOracleVotingDeploy(c StatsCollector, contractAddress common.Address, startTime uint64,
	votingMinPayment *big.Int, fact []byte, state byte, votingDuration, publicVotingDuration uint64, winnerThreshold, quorum byte,
	committeeSize uint64, ownerFee byte, ownerDeposit, oracleRewardFund *big.Int, refundRecipient *common.Address, hash []byte) {
	if c == nil {
		return
	}
	c.AddOracleVotingDeploy(contractAddress, startTime, votingMinPayment, fact, state, votingDuration,
		publicVotingDuration, winnerThreshold, quorum, committeeSize, ownerFee, ownerDeposit, oracleRewardFund, refundRecipient, hash)
}

func (c *collectorStub) AddOracleVotingCallStart(state byte, startBlock uint64, epoch uint16, votingMinPayment *big.Int, vrfSeed []byte,
	committeeSize uint64, networkSize int) {
	// do nothing
}

func AddOracleVotingCallStart(c StatsCollector, state byte, startBlock uint64, epoch uint16, votingMinPayment *big.Int, vrfSeed []byte,
	committeeSize uint64, networkSize int) {
	if c == nil {
		return
	}
	c.AddOracleVotingCallStart(state, startBlock, epoch, votingMinPayment, vrfSeed, committeeSize, networkSize)
}

func (c *collectorStub) AddOracleVotingCallVoteProof(voteHash []byte, newSecretVotesCount *uint64, discriminated bool) {
	// do nothing
}

func AddOracleVotingCallVoteProof(c StatsCollector, voteHash []byte, newSecretVotesCount *uint64, discriminated bool) {
	if c == nil {
		return
	}
	c.AddOracleVotingCallVoteProof(voteHash, newSecretVotesCount, discriminated)
}

func (c *collectorStub) AddOracleVotingCallVote(vote byte, salt []byte, newOptionVotes *uint64, newOptionAllVotes uint64,
	newSecretVotesCount *uint64, delegatee *common.Address, prevPoolVote []byte, newPrevOptionVotes *uint64, discriminated bool) {
	// do nothing
}

func AddOracleVotingCallVote(c StatsCollector, vote byte, salt []byte, newOptionVotes *uint64, newOptionAllVotes uint64,
	newSecretVotesCount *uint64, delegatee *common.Address, prevPoolVote []byte, newPrevOptionVotes *uint64, discriminated bool) {
	if c == nil {
		return
	}
	c.AddOracleVotingCallVote(vote, salt, newOptionVotes, newOptionAllVotes, newSecretVotesCount, delegatee, prevPoolVote, newPrevOptionVotes, discriminated)
}

func (c *collectorStub) AddOracleVotingCallFinish(state byte, result *byte, fund, oracleReward, ownerReward *big.Int) {
	// do nothing
}

func AddOracleVotingCallFinish(c StatsCollector, state byte, result *byte, fund, oracleReward, ownerReward *big.Int) {
	if c == nil {
		return
	}
	c.AddOracleVotingCallFinish(state, result, fund, oracleReward, ownerReward)
}

func (c *collectorStub) AddOracleVotingCallProlongation(startBlock *uint64, epoch uint16, vrfSeed []byte, committeeSize, networkSize uint64,
	newEpochWithoutGrowth *byte, newProlongVoteCount *uint64) {
	// do nothing
}

func AddOracleVotingCallProlongation(c StatsCollector, startBlock *uint64, epoch uint16, vrfSeed []byte, committeeSize, networkSize uint64,
	newEpochWithoutGrowth *byte, newProlongVoteCount *uint64) {
	if c == nil {
		return
	}
	c.AddOracleVotingCallProlongation(startBlock, epoch, vrfSeed, committeeSize, networkSize, newEpochWithoutGrowth, newProlongVoteCount)
}

func (c *collectorStub) AddOracleVotingCallAddStake() {
	// do nothing
}

func AddOracleVotingCallAddStake(c StatsCollector) {
	if c == nil {
		return
	}
	c.AddOracleVotingCallAddStake()
}

func (c *collectorStub) AddOracleVotingTermination(fund, oracleReward, ownerReward *big.Int) {
	// do nothing
}

func AddOracleVotingTermination(c StatsCollector, fund, oracleReward, ownerReward *big.Int) {
	if c == nil {
		return
	}
	c.AddOracleVotingTermination(fund, oracleReward, ownerReward)
}

func (c *collectorStub) AddOracleLockDeploy(contractAddress common.Address, oracleVotingAddress common.Address,
	value byte, successAddress common.Address, failAddress common.Address) {
	// do nothing
}

func AddOracleLockDeploy(c StatsCollector, contractAddress common.Address, oracleVotingAddress common.Address,
	value byte, successAddress common.Address, failAddress common.Address) {
	if c == nil {
		return
	}
	c.AddOracleLockDeploy(contractAddress, oracleVotingAddress, value, successAddress, failAddress)
}

func (c *collectorStub) AddOracleLockCallPush(success bool, oracleVotingResult byte, transfer *big.Int) {
	// do nothing
}

func AddOracleLockCallPush(c StatsCollector, success bool, oracleVotingResult byte, transfer *big.Int) {
	if c == nil {
		return
	}
	c.AddOracleLockCallPush(success, oracleVotingResult, transfer)
}

func (c *collectorStub) AddOracleLockCallCheckOracleVoting(votedValue byte, err error) {
	// do nothing
}

func AddOracleLockCallCheckOracleVoting(c StatsCollector, votedValue byte, err error) {
	if c == nil {
		return
	}
	c.AddOracleLockCallCheckOracleVoting(votedValue, err)
}

func (c *collectorStub) AddOracleLockTermination(dest common.Address) {
	// do nothing
}

func AddOracleLockTermination(c StatsCollector, dest common.Address) {
	if c == nil {
		return
	}
	c.AddOracleLockTermination(dest)
}

func (c *collectorStub) AddRefundableOracleLockDeploy(contractAddress common.Address, oracleVotingAddress common.Address,
	value byte, successAddress common.Address, successAddressErr error, failAddress common.Address, failAddressErr error,
	refundDelay, depositDeadline uint64, oracleVotingFee byte, state byte, sum *big.Int) {
	// do nothing
}

func AddRefundableOracleLockDeploy(c StatsCollector, contractAddress common.Address, oracleVotingAddress common.Address,
	value byte, successAddress common.Address, successAddressErr error, failAddress common.Address, failAddressErr error,
	refundDelay, depositDeadline uint64, oracleVotingFee byte, state byte, sum *big.Int) {
	if c == nil {
		return
	}
	c.AddRefundableOracleLockDeploy(contractAddress, oracleVotingAddress, value, successAddress, successAddressErr,
		failAddress, failAddressErr, refundDelay, depositDeadline, oracleVotingFee, state, sum)
}

func (c *collectorStub) AddRefundableOracleLockCallDeposit(ownSum, sum, fee *big.Int) {
	// do nothing
}

func AddRefundableOracleLockCallDeposit(c StatsCollector, ownSum, sum, fee *big.Int) {
	if c == nil {
		return
	}
	c.AddRefundableOracleLockCallDeposit(ownSum, sum, fee)
}

func (c *collectorStub) AddRefundableOracleLockCallPush(state byte, oracleVotingExists bool, oracleVotingResult byte, oracleVotingResultErr error, transfer *big.Int, refundBlock uint64) {
	// do nothing
}

func AddRefundableOracleLockCallPush(c StatsCollector, state byte, oracleVotingExists bool, oracleVotingResult byte, oracleVotingResultErr error, transfer *big.Int, refundBlock uint64) {
	if c == nil {
		return
	}
	c.AddRefundableOracleLockCallPush(state, oracleVotingExists, oracleVotingResult, oracleVotingResultErr, transfer, refundBlock)
}

func (c *collectorStub) AddRefundableOracleLockCallRefund(balance *big.Int, coef decimal.Decimal) {
	// do nothing
}

func AddRefundableOracleLockCallRefund(c StatsCollector, balance *big.Int, coef decimal.Decimal) {
	if c == nil {
		return
	}
	c.AddRefundableOracleLockCallRefund(balance, coef)
}

func (c *collectorStub) AddRefundableOracleLockTermination(dest common.Address) {
	// do nothing
}

func AddRefundableOracleLockTermination(c StatsCollector, dest common.Address) {
	if c == nil {
		return
	}
	c.AddRefundableOracleLockTermination(dest)
}

func (c *collectorStub) AddMultisigDeploy(contractAddress common.Address, minVotes, maxVotes, state byte) {
	// do nothing
}

func AddMultisigDeploy(c StatsCollector, contractAddress common.Address, minVotes, maxVotes, state byte) {
	if c == nil {
		return
	}
	c.AddMultisigDeploy(contractAddress, minVotes, maxVotes, state)
}

func (c *collectorStub) AddMultisigCallAdd(address common.Address, newState *byte) {
	// do nothing
}

func AddMultisigCallAdd(c StatsCollector, address common.Address, newState *byte) {
	if c == nil {
		return
	}
	c.AddMultisigCallAdd(address, newState)
}

func (c *collectorStub) AddMultisigCallSend(dest common.Address, amount []byte) {
	// do nothing
}

func AddMultisigCallSend(c StatsCollector, dest common.Address, amount []byte) {
	if c == nil {
		return
	}
	c.AddMultisigCallSend(dest, amount)
}

func (c *collectorStub) AddMultisigCallPush(dest common.Address, amount []byte, voteAddressCnt, voteAmountCnt int) {
	// do nothing
}

func AddMultisigCallPush(c StatsCollector, dest common.Address, amount []byte, voteAddressCnt, voteAmountCnt int) {
	if c == nil {
		return
	}
	c.AddMultisigCallPush(dest, amount, voteAddressCnt, voteAmountCnt)
}

func (c *collectorStub) AddMultisigTermination(dest common.Address) {
	// do nothing
}

func AddMultisigTermination(c StatsCollector, dest common.Address) {
	if c == nil {
		return
	}
	c.AddMultisigTermination(dest)
}

func (c *collectorStub) AddTimeLockDeploy(contractAddress common.Address, timestamp uint64) {
	// do nothing
}

func AddTimeLockDeploy(c StatsCollector, contractAddress common.Address, timestamp uint64) {
	if c == nil {
		return
	}
	c.AddTimeLockDeploy(contractAddress, timestamp)
}

func (c *collectorStub) AddTimeLockCallTransfer(dest common.Address, amount *big.Int) {
	// do nothing
}

func AddTimeLockCallTransfer(c StatsCollector, dest common.Address, amount *big.Int) {
	if c == nil {
		return
	}
	c.AddTimeLockCallTransfer(dest, amount)
}

func (c *collectorStub) AddTimeLockTermination(dest common.Address) {
	// do nothing
}

func AddTimeLockTermination(c StatsCollector, dest common.Address) {
	if c == nil {
		return
	}
	c.AddTimeLockTermination(dest)
}

func (c *collectorStub) AddTxReceipt(txReceipt *types.TxReceipt, appState *appstate.AppState) {
	// do nothing
}

func AddTxReceipt(c StatsCollector, txReceipt *types.TxReceipt, appState *appstate.AppState) {
	if c == nil {
		return
	}
	c.AddTxReceipt(txReceipt, appState)
}

func (c *collectorStub) AddRemovedTransitiveDelegation(delegator, delegatee common.Address) {
	// do nothing
}

func AddRemovedTransitiveDelegation(c StatsCollector, delegator, delegatee common.Address) {
	if c == nil {
		return
	}
	c.AddRemovedTransitiveDelegation(delegator, delegatee)
}
