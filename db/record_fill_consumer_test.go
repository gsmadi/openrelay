package db_test

import (
	"fmt"
	"github.com/notegio/openrelay/channels"
	dbModule "github.com/notegio/openrelay/db"
	"reflect"
	"testing"
	"time"
)

func TestFillConsumer(t *testing.T) {
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
	order := sampleOrder(t)
	if !order.Signature.Verify(order.Maker, order.Hash()) {
		t.Errorf("Failed to verify signature")
	}
	if err := indexer.Index(order); err != nil {
		t.Errorf(err.Error())
	}
	takerAssetAmount := order.TakerAssetAmount.Big()
	fillString := fmt.Sprintf(
		"{\"orderHash\": \"%#x\", \"filledTakerAssetAmount\": \"%v\"}",
		order.Hash(),
		takerAssetAmount.String(),
	)
	publisher, channel := channels.MockChannel()
	consumer := dbModule.NewRecordFillConsumer(tx, 1)
	channel.AddConsumer(consumer)
	channel.StartConsuming()
	defer channel.StopConsuming()
	publisher.Publish(fillString)
	time.Sleep(100 * time.Millisecond)

	dbOrder := &dbModule.Order{}
	dbOrder.Initialize()
	if err := tx.Model(&dbModule.Order{}).Where("order_hash = ?", order.Hash()).First(dbOrder).Error; err != nil {
		t.Errorf(err.Error())
	}
	if !reflect.DeepEqual(dbOrder.TakerAssetAmount, dbOrder.TakerAssetAmountFilled) {
		t.Errorf("TakerAssetAmount should match TakerAssetAmountFilled, got %#x != %#x", dbOrder.TakerAssetAmount[:], dbOrder.TakerAssetAmountFilled[:])
	}
	if dbOrder.Status != dbModule.StatusFilled {
		t.Errorf("Order status should be filled, got %v", dbOrder.Status)
	}
}
