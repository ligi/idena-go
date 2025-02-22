package embedded

import (
	"bytes"
	"github.com/golang/protobuf/proto"
	"github.com/idena-network/idena-go/common"
	"github.com/idena-network/idena-go/common/math"
	"github.com/idena-network/idena-go/crypto"
	models "github.com/idena-network/idena-go/protobuf"
	"github.com/idena-network/idena-go/stats/collector"
	"github.com/idena-network/idena-go/vm/env"
	"github.com/idena-network/idena-go/vm/helpers"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	math2 "math"
	"math/big"
	"sort"
	"time"
)

var (
	maxHash *big.Float
)

const (
	oracleVotingStatePending  = byte(0)
	oracleVotingStateStarted  = byte(1)
	oracleVotingStateFinished = byte(2)
	keyFact                   = "fact"
	keyResult                 = "result"
	keyHash                   = "hash"
)

const FinishVotingMethod = "finishVoting"

func init() {
	var max [32]byte
	for i := range max {
		max[i] = 0xFF
	}
	i := new(big.Int)
	i.SetBytes(max[:])
	maxHash = new(big.Float).SetInt(i)
}

type OracleVoting6 struct {
	*BaseContract
	voteHashes  *env.Map
	votes       *env.Map
	voteOptions *env.Map
	poolVotes   *env.Map
	allVotes    *env.Map
}

func NewOracleVotingContract6(ctx env.CallContext, e env.Env, statsCollector collector.StatsCollector) *OracleVoting6 {
	return &OracleVoting6{
		&BaseContract{
			ctx:            ctx,
			env:            e,
			statsCollector: statsCollector,
		},
		env.NewMap([]byte("voteHashes"), e, ctx),
		env.NewMap([]byte("votes"), e, ctx),
		env.NewMap([]byte("voteOptions"), e, ctx),
		env.NewMap([]byte("poolVotes"), e, ctx),
		env.NewMap([]byte("allVotes"), e, ctx),
	}
}

func (f *OracleVoting6) Call(method string, args ...[]byte) error {
	switch method {
	case "startVoting":
		return f.startVoting()
	case "sendVoteProof":
		return f.sendVoteProof(args...)
	case "sendVote":
		return f.sendVote(args...)
	case FinishVotingMethod:
		return f.finishVoting(args...)
	case "prolongVoting":
		return f.prolongVoting(args...)
	case "addStake":
		return f.addStake(args...)
	default:
		return errors.New("unknown method")
	}
}

func hashArgs(args ...[]byte) ([]byte, error) {
	protoData := &models.ProtoOracleVotingHashData{
		Args: args,
	}
	data, err := proto.Marshal(protoData)
	if err != nil {
		return nil, err
	}
	hash := crypto.Hash128(data)
	return hash[:], nil
}

func (f *OracleVoting6) Deploy(args ...[]byte) error {
	fact, err := helpers.ExtractArray(0, args...)
	if err != nil {
		return err
	}
	startTime, err := helpers.ExtractUInt64(1, args...)
	if err != nil {
		return err
	}

	votingDuration := uint64(4320)
	publicVotingDuration := uint64(4320)
	winnerThreshold := byte(51)
	quorum := byte(20)
	networkSize := uint64(f.env.NetworkSize())
	committeeSize := math.Min(100, networkSize)
	ownerFee := byte(0)
	var votingMinPayment, oracleRewardFund *big.Int
	state := oracleVotingStatePending
	var refundRecipient *common.Address

	if value, err := helpers.ExtractUInt64(2, args...); err == nil {
		votingDuration = value
	}
	if value, err := helpers.ExtractUInt64(3, args...); err == nil {
		publicVotingDuration = uint64(math.MaxInt(100, int(value)))
	}
	if value, err := helpers.ExtractByte(4, args...); err == nil {
		winnerThreshold = byte(math.MinInt(math.MaxInt(51, int(value)), 100))
	}
	if value, err := helpers.ExtractByte(5, args...); err == nil {
		quorum = byte(math.MinInt(math.MaxInt(1, int(value)), 100))
	}

	if value, err := helpers.ExtractUInt64(6, args...); err == nil {
		committeeSize = math.Min(value, networkSize)
	}

	if value, err := helpers.ExtractBigInt(7, args...); err == nil {
		votingMinPayment = value
	}

	if value, err := helpers.ExtractByte(8, args...); err == nil {
		ownerFee = byte(math.MinInt(math.MaxInt(0, int(value)), 100))
	}
	if ownerFee > 0 {
		if value, err := helpers.ExtractBigInt(9, args...); err == nil {
			oracleRewardFund = value
		}
	}
	if value, err := helpers.ExtractAddr(10, args...); err == nil {
		refundRecipient = &value
	}
	ownerDeposit := calculateOwnerDeposit(committeeSize, networkSize)
	hash, err := hashArgs(args...)
	if err != nil {
		return errors.New("failed to hash args")
	}

	f.SetOwner(f.ctx.Sender())
	f.SetUint64("startTime", startTime)
	f.SetArray(keyFact, fact)
	f.SetByte("state", state)
	f.SetUint64("votingDuration", votingDuration)
	f.SetUint64("publicVotingDuration", publicVotingDuration)
	f.SetByte("winnerThreshold", winnerThreshold)
	f.SetByte("quorum", quorum)
	f.SetUint64("committeeSize", committeeSize)
	f.SetByte("ownerFee", ownerFee)
	if votingMinPayment != nil {
		f.SetBigInt("votingMinPayment", votingMinPayment)
	}
	f.SetByte("dis", 1)
	f.SetBigInt("ownerDeposit", ownerDeposit)
	if oracleRewardFund != nil {
		f.SetBigInt("oracleRewardFund", oracleRewardFund)
	}
	if refundRecipient != nil {
		f.SetArray("refundRecipient", refundRecipient.Bytes())
	}
	f.SetArray(keyHash, hash)

	collector.AddOracleVotingDeploy(f.statsCollector, f.ctx.ContractAddr(), startTime, votingMinPayment, fact,
		state, votingDuration, publicVotingDuration, winnerThreshold, quorum, committeeSize, ownerFee, ownerDeposit, oracleRewardFund, refundRecipient, hash)
	return nil
}

func calculateOwnerDeposit(committeeSize, networkSize uint64) *big.Int {
	return big.NewInt(0).Mul(minOracleReward(int(networkSize)), big.NewInt(int64(committeeSize)))
}

func minOracleReward(networkSize int) *big.Int {
	network := float64(networkSize)
	if network == 0 {
		network = 1
	}
	dnaReward := decimal.NewFromFloat(5000).Div(decimal.NewFromFloat(network))
	decimalOneDna := decimal.NewFromBigInt(common.DnaBase, 0)
	return math.ToInt(dnaReward.Mul(decimalOneDna))
}

func (f *OracleVoting6) startVoting() error {
	if f.GetByte("state") != oracleVotingStatePending {
		return errors.New("contract is not in pending state")
	}
	if uint64(f.env.BlockTimeStamp()) < f.GetUint64("startTime") {
		return errors.New("starting is locked")
	}

	balance := f.env.Balance(f.ctx.ContractAddr())
	committeeSize := f.GetUint64("committeeSize")
	networkSize := f.env.NetworkSize()
	if ownerDeposit := f.GetBigInt("ownerDeposit"); ownerDeposit != nil {
		if balance.Cmp(ownerDeposit) < 0 {
			return errors.New("contract balance is less than minimal deposit")
		}
	} else {
		oracleReward := minOracleReward(networkSize)
		minBalance := big.NewInt(0).Mul(oracleReward, big.NewInt(int64(committeeSize)))
		if balance.Cmp(minBalance) < 0 {
			return errors.New("contract balance is less than minimal oracles reward")
		}
	}
	f.SetByte("state", oracleVotingStateStarted)
	startBlock := f.env.BlockNumber()
	f.SetUint64("startBlock", startBlock)
	f.SetUint64("network", uint64(networkSize))
	var votingMinPayment *big.Int
	if f.GetBigInt("votingMinPayment") == nil {
		payment := decimal.NewFromBigInt(balance, 0)
		payment = payment.Div(decimal.New(20, 0))
		votingMinPayment = math.ToInt(payment)
		f.SetBigInt("votingMinPayment", votingMinPayment)
	}
	vrfSeed := f.env.BlockSeed()
	f.SetArray("vrfSeed", vrfSeed)
	epoch := f.env.Epoch()
	f.SetUint16("epoch", epoch)
	collector.AddOracleVotingCallStart(f.statsCollector, oracleVotingStateStarted, startBlock, epoch, votingMinPayment, vrfSeed, committeeSize, networkSize)
	return nil
}

func (f *OracleVoting6) sendVoteProof(args ...[]byte) error {
	voteHash, err := helpers.ExtractArray(0, args...)
	if err != nil {
		return err
	}
	if !f.env.State(f.ctx.Sender()).NewbieOrBetter() {
		return errors.New("sender is not identity")
	}
	if f.env.Epoch() != f.GetUint16("epoch") {
		return errors.New("voting should be prolonged")
	}
	if f.GetByte("state") != oracleVotingStateStarted {
		return errors.New("contract is not in running state")
	}
	if f.votes.Get(f.ctx.Sender().Bytes()) != nil {
		return errors.New("sender has voted already")
	}

	votingDuration := f.GetUint64("votingDuration")
	duration := f.env.BlockNumber() - f.GetUint64("startBlock")

	if duration >= votingDuration {
		return errors.New("too late to accept secret vote")
	}
	payment := f.GetBigInt("votingMinPayment")
	if payment.Cmp(f.ctx.PayAmount()) > 0 {
		return errors.New("tx amount is less than voting minimal payment")
	}

	pubKeyData := f.env.PubKey(f.ctx.Sender())

	selectionHash := crypto.Hash(append(pubKeyData, f.GetArray("vrfSeed")...))

	v := new(big.Float).SetInt(new(big.Int).SetBytes(selectionHash[:]))

	q := new(big.Float).Quo(v, maxHash)

	committeeSize := f.GetUint64("committeeSize")
	networkSize := float64(f.GetUint64("network"))
	if networkSize == 0 {
		networkSize = 1
	}
	if q.Cmp(big.NewFloat(1-float64(committeeSize)/networkSize)) < 0 {
		return errors.New("invalid proof")
	}

	var newSecretVotesCount *uint64
	if f.voteHashes.Get(f.ctx.Sender().Bytes()) == nil {
		v := f.getSecretVotesCount() + 1
		f.setSecretVotesCount(v)
		newSecretVotesCount = &v
	}
	f.voteHashes.Set(f.ctx.Sender().Bytes(), voteHash)

	var discriminated bool
	if enabledDiscrimination := f.GetByte("dis") == 1; enabledDiscrimination {
		if notDiscriminatedProof := f.GetByte("notDisP"); notDiscriminatedProof == 0 {
			discriminated = f.env.IsDiscriminated(f.ctx.Sender())
			if !discriminated {
				f.SetByte("notDisP", 1)
			}
		}
	}

	collector.AddOracleVotingCallVoteProof(f.statsCollector, voteHash, newSecretVotesCount, discriminated)

	return nil
}

func (f *OracleVoting6) sendVote(args ...[]byte) error {

	//vote = [0..255]
	vote, err := helpers.ExtractByte(0, args...)
	if err != nil {
		return err
	}

	salt, err := helpers.ExtractArray(1, args...)
	if err != nil {
		return err
	}
	if f.GetByte("state") != oracleVotingStateStarted {
		return errors.New("contract is not in running state")
	}

	votingDuration := f.GetUint64("votingDuration")
	publicVotingDuration := f.GetUint64("publicVotingDuration")

	duration := f.env.BlockNumber() - f.GetUint64("startBlock")
	if duration < votingDuration {
		return NewContractError("too early to accept open vote", true)
	}
	if duration > votingDuration+publicVotingDuration {
		return NewContractError("too late to accept open vote", true)
	}

	secretVotes := f.getSecretVotesCount()
	votedCount := f.GetUint64("votedCount")

	committeeSize := f.GetUint64("committeeSize")
	quorum := f.GetByte("quorum")
	noQuorum := float64(secretVotes+votedCount) < f.CalcPercent(committeeSize, quorum)
	if noQuorum {
		return NewContractError("quorum is not reachable", true)
	}
	enabledDiscrimination := f.GetByte("dis") == 1
	if enabledDiscrimination {
		if notDiscriminatedProof := f.GetByte("notDisP") == 1; !notDiscriminatedProof {
			return NewContractError("all vote proofs are discriminated", true)
		}
	}

	storedHash := f.voteHashes.Get(f.ctx.Sender().Bytes())

	computedHash := crypto.Hash(append(common.ToBytes(vote), salt...))

	if bytes.Compare(storedHash, computedHash[:]) != 0 {
		return errors.New("wrong vote hash")
	}
	f.votes.Set(f.ctx.Sender().Bytes(), common.ToBytes(vote))
	f.voteHashes.Remove(f.ctx.Sender().Bytes())
	secretVotesCount := f.getSecretVotesCount()
	var newSecretVotesCount *uint64
	if secretVotesCount > 0 {
		v := secretVotesCount - 1
		newSecretVotesCount = &v
		f.setSecretVotesCount(v)
	}

	var discriminated bool
	if enabledDiscrimination {
		discriminated = f.env.IsDiscriminated(f.ctx.Sender())
		if !discriminated {
			if notDiscriminatedVote := f.GetByte("notDisV"); notDiscriminatedVote == 0 {
				f.SetByte("notDisV", 1)
			}
		}
	}

	cnt, _ := helpers.ExtractUInt64(0, f.allVotes.Get(common.ToBytes(vote)))
	newOptionAllVotes := cnt + 1
	f.allVotes.Set(common.ToBytes(vote), common.ToBytes(newOptionAllVotes))

	c := f.GetUint64("votedCount") + 1
	f.SetUint64("votedCount", c)

	delegatee := f.env.Delegatee(f.ctx.Sender())

	var changeVoteOptions = func(vote byte, diff int64) *uint64 {
		if discriminated {
			return nil
		}
		cnt, _ := helpers.ExtractUInt64(0, f.voteOptions.Get(common.ToBytes(vote)))
		if diff >= 0 || cnt > 0 {
			cnt = uint64(int64(cnt) + diff)
		}
		f.voteOptions.Set(common.ToBytes(vote), common.ToBytes(cnt))
		return &cnt
	}

	if delegatee == nil {
		newOptionVotes := changeVoteOptions(vote, 1)
		collector.AddOracleVotingCallVote(f.statsCollector, vote, salt, newOptionVotes, newOptionAllVotes, newSecretVotesCount, nil, nil, nil, discriminated)
		return nil
	}

	prevPoolVote := f.poolVotes.Get(delegatee.Bytes())
	if !discriminated {
		f.poolVotes.Set(delegatee.Bytes(), common.ToBytes(vote))
	}

	if prevPoolVote == nil {
		newOptionVotes := changeVoteOptions(vote, 1)
		collector.AddOracleVotingCallVote(f.statsCollector, vote, salt, newOptionVotes, newOptionAllVotes, newSecretVotesCount, delegatee, nil, nil, discriminated)
		return nil
	}
	var newOptionVotes, newPrevOptionVotes *uint64
	if bytes.Compare(prevPoolVote, common.ToBytes(vote)) != 0 {
		prevVote, _ := helpers.ExtractByte(0, prevPoolVote)
		newPrevOptionVotes = changeVoteOptions(prevVote, -1)
		newOptionVotes = changeVoteOptions(vote, 1)
	}
	collector.AddOracleVotingCallVote(f.statsCollector, vote, salt, newOptionVotes, newOptionAllVotes, newSecretVotesCount, delegatee, prevPoolVote, newPrevOptionVotes, discriminated)
	return nil
}

func (f *OracleVoting6) finishVoting(args ...[]byte) error {
	if f.GetByte("state") != oracleVotingStateStarted {
		return errors.New("contract is not in running state")
	}
	enabledDiscrimination := f.GetByte("dis") == 1
	if enabledDiscrimination {
		if notDiscriminatedVote := f.GetByte("notDisV") == 1; !notDiscriminatedVote {
			return errors.New("all votes are discriminated")
		}
	}
	duration := f.env.BlockNumber() - f.GetUint64("startBlock")

	winnerVotesCnt := uint64(0)
	winner := byte(0)

	actualVotedCount := uint64(0)

	f.voteOptions.Iterate(func(key []byte, value []byte) bool {
		cnt, _ := helpers.ExtractUInt64(0, value)
		if cnt > winnerVotesCnt {
			winnerVotesCnt = cnt
			winner = key[0]
		}
		actualVotedCount += cnt
		return false
	})
	originalWinners, _ := helpers.ExtractUInt64(0, f.allVotes.Get(common.ToBytes(winner)))
	if winnerVotesCnt > originalWinners {
		originalWinners = winnerVotesCnt
	}

	committeeSize := f.GetUint64("committeeSize")
	winnerThreshold := f.GetByte("winnerThreshold")

	votingDuration := f.GetUint64("votingDuration")
	publicVotingDuration := f.GetUint64("publicVotingDuration")

	votedCount := f.GetUint64("votedCount")
	secretVotes := f.getSecretVotesCount()

	quorum := f.GetByte("quorum")

	hasWinner := float64(winnerVotesCnt) >= f.CalcPercent(committeeSize, winnerThreshold)
	hasQuorum := float64(votedCount+secretVotes) >= f.CalcPercent(committeeSize, quorum)

	if hasWinner || hasQuorum && (duration >= votingDuration+publicVotingDuration || secretVotes == 0) {
		f.SetByte("state", oracleVotingStateFinished)
		var result *byte
		fundInt := f.env.Balance(f.ctx.ContractAddr())
		fund := decimal.NewFromBigInt(fundInt, 0)
		winnersCnt := uint64(0)

		if float64(winnerVotesCnt) >= f.CalcPercent(actualVotedCount, winnerThreshold) {
			result = &winner
			winnersCnt = originalWinners
		} else {
			result = nil
			winnersCnt = votedCount
		}

		ownerReward := f.calculateOwnerReward(fundInt, fund, votedCount, secretVotes)
		oracleReward := math.ToInt(fund.Sub(decimal.NewFromBigInt(ownerReward, 0)).Div(decimal.NewFromInt(int64(winnersCnt))))

		poolRewards := map[common.Address]*big.Int{}

		var err error
		f.votes.Iterate(func(key []byte, value []byte) bool {
			vote, _ := helpers.ExtractByte(0, value)
			if result == nil || vote == *result {
				dest := common.Address{}
				dest.SetBytes(key)

				if d := f.env.Delegatee(dest); d != nil {
					if sum, ok := poolRewards[*d]; ok {
						poolRewards[*d] = sum.Add(sum, oracleReward)
					} else {
						poolRewards[*d] = big.NewInt(0).Set(oracleReward)
					}
				} else {
					err = f.env.Send(f.ctx, dest, oracleReward)
					f.env.Event("reward", dest.Bytes(), oracleReward.Bytes())
				}
			}
			return err != nil
		})
		if err != nil {
			return err
		}

		pools := make([]common.Address, 0, len(poolRewards))
		for pool := range poolRewards {
			pools = append(pools, pool)
		}
		sort.SliceStable(pools, func(i, j int) bool {
			return bytes.Compare(pools[i].Bytes(), pools[j].Bytes()) > 0
		})
		for _, pool := range pools {
			reward := poolRewards[pool]
			err = f.env.Send(f.ctx, pool, reward)
			if err != nil {
				return err
			}
			f.env.Event("reward", pool.Bytes(), reward.Bytes())
		}

		if ownerReward.Sign() > 0 {
			ownerRewardRecipient := f.getRefundRecipient()
			if err := f.env.Send(f.ctx, ownerRewardRecipient, ownerReward); err != nil {
				return err
			}
		}

		f.env.BurnAll(f.ctx)
		if result != nil {
			f.SetByte(keyResult, *result)
		}

		collector.AddOracleVotingCallFinish(f.statsCollector, oracleVotingStateFinished, result, fundInt, oracleReward, ownerReward)
		return nil
	}
	return errors.New("not enough votes to finish voting")
}

func (f *OracleVoting6) calculateOwnerReward(fundI *big.Int, fundD decimal.Decimal, votedCount, secretVotes uint64) *big.Int {
	ownerDeposit := f.GetBigInt("ownerDeposit")
	ownerFee := f.GetByte("ownerFee")
	if ownerDeposit != nil {
		result := new(big.Int).Set(ownerDeposit)
		if ownerFee > 0 {
			replenishedAmount := new(big.Int).Sub(fundI, ownerDeposit)
			if oracleRewardFund := f.GetBigInt("oracleRewardFund"); oracleRewardFund != nil {
				replenishedAmount = new(big.Int).Sub(replenishedAmount, oracleRewardFund)
			}
			if replenishedAmount.Sign() > 0 {
				feeAmount := decimal.NewFromBigInt(replenishedAmount, 0).Mul(decimal.NewFromFloat(float64(ownerFee) / 100.0))
				result = new(big.Int).Add(result, math.ToInt(feeAmount))
			}
		}
		return result
	}
	if ownerFee > 0 {
		payment := f.GetBigInt("votingMinPayment")
		userLocks := decimal.NewFromBigInt(big.NewInt(0).Mul(payment, big.NewInt(int64(votedCount+secretVotes))), 0)
		ownerRewardD := fundD.Sub(userLocks).Mul(decimal.NewFromFloat(float64(ownerFee) / 100.0))
		return math.ToInt(ownerRewardD)
	}
	return common.Big0
}

func (f *OracleVoting6) getRefundRecipient() common.Address {
	if v := f.GetArray("refundRecipient"); len(v) > 0 {
		return common.BytesToAddress(v)
	}
	return f.Owner()
}

func (f *OracleVoting6) prolongVoting(args ...[]byte) error {
	if f.GetByte("state") != oracleVotingStateStarted {
		return errors.New("contract is not in running state")
	}

	winnerVotesCnt := uint64(0)

	f.voteOptions.Iterate(func(key []byte, value []byte) bool {
		cnt, _ := helpers.ExtractUInt64(0, value)
		if cnt > winnerVotesCnt {
			winnerVotesCnt = cnt
		}
		return false
	})

	duration := f.env.BlockNumber() - f.GetUint64("startBlock")

	committeeSize := f.GetUint64("committeeSize")
	winnerThreshold := f.GetByte("winnerThreshold")

	votingDuration := f.GetUint64("votingDuration")
	publicVotingDuration := f.GetUint64("publicVotingDuration")

	votedCount := f.GetUint64("votedCount")
	quorum := f.GetByte("quorum")

	epochWithoutGrowth := f.GetByte("no-growth")

	if epochWithoutGrowth >= 3 {
		return errors.New("voting can not be prolonged anymore")
	}

	secretVotes := f.getSecretVotesCount()

	noWinnerVotes := float64(winnerVotesCnt) < f.CalcPercent(committeeSize, winnerThreshold)
	noQuorum := float64(votedCount+secretVotes) < f.CalcPercent(committeeSize, quorum)

	var newEpochWithoutGrowth *byte
	var newProlongVoteCount *uint64
	noWinnerAfterPublicVoting := duration >= votingDuration+publicVotingDuration && noWinnerVotes && noQuorum
	noConsensusAfterSecretVoting := duration >= votingDuration && float64(votedCount+secretVotes) < f.CalcPercent(committeeSize, quorum)
	enabledDiscrimination := f.GetByte("dis") == 1
	allVotesDiscriminatedAfterSecretVoting := duration >= votingDuration && enabledDiscrimination && f.GetByte("notDisP") == 0
	allVotesDiscriminatedAfterPublicVoting := duration >= votingDuration+publicVotingDuration && enabledDiscrimination && f.GetByte("notDisV") == 0
	if f.env.Epoch() != f.GetUint16("epoch") || noWinnerAfterPublicVoting || noConsensusAfterSecretVoting || allVotesDiscriminatedAfterSecretVoting || allVotesDiscriminatedAfterPublicVoting {
		vrfSeed := f.env.BlockSeed()
		f.SetArray("vrfSeed", vrfSeed)
		var startBlock *uint64
		if noWinnerAfterPublicVoting || noConsensusAfterSecretVoting || allVotesDiscriminatedAfterSecretVoting || allVotesDiscriminatedAfterPublicVoting {
			v := f.env.BlockNumber()
			startBlock = &v
			f.SetUint64("startBlock", v)
			prevVoteCount := f.GetUint64("prolongVoteCount")
			const noGrowthThreshold = 0.1
			if (votedCount+secretVotes) == 0 || (float64(votedCount+secretVotes-prevVoteCount)/float64(prevVoteCount)) < noGrowthThreshold {
				v := epochWithoutGrowth + 1
				f.SetByte("no-growth", v)
				newEpochWithoutGrowth = &v
			} else {
				v := byte(0)
				f.SetByte("no-growth", v)
				newEpochWithoutGrowth = &v
			}
			allVotes := votedCount + secretVotes
			f.SetUint64("prolongVoteCount", allVotes)
			newProlongVoteCount = &allVotes
		}
		epoch := f.env.Epoch()
		f.SetUint16("epoch", epoch)
		networkSize := uint64(f.env.NetworkSize())
		f.SetUint64("network", networkSize)
		collector.AddOracleVotingCallProlongation(f.statsCollector, startBlock, epoch, vrfSeed, committeeSize, networkSize, newEpochWithoutGrowth, newProlongVoteCount)
		return nil
	}
	return errors.New("voting can not be prolonged")
}

func (f *OracleVoting6) addStake(args ...[]byte) error {
	var err error
	if f.ctx.PayAmount() != nil && f.ctx.PayAmount().Sign() > 0 {
		err = f.env.MoveToStake(f.ctx, f.ctx.PayAmount())
	}
	if err == nil {
		collector.AddOracleVotingCallAddStake(f.statsCollector)
	}
	return err
}

func (f *OracleVoting6) Read(method string, args ...[]byte) ([]byte, error) {

	switch method {
	case "proof":

		addr, err := helpers.ExtractAddr(0, args...)
		if err != nil {
			return nil, err
		}

		if f.GetByte("state") != oracleVotingStateStarted {
			return nil, errors.New("contract is not in running state")
		}

		duration := f.GetUint64("votingDuration")

		if f.env.BlockNumber()-f.GetUint64("startBlock") >= duration {
			return nil, errors.New("too late to accept secret vote")
		}

		seed := f.GetArray("vrfSeed")

		pubKey := f.env.PubKey(addr)

		h := crypto.Hash(append(pubKey, seed...))

		v := new(big.Float).SetInt(new(big.Int).SetBytes(h[:]))

		q := new(big.Float).Quo(v, maxHash)

		committeeSize := f.GetUint64("committeeSize")
		networkSize := float64(f.env.NetworkSize())
		if networkSize == 0 {
			networkSize = 1
		}
		if q.Cmp(big.NewFloat(1-float64(committeeSize)/networkSize)) < 0 {
			return nil, errors.New("invalid proof")
		}
		return []byte{1}, nil
	case "voteHash":
		vote, err := helpers.ExtractByte(0, args...)
		if err != nil {
			return nil, err
		}
		salt, err := helpers.ExtractArray(1, args...)
		if err != nil {
			return nil, err
		}
		hash := crypto.Hash(append(common.ToBytes(vote), salt...))
		return hash[:], nil
	case "voteBlock":
		block := f.GetUint64("startBlock") + f.GetUint64("votingDuration")
		return common.ToBytes(block), nil
	default:
		return nil, errors.New("unknown method")
	}
}

func (f *OracleVoting6) getSecretVotesCount() uint64 {
	data := f.env.GetValue(f.ctx, []byte("secretVotesCount"))
	if data != nil {
		ret, _ := helpers.ExtractUInt64(0, data)
		return ret
	}
	var secretVotes uint64
	f.voteHashes.Iterate(func(key []byte, value []byte) bool {
		secretVotes++
		return false
	})
	f.setSecretVotesCount(secretVotes)
	return secretVotes
}

func (f *OracleVoting6) setSecretVotesCount(newValue uint64) {
	f.SetUint64("secretVotesCount", newValue)
}

func (f *OracleVoting6) Terminate(args ...[]byte) (common.Address, [][]byte, error) {
	if f.GetByte("state") == oracleVotingStatePending {

		period := time.Duration(uint64(f.env.BlockTimeStamp())-f.GetUint64("startTime")) * time.Second
		if period > time.Hour*24*30 {
			balance := f.env.Balance(f.ctx.ContractAddr())
			if balance.Sign() > 0 {
				if err := f.env.Send(f.ctx, f.ctx.Sender(), balance); err != nil {
					return common.Address{}, nil, err
				}
			}
			collector.AddOracleVotingTermination(f.statsCollector, nil, nil, nil)
			return f.Owner(), nil, nil
		}

		return common.Address{}, nil, errors.New("contract is not in running state")
	}
	duration := f.env.BlockNumber() - f.GetUint64("startBlock")

	votingDuration := f.GetUint64("votingDuration")
	publicVotingDuration := f.GetUint64("publicVotingDuration")

	stake := decimal.NewFromBigInt(f.env.ContractStake(f.ctx.ContractAddr()), -18)
	d, _ := stake.Mul(decimal.NewFromInt(int64(f.env.NetworkSize()))).Div(decimal.NewFromInt(100)).Float64()
	terminationDays := uint64(math2.Round(math2.Pow(d, 1.0/3)))
	const blocksInDay = 4320
	if duration >= votingDuration+publicVotingDuration+terminationDays*blocksInDay {
		balance := f.env.Balance(f.ctx.ContractAddr())
		var fundInt, ownerReward, oracleReward *big.Int
		if balance.Sign() > 0 {
			fundInt = new(big.Int).Set(balance)
			fund := decimal.NewFromBigInt(fundInt, 0)
			votedCount := f.GetUint64("votedCount")
			secretVotes := f.getSecretVotesCount()
			ownerReward = f.calculateOwnerReward(fundInt, fund, votedCount, secretVotes)
			if ownerReward.Sign() > 0 {
				ownerRewardRecipient := f.getRefundRecipient()
				if err := f.env.Send(f.ctx, ownerRewardRecipient, ownerReward); err != nil {
					return common.Address{}, nil, err
				}
			}

			if votedCount+secretVotes == 0 {
				if err := f.env.Send(f.ctx, f.ctx.Sender(), balance.Sub(balance, ownerReward)); err != nil {
					return common.Address{}, nil, err
				}
			} else {
				oracleReward = math.ToInt(fund.Sub(decimal.NewFromBigInt(ownerReward, 0)).Div(decimal.NewFromInt(int64(votedCount + secretVotes))))
				var err error
				f.votes.Iterate(func(key []byte, value []byte) bool {
					dest := common.Address{}
					dest.SetBytes(key)
					err = f.env.Send(f.ctx, dest, oracleReward)
					f.env.Event("reward", dest.Bytes(), oracleReward.Bytes())
					return err != nil
				})
				f.voteHashes.Iterate(func(key []byte, value []byte) bool {
					dest := common.Address{}
					dest.SetBytes(key)
					err = f.env.Send(f.ctx, dest, oracleReward)
					f.env.Event("reward", dest.Bytes(), oracleReward.Bytes())
					return err != nil
				})
				if err != nil {
					return common.Address{}, nil, err
				}
				f.env.BurnAll(f.ctx)
			}
		}
		collector.AddOracleVotingTermination(f.statsCollector, fundInt, oracleReward, ownerReward)
		var keysToSave [][]byte
		if f.env.GetValue(f.ctx, []byte(keyResult)) != nil {
			keysToSave = [][]byte{[]byte(keyFact), []byte(keyResult)}
			if f.env.GetValue(f.ctx, []byte(keyHash)) != nil {
				keysToSave = append(keysToSave, []byte(keyHash))
			}
		}
		return f.Owner(), keysToSave, nil
	}
	return common.Address{}, nil, errors.New("voting can not be terminated")
}

type ContractError struct {
	error    string
	tryLater bool
}

func NewContractError(text string, tryLater bool) *ContractError {
	return &ContractError{
		error:    text,
		tryLater: tryLater,
	}
}

func (e *ContractError) Error() string {
	return e.error
}

func (e *ContractError) TryLater() bool {
	return e.tryLater
}
