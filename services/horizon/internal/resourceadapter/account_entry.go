package resourceadapter

import (
	"context"
	"fmt"
	"strconv"

	protocol "github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/httpx"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/support/render/hal"
	"github.com/stellar/go/xdr"
)

// PopulateAccountEntry fills out the resource's fields
func PopulateAccountEntry(
	ctx context.Context,
	dest *protocol.Account,
	account history.AccountEntry,
	accountData []history.Data,
	accountSigners []history.AccountSigner,
	trustLines []history.TrustLine,
) error {
	dest.ID = account.AccountID
	dest.PT = account.AccountID
	dest.AccountID = account.AccountID
	dest.Sequence = strconv.FormatInt(account.SequenceNumber, 10)
	dest.SubentryCount = int32(account.NumSubEntries)
	dest.InflationDestination = account.InflationDestination
	dest.HomeDomain = account.HomeDomain
	dest.LastModifiedLedger = account.LastModifiedLedger

	dest.Flags.AuthRequired = account.IsAuthRequired()
	dest.Flags.AuthRevocable = account.IsAuthRevocable()
	dest.Flags.AuthImmutable = account.IsAuthImmutable()

	dest.Thresholds.LowThreshold = account.ThresholdLow
	dest.Thresholds.MedThreshold = account.ThresholdMedium
	dest.Thresholds.HighThreshold = account.ThresholdHigh

	// populate balances
	dest.Balances = make([]protocol.Balance, len(trustLines)+1)
	for i, tl := range trustLines {
		err := PopulateHistoryBalance(&dest.Balances[i], tl)
		if err != nil {
			return errors.Wrap(err, "populating balance")
		}
	}

	// add native balance
	err := PopulateNativeBalance(
		&dest.Balances[len(dest.Balances)-1],
		xdr.Int64(account.Balance),
		xdr.Int64(account.BuyingLiabilities),
		xdr.Int64(account.SellingLiabilities),
	)
	if err != nil {
		return errors.Wrap(err, "populating native balance")
	}

	// populate data
	dest.Data = make(map[string]string)
	for _, d := range accountData {
		dest.Data[d.Name] = d.Value.Base64()
	}

	masterKeyIncluded := false

	// populate signers
	dest.Signers = make([]protocol.Signer, len(accountSigners))
	for i, signer := range accountSigners {
		dest.Signers[i].Weight = signer.Weight
		dest.Signers[i].Key = signer.Signer
		dest.Signers[i].Type = protocol.MustKeyTypeFromAddress(signer.Signer)

		if account.AccountID == signer.Signer {
			masterKeyIncluded = true
		}
	}

	if !masterKeyIncluded {
		dest.Signers = append(dest.Signers, protocol.Signer{
			Weight: int32(account.MasterWeight),
			Key:    account.AccountID,
			Type:   protocol.MustKeyTypeFromAddress(account.AccountID),
		})
	}

	lb := hal.LinkBuilder{httpx.BaseURL(ctx)}
	self := fmt.Sprintf("/accounts/%s", account.AccountID)
	dest.Links.Self = lb.Link(self)
	dest.Links.Transactions = lb.PagedLink(self, "transactions")
	dest.Links.Operations = lb.PagedLink(self, "operations")
	dest.Links.Payments = lb.PagedLink(self, "payments")
	dest.Links.Effects = lb.PagedLink(self, "effects")
	dest.Links.Offers = lb.PagedLink(self, "offers")
	dest.Links.Trades = lb.PagedLink(self, "trades")
	dest.Links.Data = lb.Link(self, "data/{key}")
	dest.Links.Data.PopulateTemplated()
	return nil
}
