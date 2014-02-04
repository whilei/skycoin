package coin

import (
    "bytes"
    "errors"
    "github.com/skycoin/encoder"
    "log"
    "math"
)

/*
	Base Transaction Type
*/

/*
Compute Later:

type TransactionMeta struct {
	Fee uint64
}
*/

type Transaction struct {
    Header TransactionHeader //Outer Hash
    In     []TransactionInput
    Out    []TransactionOutput
}

type TransactionHeader struct { //not hashed
    Hash SHA256 //inner hash
    Sigs []Sig  //list of signatures, 64+1 bytes
}

/*
	Can remove SigIdx; recover address from signature
	- only saves 2 bytes
	Require Sigs are sorted to enforce immutability?
	- SidIdx enforces immutability
*/
type TransactionInput struct {
    SigIdx uint16 //signature index
    UxOut  SHA256 //Unspent Block that is being spent
}

//hash output/name is function of Hash
type TransactionOutput struct {
    DestinationAddress Address //address to send to
    Coins              uint64  //amount to be sent in coins
    Hours              uint64  //amount to be sent in coin hours
}

/*
	Add immutability and hash checks here
*/

// Verify attempts to determine if the transaction is well formed
// Verify cannot check transaction signatures, it needs the address from unspents
// Verify cannot check if outputs being spent exist
// Verify cannot check if the transaction would create or destroy coins
// or if the inputs have the required coin base
func (self *Transaction) Verify() error {
    //TODO: optionally check that each signature is used at least once

    h := txnhashInner()
    if h != txnHeader.Hash {
        return errors.New("Invalid header hash")
    }

    if len(txnIn) == 0 {
        return errors.New("No inputs")
    }

    if len(txnOut) == 0 {
        return errors.New("No outputs")
    }

    // Check signature index fields
    if len(txnHeader.Sigs) >= math.MaxUint16 {
        return errors.New("signatures count exceeds uint16")
    }

    // Check duplicate inputs
    for i := 0; i < len(txnIn); i++ {
        for j := i + 1; i < len(txnIn); j++ {
            if txnIn[i].UxOut == txnIn[j].UxOut {
                return errors.New("Duplicate spend")
            }
        }
    }

    // Check for hash collisions in outputs
    outputs := make([]SHA256, 0)

    for _, to := range txnOut {
        var uxb UxOut
        uxb.SrcTransaction = txnHeader.Hash
        uxb.Coins = to.Coins
        uxb.Hours = to.Hours
        uxb.Address = to.DestinationAddress
        outputs = append(outputs, uxb.Hash())
    }

    if  HashArrayHasDupes(outputs) == true {
        return errors.New("Duplicate output in transaction")
    }

    //validate signature
    for _, txi := range txn.In {

        sig := txn.Header.Sigs[txi.SigIdx]
        hash := txi.UxOut
        pubkey, err := PubKeyFromSig(sig)

        if err != nil {
            return errors.New("pubkey recovery from signature failed")
        }

        err := VerifySignature(pubkey, sig, hash)
        if err != nil {
            return errors.New("signature verification failed")
        }
    }

    return nil
}

// Adds a TransactionInput to the Transaction given the hash of a UxOut.
// Returns the signature index for later signing
func (self *Transaction) PushInput(uxOut SHA256) uint16 {
    //TODO: do no create new si
    if len(self.In) >= math.MaxUint16 {
        log.Panic("Max transaction inputs reached")
    }
    sigIdx := uint16(len(self.In))
    ti := TransactionInput{
        SigIdx: sigIdx,
        UxOut:  uxOut,
    }
    self.In = append(self.In, ti)
    return sigIdx
}

// Adds a TransactionOutput, sending coins & hours to an Address
func (self *Transaction) PushOutput(dst Address, coins, hours uint64) {
    to := TransactionOutput{
        DestinationAddress: dst,
        Coins:              coins,
        Hours:              hours,
    }
    self.Out = append(self.Out, to)
}

// Signs a TransactionInput at its signature index
func (self *Transaction) signInput(idx uint16, sec SecKey) {
    hash := self.hashInner()
    sig, err := SignHash(hash, sec)
    if err != nil {
        log.Panic("Failed to sign hash")
    }
    txInLen := len(self.In)
    if txInLen > math.MaxUint16 {
        log.Panic("In too large")
    }
    if idx >= uint16(txInLen) {
        log.Panic("Invalid In idx")
    }
    for len(self.Header.Sigs) <= int(idx) {
        self.Header.Sigs = append(self.Header.Sigs, Sig{})
    }
    self.Header.Sigs[idx] = sig
}

// Signs all inputs in the transaction
func (self *Transaction) SignInputs(keys map[uint16]SecKey) {
    for _, ti := range self.In {
        self.signInput(ti.SigIdx, keys[ti.SigIdx])
    }
}

// Hashes an entire Transaction struct
func (self *Transaction) Hash() SHA256 {
    b1 := encoder.Serialize(*self)
    return SumDoubleSHA256(b1) //double SHA256 hash
}

func (self *Transaction) Serialize() []byte {
    return encoder.Serialize(*self)
}

func TransactionDeserialize(b []byte) Transaction {
    var t Transaction
    if err := encoder.DeserializeRaw(b, t); err != nil {
        log.Panic("Failed to deserialize transaction")
    }
    return t
}

// Saves the txn body hash to TransactionHeader.Hash
func (self *Transaction) UpdateHeader() {
    self.Header.Hash = self.hashInner()
}

// Hashes only the Transaction Inputs & Outputs
func (self *Transaction) hashInner() SHA256 {
    b1 := encoder.Serialize(self.In)
    b2 := encoder.Serialize(self.Out)
    b3 := append(b1, b2...)
    return SumSHA256(b3)
}

type Transactions []Transaction

func (self Transactions) Len() int {
    return len(self)
}

func (self Transactions) Less(i, j int) bool {
    return bytes.Compare(self[i].Header.Hash[:], self[j].Header.Hash[:]) < 0
}

func (self Transactions) Swap(i, j int) {
    t := self[i]
    self[i] = self[j]
    self[j] = t
}
