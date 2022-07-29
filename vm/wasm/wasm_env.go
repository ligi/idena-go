package wasm

import (
	"github.com/idena-network/idena-go/blockchain/types"
	"github.com/idena-network/idena-go/common"
	"github.com/idena-network/idena-go/core/appstate"
	"github.com/idena-network/idena-go/crypto"
	"github.com/idena-network/idena-go/vm/costs"
	"github.com/idena-network/idena-wasm-binding/lib"
	"github.com/pkg/errors"
	"math/big"
)

type contractValue struct {
	value   []byte
	removed bool
}

type ContractData struct {
	Code  []byte
	Stake *big.Int
}

type WasmEnv struct {
	id       int
	appState *appstate.AppState
	ctx      *ContractContext
	head     *types.Header
	parent   *WasmEnv

	contractStoreCache    map[common.Address]map[string]*contractValue
	balancesCache         map[common.Address]*big.Int
	deployedContractCache map[common.Address]ContractData
	contractStakeCache    map[common.Address]*big.Int
}

func (w *WasmEnv) CodeHash(meter *lib.GasMeter) []byte {
	meter.ConsumeGas(costs.GasToWasmGas(costs.ComputeHashGas))
	hash := crypto.Hash(w.OwnCode(meter))
	return hash[:]
}

func (w *WasmEnv) ContractAddrByHash(meter *lib.GasMeter, hash []byte, args []byte, nonce []byte) lib.Address {
	meter.ConsumeGas(costs.GasToWasmGas(costs.ComputeHashGas))
	return ComputeContractAddrByHash(hash, args, nonce)
}

func (w *WasmEnv) OwnCode(meter *lib.GasMeter) []byte {
	addr := w.ContractAddress(meter)
	return w.ContractCode(meter, addr)
}

func (w *WasmEnv) ContractAddr(meter *lib.GasMeter, code []byte, args []byte, nonce []byte) lib.Address {
	meter.ConsumeGas(costs.GasToWasmGas(costs.ComputeHashGas * 2))
	return ComputeContractAddr(code, args, nonce)
}

func (w *WasmEnv) MinFeePerGas(meter *lib.GasMeter) *big.Int {
	meter.ConsumeGas(costs.GasToWasmGas(costs.ReadGlobalStateGas))
	return w.appState.State.FeePerGas()
}

func (w *WasmEnv) BlockSeed(meter *lib.GasMeter) []byte {
	meter.ConsumeGas(costs.GasToWasmGas(costs.ReadBlockGas))
	return w.head.Seed().Bytes()
}

func (w *WasmEnv) SubBalance(meter *lib.GasMeter, amount *big.Int) error {
	meter.ConsumeGas(costs.GasToWasmGas(costs.MoveBalanceGas))
	balance := w.getBalance(w.ctx.ContractAddr())
	if balance.Cmp(amount) < 0 {
		return errors.New("insufficient funds")
	}
	if amount.Sign() < 0 {
		return errors.New("value must be non-negative")
	}
	w.subBalance(w.ctx.ContractAddr(), amount)
	return nil
}

func (w *WasmEnv) AddBalance(meter *lib.GasMeter, address lib.Address, amount *big.Int) {
	meter.ConsumeGas(costs.GasToWasmGas(costs.MoveBalanceGas))
	w.addBalance(address, amount)
}

func (w *WasmEnv) ContractAddress(meter *lib.GasMeter) lib.Address {
	meter.ConsumeGas(costs.GasToWasmGas(costs.ReadBlockGas))
	return w.ctx.ContractAddr()
}

func (w *WasmEnv) ContractCode(meter *lib.GasMeter, addr lib.Address) []byte {
	meter.ConsumeGas(costs.GasToWasmGas(costs.ReadStateGas))
	return w.GetCode(addr)
}

func NewWasmEnv(appState *appstate.AppState, ctx *ContractContext, head *types.Header) *WasmEnv {
	return &WasmEnv{
		id:                    1,
		appState:              appState,
		ctx:                   ctx,
		head:                  head,
		contractStoreCache:    map[common.Address]map[string]*contractValue{},
		balancesCache:         map[common.Address]*big.Int{},
		deployedContractCache: map[common.Address]ContractData{},
		contractStakeCache:    map[common.Address]*big.Int{},
	}
}

func (w *WasmEnv) readContractData(contractAddr common.Address, key []byte) []byte {
	if cache, ok := w.contractStoreCache[contractAddr]; ok {
		if value, ok := cache[string(key)]; ok {
			if value.removed {
				return nil
			}
			return value.value
		}
	}

	if w.parent != nil {
		return w.parent.readContractData(contractAddr, key)
	}

	value := w.appState.State.GetContractValue(contractAddr, key)

	return value
}

func (w *WasmEnv) SetStorage(meter *lib.GasMeter, key []byte, value []byte) {
	ctx := w.ctx
	if len(key) > common.MaxContractStoreKeyLength {
		panic("key is too big")
	}
	addr := ctx.ContractAddr()
	var cache map[string]*contractValue
	var ok bool
	if cache, ok = w.contractStoreCache[addr]; !ok {
		cache = make(map[string]*contractValue)
		w.contractStoreCache[addr] = cache
	}
	cache[string(key)] = &contractValue{
		value:   value,
		removed: false,
	}
	meter.ConsumeGas(costs.GasToWasmGas(uint64(costs.WriteStatePerByteGas * (len(key) + len(value)))))
}

func (w *WasmEnv) GetStorage(meter *lib.GasMeter, key []byte) []byte {
	value := w.readContractData(w.ctx.ContractAddr(), key)
	meter.ConsumeGas(costs.GasToWasmGas(uint64(costs.WriteStatePerByteGas * len(value))))
	return value
}

func (w *WasmEnv) RemoveStorage(meter *lib.GasMeter, key []byte) {
	addr := w.ctx.ContractAddr()
	var cache map[string]*contractValue
	var ok bool
	if cache, ok = w.contractStoreCache[addr]; !ok {
		cache = map[string]*contractValue{}
		w.contractStoreCache[addr] = cache
	}
	cache[string(key)] = &contractValue{removed: true}
	meter.ConsumeGas(costs.GasToWasmGas(costs.RemoveStateGas))
}

func (w *WasmEnv) BlockNumber(meter *lib.GasMeter) uint64 {
	meter.ConsumeGas(costs.GasToWasmGas(costs.ReadBlockGas))
	return w.head.Height()
}

func (w *WasmEnv) BlockTimestamp(meter *lib.GasMeter) int64 {
	meter.ConsumeGas(costs.GasToWasmGas(costs.ReadBlockGas))
	return w.head.Time()
}

func (w *WasmEnv) Send(meter *lib.GasMeter, dest lib.Address, amount *big.Int) error {
	panic("deprecated")
	balance := w.getBalance(w.ctx.ContractAddr())
	if balance.Cmp(amount) < 0 {
		return errors.New("insufficient funds")
	}
	if amount.Sign() < 0 {
		return errors.New("value must be non-negative")
	}
	w.subBalance(w.ctx.ContractAddr(), amount)
	w.addBalance(dest, amount)

	meter.ConsumeGas(30)
	return nil
}

func (w *WasmEnv) Balance(meter *lib.GasMeter, address lib.Address) *big.Int {
	meter.ConsumeGas(costs.GasToWasmGas(costs.ReadBlockGas))
	return w.getBalance(address)
}

func (w *WasmEnv) NetworkSize(meter *lib.GasMeter) uint64 {
	meter.ConsumeGas(costs.GasToWasmGas(costs.ReadBlockGas))
	return uint64(w.appState.ValidatorsCache.NetworkSize())
}

func (w *WasmEnv) IdentityState(meter *lib.GasMeter, address lib.Address) byte {
	meter.ConsumeGas(costs.GasToWasmGas(costs.ReadIdentityStateGas))
	return byte(w.appState.State.GetIdentityState(address))
}

func (w *WasmEnv) Identity(meter *lib.GasMeter, address lib.Address) []byte {
	meter.ConsumeGas(costs.GasToWasmGas(costs.ReadStateGas))
	return w.appState.State.RawIdentity(address)
}

func (w *WasmEnv) CreateSubEnv(contract lib.Address, payAmount *big.Int, isDeploy bool) (lib.HostEnv, error) {

	if payAmount.Sign() < 0 {
		return nil, errors.New("value must be non-negative")
	}

	subContractBalance := w.getBalance(contract)
	subContractBalance = new(big.Int).Add(subContractBalance, payAmount)

	subEnv := &WasmEnv{
		id:                    w.id + 1,
		appState:              w.appState,
		parent:                w,
		ctx:                   w.ctx.CreateSubContext(contract, payAmount),
		contractStoreCache:    map[common.Address]map[string]*contractValue{},
		balancesCache:         map[common.Address]*big.Int{contract: subContractBalance},
		deployedContractCache: map[common.Address]ContractData{},
		contractStakeCache:    map[common.Address]*big.Int{},
		head:                  w.head,
	}
	return subEnv, nil
}

func (w *WasmEnv) GetCode(addr lib.Address) []byte {
	if w.parent != nil {
		return w.parent.GetCode(addr)
	}
	if data, ok := w.deployedContractCache[addr]; ok {
		return data.Code
	}
	return w.appState.State.GetContractCode(addr)
}

func (w *WasmEnv) getBalance(address common.Address) *big.Int {
	if b, ok := w.balancesCache[address]; ok {
		return b
	}
	if w.parent != nil {
		return w.parent.getBalance(address)
	}
	return w.appState.State.GetBalance(address)
}

func (w *WasmEnv) addBalance(address common.Address, amount *big.Int) {
	b := w.getBalance(address)
	w.setBalance(address, new(big.Int).Add(b, amount))
}

func (w *WasmEnv) subBalance(address common.Address, amount *big.Int) {
	b := w.getBalance(address)
	w.setBalance(address, new(big.Int).Sub(b, amount))
}

func (w *WasmEnv) setBalance(address common.Address, amount *big.Int) {
	w.balancesCache[address] = amount
}

func (w *WasmEnv) Deploy(code []byte) {
	contractAddr := w.ctx.ContractAddr()
	w.deployedContractCache[contractAddr] = ContractData{
		Stake: big.NewInt(0),
		Code:  code,
	}
}

func (w *WasmEnv) Caller(meter *lib.GasMeter) lib.Address {
	meter.ConsumeGas(costs.GasToWasmGas(costs.ReadBlockGas))
	return w.ctx.Caller()
}

func (w *WasmEnv) OriginalCaller(meter *lib.GasMeter) lib.Address {
	meter.ConsumeGas(costs.GasToWasmGas(costs.ReadBlockGas))
	return w.ctx.originCaller
}

func (w *WasmEnv) Commit() {
	println("Commit to wasm env id=", w.id)
	if w.parent != nil {
		for contract, cache := range w.contractStoreCache {
			w.parent.contractStoreCache[contract] = cache
		}
		for addr, b := range w.balancesCache {
			w.parent.balancesCache[addr] = b
		}
		for contract, data := range w.deployedContractCache {
			w.parent.deployedContractCache[contract] = data
		}
		for contract, stake := range w.contractStakeCache {
			w.parent.contractStakeCache[contract] = stake
		}
		return
	}

	for contract, cache := range w.contractStoreCache {
		for k, v := range cache {
			if v.removed {
				w.appState.State.RemoveContractValue(contract, []byte(k))
			} else {
				w.appState.State.SetContractValue(contract, []byte(k), v.value)
			}
		}
	}
	for addr, b := range w.balancesCache {
		w.appState.State.SetBalance(addr, b)
	}
	for contract, data := range w.deployedContractCache {
		w.appState.State.DeployWasmContract(contract, data.Code, data.Stake)
	}
	for contract, stake := range w.contractStakeCache {
		w.appState.State.SetContractStake(contract, stake)
	}
}

func (w *WasmEnv) Clear() {
	w.contractStoreCache = map[common.Address]map[string]*contractValue{}
	w.balancesCache = map[common.Address]*big.Int{}
	w.deployedContractCache = map[common.Address]ContractData{}
	w.contractStakeCache = map[common.Address]*big.Int{}
}
