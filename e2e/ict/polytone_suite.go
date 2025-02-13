// source: https://github.com/DA0-DA0/polytone/blob/main/tests/strangelove/suite.go
package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"testing"

	sdkmath "cosmossdk.io/math"
	w "github.com/CosmWasm/wasmvm/types"
	"github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	interchaintestrelayer "github.com/strangelove-ventures/interchaintest/v7/relayer"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type Suite struct {
	t        *testing.T
	reporter *testreporter.RelayerExecReporter
	ctx      context.Context

	ChainA SuiteChain
	ChainB SuiteChain

	Relayer ibc.Relayer
	PathAB  string
}

type SuiteChain struct {
	Ibc    ibc.Chain
	Cosmos *cosmos.CosmosChain
	User   ibc.Wallet

	Note     string
	Listener string
	Voice    string
	Tester   string
}

func NewPolytoneSuite(t *testing.T) Suite {
	if testing.Short() {
		t.Skip()
	}

	var (
		ctx                  = context.Background()
		chainID_A, chainID_B = "chain-a", "chain-b"
		chainA, chainB       *cosmos.CosmosChain
	)

	// Set specific chain ids for each so they are their own unique networks
	baseCfg.ChainID = chainID_A
	configA := baseCfg

	baseCfg.ChainID = chainID_B
	configB := baseCfg

	// Create chain factory with multiple Bitsong individual networks.
	numVals := 1
	numFullNodes := 0

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:          "bitsong",
			ChainName:     "bitsong1",
			ChainConfig:   configA,
			NumValidators: &numVals,
			NumFullNodes:  &numFullNodes,
		},
		{
			Name:          "bitsong",
			ChainName:     "bitsong2",
			ChainConfig:   configB,
			NumValidators: &numVals,
			NumFullNodes:  &numFullNodes,
		},
	})

	// Get chains from the chain factory
	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	chainA, chainB = chains[0].(*cosmos.CosmosChain), chains[1].(*cosmos.CosmosChain)

	dockerClient, dockerNetwork := interchaintest.DockerSetup(t)
	r := interchaintest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		interchaintestrelayer.CustomDockerImage(IBCRelayerImage, IBCRelayerVersion, "100:1000"),
		interchaintestrelayer.StartupFlags("--processor", "events", "--block-history", "100"),
	).Build(t, dockerClient, dockerNetwork)

	const pathAB = "ab"

	ic := interchaintest.NewInterchain().
		AddChain(chainA).
		AddChain(chainB).
		AddRelayer(r, "relayer").
		AddLink(interchaintest.InterchainLink{
			Chain1:  chainA,
			Chain2:  chainB,
			Relayer: r,
			Path:    pathAB,
		})

	reporter := testreporter.NewNopReporter().RelayerExecReporter(t)
	require.NoError(t, ic.Build(ctx, reporter, interchaintest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            dockerClient,
		NetworkID:         dockerNetwork,
		BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),

		SkipPathCreation: false,
	}))

	t.Cleanup(func() {
		_ = ic.Close()
	})
	err = r.StartRelayer(ctx, reporter, pathAB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		err := r.StopRelayer(ctx, reporter)
		if err != nil {
			t.Logf("couldn't stop relayer: %s", err)
		}
	})

	// Get original account balances
	userFunds := sdkmath.NewInt(10_000_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", userFunds, chainA, chainB)

	userA, userB := users[0], users[1]
	t.Logf("userA: %s", userA)

	suite := Suite{
		t:        t,
		reporter: reporter,
		ctx:      ctx,
		ChainA: SuiteChain{
			Ibc:    chainA,
			Cosmos: chainA,
			User:   userA,
		},
		ChainB: SuiteChain{
			Ibc:    chainB,
			Cosmos: chainB,
			User:   userB,
		},
		Relayer: r,
		PathAB:  pathAB,
	}

	suite.SetupChain(&suite.ChainA)
	suite.SetupChain(&suite.ChainB)
	return suite
}

func (s *Suite) SetupChain(chain *SuiteChain) {
	user := chain.User
	cc := chain.Cosmos
	noteId, err := cc.StoreContract(s.ctx, user.KeyName(), "contracts/polytone_note.wasm")
	if err != nil {
		s.t.Fatal(err)
	}
	voiceId, err := cc.StoreContract(s.ctx, user.KeyName(), "contracts/polytone_voice.wasm")
	if err != nil {
		s.t.Fatal(err)
	}
	proxyId, err := cc.StoreContract(s.ctx, user.KeyName(), "contracts/polytone_proxy.wasm")
	if err != nil {
		s.t.Fatal(err)
	}

	listenerId, err := cc.StoreContract(s.ctx, user.KeyName(), "contracts/polytone_listener.wasm")
	if err != nil {
		s.t.Fatal(err)
	}

	testerId, err := cc.StoreContract(s.ctx, user.KeyName(), "contracts/polytone_tester.wasm")
	if err != nil {
		s.t.Fatal(err)
	}

	proxyUint, err := strconv.Atoi(proxyId)
	if err != nil {
		s.t.Fatal(err)
	}

	chain.Note, err = s.Instantiate(cc, user, noteId, NoteInstantiate{
		BlockMaxGas: 100_000_000,
	})
	require.NoError(s.t, err)

	chain.Voice, err = s.Instantiate(cc, user, voiceId, VoiceInstantiate{
		ProxyCodeId:     uint64(proxyUint),
		BlockMaxGas:     100_000_000,
		ContractAddrLen: 32,
	})
	require.NoError(s.t, err)

	chain.Listener, err = s.Instantiate(cc, user, listenerId, ListenerInstantiate{
		Note: chain.Note,
	})
	require.NoError(s.t, err)

	chain.Tester, err = s.Instantiate(cc, user, testerId, TesterInstantiate{})
	require.NoError(s.t, err)
}

func (s *Suite) Instantiate(chain *cosmos.CosmosChain, user ibc.Wallet, codeId string, msg any) (string, error) {
	str, err := json.Marshal(msg)
	if err != nil {
		return "", err
	}
	address, err := chain.InstantiateContract(s.ctx, user.KeyName(), codeId, string(str), true)
	if err != nil {
		return "", err
	}
	return address, nil
}

func (s *Suite) CreateChannel(initModule string, tryModule string, initChain, tryChain *SuiteChain, path string) (initChannel, tryChannel string, err error) {
	initStartChannels := s.QueryOpenChannels(initChain)
	err = s.Relayer.CreateChannel(s.ctx, s.reporter, path, ibc.CreateChannelOptions{
		SourcePortName: "wasm." + initModule,
		DestPortName:   "wasm." + tryModule,
		Order:          ibc.Unordered,
		Version:        "polytone-1",
	})
	if err != nil {
		return
	}
	err = testutil.WaitForBlocks(s.ctx, 10, initChain.Ibc, tryChain.Ibc)
	if err != nil {
		return
	}

	err = s.Relayer.StopRelayer(s.ctx, s.reporter)
	if err != nil {
		return
	}

	err = s.Relayer.StartRelayer(s.ctx, s.reporter)
	if err != nil {
		return
	}

	initChannels := s.QueryOpenChannels(initChain)

	if len(initChannels) == len(initStartChannels) {
		err = errors.New("no new channels created")
		return
	}

	initChannel = initChannels[len(initChannels)-1].ChannelID
	tryChannel = initChannels[len(initChannels)-1].Counterparty.ChannelID
	return
}

const CHANNEL_STATE_OPEN = "STATE_OPEN"
const CHANNEL_STATE_TRY = "STATE_TRYOPEN"
const CHANNEL_STATE_INIT = "STATE_INIT"

func (s *Suite) QueryOpenChannels(chain *SuiteChain) []ibc.ChannelOutput {
	eq := s.QueryChannelsInState(chain, CHANNEL_STATE_OPEN)
	fmt.Println("QueryChannelsInState", eq)
	return eq
}

func (s *Suite) QueryChannelsInState(chain *SuiteChain, state string) []ibc.ChannelOutput {
	channels, err := s.Relayer.GetChannels(s.ctx, s.reporter, chain.Ibc.Config().ChainID)
	if err != nil {
		s.t.Fatal(err)
	}
	openChannels := []ibc.ChannelOutput{}
	for index := range channels {
		if channels[index].State == state {
			openChannels = append(openChannels, channels[index])
		}
	}
	return openChannels
}

func (s *Suite) RoundtripMessage(note string, chain *SuiteChain, msg NoteExecute) (Callback, error) {
	callbacksStart := s.QueryTesterCallbackHistory(&s.ChainA).History

	marshalled, err := json.Marshal(msg)
	require.NoError(s.t, err)

	_, err = chain.Cosmos.ExecuteContract(s.ctx, chain.User.KeyName(), note, string(marshalled))
	require.NoError(s.t, err)

	// wait for packet to relay.
	err = testutil.WaitForBlocks(s.ctx, 10, s.ChainA.Ibc, s.ChainB.Ibc)
	require.NoError(s.t, err)

	callbacksEnd := s.QueryTesterCallbackHistory(&s.ChainA).History
	if len(callbacksEnd) == len(callbacksStart) {
		return Callback{}, errors.New("no new callback")
	}
	callback := callbacksEnd[len(callbacksEnd)-1]
	require.Equal(
		s.t,
		chain.User.Address(),
		callback.Initiator,
	)
	require.Equal(s.t, "aGVsbG8K", callback.InitiatorMsg)
	return callback.Result, nil
}

func (s *Suite) RoundtripExecute(note string, chain *SuiteChain, msgs []w.CosmosMsg) (Callback, error) {
	msg := NoteExecuteMsg{
		Msgs:           msgs,
		TimeoutSeconds: 100,
		Callback: &CallbackRequest{
			Receiver: chain.Tester,
			Msg:      "aGVsbG8K",
		},
	}
	return s.RoundtripMessage(note, chain, NoteExecute{
		Execute: &msg,
	})
}

func (s *Suite) RoundtripQuery(note string, chain *SuiteChain, msgs []w.CosmosMsg) (Callback, error) {
	msg := NoteQuery{
		Msgs:           msgs,
		TimeoutSeconds: 100,
		Callback: CallbackRequest{
			Receiver: chain.Tester,
			Msg:      "aGVsbG8K",
		},
	}
	return s.RoundtripMessage(note, chain, NoteExecute{
		Query: &msg,
	})
}

func (s *Suite) QueryTesterCallbackHistory(chain *SuiteChain) HistoryResponse {
	var response DataWrappedHistoryResponse
	query := TesterQuery{
		History: &Empty{},
	}
	err := chain.Cosmos.QueryContract(s.ctx, chain.Tester, query, &response)
	if err != nil {
		s.t.Fatal(err)
	}
	return response.Data
}
