package orderbook

import (
	"testing"
)

func TestPlaceBuyOrder(t *testing.T) {
	book := New(1000)

	o := &Order{
		Side:     Buy,
		Type:     Limit,
		Price:    10000,
		Quantity: 10,
	}

	trades, err := book.PlaceOrder(o)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trades) != 0 {
		t.Fatalf("expected 0 trades, got %d", len(trades))
	}
	if o.Status != Open {
		t.Fatalf("expected OPEN, got %s", o.Status)
	}
	if o.ID == "" {
		t.Fatal("expected non-empty order ID")
	}
}

func TestPlaceMatchingOrders(t *testing.T) {
	book := New(1000)

	// Place a sell order first
	sell := &Order{Side: Sell, Type: Limit, Price: 10000, Quantity: 5}
	trades, _ := book.PlaceOrder(sell)
	if len(trades) != 0 {
		t.Fatalf("expected 0 trades, got %d", len(trades))
	}

	// Place a matching buy order
	buy := &Order{Side: Buy, Type: Limit, Price: 10000, Quantity: 5}
	trades, _ = book.PlaceOrder(buy)
	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}

	if buy.Status != Filled {
		t.Fatalf("expected buy FILLED, got %s", buy.Status)
	}
	if sell.Status != Filled {
		t.Fatalf("expected sell FILLED, got %s", sell.Status)
	}

	trade := trades[0]
	if trade.Price != 10000 || trade.Quantity != 5 {
		t.Fatalf("unexpected trade: price=%f qty=%f", trade.Price, trade.Quantity)
	}
}

func TestPartialFill(t *testing.T) {
	book := New(1000)

	// Sell 10 units
	sell := &Order{Side: Sell, Type: Limit, Price: 5000, Quantity: 10}
	book.PlaceOrder(sell)

	// Buy only 3 units
	buy := &Order{Side: Buy, Type: Limit, Price: 5000, Quantity: 3}
	trades, _ := book.PlaceOrder(buy)

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if buy.Status != Filled {
		t.Fatalf("expected buy FILLED, got %s", buy.Status)
	}
	if sell.Status != Partial {
		t.Fatalf("expected sell PARTIAL, got %s", sell.Status)
	}
	if sell.remaining() != 7 {
		t.Fatalf("expected sell remaining 7, got %f", sell.remaining())
	}
}

func TestPriceTimePriority(t *testing.T) {
	book := New(1000)

	// Place 3 sell orders at different prices
	sell1 := &Order{Side: Sell, Type: Limit, Price: 5000, Quantity: 5}
	sell2 := &Order{Side: Sell, Type: Limit, Price: 5100, Quantity: 5}
	sell3 := &Order{Side: Sell, Type: Limit, Price: 4900, Quantity: 5}

	book.PlaceOrder(sell1)
	book.PlaceOrder(sell2)
	book.PlaceOrder(sell3)

	// Buy at 5100 — should match sell3 (4900, lowest price) first
	buy := &Order{Side: Buy, Type: Limit, Price: 5100, Quantity: 5}
	trades, _ := book.PlaceOrder(buy)

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if trades[0].Price != 4900 {
		t.Fatalf("expected trade at 4900 (lowest sell), got %f", trades[0].Price)
	}
	if sell3.Status != Filled {
		t.Fatalf("expected sell3 FILLED, got %s", sell3.Status)
	}
}

func TestCancelOrder(t *testing.T) {
	book := New(1000)

	o := &Order{Side: Buy, Type: Limit, Price: 10000, Quantity: 5}
	book.PlaceOrder(o)

	if !book.CancelOrder(o.ID) {
		t.Fatal("expected cancel to succeed")
	}
	if book.CancelOrder(o.ID) {
		t.Fatal("expected second cancel to fail")
	}

	retrieved := book.GetOrder(o.ID)
	if retrieved.Status != Cancelled {
		t.Fatalf("expected CANCELLED, got %s", retrieved.Status)
	}
}

func TestGetOrderNotFound(t *testing.T) {
	book := New(1000)
	if o := book.GetOrder("NONEXISTENT"); o != nil {
		t.Fatal("expected nil for non-existent order")
	}
}

func TestBookSnapshot(t *testing.T) {
	book := New(1000)

	book.PlaceOrder(&Order{Side: Buy, Type: Limit, Price: 10000, Quantity: 5})
	book.PlaceOrder(&Order{Side: Buy, Type: Limit, Price: 9900, Quantity: 3})
	book.PlaceOrder(&Order{Side: Sell, Type: Limit, Price: 10100, Quantity: 4})

	bids, asks := book.BookSnapshot()

	if len(bids) != 2 {
		t.Fatalf("expected 2 bid levels, got %d", len(bids))
	}
	if bids[0].Price != 10000 {
		t.Fatalf("expected highest bid first (10000), got %f", bids[0].Price)
	}

	if len(asks) != 1 {
		t.Fatalf("expected 1 ask level, got %d", len(asks))
	}
}

func TestTradesRingBuffer(t *testing.T) {
	book := New(3) // Small buffer

	for i := 0; i < 5; i++ {
		sell := &Order{Side: Sell, Type: Limit, Price: 10000, Quantity: 1}
		buy := &Order{Side: Buy, Type: Limit, Price: 10000, Quantity: 1}
		book.PlaceOrder(sell)
		book.PlaceOrder(buy)
	}

	trades := book.GetTrades(10)
	if len(trades) != 3 {
		t.Fatalf("expected 3 trades (ring buffer max), got %d", len(trades))
	}
}

func TestMarketOrder(t *testing.T) {
	book := New(1000)

	// Pre-seed with sell orders
	book.PlaceOrder(&Order{Side: Sell, Type: Limit, Price: 10000, Quantity: 5})
	book.PlaceOrder(&Order{Side: Sell, Type: Limit, Price: 10100, Quantity: 5})

	// Market buy — should match both
	buy := &Order{Side: Buy, Type: Market, Price: 0, Quantity: 10}
	trades, _ := book.PlaceOrder(buy)

	if len(trades) != 2 {
		t.Fatalf("expected 2 trades, got %d", len(trades))
	}
	if buy.Status != Filled {
		t.Fatalf("expected FILLED, got %s", buy.Status)
	}
}

// ---- Benchmarks ----

func BenchmarkPlaceOrder(b *testing.B) {
	book := New(10000)

	// Pre-seed with opposing orders so matching happens
	for i := 0; i < 100; i++ {
		book.PlaceOrder(&Order{
			Side: Sell, Type: Limit,
			Price: float64(10000 + i), Quantity: float64(1000),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		book.PlaceOrder(&Order{
			Side: Buy, Type: Limit,
			Price: float64(10000 + (i % 100)), Quantity: 1,
		})
	}
}

func BenchmarkPlaceOrderParallel(b *testing.B) {
	book := New(10000)

	// Pre-seed
	for i := 0; i < 500; i++ {
		book.PlaceOrder(&Order{
			Side: Sell, Type: Limit,
			Price: float64(10000 + (i % 100)), Quantity: float64(1000),
		})
		book.PlaceOrder(&Order{
			Side: Buy, Type: Limit,
			Price: float64(9900 - (i % 100)), Quantity: float64(1000),
		})
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			side := Buy
			if i%2 == 0 {
				side = Sell
			}
			book.PlaceOrder(&Order{
				Side: side, Type: Limit,
				Price: float64(10000), Quantity: 1,
			})
			i++
		}
	})
}
