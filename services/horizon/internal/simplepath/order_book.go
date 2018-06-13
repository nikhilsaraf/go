package simplepath

import (
	"errors"
	"math/big"

	sq "github.com/Masterminds/squirrel"
	"github.com/stellar/go/services/horizon/internal/db2/core"
	"github.com/stellar/go/xdr"
)

// ErrNotEnough represents an error that occurs when pricing a trade on an
// orderbook.  This error occurs when the orderbook cannot fulfill the
// requested amount.
var ErrNotEnough = errors.New("not enough depth")

// orderbook represents a one-way orderbook that is selling you a specific asset (ob.Selling)
type orderBook struct {
	Selling xdr.Asset // the offers are selling this asset
	Buying  xdr.Asset // the offers are buying this asset
	Q       *core.Q
}

// CostToConsumeLiquidity returns the buyingAmount (ob.Buying) needed to consume the sellingAmount (ob.Selling)
func (ob *orderBook) CostToConsumeLiquidity(sellingAmount xdr.Int64) (xdr.Int64, error) {
	// load orderbook from core's db
	sql, e := ob.query()
	if e != nil {
		return 0, e
	}
	rows, e := ob.Q.Query(sql)
	if e != nil {
		return 0, e
	}
	defer rows.Close()

	// remaining is the units of ob.Selling that we want to consume
	remaining := int64(sellingAmount)
	var buyingAmount int64
	for rows.Next() {
		// load data from the row
		var offerAmount, pricen, priced, offerid int64
		e = rows.Scan(&offerAmount, &pricen, &priced, &offerid)
		if e != nil {
			return 0, e
		}

		if offerAmount >= remaining {
			buyingAmount += convertToBuyingUnits(remaining, pricen, priced)
			return xdr.Int64(buyingAmount), nil
		}

		buyingAmount += convertToBuyingUnits(offerAmount, pricen, priced)
		remaining -= offerAmount
	}
	return 0, ErrNotEnough
}

func (ob *orderBook) query() (sq.SelectBuilder, error) {
	var (
		// selling/buying types
		st, bt xdr.AssetType
		// selling/buying codes
		sc, bc string
		// selling/buying issuers
		si, bi string
	)
	e := ob.Selling.Extract(&st, &sc, &si)
	if e != nil {
		return sq.SelectBuilder{}, e
	}
	e = ob.Buying.Extract(&bt, &bc, &bi)
	if e != nil {
		return sq.SelectBuilder{}, e
	}

	sql := sq.
		Select("amount", "pricen", "priced", "offerid").
		From("offers").
		Where(sq.Eq{
			"sellingassettype":               st,
			"COALESCE(sellingassetcode, '')": sc,
			"COALESCE(sellingissuer, '')":    si}).
		Where(sq.Eq{
			"buyingassettype":               bt,
			"COALESCE(buyingassetcode, '')": bc,
			"COALESCE(buyingissuer, '')":    bi}).
		OrderBy("price ASC")
	return sql, nil
}

// convertToBuyingUnits uses special rounding logic to multiply the amount by the price
/*
	offerSellingBound = (offer.price.n > offer.price.d)
		? offer.amount : ceil(floor(offer.amount * offer.price) / offer.price)
	pathPaymentAmountBought = min(offerSellingBound, pathPaymentBuyingBound)
	pathPaymentAmountSold = ceil(pathPaymentAmountBought * offer.price)

	offer.amount = amount selling
	offerSellingBound = roundingCorrectedOffer
	pathPaymentBuyingBound = needed
	pathPaymentAmountBought = what we are consuming from offer
	pathPaymentAmountSold = amount we are giving to the buyer
	Sell units = pathPaymentAmountSold and buy units = pathPaymentAmountBought
*/
func convertToBuyingUnits(sellingAmount int64, pricen int64, priced int64) int64 {
	var a, n, d big.Int
	var r big.Int // result

	a.SetInt64(sellingAmount)
	n.SetInt64(pricen)
	d.SetInt64(priced)

	// offerSellingBound
	r.SetInt64(sellingAmount)
	if pricen <= priced {
		mulFractionRoundDown(r, n, d)
		mulFractionRoundUp(r, d, n)
	}

	// pathPaymentAmountBought
	r = min(r, a)

	// pathPaymentAmountSold
	mulFractionRoundUp(r, n, d)

	return r.Int64()
}

// mulFractionRoundDown sets x = (x * n) / d, which is a round-down operation
func mulFractionRoundDown(x big.Int, n big.Int, d big.Int) {
	x.Mul(&x, &n)
	x.Quo(&x, &d)
}

// mulFractionRoundUp sets x = ((x * n) + d - 1) / d, which is a round-up operation
func mulFractionRoundUp(x big.Int, n big.Int, d big.Int) {
	var one big.Int
	one.SetInt64(1)

	x.Mul(&x, &n)
	x.Add(&x, &d)
	x.Sub(&x, &one)
	x.Quo(&x, &d)
}

// min impl
func min(x big.Int, y big.Int) big.Int {
	cmp := x.Cmp(&y)
	if cmp <= 0 {
		return x
	}
	return y
}
