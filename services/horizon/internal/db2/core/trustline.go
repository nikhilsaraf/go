package core

import (
	"errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/stellar/go/xdr"
	"github.com/stellar/go/services/horizon/internal/log"
)

// AssetsForAddress loads `dest` as `[]xdr.Asset` with every asset the account
// at `addy` can hold.
func (q *Q) AssetsForAddress(dest interface{}, addy string) error {
	log.Debug(8, addy)
	var tls []Trustline

	err := q.TrustlinesByAddress(&tls, addy)
	if err != nil {
		return err
	}

	dtl, ok := dest.(*[]xdr.Asset)
	if !ok {
		return errors.New("Invalid destination")
	}

	result := make([]xdr.Asset, len(tls)+1)
	*dtl = result

	for i, tl := range tls {
		log.Debug("5", tl)
		result[i], err = AssetFromDB(tl.Assettype, tl.Assetcode, tl.Issuer)
		if err != nil {
			return err
		}
	}

	result[len(result)-1], err = xdr.NewAsset(xdr.AssetTypeAssetTypeNative, nil)

	return err
}

// LoadAssetForAddress loads `dest` as `[]xdr.Asset` with the provided asset if it's in the trustline
func (q *Q) LoadAssetForAddress(
	dest interface{},
	address string,
	sourceAssetType string,
	sourceAssetCode string,
	sourceIssuer string,
) error {
	log.Debug("2", address, sourceAssetType, sourceAssetCode, sourceIssuer)
	dtl, ok := dest.(*[]xdr.Asset)
	if !ok {
		return errors.New("Invalid destination")
	}
	result := make([]xdr.Asset, 1)
	*dtl = result

	log.Debug("3")
	var err error
	if sourceAssetType == "native" {
		result[0], err = xdr.NewAsset(xdr.AssetTypeAssetTypeNative, nil)
		return err
	}

	log.Debug("4")
	var tls []Trustline
	err = q.TrustlinesByAsset(&tls, address, sourceAssetType, sourceAssetCode, sourceIssuer)
	if err != nil {
		return err
	}

	log.Debug("5", tls)
	result[0], err = AssetFromDB(tls[0].Assettype, tls[0].Assetcode, tls[0].Issuer)

	log.Debug("6")
	log.Debug(result[0])
	log.Debug("7")
	return err
}

// TrustlinesByAddress loads all trustlines for `addy`
func (q *Q) TrustlinesByAddress(dest interface{}, addy string) error {
	sql := selectTrustline.Where("accountid = ?", addy)
	return q.Select(dest, sql)
}

// TrustlinesByAsset loads a single trustlines for `address`, `assetType`, `assetCode`, and `assetIssuer`
func (q *Q) TrustlinesByAsset(
	dest interface{},
	address string,
	assetType string,
	assetCode string,
	assetIssuer string,
) error {
	sql := selectTrustline.Where("accountid = ?", address)
		//.Where("assettype = ?", assetType)
		//.Where("assetcode = ?", assetCode)
		//.Where("issuer = ?", assetIssuer)
		//.Limit(1)
	return q.Select(dest, sql)
}

var selectTrustline = sq.Select(
	"tl.accountid",
	"tl.assettype",
	"tl.issuer",
	"tl.assetcode",
	"tl.tlimit",
	"tl.balance",
	"tl.flags",
).From("trustlines tl")
