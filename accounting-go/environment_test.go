package accounting_test

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/alexeyco/simpletable"
	eostest "github.com/digital-scarcity/eos-go-test"
	"github.com/eoscanada/eos-go"
	"github.com/eoscanada/eos-go/ecc"
	"github.com/eoscanada/eos-go/system"
	"github.com/hypha-dao/dao-go"
	"github.com/hypha-dao/document/docgraph"
	"gotest.tools/assert"
)

//const defaultKey = "5KQwrPbwdL6PhXujxW37FSSQZ1JiwsST4cqQzDeyXtP79zkvFD3"

var accountingHome, accountingWasm, accountingAbi, devHome, daoHome, daoWasm, daoAbi, tokenHome, tokenWasm, tokenAbi, tdHome, tdWasm, tdAbi string
var treasuryHome, treasuryWasm, treasuryAbi, monitorHome, monitorWasm, monitorAbi string
var seedsHome, escrowWasm, escrowAbi, exchangeWasm, exchangeAbi string

const testingEndpoint = "http://localhost:8888"

type Member struct {
	Member eos.AccountName
	Doc    docgraph.Document
}

type Environment struct {
	ctx context.Context
	api eos.API

	DAO           eos.AccountName
	Accounting    eos.AccountName
	HusdToken     eos.AccountName
	HyphaToken    eos.AccountName
	HvoiceToken   eos.AccountName
	SeedsToken    eos.AccountName
	Bank          eos.AccountName
	SeedsEscrow   eos.AccountName
	SeedsExchange eos.AccountName
	Events        eos.AccountName
	TelosDecide   eos.AccountName
	Whale         Member
	Root          docgraph.Document

	VotingDurationSeconds int64
	HyphaDeferralFactor   int64
	SeedsDeferralFactor   int64

	NumPeriods     int
	PeriodDuration time.Duration

	Members []Member
}

func envHeader() *simpletable.Header {
	return &simpletable.Header{
		Cells: []*simpletable.Cell{
			{Align: simpletable.AlignCenter, Text: "Variable"},
			{Align: simpletable.AlignCenter, Text: "Value"},
		},
	}
}

func (e *Environment) String() string {
	table := simpletable.New()
	table.Header = envHeader()

	kvs := make(map[string]string)
	kvs["DAO"] = string(e.DAO)
	kvs["HUSD Token"] = string(e.HusdToken)
	kvs["HVOICE Token"] = string(e.HvoiceToken)
	kvs["HYPHA Token"] = string(e.HyphaToken)
	kvs["SEEDS Token"] = string(e.SeedsToken)
	kvs["Bank"] = string(e.Bank)
	kvs["Escrow"] = string(e.SeedsEscrow)
	kvs["Exchange"] = string(e.SeedsExchange)
	kvs["Telos Decide"] = string(e.TelosDecide)
	kvs["Whale"] = string(e.Whale.Member)
	kvs["Voting Duration (s)"] = strconv.Itoa(int(e.VotingDurationSeconds))
	kvs["HYPHA deferral X"] = strconv.Itoa(int(e.HyphaDeferralFactor))
	kvs["SEEDS deferral X"] = strconv.Itoa(int(e.SeedsDeferralFactor))

	for key, value := range kvs {
		r := []*simpletable.Cell{
			{Align: simpletable.AlignLeft, Text: key},
			{Align: simpletable.AlignRight, Text: value},
		}
		table.Body.Cells = append(table.Body.Cells, r)
	}

	return table.String()
}

func SetupEnvironment(t *testing.T) *Environment {

	home, exists := os.LookupEnv("HOME")
	if exists {
		devHome = home
	} else {
		devHome = "."
	}
	devHome = devHome + ""
	devHome = "/src"

	accountingHome = devHome + "/accounting-contracts"
	accountingWasm = accountingHome + "/build/accounting/accounting.wasm"
	accountingAbi = accountingHome + "/build/accounting/accounting.abi"

	daoHome = devHome + "/develop/eosio-contracts"
	daoWasm = daoHome + "/build/hyphadao/hyphadao.wasm"
	daoAbi = daoHome + "/build/hyphadao/hyphadao.abi"

	tokenHome = devHome + "/token"
	tokenWasm = tokenHome + "/token/token.wasm"
	tokenAbi = tokenHome + "/token/token.abi"

	tdHome = devHome + "/telosnetwork/telos-decide"
	tdWasm = tdHome + "/build/contracts/decide/decide.wasm"
	tdAbi = tdHome + "/build/contracts/decide/decide.abi"

	treasuryHome = devHome + "/hypha/treasury-contracts"
	treasuryWasm = treasuryHome + "/treasury/treasury.wasm"
	treasuryAbi = treasuryHome + "/treasury/treasury.abi"

	monitorHome = devHome + "/hypha/monitor"
	monitorWasm = monitorHome + "/monitor/monitor.wasm"
	monitorAbi = monitorHome + "/monitor/monitor.abi"

	seedsHome = devHome + "/hypha/seeds-contracts"
	escrowWasm = seedsHome + "/artifacts/escrow.wasm"
	escrowAbi = seedsHome + "/artifacts/escrow.abi"
	exchangeWasm = "mocks/seedsexchg/seedsexchg/seedsexchg.wasm"
	exchangeAbi = "mocks/seedsexchg/seedsexchg/seedsexchg.abi"

	var env Environment

	env.api = *eos.New(testingEndpoint)
	// api.Debug = true
	env.ctx = context.Background()

	keyBag := &eos.KeyBag{}
	err := keyBag.ImportPrivateKey(env.ctx, eostest.DefaultKey())
	assert.NilError(t, err)

	env.api.SetSigner(keyBag)

	env.VotingDurationSeconds = 2
	env.SeedsDeferralFactor = 100
	env.HyphaDeferralFactor = 25

	env.PeriodDuration, _ = time.ParseDuration("6s")
	env.NumPeriods = 10

	var bankKey ecc.PublicKey

	env.Accounting, _ = eostest.CreateAccountFromString(env.ctx, &env.api, "accounting", eostest.DefaultKey())

	env.DAO, _ = eostest.CreateAccountFromString(env.ctx, &env.api, "dao.hypha", eostest.DefaultKey())
	bankKey, env.Bank, _ = eostest.CreateAccountWithRandomKey(env.ctx, &env.api, "bank.hypha")

	bankPermissionActions := []*eos.Action{system.NewUpdateAuth(env.Bank,
		"active",
		"owner",
		eos.Authority{
			Threshold: 1,
			Keys: []eos.KeyWeight{{
				PublicKey: bankKey,
				Weight:    1,
			}},
			Accounts: []eos.PermissionLevelWeight{
				{
					Permission: eos.PermissionLevel{
						Actor:      env.Bank,
						Permission: "eosio.code",
					},
					Weight: 1,
				},
				{
					Permission: eos.PermissionLevel{
						Actor:      env.DAO,
						Permission: "eosio.code",
					},
					Weight: 1,
				}},
			Waits: []eos.WaitWeight{},
		}, "owner")}

	_, err = eostest.ExecTrx(env.ctx, &env.api, bankPermissionActions)
	assert.NilError(t, err)

	_, env.HusdToken, _ = eostest.CreateAccountWithRandomKey(env.ctx, &env.api, "husd.hypha")
	_, env.HvoiceToken, _ = eostest.CreateAccountWithRandomKey(env.ctx, &env.api, "hvoice.hypha")
	_, env.HyphaToken, _ = eostest.CreateAccountWithRandomKey(env.ctx, &env.api, "token.hypha")
	_, env.Events, _ = eostest.CreateAccountWithRandomKey(env.ctx, &env.api, "publsh.hypha")
	_, env.SeedsToken, _ = eostest.CreateAccountWithRandomKey(env.ctx, &env.api, "token.seeds")
	_, env.SeedsEscrow, _ = eostest.CreateAccountWithRandomKey(env.ctx, &env.api, "escrow.seeds")
	_, env.SeedsExchange, _ = eostest.CreateAccountWithRandomKey(env.ctx, &env.api, "tlosto.seeds")

	_, env.TelosDecide, _ = eostest.CreateAccountWithRandomKey(env.ctx, &env.api, "telos.decide")

	// t.Log("Deploying DAO contract to 		: ", env.DAO)
	// setCodeActions, err := system.NewSetCode(env.DAO, daoWasm)
	// _, err = eostest.ExecTrx(env.ctx, &env.api, []*eos.Action{setCodeActions})
	// assert.NilError(t, err)

	// setAbiActions, err := system.NewSetABI(env.DAO, daoAbi)
	// _, err = eostest.ExecTrx(env.ctx, &env.api, []*eos.Action{setAbiActions})
	// assert.NilError(t, err)

	t.Log("Deploying Accounting contract to 		: ", env.Accounting)
	_, err = eostest.SetContract(env.ctx, &env.api, env.Accounting, accountingWasm, accountingAbi)
	assert.NilError(t, err)
	// _, err = eostest.SetContract(env.ctx, &env.api, env.DAO, daoWasm, daoAbi)
	// assert.NilError(t, err)

	/*t.Log("Deploying Treasury contract to 		: ", env.Bank)
	_, err = eostest.SetContract(env.ctx, &env.api, env.Bank, treasuryWasm, treasuryAbi)
	assert.NilError(t, err)

	t.Log("Deploying Escrow contract to 		: ", env.SeedsEscrow)
	_, err = eostest.SetContract(env.ctx, &env.api, env.SeedsEscrow, escrowWasm, escrowAbi)
	assert.NilError(t, err)

	// t.Log("Deploying SeedsExchange contract to 		: ", env.SeedsExchange)
	// _, err = eostest.SetContract(env.ctx, &env.api, env.SeedsExchange, exchangeWasm, exchangeAbi)
	// assert.NilError(t, err)
	//loadSeedsTablesFromProd(t, &env, "https://api.telos.kitchen")

	t.Log("Deploying Events contract to 		: ", env.Events)
	_, err = eostest.SetContract(env.ctx, &env.api, env.Events, monitorWasm, monitorAbi)
	assert.NilError(t, err)

	// _, err = dao.CreateRoot(env.ctx, &env.api, env.DAO)
	// assert.NilError(t, err)
	// //This no longer works since CreateRoot also creates the settings document
	// //env.Root, err = docgraph.GetLastDocument(env.ctx, &env.api, env.DAO)
	// rootHash := string("d4ec74355830056924c83f20ffb1a22ad0c5145a96daddf6301897a092de951e")
	// env.Root, err = docgraph.LoadDocument(env.ctx, &env.api, env.DAO, rootHash)
	// assert.NilError(t, err)
  
	husdMaxSupply, _ := eos.NewAssetFromString("1000000000.00 HUSD")
	_, err = eostest.DeployAndCreateToken(env.ctx, t, &env.api, tokenHome, env.HusdToken, env.Bank, husdMaxSupply)
	assert.NilError(t, err)

	hyphaMaxSupply, _ := eos.NewAssetFromString("1000000000.00 HYPHA")
	_, err = eostest.DeployAndCreateToken(env.ctx, t, &env.api, tokenHome, env.HyphaToken, env.DAO, hyphaMaxSupply)
	assert.NilError(t, err)

	hvoiceMaxSupply, _ := eos.NewAssetFromString("1000000000.00 HVOICE")
	_, err = eostest.DeployAndCreateToken(env.ctx, t, &env.api, tokenHome, env.HvoiceToken, env.DAO, hvoiceMaxSupply)
	assert.NilError(t, err)

	seedsMaxSupply, _ := eos.NewAssetFromString("1000000000.0000 SEEDS")
	_, err = eostest.DeployAndCreateToken(env.ctx, t, &env.api, tokenHome, env.SeedsToken, env.DAO, seedsMaxSupply)
	assert.NilError(t, err)

	_, err = dao.Issue(env.ctx, &env.api, env.SeedsToken, env.DAO, seedsMaxSupply)
	assert.NilError(t, err)

	// t.Log("Setting configuration options on DAO 		: ", env.DAO)
	// _, err = dao.SetIntSetting(env.ctx, &env.api, env.DAO, "voting_duration_sec", env.VotingDurationSeconds)
	// assert.NilError(t, err)

	// _, err = dao.SetIntSetting(env.ctx, &env.api, env.DAO, "seeds_deferral_factor_x100", env.SeedsDeferralFactor)
	// assert.NilError(t, err)

	// _, err = dao.SetIntSetting(env.ctx, &env.api, env.DAO, "hypha_deferral_factor_x100", env.HyphaDeferralFactor)
	// assert.NilError(t, err)

	// _, err = dao.SetIntSetting(env.ctx, &env.api, env.DAO, "paused", 0)
	// assert.NilError(t, err)

	// dao.SetNameSetting(env.ctx, &env.api, env.DAO, "hypha_token_contract", env.HyphaToken)
	// dao.SetNameSetting(env.ctx, &env.api, env.DAO, "hvoice_token_contract", env.HvoiceToken)
	// dao.SetNameSetting(env.ctx, &env.api, env.DAO, "husd_token_contract", env.HusdToken)
	// dao.SetNameSetting(env.ctx, &env.api, env.DAO, "seeds_token_contract", env.SeedsToken)
	// dao.SetNameSetting(env.ctx, &env.api, env.DAO, "seeds_escrow_contract", env.SeedsEscrow)
	// dao.SetNameSetting(env.ctx, &env.api, env.DAO, "publisher_contract", env.Events)
	// dao.SetNameSetting(env.ctx, &env.api, env.DAO, "treasury_contract", env.Bank)
	// dao.SetNameSetting(env.ctx, &env.api, env.DAO, "telos_decide_contract", env.TelosDecide)

	t.Log("Adding "+strconv.Itoa(env.NumPeriods)+" periods with duration 		: ", env.PeriodDuration)
	_, err = dao.AddPeriods(env.ctx, &env.api, env.DAO, env.NumPeriods, env.PeriodDuration)
	assert.NilError(t, err)

	// setup TLOS system contract
	_, tlosToken, err := eostest.CreateAccountWithRandomKey(env.ctx, &env.api, "eosio.token")
	assert.NilError(t, err)

	tlosMaxSupply, _ := eos.NewAssetFromString("1000000000.0000 TLOS")
	_, err = eostest.DeployAndCreateToken(env.ctx, t, &env.api, tokenHome, tlosToken, env.DAO, tlosMaxSupply)
	assert.NilError(t, err)

	_, err = dao.Issue(env.ctx, &env.api, tlosToken, env.DAO, tlosMaxSupply)
	assert.NilError(t, err)

	// deploy TD contract
	t.Log("Deploying/configuring Telos Decide contract 		: ", env.TelosDecide)
	_, err = eostest.SetContract(env.ctx, &env.api, env.TelosDecide, tdWasm, tdAbi)
	assert.NilError(t, err)

	// call init action
	_, err = dao.InitTD(env.ctx, &env.api, env.TelosDecide)
	assert.NilError(t, err)

	// transfer
	_, err = dao.Transfer(env.ctx, &env.api, tlosToken, env.DAO, env.TelosDecide, tlosMaxSupply, "deposit")
	assert.NilError(t, err)

	_, err = dao.NewTreasury(env.ctx, &env.api, env.TelosDecide, env.DAO)
	assert.NilError(t, err)

	_, err = dao.RegVoter(env.ctx, &env.api, env.TelosDecide, env.DAO)
	assert.NilError(t, err)

	daoTokens, _ := eos.NewAssetFromString("1.00 HVOICE")
	_, err = dao.Mint(env.ctx, &env.api, env.TelosDecide, env.DAO, env.DAO, daoTokens)
	assert.NilError(t, err)

	//whaleTokens, _ := eos.NewAssetFromString("100.00 HVOICE")
	// env.Whale, err = SetupMember(t, env.ctx, &env.api, env.DAO, env.TelosDecide, "whale", whaleTokens)
	// assert.NilError(t, err)

	// index := 1
	// for index < 5 {

	// 	memberNameIn := "member" + strconv.Itoa(index)

	// 	newMember, err := SetupMember(t, env.ctx, &env.api, env.DAO, env.TelosDecide, memberNameIn, daoTokens)
	// 	assert.NilError(t, err)

	// 	env.Members = append(env.Members, newMember)
	// 	index++
	// }

	*/

	return &env
}

func SetupMember(t *testing.T, ctx context.Context, api *eos.API,
	contract, telosDecide eos.AccountName, memberName string, hvoice eos.Asset) (Member, error) {

	t.Log("Creating and enrolling new member  		: ", memberName, " 	with voting power	: ", hvoice.String())
	memberAccount, err := eostest.CreateAccountFromString(ctx, api, memberName, eostest.DefaultKey())
	assert.NilError(t, err)

	_, err = dao.RegVoter(ctx, api, telosDecide, memberAccount)
	assert.NilError(t, err)

	_, err = dao.Mint(ctx, api, telosDecide, contract, memberAccount, hvoice)
	assert.NilError(t, err)

	_, err = dao.Apply(ctx, api, contract, memberAccount, "apply to DAO")
	assert.NilError(t, err)

	_, err = dao.Enroll(ctx, api, contract, contract, memberAccount)
	assert.NilError(t, err)

	memberDoc, err := docgraph.GetLastDocument(ctx, api, contract)
	assert.NilError(t, err)

	memberNameFV, err := memberDoc.GetContent("member")
	assert.NilError(t, err)
	assert.Equal(t, eos.AN(string(memberNameFV.Impl.(eos.Name))), memberAccount)

	return Member{
		Member: memberAccount,
		Doc:    memberDoc,
	}, nil
}