package internal

import (
	"strconv"
	"sync"

	b "github.com/stellar/go/build"
	client "github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/keypair"
)

// Bot represents the friendbot subsystem.
type Bot struct {
	Client          *client.Client
	Secret          string
	Network         string
	StartingBalance string

	sequence uint64
	lock     sync.Mutex
}

// Pay funds the account at `destAddress`
func (bot *Bot) Pay(destAddress string) (client.TransactionSuccess, error) {
	err := bot.checkSequenceRefresh()
	if err != nil {
		return client.TransactionSuccess{}, err
	}

	// var envelope string
	signed, err := bot.makeTx(destAddress)
	if err != nil {
		return client.TransactionSuccess{}, err
	}

	return bot.Client.SubmitTransaction(signed)
}

// establish initial sequence if needed
func (bot *Bot) checkSequenceRefresh() error {
	if bot.sequence != 0 {
		return nil
	}

	bot.lock.Lock()
	defer bot.lock.Unlock()

	// short-circuit here if the thread that previously had the lock was successful in refreshing the sequence
	if bot.sequence != 0 {
		return nil
	}

	return bot.refreshSequence()
}

func (bot *Bot) makeTx(destAddress string) (string, error) {
	bot.lock.Lock()
	defer bot.lock.Unlock()

	txn := b.Transaction(
		b.SourceAccount{AddressOrSeed: bot.Secret},
		b.Sequence{Sequence: bot.sequence + 1},
		b.Network{Passphrase: bot.Network},
		b.CreateAccount(
			b.Destination{AddressOrSeed: destAddress},
			b.NativeAmount{Amount: bot.StartingBalance},
		),
	)

	if txn.Err != nil {
		return "", txn.Err
	}

	txs := txn.Sign(bot.Secret)
	base64, err := txs.Base64()

	// only increment the in-memory sequence number if we are going to submit the transaction, while we hold the lock
	if err == nil {
		bot.sequence++
	}
	return base64, err
}

// refreshes the sequence from the bot account and increments by 1
func (bot *Bot) refreshSequence() error {
	botAccount, err := bot.Client.LoadAccount(bot.address())
	if err != nil {
		return err
	}

	seq, err := strconv.ParseInt(botAccount.Sequence, 10, 0)
	if err != nil {
		return err
	}

	bot.sequence = uint64(seq)
	return nil
}

func (bot *Bot) address() string {
	kp := keypair.MustParse(bot.Secret)
	return kp.Address()
}
