package db_test

import (
	"encoding/json"
	"fmt"
	dbModule "github.com/notegio/openrelay/db"
	"github.com/notegio/openrelay/types"
	"math/big"
	"reflect"
	"testing"
)

func TestIndexOrder(t *testing.T) {
	db, err := getDb()
	if err != nil {
		t.Errorf(err.Error())
		return
	}
	tx := db.Begin()
	defer func() {
		tx.Rollback()
		db.Close()
	}()
	if err := tx.AutoMigrate(&dbModule.Order{}).Error; err != nil {
		t.Errorf(err.Error())
	}
	indexer := dbModule.NewIndexer(tx, dbModule.StatusOpen)
	order := sampleOrder()
	if !order.Signature.Verify(order.Maker) {
		t.Errorf("Failed to verify signature")
	}
	if err := indexer.Index(order); err != nil {
		t.Errorf(err.Error())
	}
}

func TestFillIndex(t *testing.T) {
	db, err := getDb()
	if err != nil {
		t.Errorf(err.Error())
		return
	}
	tx := db.Begin()
	defer func() {
		tx.Rollback()
		db.Close()
	}()
	if err := tx.AutoMigrate(&dbModule.Order{}).Error; err != nil {
		t.Errorf(err.Error())
	}
	indexer := dbModule.NewIndexer(tx, dbModule.StatusOpen)
	order := sampleOrder()
	if err := indexer.Index(order); err != nil {
		t.Errorf(err.Error())
	}
	takerTokenAmount := new(big.Int).SetBytes(order.TakerTokenAmount[:])
	fillString := fmt.Sprintf(
		"{\"orderHash\": \"%#x\", \"filledTakerTokenAmount\": \"%v\"}",
		order.Hash(),
		takerTokenAmount.String(),
	)
	fillRecord := &dbModule.FillRecord{}
	if err := json.Unmarshal([]byte(fillString), fillRecord); err != nil {
		t.Errorf(err.Error())
	}
	if err := indexer.RecordFill(fillRecord); err != nil {
		t.Errorf(err.Error())
	}
	dbOrder := &dbModule.Order{}
	dbOrder.Initialize()
	tx.Model(&dbModule.Order{}).Where("order_hash = ?", order.Hash()).First(dbOrder)
	if !reflect.DeepEqual(dbOrder.TakerTokenAmount, dbOrder.TakerTokenAmountFilled) {
		t.Errorf("TakerTokenAmount should match TakerTokenAmountFilled, got %#x != %#x", dbOrder.TakerTokenAmount[:], dbOrder.TakerTokenAmountFilled[:])
	}
	if dbOrder.Status != dbModule.StatusFilled {
		t.Errorf("Order status should be filled, got %v", dbOrder.Status)
	}
}

func TestCheckUnfundedSufficient(t *testing.T) {
	db, err := getDb()
	if err != nil {
		t.Errorf(err.Error())
		return
	}
	tx := db.Begin()
	defer func() {
		tx.Rollback()
		db.Close()
	}()
	if err := tx.AutoMigrate(&dbModule.Order{}).Error; err != nil {
		t.Errorf(err.Error())
	}
	indexer := dbModule.NewIndexer(tx, dbModule.StatusUnfunded)
	order := sampleOrder()
	dbOrder := &dbModule.Order{}
	dbOrder.Order = *order
	if err := dbOrder.Save(tx, dbModule.StatusOpen).Error; err != nil {
		t.Errorf(err.Error())
	}
	// Checking that the MakerAddress has enough of MakerToken, asserting that they have exactly MakerTokenAmount of the token
	// This check ignores ZRX, by saying that the TakerToken is ZRX, rather than the MakerToken.
	if err := indexer.RecordSpend(dbOrder.Maker, dbOrder.MakerToken, dbOrder.TakerToken, dbOrder.MakerTokenAmount); err != nil {
		t.Errorf(err.Error())
	}
	dbOrders := []dbModule.Order{}
	tx.Model(&dbModule.Order{}).Where("order_hash = ?", order.Hash()).Find(&dbOrders)
	dbOrder = &dbOrders[0]
	if dbOrder.Status != dbModule.StatusOpen {
		t.Errorf("Order status should not have changed, but is now %v", dbOrder.Status)
	}
}

func TestCheckUnfundedNotFound(t *testing.T) {
	db, err := getDb()
	if err != nil {
		t.Errorf(err.Error())
		return
	}
	tx := db.Begin()
	defer func() {
		tx.Rollback()
		db.Close()
	}()
	if err := tx.AutoMigrate(&dbModule.Order{}).Error; err != nil {
		t.Errorf(err.Error())
	}
	indexer := dbModule.NewIndexer(tx, dbModule.StatusUnfunded)
	order := sampleOrder()
	dbOrder := &dbModule.Order{}
	dbOrder.Order = *order
	if err := dbOrder.Save(tx, dbModule.StatusOpen).Error; err != nil {
		t.Errorf(err.Error())
	}
	// Checking that the Taker has enough of MakerToken, asserting that they have exactly MakerTokenAmount of the token
	// This check ignores ZRX, by saying that the TakerToken is ZRX, rather than the MakerToken.
	// This should not change anything, because no orders will match
	if err := indexer.RecordSpend(dbOrder.Taker, dbOrder.MakerToken, dbOrder.TakerToken, dbOrder.MakerTokenAmount); err != nil {
		t.Errorf(err.Error())
	}
	dbOrders := []dbModule.Order{}
	tx.Model(&dbModule.Order{}).Where("order_hash = ?", order.Hash()).Find(&dbOrders)
	dbOrder = &dbOrders[0]
	if dbOrder.Status != dbModule.StatusOpen {
		t.Errorf("Order status should not have changed, but is now %v", dbOrder.Status)
	}
}

func TestCheckUnfundedInsufficient(t *testing.T) {
	db, err := getDb()
	if err != nil {
		t.Errorf(err.Error())
		return
	}
	tx := db.Begin()
	defer func() {
		tx.Rollback()
		db.Close()
	}()
	if err := tx.AutoMigrate(&dbModule.Order{}).Error; err != nil {
		t.Errorf(err.Error())
	}
	indexer := dbModule.NewIndexer(tx, dbModule.StatusUnfunded)
	order := sampleOrder()
	dbOrder := &dbModule.Order{}
	dbOrder.Order = *order
	if err := dbOrder.Save(tx, dbModule.StatusOpen).Error; err != nil {
		t.Errorf(err.Error())
	}
	// Checking that the Taker has enough of MakerToken, asserting that they have exactly MakerTokenAmount of the token
	// This check ignores ZRX, by saying that the TakerToken is ZRX, rather than the MakerToken.
	// This should not change anything, because no orders will match
	zero := &types.Uint256{}
	if err := indexer.RecordSpend(dbOrder.Maker, dbOrder.MakerToken, dbOrder.TakerToken, zero); err != nil {
		t.Errorf(err.Error())
	}
	dbOrders := []dbModule.Order{}
	tx.Model(&dbModule.Order{}).Where("order_hash = ?", order.Hash()).Find(&dbOrders)
	dbOrder = &dbOrders[0]
	if dbOrder.Status != dbModule.StatusUnfunded {
		t.Errorf("Order status should have changed, but is now %v", dbOrder.Status)
	}
}
