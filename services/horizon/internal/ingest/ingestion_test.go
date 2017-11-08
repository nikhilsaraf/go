package ingest

import (
	"testing"

	sq "github.com/Masterminds/squirrel"
	"github.com/stellar/go/services/horizon/internal/db2/core"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/test"
	testDB "github.com/stellar/go/services/horizon/internal/test/db"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/xdr"
	"github.com/stretchr/testify/assert"
)

func TestEmptySignature(t *testing.T) {
	ingestion := Ingestion{
		DB: &db.Session{
			DB: testDB.Horizon(t),
		},
	}
	ingestion.Start()

	envelope := xdr.TransactionEnvelope{}
	resultPair := xdr.TransactionResultPair{}
	meta := xdr.TransactionMeta{}

	xdr.SafeUnmarshalBase64("AAAAAMIK9djC7k75ziKOLJcvMAIBG7tnBuoeI34x+Pi6zqcZAAAAZAAZphYAAAABAAAAAAAAAAAAAAABAAAAAAAAAAEAAAAAynnCTTyw53VVRLOWX6XKTva63IM1LslPNW01YB0hz/8AAAAAAAAAAlQL5AAAAAAAAAAAAh0hz/8AAABA8qkkeKaKfsbgInyIkzXJhqJE5/Ufxri2LdxmyKkgkT6I3sPmvrs5cPWQSzEQyhV750IW2ds97xTHqTpOfuZCAnhSuFUAAAAA", &envelope)
	xdr.SafeUnmarshalBase64("AAAAAAAAAGQAAAAAAAAAAQAAAAAAAAABAAAAAAAAAAA=", &resultPair.Result)
	xdr.SafeUnmarshalBase64("AAAAAAAAAAEAAAADAAAAAQAZphoAAAAAAAAAAMIK9djC7k75ziKOLJcvMAIBG7tnBuoeI34x+Pi6zqcZAAAAF0h255wAGaYWAAAAAQAAAAMAAAAAAAAAAAAAAAADBQUFAAAAAwAAAAAtkqVYLPLYhqNMmQLPc+T9eTWp8LIE8eFlR5K4wNJKTQAAAAMAAAAAynnCTTyw53VVRLOWX6XKTva63IM1LslPNW01YB0hz/8AAAADAAAAAuOwxEKY/BwUmvv0yJlvuSQnrkHkZJuTTKSVmRt4UrhVAAAAAwAAAAAAAAAAAAAAAwAZphYAAAAAAAAAAMp5wk08sOd1VUSzll+lyk72utyDNS7JTzVtNWAdIc//AAAAF0h26AAAGaYWAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAQAZphoAAAAAAAAAAMp5wk08sOd1VUSzll+lyk72utyDNS7JTzVtNWAdIc//AAAAGZyCzAAAGaYWAAAAAAAAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAA", &meta)

	transaction := &core.Transaction{
		TransactionHash: "1939a8de30981e4171e1aaeca54a058a7fb06684864facba0620ab8cc5076d4f",
		LedgerSequence:  1680922,
		Index:           1,
		Envelope:        envelope,
		Result:          resultPair,
		ResultMeta:      meta,
	}

	transactionFee := &core.TransactionFee{}

	builder := ingestion.transactionInsertBuilder(1, transaction, transactionFee)
	sql, args, err := builder.ToSql()
	assert.Equal(t, "INSERT INTO history_transactions (id,transaction_hash,ledger_sequence,application_order,account,account_sequence,fee_paid,operation_count,tx_envelope,tx_result,tx_meta,tx_fee_meta,signatures,time_bounds,memo_type,memo,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?::character varying[],?,?,?,?,?)", sql)
	assert.Equal(t, `{"8qkkeKaKfsbgInyIkzXJhqJE5/Ufxri2LdxmyKkgkT6I3sPmvrs5cPWQSzEQyhV750IW2ds97xTHqTpOfuZCAg==",""}`, args[12])
	assert.NoError(t, err)

	err = ingestion.Transaction(1, transaction, transactionFee)
	assert.NoError(t, err)

	err = ingestion.Close()
	assert.NoError(t, err)
}

func TestAssetIngest(t *testing.T) {
	//ingest kahuna and sample a single expected asset output

	tt := test.Start(t).ScenarioWithoutHorizon("kahuna")
	defer tt.Finish()
	s := ingest(tt)
	q := history.Q{Session: s.Ingestion.DB}

	expectedAsset := history.Asset{
		ID:     4,
		Type:   "credit_alphanum4",
		Code:   "USD",
		Issuer: "GB2QIYT2IAUFMRXKLSLLPRECC6OCOGJMADSPTRK7TGNT2SFR2YGWDARD",
	}

	actualAsset := history.Asset{}
	err := q.GetAssetByID(&actualAsset, 4)
	tt.Require.NoError(err)
	tt.Assert.Equal(expectedAsset, actualAsset)
}

func TestAssetStatsCount(t *testing.T) {
	tt := test.Start(t).ScenarioWithoutHorizon("kahuna")
	defer tt.Finish()
	s := ingest(tt)
	q := history.Q{Session: s.Ingestion.DB}

	var countHistory int
	err := q.Get(&countHistory, sq.Select("COUNT(*)").From("history_assets").Where(sq.NotEq{"asset_type": "native"}))
	tt.Require.NoError(err)

	var countStats int
	err = q.Get(&countStats, sq.Select("COUNT(*)").From("asset_stats"))
	tt.Require.NoError(err)

	tt.Assert.Equal(countHistory, countStats)
}

func TestAssetStatsChangeTrust(t *testing.T) {
	tt := test.Start(t).ScenarioWithoutHorizon("change_trust")
	defer tt.Finish()
	s := ingest(tt)
	q := history.Q{Session: s.Ingestion.DB}

	var countStats int
	err := q.Get(&countStats, sq.Select("COUNT(*)").From("asset_stats"))
	tt.Require.NoError(err)
	tt.Assert.Equal(1, countStats)

	sql := sq.
		Select(
			"hist.id",
			"hist.asset_type",
			"hist.asset_code",
			"hist.asset_issuer",
			"stats.amount",
			"stats.num_accounts",
			"stats.flags",
			"stats.toml",
		).
		From("history_assets hist").
		Join("asset_stats stats ON hist.id = stats.id").
		Limit(1)

	type AssetStatResult struct {
		ID          int64  `db:"id"`
		Type        string `db:"asset_type"`
		Code        string `db:"asset_code"`
		Issuer      string `db:"asset_issuer"`
		Amount      int64  `db:"amount"`
		NumAccounts int32  `db:"num_accounts"`
		Flags       int8   `db:"flags"`
		Toml        string `db:"toml"`
	}
	actualStat := AssetStatResult{}
	err = q.Get(&actualStat, sql)
	tt.Require.NoError(err)
	tt.Assert.Equal(AssetStatResult{
		ID:          1,
		Type:        "credit_alphanum4",
		Code:        "USD",
		Issuer:      "GC23QF2HUE52AMXUFUH3AYJAXXGXXV2VHXYYR6EYXETPKDXZSAW67XO4",
		Amount:      10,
		NumAccounts: 5,
		Flags:       0,
		Toml:        "https://www.stellar.org/.well-known/stellar.toml",
	}, actualStat)
}
